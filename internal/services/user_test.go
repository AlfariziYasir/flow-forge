package services

import (
	"context"
	"errors"
	"flowforge/config"
	"flowforge/internal/model"
	"flowforge/pkg/jwt"
	"flowforge/pkg/logger"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func TestUserService_Login_Success(t *testing.T) {
	uRepo := new(MockUserRepository)
	tRepo := new(MockTenantRepository)
	cache := new(MockCache)
	log := logger.NewNop()
	cfg := &config.Config{
		AccessTokenKey:  "access-key-should-be-long-enough-32-chars-at-least",
		RefreshTokenKey: "refresh-key-should-be-long-enough-32-chars-at-least",
		AccessTokenExp:  time.Hour,
		RefreshTokenExp: time.Hour * 24,
	}

	svc := NewUserService(uRepo, tRepo, log, cfg, cache)

	ctx := context.Background()
	password := "password123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), 10)
	user := &model.User{
		ID:           uuid.New().String(),
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Role:         "admin",
		TenantID:     uuid.New().String(),
	}

	uRepo.On("Get", ctx, map[string]any{"email": user.Email}, true, mock.AnythingOfType("*model.User")).
		Return(user, nil)
	
	cache.On("Set", ctx, mock.Anything, mock.Anything, cfg.RefreshTokenExp).Return(nil)

	acc, ref, err := svc.Login(ctx, model.UserRequest{Email: user.Email, Password: password})

	assert.NoError(t, err)
	assert.NotEmpty(t, acc)
	assert.NotEmpty(t, ref)
	uRepo.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestUserService_Create_ForbiddenTenant(t *testing.T) {
	uRepo := new(MockUserRepository)
	tRepo := new(MockTenantRepository)
	cache := new(MockCache)
	log := logger.NewNop()
	cfg := &config.Config{}

	svc := NewUserService(uRepo, tRepo, log, cfg, cache)

	ctx := context.WithValue(context.Background(), jwt.RoleKey{}, "admin")
	ctx = context.WithValue(ctx, jwt.TenantKey{}, "tenant-1")

	req := model.UserRequest{
		Email:    "new@example.com",
		TenantID: "tenant-2",
		Role:     "editor",
	}

	res, err := svc.Create(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "forbidden to cross-create users")
}

func TestUserService_List(t *testing.T) {
	uRepo := new(MockUserRepository)
	tRepo := new(MockTenantRepository)
	cache := new(MockCache)
	log := logger.NewNop()
	cfg := &config.Config{}

	svc := NewUserService(uRepo, tRepo, log, cfg, cache)

	ctx := context.Background()
	req := model.ListRequest{
		PageSize: 10,
		TenantID: "tenant-1",
		Role:     "editor",
	}

	users := []*model.User{
		{ID: uuid.New().String(), Email: "u1@e.com", TenantID: "tenant-1"},
	}

	uRepo.On("List", ctx, uint64(10), uint64(0), map[string]any{"tenant_id": "tenant-1", "role": "editor"}).
		Return(users, 1, nil)

	res, count, _, err := svc.List(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Len(t, res, 1)
	uRepo.AssertExpectations(t)
}

func TestUserService_Logout(t *testing.T) {
	uRepo := new(MockUserRepository)
	cache := new(MockCache)
	svc := NewUserService(uRepo, nil, logger.NewNop(), &config.Config{}, cache)

	ctx := context.Background()
	accUuid := "acc"
	refUuid := "ref"

	cache.On("Delete", ctx, mock.Anything).Return(nil)
	cache.On("Set", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := svc.Logout(ctx, accUuid, refUuid)

	assert.NoError(t, err)
}

func TestUserService_Refresh_Success(t *testing.T) {
	uRepo := new(MockUserRepository)
	cache := new(MockCache)
	svc := NewUserService(uRepo, nil, logger.NewNop(), &config.Config{
		AccessTokenKey:  "access-key-should-be-long-enough-32-chars-at-least",
		AccessTokenExp:  time.Hour,
		RefreshTokenExp: time.Hour * 24,
	}, cache)

	ctx := context.Background()
	refUuid := "ref"
	val := `{"acc_uuid": "old-acc", "user_id": "u1", "tenant_id": "t1"}`

	cache.On("Get", ctx, mock.Anything).Return(val, nil)
	cache.On("Set", ctx, mock.Anything, "revoked", mock.Anything).Return(nil)
	cache.On("Set", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	acc, err := svc.Refresh(ctx, refUuid, "u1", "admin", "t1")

	assert.NoError(t, err)
	assert.NotEmpty(t, acc)
}

func TestUserService_Get(t *testing.T) {
	uRepo := new(MockUserRepository)
	tRepo := new(MockTenantRepository)
	svc := NewUserService(uRepo, tRepo, logger.NewNop(), nil, nil)

	ctx := context.Background()
	u := &model.User{ID: "u1", TenantID: "t1"}
	tenant := &model.Tenant{ID: "t1", Name: "Tenant1"}

	uRepo.On("Get", ctx, mock.Anything, true, mock.Anything).Return(u, nil)
	tRepo.On("Get", ctx, mock.Anything, true, mock.Anything).Return(tenant, nil)

	res, err := svc.Get(ctx, "u1")

	assert.NoError(t, err)
	assert.Equal(t, "u1", res.UserID)
}

func TestUserService_Update(t *testing.T) {
	uRepo := new(MockUserRepository)
	svc := NewUserService(uRepo, nil, logger.NewNop(), nil, nil)

	ctx := context.Background()
	u := &model.User{ID: "u1", Email: "old@e.com", Version: 1}
	
	uRepo.On("Get", ctx, map[string]any{"id": "u1"}, true, mock.Anything).Return(u, nil)
	uRepo.On("Get", ctx, map[string]any{"email": "new@e.com"}, true, mock.Anything).Return(nil, errors.New("not found"))
	uRepo.On("Update", ctx, "u1", 1, mock.Anything).Return(nil)

	res, err := svc.Update(ctx, model.UserRequest{UserID: "u1", Email: "new@e.com"})

	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func TestUserService_Delete(t *testing.T) {
	uRepo := new(MockUserRepository)
	svc := NewUserService(uRepo, nil, logger.NewNop(), nil, nil)

	ctx := context.Background()
	u := &model.User{ID: "u1", Version: 1}

	uRepo.On("Get", ctx, map[string]any{"id": "u1"}, true, mock.Anything).Return(u, nil)
	uRepo.On("Delete", ctx, "u1", 1).Return(nil)

	err := svc.Delete(ctx, "u1")

	assert.NoError(t, err)
}
