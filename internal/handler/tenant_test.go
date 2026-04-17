package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flowforge/internal/handler"
	"flowforge/internal/model"
	"flowforge/pkg/logger"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockTenantService struct {
	mock.Mock
}

func (m *mockTenantService) Create(ctx context.Context, req model.TenantRequest) (*model.TenantResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*model.TenantResponse), args.Error(1)
}
func (m *mockTenantService) Get(ctx context.Context, id string) (*model.TenantResponse, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*model.TenantResponse), args.Error(1)
}
func (m *mockTenantService) List(ctx context.Context, req model.ListTenantRequest) ([]*model.TenantResponse, int, string, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]*model.TenantResponse), args.Int(1), args.String(2), args.Error(3)
}
func (m *mockTenantService) Update(ctx context.Context, req model.TenantRequest) (*model.TenantResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*model.TenantResponse), args.Error(1)
}
func (m *mockTenantService) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func TestTenantHandler_Create(t *testing.T) {
	svc := new(mockTenantService)
	l := logger.NewNop()
	h := handler.NewTenantHandler(svc, l)

	tenantReq := model.TenantRequest{Name: "T-1"}
	body, _ := json.Marshal(tenantReq)

	svc.On("Create", mock.Anything, tenantReq).Return(&model.TenantResponse{ID: "t-1", Name: "T-1"}, nil)

	req := httptest.NewRequest("POST", "/tenants", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	
	var resp model.TenantResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "t-1", resp.ID)
	
	svc.AssertExpectations(t)
}
