package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flowforge/config"
	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/errorx"
	"flowforge/pkg/logger"
	"flowforge/pkg/redis"
	"fmt"
	"slices"
	"strconv"
	"time"

	"flowforge/pkg/jwt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	Login(ctx context.Context, req model.UserRequest) (string, string, error)
	Logout(ctx context.Context, accUuid, refUuid string) error
	Refresh(ctx context.Context, refUuid, userId, role, tenantID string) (string, error)
	Create(ctx context.Context, req model.UserRequest) (*model.UserResponse, error)
	Get(ctx context.Context, id string) (*model.UserResponse, error)
	List(ctx context.Context, req model.ListRequest) ([]*model.UserResponse, int, string, error)
	Update(ctx context.Context, req model.UserRequest) (*model.UserResponse, error)
	Delete(ctx context.Context, id string) error
}

type userService struct {
	uRepo repository.UserRepository
	tRepo repository.TenantRepository
	log   *logger.Logger
	cfg   *config.Config
	cache redis.Cache
}

func NewUserService(
	uRepo repository.UserRepository,
	tRepo repository.TenantRepository,
	log *logger.Logger,
	cfg *config.Config,
	cache redis.Cache,
) UserService {
	return &userService{
		uRepo: uRepo,
		tRepo: tRepo,
		log:   log,
		cfg:   cfg,
		cache: cache,
	}
}

func (s *userService) Login(ctx context.Context, req model.UserRequest) (string, string, error) {
	var user model.User

	err := s.uRepo.Get(ctx, map[string]any{"email": req.Email}, true, &user)
	if err != nil {
		s.log.Error("failed get user", zap.Error(err))
		return "", "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		s.log.Error("password invalid", zap.Error(err))
		return "", "", errorx.NewValidationError(map[string]string{"password": "invalid password"})
	}

	baseToken := jwt.BaseRequest{
		RefUuid:     uuid.New().String(),
		AccUuid:     uuid.New().String(),
		RefKey:      s.cfg.RefreshTokenKey,
		AccKey:      s.cfg.AccessTokenKey,
		RefDuration: s.cfg.RefreshTokenExp,
		AccDuration: s.cfg.AccessTokenExp,
	}
	acc, ref, err := jwt.TokenPair(
		baseToken,
		jwt.WithOption("user_id", user.ID),
		jwt.WithOption("role", user.Role),
		jwt.WithOption("tenant_id", user.TenantID),
		jwt.WithOption("ref_uuid", baseToken.RefUuid),
	)
	if err != nil {
		s.log.Error("failed generate token", zap.Error(err))
		return "", "", errorx.NewError(errorx.ErrTypeInternal, "failed create token", err)
	}

	val := map[string]any{
		"acc_uuid":  baseToken.AccUuid,
		"user_id":   user.ID,
		"tenant_id": user.TenantID,
	}
	err = s.cache.Set(
		ctx,
		fmt.Sprintf("%s:%s", jwt.RefKey, baseToken.RefUuid),
		val,
		baseToken.RefDuration,
	)
	if err != nil {
		s.log.Error("failed to store refresh auth", zap.Error(err))
		return "", "", errorx.NewError(errorx.ErrTypeInternal, "failed to store refresh auth", err)
	}

	return acc, ref, nil
}

