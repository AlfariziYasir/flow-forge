package services

import (
	"context"
	"encoding/json"
	"errors"
	"flowforge/internal/model"
	"flowforge/pkg/logger"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExecutionService_Get(t *testing.T) {
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	wRepo := new(MockWorkflowRepository)
	trx := new(MockTrx)
	log := logger.NewNop()

	svc := NewExecutionService(eRepo, sRepo, wRepo, log, trx, nil)

	ctx := context.Background()
	id := uuid.New().String()
	tenantID := uuid.New().String()

	exec := &model.Execution{
		ID:         id,
		TenantID:   tenantID,
		WorkflowID: uuid.New().String(),
		Status:     "PENDING",
		CreatedAt:  time.Now(),
	}

	eRepo.On("Get", ctx, map[string]any{"id": id, "tenant_id": tenantID}, mock.AnythingOfType("*model.Execution")).
		Return(exec, nil)

	res, err := svc.Get(ctx, tenantID, id)

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, id, res.ID)
	eRepo.AssertExpectations(t)
}

func TestExecutionService_List(t *testing.T) {
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	wRepo := new(MockWorkflowRepository)
	trx := new(MockTrx)
	log := logger.NewNop()

	svc := NewExecutionService(eRepo, sRepo, wRepo, log, trx, nil)

	ctx := context.Background()
	tenantID := uuid.New().String()
	req := model.ListExecutionRequest{
		PageSize: 10,
		TenantID: tenantID,
	}

	executions := []*model.Execution{
		{ID: uuid.New().String(), TenantID: tenantID},
		{ID: uuid.New().String(), TenantID: tenantID},
	}

	eRepo.On("List", ctx, uint64(10), uint64(0), mock.Anything).Return(executions, 2, nil)

	res, count, token, err := svc.List(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Len(t, res, 2)
	assert.Empty(t, token)
	eRepo.AssertExpectations(t)
}

func TestExecutionService_Retry_NotFound(t *testing.T) {
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	wRepo := new(MockWorkflowRepository)
	trx := new(MockTrx)
	log := logger.NewNop()

	svc := NewExecutionService(eRepo, sRepo, wRepo, log, trx, nil)

	ctx := context.Background()
	id := uuid.New().String()
	tenantID := uuid.New().String()

	eRepo.On("Get", ctx, map[string]any{"id": id, "tenant_id": tenantID}, mock.AnythingOfType("*model.Execution")).
		Return(nil, errors.New("not found"))

	err := svc.Retry(ctx, tenantID, id)

	assert.Error(t, err)
}

func TestExecutionService_Retry_Success(t *testing.T) {
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	wRepo := new(MockWorkflowRepository)
	trx := new(MockTrx)
	svc := NewExecutionService(eRepo, sRepo, wRepo, logger.NewNop(), trx, nil)

	ctx := context.Background()
	tenantID := "T1"
	execID := "E1"

	oldExec := &model.Execution{
		ID:         execID,
		TenantID:   tenantID,
		WorkflowID: "W1",
		Status:     model.StatusExecutionFailed,
		Version:    1,
	}

	wf := &model.Workflow{
		ID:            "W1",
		Version:       1,
		DAGDefinition: json.RawMessage(`[{"id": "s1"}]`),
	}

	eRepo.On("Get", ctx, map[string]any{"id": execID, "tenant_id": tenantID}, mock.Anything).Return(oldExec, nil)
	wRepo.On("Get", ctx, mock.Anything, mock.Anything).Return(wf, nil)
	trx.On("Begin", ctx).Return(ctx, nil)
	eRepo.On("Create", ctx, mock.Anything).Return(nil)
	sRepo.On("Create", ctx, mock.Anything).Return(nil)
	trx.On("Commit", ctx).Return(nil)
	trx.On("Rollback", ctx).Return(nil)

	err := svc.Retry(ctx, tenantID, execID)

	assert.NoError(t, err)
}

func TestExecutionService_Cancel_Success(t *testing.T) {
	eRepo := new(MockExecutionRepository)
	svc := NewExecutionService(eRepo, nil, nil, logger.NewNop(), nil, nil)

	ctx := context.Background()
	tenantID := "T1"
	execID := "E1"

	exec := &model.Execution{
		ID:      execID,
		Status:  model.StatusExecutionRunning,
		Version: 1,
	}

	eRepo.On("Get", ctx, map[string]any{"id": execID, "tenant_id": tenantID}, mock.Anything).Return(exec, nil)
	eRepo.On("Update", ctx, execID, 1, mock.Anything).Return(nil)

	err := svc.Cancel(ctx, tenantID, execID)

	assert.NoError(t, err)
}
