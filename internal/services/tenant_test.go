package services

import (
	"context"
	"flowforge/internal/model"
	"flowforge/pkg/logger"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTenantService_Create(t *testing.T) {
	tRepo := new(MockTenantRepository)
	svc := NewTenantService(tRepo, logger.NewNop())

	ctx := context.Background()
	req := model.TenantRequest{Name: "New Tenant", IsActive: true}

	tRepo.On("Get", ctx, map[string]any{"name": req.Name}, true, mock.AnythingOfType("*model.Tenant")).
		Return(nil, nil)
	tRepo.On("Create", ctx, mock.AnythingOfType("*model.Tenant")).Return(nil)

	res, err := svc.Create(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, req.Name, res.Name)
	tRepo.AssertExpectations(t)
}

func TestTenantService_Get(t *testing.T) {
	tRepo := new(MockTenantRepository)
	svc := NewTenantService(tRepo, logger.NewNop())

	ctx := context.Background()
	id := uuid.New().String()
	tenant := &model.Tenant{ID: id, Name: "Tenant1"}

	tRepo.On("Get", ctx, map[string]any{"id": id}, true, mock.AnythingOfType("*model.Tenant")).
		Return(tenant, nil)

	res, err := svc.Get(ctx, id)

	assert.NoError(t, err)
	assert.Equal(t, id, res.ID)
}

func TestTenantService_List(t *testing.T) {
	tRepo := new(MockTenantRepository)
	svc := NewTenantService(tRepo, logger.NewNop())

	ctx := context.Background()
	req := model.ListTenantRequest{PageSize: 10}
	tenants := []*model.Tenant{{ID: "1", Name: "T1"}}

	tRepo.On("List", ctx, uint64(10), uint64(0), mock.Anything).Return(tenants, 1, nil)

	res, count, _, err := svc.List(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Len(t, res, 1)
}

func TestTenantService_Update(t *testing.T) {
	tRepo := new(MockTenantRepository)
	svc := NewTenantService(tRepo, logger.NewNop())

	ctx := context.Background()
	id := "t1"
	tenant := &model.Tenant{ID: id, Name: "Old Name"}

	tRepo.On("Get", ctx, map[string]any{"id": id}, true, mock.Anything).Return(tenant, nil)
	tRepo.On("Update", ctx, id, map[string]any{"name": "New Name"}).Return(nil)

	res, err := svc.Update(ctx, model.TenantRequest{ID: id, Name: "New Name"})

	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func TestTenantService_Delete(t *testing.T) {
	tRepo := new(MockTenantRepository)
	svc := NewTenantService(tRepo, logger.NewNop())

	ctx := context.Background()
	id := "t1"
	tRepo.On("Delete", ctx, id).Return(nil)

	err := svc.Delete(ctx, id)

	assert.NoError(t, err)
}
