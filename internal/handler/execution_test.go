package handler_test

import (
	"context"
	"encoding/json"
	"flowforge/internal/handler"
	"flowforge/internal/model"
	"flowforge/pkg/logger"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockExecService struct {
	mock.Mock
}

func (m *mockExecService) Get(ctx context.Context, tenantID, id string) (*model.ExecutionResponse, error) {
	args := m.Called(ctx, tenantID, id)
	return args.Get(0).(*model.ExecutionResponse), args.Error(1)
}
func (m *mockExecService) List(ctx context.Context, req model.ListExecutionRequest) ([]*model.ExecutionResponse, int, string, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]*model.ExecutionResponse), args.Int(1), args.String(2), args.Error(3)
}
func (m *mockExecService) Retry(ctx context.Context, tenantID, id string) error {
	return m.Called(ctx, tenantID, id).Error(0)
}
func (m *mockExecService) Cancel(ctx context.Context, tenantID, id string) error {
	return m.Called(ctx, tenantID, id).Error(0)
}

func TestExecutionHandler_Get(t *testing.T) {
	svc := new(mockExecService)
	l := logger.NewNop()
	h := handler.NewExecutionHandler(svc, l)

	r := chi.NewRouter()
	r.Get("/executions/{id}", h.Get)

	svc.On("Get", mock.Anything, "t-1", "exec-1").Return(&model.ExecutionResponse{
		ID:     "exec-1",
		Status: "SUCCESS",
	}, nil)

	req := httptest.NewRequest("GET", "/executions/exec-1", nil)
	ctx := context.WithValue(req.Context(), "tenant_id", "t-1")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req.WithContext(ctx))

	assert.Equal(t, http.StatusOK, rr.Code)
	
	var resp model.ExecutionResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "exec-1", resp.ID)
	
	svc.AssertExpectations(t)
}
