package handler_test

import (
	"context"
	"flowforge/internal/handler"
	"flowforge/internal/model"
	"flowforge/pkg/jwt"
	"flowforge/pkg/logger"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockWFService struct {
	mock.Mock
}

func (m *mockWFService) Create(ctx context.Context, tenantID string, req *model.WorkflowRequest) (*model.WorkflowResponse, error) {
	args := m.Called(ctx, tenantID, req)
	return args.Get(0).(*model.WorkflowResponse), args.Error(1)
}
func (m *mockWFService) Get(ctx context.Context, tenantID, id string) (*model.WorkflowResponse, error) {
	args := m.Called(ctx, tenantID, id)
	return args.Get(0).(*model.WorkflowResponse), args.Error(1)
}
func (m *mockWFService) List(ctx context.Context, req model.ListWorkflowRequest) ([]*model.WorkflowResponse, int, string, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]*model.WorkflowResponse), args.Int(1), args.String(2), args.Error(3)
}
func (m *mockWFService) ListVersions(ctx context.Context, tenantID, name string) ([]*model.WorkflowVersion, error) {
	args := m.Called(ctx, tenantID, name)
	return args.Get(0).([]*model.WorkflowVersion), args.Error(1)
}
func (m *mockWFService) Update(ctx context.Context, req *model.WorkflowRequest) error {
	return m.Called(ctx, req).Error(0)
}
func (m *mockWFService) Rollback(ctx context.Context, req *model.WorkflowRollbackRequest) error {
	return m.Called(ctx, req).Error(0)
}
func (m *mockWFService) Delete(ctx context.Context, tenantID, name string) error {
	return m.Called(ctx, tenantID, name).Error(0)
}
func (m *mockWFService) Trigger(ctx context.Context, tenantID, workflowID string) (*model.Execution, error) {
	args := m.Called(ctx, tenantID, workflowID)
	return args.Get(0).(*model.Execution), args.Error(1)
}

func TestWorkflowHandler_Delete(t *testing.T) {
	svc := new(mockWFService)
	l := logger.NewNop()
	h := handler.NewWorkflowHandler(svc, l)

	r := chi.NewRouter()
	r.Delete("/workflows/{name}", h.Delete)

	svc.On("Delete", mock.Anything, "t-1", "wf-1").Return(nil)

	req := httptest.NewRequest("DELETE", "/workflows/wf-1", nil)
	ctx := jwt.SetContext(req.Context(), jwt.TenantKey, "t-1")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req.WithContext(ctx))

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}
