package services

import (
	"context"
	"encoding/json"
	"errors"
	"flowforge/internal/model"
	"flowforge/pkg/logger"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestWorkflowService_Create(t *testing.T) {
	wRepo := new(MockWorkflowRepository)
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	trx := new(MockTrx)
	log := logger.NewNop()

	svc := NewWorkflowService(wRepo, eRepo, sRepo, trx, nil, log)

	ctx := context.Background()
	tenantID := uuid.New().String()
	req := &model.WorkflowRequest{
		Name:          "Test Workflow",
		Description:   "Description",
		DAGDefinition: json.RawMessage(`[{"id": "step_1", "action": "HTTP_CALL", "parameters": {}, "depends_on": [], "max_retries": 0}]`),
	}

	wRepo.On("Create", ctx, mock.AnythingOfType("*model.Workflow")).Return(nil)

	res, err := svc.Create(ctx, tenantID, req)

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, req.Name, res.Name)
	wRepo.AssertExpectations(t)
}

func TestWorkflowService_Create_InvalidDAG(t *testing.T) {
	wRepo := new(MockWorkflowRepository)
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	trx := new(MockTrx)
	log := logger.NewNop()

	svc := NewWorkflowService(wRepo, eRepo, sRepo, trx, nil, log)

	ctx := context.Background()
	tenantID := uuid.New().String()
	req := &model.WorkflowRequest{
		Name:          "Test Workflow",
		DAGDefinition: json.RawMessage(`invalid json`),
	}

	res, err := svc.Create(ctx, tenantID, req)

	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestWorkflowService_Get(t *testing.T) {
	wRepo := new(MockWorkflowRepository)
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	trx := new(MockTrx)
	log := logger.NewNop()

	svc := NewWorkflowService(wRepo, eRepo, sRepo, trx, nil, log)

	ctx := context.Background()
	id := uuid.New().String()
	tenantID := uuid.New().String()

	expectedWf := &model.Workflow{
		ID:            id,
		TenantID:      tenantID,
		Name:          "Test Workflow",
		DAGDefinition: json.RawMessage(`[]`),
	}

	wRepo.On("Get", ctx, map[string]any{"id": id, "tenant_id": tenantID}, mock.AnythingOfType("*model.Workflow")).
		Return(expectedWf, nil)

	res, err := svc.Get(ctx, tenantID, id)

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, id, res.WorkflowID)
	wRepo.AssertExpectations(t)
}