func (s *userService) Logout(ctx context.Context, accUuid, refUuid string) error {
	err := s.cache.Delete(
		ctx,
		fmt.Sprintf("%s:%s", jwt.RefKey, refUuid),
	)
	if err != nil {
		s.log.Error("failed to delete auth", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to delete auth", err)
	}

	err = s.cache.Set(
		ctx,
		fmt.Sprintf("%s:%s", jwt.BlacklistKey, accUuid),
		"revoked",
		s.cfg.AccessTokenExp,
	)
	if err != nil {
		s.log.Error("failed to store refresh auth", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to store refresh auth", err)
	}

	return nil
}

func (s *userService) Refresh(ctx context.Context, refUuid, userId, role, tenantID string) (string, error) {
	var cache map[string]any
	value, err := s.cache.Get(ctx, fmt.Sprintf("%s:%s", jwt.RefKey, refUuid))
	if err != nil {
		s.log.Error("failed get refresh token", zap.Error(err))
		return "", errorx.NewError(errorx.ErrTypeUnauthorized, "token invalid", err)
	}

	if value == "" {
		s.log.Error("refresh token value is empty")
		return "", errorx.NewError(errorx.ErrTypeUnauthorized, "token invalid", errors.New("refresh token value is empty"))
	}
	json.Unmarshal([]byte(string(value)), &cache)

	err = s.cache.Set(
		ctx,
		fmt.Sprintf("%s:%s", jwt.BlacklistKey, cache["acc_uuid"].(string)),
		"revoked",
		s.cfg.AccessTokenExp,
	)
	if err != nil {
		s.log.Error("failed to store refresh auth", zap.Error(err))
		return "", errorx.NewError(errorx.ErrTypeInternal, "failed to store refresh auth", err)
	}

	baseToken := jwt.BaseRequest{
		AccUuid:     uuid.New().String(),
		AccKey:      s.cfg.AccessTokenKey,
		AccDuration: s.cfg.AccessTokenExp,
	}
	acc, err := jwt.AccessToken(
		baseToken,
		jwt.WithOption("user_id", userId),
		jwt.WithOption("role", role),
		jwt.WithOption("ref_uuid", refUuid),
		jwt.WithOption("tenant_id", tenantID),
	)
	if err != nil {
		s.log.Error("failed generate token", zap.Error(err))
		return "", errorx.NewError(errorx.ErrTypeInternal, "failed create token", err)
	}

	val := map[string]any{
		"acc_uuid":  baseToken.AccUuid,
		"user_id":   userId,
		"tenant_id": tenantID,
	}
	err = s.cache.Set(
		ctx,
		fmt.Sprintf("%s:%s", jwt.RefKey, refUuid),
		val,
		baseToken.RefDuration,
	)
	if err != nil {
		s.log.Error("failed to store refresh auth", zap.Error(err))
		return "", errorx.NewError(errorx.ErrTypeInternal, "failed to store refresh auth", err)
	}

	return acc, nil
}

func (s *userService) Create(ctx context.Context, req model.UserRequest) (*model.UserResponse, error) {
	executorRole, okRole := ctx.Value(jwt.RoleKey{}).(string)
	executorTenant, okTenant := ctx.Value(jwt.TenantKey{}).(string)

	if !okRole || !okTenant {
		s.log.Error("missing authorization context")
		return nil, errorx.NewError(errorx.ErrTypeUnauthorized, "unauthorized request", nil)
	}

	if executorTenant != req.TenantID {
		return nil, errorx.NewError(errorx.ErrTypeValidation, "forbidden to cross-create users in another tenant", nil)
	}

	switch executorRole {
	case string(model.RoleAdmin):
		if req.Role != "editor" && req.Role != "viewer" && req.Role != "admin" {
			return nil, errorx.NewValidationError(map[string]string{
				"role": "admin can only create admin, editor or viewer role",
			})
		}
	case string(model.RoleEditor):
		if req.Role != "viewer" {
			return nil, errorx.NewValidationError(map[string]string{
				"role": "editor can only create user with viewer role",
			})
		}
	case string(model.RoleViewer):
		return nil, errorx.NewError(errorx.ErrTypeUnauthorized, "viewer cannot create users", nil)
	default:
		return nil, errorx.NewError(errorx.ErrTypeUnauthorized, "invalid executor role", nil)
	}

	hashPass, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		s.log.Error("failed to hash password", zap.Error(err))
		return nil, errorx.NewError(errorx.ErrTypeInternal, "failed processing request", err)
	}

	var user model.User
	err = s.uRepo.Get(ctx, map[string]any{"email": req.Email}, true, &user)
	if err == nil && user.ID != "" {
		return nil, errorx.NewError(errorx.ErrTypeConflict, "email already registered", nil)
	}

	var tenant model.Tenant
	err = s.tRepo.Get(ctx, map[string]any{"id": req.TenantID}, true, &tenant)
	if err != nil {
		return nil, err
	}

	user.ID = uuid.NewString()
	user.Email = req.Email
	user.Role = req.Role
	user.PasswordHash = string(hashPass)
	user.CreatedAt = time.Now()
	user.UpdatedAt = nil
	user.Version = 1
	err = s.uRepo.Create(ctx, &user)
	if err != nil {
		s.log.Error("failed to create user", zap.Error(err))
		return nil, err
	}

	return &model.UserResponse{
		UserID:     user.ID,
		TenantID:   user.TenantID,
		TenantName: tenant.Name,
		Email:      user.Email,
		Role:       user.Role,
		IsActive:   user.IsActive,
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	}, nil
}

func (s *userService) Get(ctx context.Context, id string) (*model.UserResponse, error) {
	var user model.User

	err := s.uRepo.Get(ctx, map[string]any{"id": id}, true, &user)
	if err != nil {
		s.log.Error("failed to get user by id", zap.Error(err))
		return nil, err
	}

	var tenant model.Tenant
	err = s.tRepo.Get(ctx, map[string]any{"id": user.TenantID}, true, &tenant)
	if err != nil {
		return nil, err
	}

	return &model.UserResponse{
		UserID:     user.ID,
		TenantID:   user.TenantID,
		TenantName: tenant.Name,
		Email:      user.Email,
		Role:       user.Role,
		IsActive:   user.IsActive,
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	}, nil
}

func (s *userService) List(ctx context.Context, req model.ListRequest) ([]*model.UserResponse, int, string, error) {
	var offset uint64 = 0
	if req.PageToken != "" {
		decoded, _ := base64.StdEncoding.DecodeString(req.PageToken)
		offset, _ = strconv.ParseUint(string(decoded), 10, 64)
	}

	filters := make(map[string]any)
	if req.Role != string(model.RoleAdmin) {
		filters["role"] = req.Role
	}

	if req.Email != "" {
		filters["email"] = req.Email
	}

	users, count, err := s.uRepo.List(ctx, uint64(req.PageSize), offset, filters)
	if err != nil {
		s.log.Error("failed to get list user", zap.Error(err))
		return nil, 0, "", err
	}

	nextPageToken := ""
	if count == int(req.PageSize) {
		nextOffset := offset + uint64(req.PageSize)
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(nextOffset, 10)))
	}

	res := slices.Grow([]*model.UserResponse{}, len(users))
	for _, user := range users {
		res = append(res, &model.UserResponse{
			UserID:    user.ID,
			TenantID:  user.TenantID,
			Email:     user.Email,
			Role:      user.Role,
			IsActive:  user.IsActive,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		})
	}

	return res, count, nextPageToken, nil
}

