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

type mockAIService struct {
	mock.Mock
}

func (m *mockAIService) GenerateDAGFromText(ctx context.Context, prompt string) ([]model.StepDefinition, error) {
	args := m.Called(ctx, prompt)
	return args.Get(0).([]model.StepDefinition), args.Error(1)
}
func (m *mockAIService) AnalyzeFailure(ctx context.Context, errLog string) (string, error) {
	args := m.Called(ctx, errLog)
	return args.String(0), args.Error(1)
}

func TestAIHandler_GenerateDAG(t *testing.T) {
	svc := new(mockAIService)
	l := logger.NewNop()
	h := handler.NewAIHandler(svc, l)

	svc.On("GenerateDAGFromText", mock.Anything, "send email then wait").Return([]model.StepDefinition{
		{ID: "step-1", Action: "HTTP"},
	}, nil)

	req := httptest.NewRequest("POST", "/ai/generate", bytes.NewBufferString(`{"prompt": "send email then wait"}`))
	rr := httptest.NewRecorder()

	h.GenerateDAG(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	
	var resp []model.StepDefinition
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Len(t, resp, 1)
	assert.Equal(t, "step-1", resp[0].ID)
	
	svc.AssertExpectations(t)
}
