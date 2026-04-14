package services

import (
	"context"
	"encoding/base64"
	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/errorx"
	"flowforge/pkg/logger"
	"strconv"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TenantService interface {
	Create(ctx context.Context, req model.TenantRequest) (*model.TenantResponse, error)
	Get(ctx context.Context, id string) (*model.TenantResponse, error)
	List(ctx context.Context, req model.ListTenantRequest) ([]*model.TenantResponse, int, string, error)
	Update(ctx context.Context, req model.TenantRequest) (*model.TenantResponse, error)
	Delete(ctx context.Context, id string) error
}

type tenantService struct {
	tRepo repository.TenantRepository
	log   *logger.Logger
}

func NewTenantService(tRepo repository.TenantRepository, log *logger.Logger) TenantService {
	return &tenantService{
		log:   log,
		tRepo: tRepo,
	}
}

func (s *tenantService) Create(ctx context.Context, req model.TenantRequest) (*model.TenantResponse, error) {
	var tenant model.Tenant

	err := s.tRepo.Get(ctx, map[string]any{"name": req.Name}, true, &tenant)
	if err == nil && tenant.ID != "" {
		return nil, errorx.NewError(errorx.ErrTypeConflict, "name already registered", nil)
	}

	tenant.ID = uuid.New().String()
	tenant.Name = req.Name
	tenant.IsActive = req.IsActive
	err = s.tRepo.Create(ctx, &tenant)
	if err != nil {
		s.log.Error("failed to create tenant", zap.Error(err))
		return nil, err
	}

	return &model.TenantResponse{
		ID:        tenant.ID,
		Name:      tenant.Name,
		IsActive:  tenant.IsActive,
		CreatedAt: tenant.CreatedAt,
	}, nil
}

func (s *tenantService) Get(ctx context.Context, id string) (*model.TenantResponse, error) {
	var tenant model.Tenant
	err := s.tRepo.Get(ctx, map[string]any{"id": id}, true, &tenant)
	if err != nil {
		return nil, err
	}
	return &model.TenantResponse{
		ID:        tenant.ID,
		Name:      tenant.Name,
		IsActive:  tenant.IsActive,
		CreatedAt: tenant.CreatedAt,
		UpdatedAt: tenant.UpdatedAt,
	}, nil
}

func (s *tenantService) List(ctx context.Context, req model.ListTenantRequest) ([]*model.TenantResponse, int, string, error) {
	var offset uint64 = 0
	if req.PageToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.PageToken)
		if err != nil {
			return nil, 0, "", errorx.NewError(errorx.ErrTypeValidation, "invalid page token", err)
		}
		offset, err = strconv.ParseUint(string(decoded), 10, 64)
		if err != nil {
			return nil, 0, "", errorx.NewError(errorx.ErrTypeValidation, "invalid page token", err)
		}
	}

	filters := make(map[string]any, 0)
	if req.TenantName != "" {
		filters["name"] = req.TenantName
	}

	tenants, count, err := s.tRepo.List(ctx, uint64(req.PageSize), offset, filters)
	if err != nil {
		s.log.Error("failed to get list tenant", zap.Error(err))
		return nil, 0, "", err
	}

	var tenantResponses []*model.TenantResponse
	for _, tenant := range tenants {
		tenantResponses = append(tenantResponses, &model.TenantResponse{
			ID:        tenant.ID,
			Name:      tenant.Name,
			IsActive:  tenant.IsActive,
			CreatedAt: tenant.CreatedAt,
			UpdatedAt: tenant.UpdatedAt,
		})
	}
	nextPageToken := ""
	if count == int(req.PageSize) {
		nextOffset := offset + uint64(req.PageSize)
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(nextOffset, 10)))
	}

	return tenantResponses, count, nextPageToken, nil
}

func (s *tenantService) Update(ctx context.Context, req model.TenantRequest) (*model.TenantResponse, error) {
	var tenant model.Tenant
	err := s.tRepo.Get(ctx, map[string]any{"id": req.ID}, true, &tenant)
	if err != nil {
		s.log.Error("failed to get tenant by id", zap.Error(err))
		return nil, err
	}

	err = s.tRepo.Update(ctx, req.ID, map[string]any{"name": req.Name})
	if err != nil {
		s.log.Error("failed to update tenant by id", zap.Error(err))
		return nil, err
	}
	return &model.TenantResponse{
		ID:        tenant.ID,
		Name:      tenant.Name,
		IsActive:  tenant.IsActive,
		CreatedAt: tenant.CreatedAt,
	}, nil
}

func (s *tenantService) Delete(ctx context.Context, id string) error {
	return s.tRepo.Delete(ctx, id)
}