func (s *userService) Update(ctx context.Context, req model.UserRequest) (*model.UserResponse, error) {
	var user model.User

	err := s.uRepo.Get(ctx, map[string]any{"id": req.UserID}, true, &user)
	if err != nil {
		s.log.Error("failed to get user by id", zap.Error(err))
		return nil, err
	}

	if user.Email != req.Email {
		err := s.uRepo.Get(ctx, map[string]any{"email": req.Email}, true, &user)
		if err == nil {
			s.log.Warn("email update conflict", zap.String("email", req.Email))
			return nil, errorx.NewError(errorx.ErrTypeConflict, "email already taken", nil)
		}
	}

	dataUpdate := map[string]any{
		"email": req.Email,
		"role":  req.Role,
	}
	err = s.uRepo.Update(ctx, user.ID, user.Version, dataUpdate)
	if err != nil {
		s.log.Error("failed to update user by id", zap.Error(err))
		return nil, err
	}

	return &model.UserResponse{
		UserID:    user.ID,
		TenantID:  user.TenantID,
		Email:     user.Email,
		Role:      user.Role,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

func (s *userService) Delete(ctx context.Context, id string) error {
	var user model.User

	err := s.uRepo.Get(ctx, map[string]any{"id": id}, true, &user)
	if err != nil {
		s.log.Error("failed to get user by id", zap.Error(err))
		return err
	}

	err = s.uRepo.Delete(ctx, user.ID, user.Version)
	if err != nil {
		s.log.Error("failed to delete user by id", zap.Error(err))
		return err
	}

	return nil
}