func TestWorkflowService_Get_NotFound(t *testing.T) {
	wRepo := new(MockWorkflowRepository)
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	trx := new(MockTrx)
	log := logger.NewNop()

	svc := NewWorkflowService(wRepo, eRepo, sRepo, trx, nil, log)

	ctx := context.Background()
	id := uuid.New().String()
	tenantID := uuid.New().String()

	wRepo.On("Get", ctx, map[string]any{"id": id, "tenant_id": tenantID}, mock.AnythingOfType("*model.Workflow")).
		Return(nil, errors.New("not found"))

	res, err := svc.Get(ctx, tenantID, id)

	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestWorkflowService_List(t *testing.T) {
	wRepo := new(MockWorkflowRepository)
	svc := NewWorkflowService(wRepo, nil, nil, nil, nil, logger.NewNop())

	ctx := context.Background()
	tenantID := uuid.New().String()
	req := model.ListWorkflowRequest{
		PageSize: 10,
		TenantID: tenantID,
	}

	workflows := []*model.Workflow{
		{ID: uuid.New().String(), Name: "W1", TenantID: tenantID, DAGDefinition: json.RawMessage(`[]`)},
	}

	wRepo.On("List", ctx, uint64(10), uint64(0), map[string]any{"tenant_id": tenantID}).
		Return(workflows, 1, nil)

	res, count, token, err := svc.List(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Len(t, res, 1)
	assert.Empty(t, token)
}

func TestWorkflowService_ListVersions(t *testing.T) {
	wRepo := new(MockWorkflowRepository)
	svc := NewWorkflowService(wRepo, nil, nil, nil, nil, logger.NewNop())

	ctx := context.Background()
	tenantID := uuid.New().String()
	name := "Workflow1"

	versions := []*model.Workflow{
		{ID: uuid.New().String(), Name: name, Version: 1},
		{ID: uuid.New().String(), Name: name, Version: 2},
	}

	wRepo.On("ListVersions", ctx, tenantID, name).Return(versions, nil)

	res, err := svc.ListVersions(ctx, tenantID, name)

	assert.NoError(t, err)
	assert.Len(t, res, 2)
}

func TestWorkflowService_Update(t *testing.T) {
	wRepo := new(MockWorkflowRepository)
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	trx := new(MockTrx)
	svc := NewWorkflowService(wRepo, eRepo, sRepo, trx, nil, logger.NewNop())

	ctx := context.Background()
	req := &model.WorkflowRequest{
		Name:           "W1",
		TenantID:       "T1",
		CurrentVersion: 1,
		DAGDefinition:  json.RawMessage(`[{"id": "s1", "action": "HTTP", "depends_on": []}]`),
	}

	wf := &model.Workflow{
		ID:            "locked-id",
		TenantID:      "T1",
		Name:          "W1",
		DAGDefinition: req.DAGDefinition,
	}

	// Mock for Update logic
	trx.On("Begin", ctx).Return(ctx, nil)
	wRepo.On("GetForUpdate", ctx, "T1", "W1", 1, true).Return("locked-id", nil)
	wRepo.On("Create", ctx, mock.AnythingOfType("*model.Workflow")).Return(nil)
	wRepo.On("Update", ctx, "locked-id", mock.Anything).Return(nil)
	trx.On("Commit", ctx).Return(nil)
	trx.On("Rollback", ctx).Return(nil)

	// Mock for Trigger logic called inside Update
	wRepo.On("Get", ctx, map[string]any{"name": "W1", "tenant_id": "T1", "is_current": true}, mock.Anything).Return(wf, nil)
	eRepo.On("Create", ctx, mock.Anything).Return(nil)
	sRepo.On("Create", ctx, mock.Anything).Return(nil)

	err := svc.Update(ctx, req)

	assert.NoError(t, err)
}

func TestWorkflowService_Rollback(t *testing.T) {
	wRepo := new(MockWorkflowRepository)
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	trx := new(MockTrx)
	svc := NewWorkflowService(wRepo, eRepo, sRepo, trx, nil, logger.NewNop())

	ctx := context.Background()
	req := &model.WorkflowRollbackRequest{
		TenantID:       "T1",
		Name:           "W1",
		CurrentVersion: 2,
		TargetVersion:  1,
	}

	wf := &model.Workflow{
		ID:            "target-id",
		TenantID:      "T1",
		Name:          "W1",
		DAGDefinition: json.RawMessage(`[{"id": "s1"}]`),
	}

	// Mock for Rollback logic
	trx.On("Begin", ctx).Return(ctx, nil)
	wRepo.On("GetForUpdate", ctx, "T1", "W1", 2, true).Return("curr-id", nil)
	wRepo.On("GetForUpdate", ctx, "T1", "W1", 1, true).Return("target-id", nil)
	wRepo.On("Update", ctx, "curr-id", mock.Anything).Return(nil)
	wRepo.On("Update", ctx, "target-id", mock.Anything).Return(nil)
	trx.On("Commit", ctx).Return(nil)
	trx.On("Rollback", ctx).Return(nil)

	// Mock for Trigger logic called inside Rollback
	wRepo.On("Get", ctx, map[string]any{"name": "W1", "tenant_id": "T1", "is_current": true}, mock.Anything).Return(wf, nil)
	eRepo.On("Create", ctx, mock.Anything).Return(nil)
	sRepo.On("Create", ctx, mock.Anything).Return(nil)

	err := svc.Rollback(ctx, req)

	assert.NoError(t, err)
}

func TestWorkflowService_Delete(t *testing.T) {
	wRepo := new(MockWorkflowRepository)
	svc := NewWorkflowService(wRepo, nil, nil, nil, nil, logger.NewNop())

	ctx := context.Background()
	wRepo.On("Delete", ctx, "T1", "W1").Return(nil)

	err := svc.Delete(ctx, "T1", "W1")

	assert.NoError(t, err)
}

func TestWorkflowService_Trigger(t *testing.T) {
	wRepo := new(MockWorkflowRepository)
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	trx := new(MockTrx)
	svc := NewWorkflowService(wRepo, eRepo, sRepo, trx, nil, logger.NewNop())

	ctx := context.Background()
	wf := &model.Workflow{
		ID:            "W1",
		TenantID:      "T1",
		DAGDefinition: json.RawMessage(`[{"id": "s1"}]`),
	}

	wRepo.On("Get", ctx, mock.Anything, mock.Anything).Return(wf, nil)
	trx.On("Begin", ctx).Return(ctx, nil)
	eRepo.On("Create", ctx, mock.Anything).Return(nil)
	sRepo.On("Create", ctx, mock.Anything).Return(nil)
	trx.On("Commit", ctx).Return(nil)
	trx.On("Rollback", ctx).Return(nil)

	res, err := svc.Trigger(ctx, "T1", "W1", "MANUAL")

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "MANUAL", res.TriggerType)
}

func TestWorkflowService_Trigger_StepExecutionTenantID(t *testing.T) {
	wRepo := new(MockWorkflowRepository)
	eRepo := new(MockExecutionRepository)
	sRepo := new(MockStepExecutionRepository)
	trx := new(MockTrx)
	svc := NewWorkflowService(wRepo, eRepo, sRepo, trx, nil, logger.NewNop())

	ctx := context.Background()
	wf := &model.Workflow{
		ID:            "W1",
		TenantID:      "T1",
		DAGDefinition: json.RawMessage(`[{"id": "s1", "action": "HTTP", "depends_on": []}]`),
	}

	// Verify that step execution is created with workflow's TenantID
	sRepo.On("Create", ctx, mock.MatchedBy(func(step *model.StepExecution) bool {
		return step.TenantID == wf.TenantID && step.ExecutionID != "" && step.StepID == "s1"
	})).Return(nil)

	wRepo.On("Get", ctx, mock.Anything, mock.Anything).Return(wf, nil)
	trx.On("Begin", ctx).Return(ctx, nil)
	eRepo.On("Create", ctx, mock.Anything).Return(nil)
	trx.On("Commit", ctx).Return(nil)
	trx.On("Rollback", ctx).Return(nil)

	res, err := svc.Trigger(ctx, "T1", "W1", "MANUAL")

	assert.NoError(t, err)
	assert.NotNil(t, res)
	sRepo.AssertExpectations(t)
}
