package service

import (
	"context"
	"encoding/json"
	"flowforge/internal/model"
	"flowforge/pkg/logger"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAction struct {
	mock.Mock
}

func (m *MockAction) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]any), args.Error(1)
}

type MockBroadcaster struct {
	mock.Mock
}

func (m *MockBroadcaster) BroadcastToRedis(ctx context.Context, tenantID string, event any) error {
	args := m.Called(ctx, tenantID, event)
	return args.Error(0)
}

// These mocks are duplicates of what's in internal/services/service_mocks_test.go
// but for the engine_test they are easier to keep here for now to avoid package import cycles.

type mockExecRepo struct {
	mock.Mock
}

func (m *mockExecRepo) Create(ctx context.Context, exec *model.Execution) error {
	return m.Called(ctx, exec).Error(0)
}
func (m *mockExecRepo) Get(ctx context.Context, filters map[string]any, exec *model.Execution) error {
	args := m.Called(ctx, filters, exec)
	if args.Get(0) != nil {
		*exec = *args.Get(0).(*model.Execution)
	}
	return args.Error(1)
}
func (m *mockExecRepo) Update(ctx context.Context, id string, version int, data map[string]any) error {
	return m.Called(ctx, id, version, data).Error(0)
}
func (m *mockExecRepo) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.Execution, int, error) {
	args := m.Called(ctx, limit, offset, filters)
	return args.Get(0).([]*model.Execution), args.Int(1), args.Error(2)
}
func (m *mockExecRepo) AcquireForWorker(ctx context.Context, limit int) ([]*model.Execution, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*model.Execution), args.Error(1)
}
func (m *mockExecRepo) AcquireByIDForWorker(ctx context.Context, id string) (*model.Execution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Execution), args.Error(1)
}
func (m *mockExecRepo) RecoverStuckJobs(ctx context.Context, timeout time.Duration) (int64, error) {
	args := m.Called(ctx, timeout)
	return int64(args.Int(0)), args.Error(1)
}

type mockStepRepo struct {
	mock.Mock
}

func (m *mockStepRepo) Create(ctx context.Context, step *model.StepExecution) error {
	return m.Called(ctx, step).Error(0)
}
func (m *mockStepRepo) Update(ctx context.Context, id string, data map[string]any) error {
	return m.Called(ctx, id, data).Error(0)
}
func (m *mockStepRepo) Get(ctx context.Context, filters map[string]any, step *model.StepExecution) error {
	args := m.Called(ctx, filters, step)
	if args.Get(0) != nil {
		*step = *args.Get(0).(*model.StepExecution)
	}
	return args.Error(1)
}
func (m *mockStepRepo) ListByExecution(ctx context.Context, executionID string) ([]*model.StepExecution, error) {
	args := m.Called(ctx, executionID)
	return args.Get(0).([]*model.StepExecution), args.Error(1)
}
func (m *mockStepRepo) List(ctx context.Context, executionID string, limit, offset uint64) ([]*model.StepExecution, int, error) {
	args := m.Called(ctx, executionID, limit, offset)
	return args.Get(0).([]*model.StepExecution), args.Int(1), args.Error(2)
}
func (m *mockStepRepo) GetByExecutionAndStep(ctx context.Context, execID, stepID string) (*model.StepExecution, error) {
	args := m.Called(ctx, execID, stepID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.StepExecution), args.Error(1)
}

type mockTrx struct {
	mock.Mock
}

func (m *mockTrx) Begin(ctx context.Context) (context.Context, error) {
	args := m.Called(ctx)
	return args.Get(0).(context.Context), args.Error(1)
}
func (m *mockTrx) Commit(ctx context.Context) error   { return m.Called(ctx).Error(0) }
func (m *mockTrx) Rollback(ctx context.Context) error { return m.Called(ctx).Error(0) }

func TestEngine_SimpleWorkflow_Success(t *testing.T) {
	eRepo := new(mockExecRepo)
	sRepo := new(mockStepRepo)
	trx := new(mockTrx)
	l := logger.NewNop()
	broadcaster := new(MockBroadcaster)

	registry := NewRegistry()
	mockAction := new(MockAction)
	registry.Registry("HTTP", mockAction)

	engine := NewExecutionEngine(eRepo, sRepo, trx, l, broadcaster, 5*time.Minute)
	// Override registry for testing
	engine.registry = registry

	workflow := &model.Workflow{
		ID:            "wf-1",
		DAGDefinition: json.RawMessage(`[{"id": "step-1", "action": "HTTP", "parameters": {"url": "test"}}]`),
	}
	execution := &model.Execution{
		ID:      "exec-1",
		Version: 1,
	}

	// Mock expectations
	mockAction.On("Execute", mock.Anything, mock.Anything).Return(map[string]any{"data": "ok"}, nil)

	sRepo.On("GetByExecutionAndStep", mock.Anything, "exec-1", "step-1").Return(&model.StepExecution{
		ID: "se-1", StepID: "step-1",
	}, nil).Maybe()

	sRepo.On("ListByExecution", mock.Anything, "exec-1").Return([]*model.StepExecution{}, nil).Maybe()

	sRepo.On("Update", mock.Anything, "se-1", mock.Anything).Return(nil).Maybe()

	eRepo.On("Update", mock.Anything, "exec-1", mock.Anything, mock.MatchedBy(func(data map[string]any) bool {
		return data["status"] == string(model.StatusExecutionSuccess)
	})).Return(nil)

	// Expect two broadcasts: one for step running, one for step success
	broadcaster.On("BroadcastToRedis", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)

	engine.RunExecution(context.Background(), execution, workflow)

	mockAction.AssertExpectations(t)
	eRepo.AssertExpectations(t)
	sRepo.AssertExpectations(t)
	broadcaster.AssertExpectations(t)
}

func TestEngine_RetryWithBackoff(t *testing.T) {
	eRepo := new(mockExecRepo)
	sRepo := new(mockStepRepo)
	trx := new(mockTrx)
	l := logger.NewNop()
	broadcaster := new(MockBroadcaster)

	registry := NewRegistry()
	mockAction := new(MockAction)
	registry.Registry("HTTP", mockAction)

	engine := NewExecutionEngine(eRepo, sRepo, trx, l, broadcaster, 5*time.Minute)
	engine.registry = registry

	workflow := &model.Workflow{
		ID:            "wf-retry",
		DAGDefinition: json.RawMessage(`[{"id": "step-1", "action": "HTTP", "max_retries": 1}]`),
	}
	execution := &model.Execution{
		ID:      "exec-retry",
		Version: 1,
	}

	// First call fails, second succeeds
	mockAction.On("Execute", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("temporary failure")).Once()
	mockAction.On("Execute", mock.Anything, mock.Anything).Return(map[string]any{"data": "ok"}, nil).Once()

	sRepo.On("Update", mock.Anything, "se-1", mock.Anything).Return(nil).Maybe()
	sRepo.On("GetByExecutionAndStep", mock.Anything, "exec-retry", "step-1").Return(&model.StepExecution{
		ID: "se-1", StepID: "step-1",
	}, nil).Maybe()
	sRepo.On("ListByExecution", mock.Anything, "exec-retry").Return([]*model.StepExecution{}, nil).Maybe()
	eRepo.On("Update", mock.Anything, "exec-retry", mock.Anything, mock.Anything).Return(nil)

	// Broadcaster expectations:
	// 1. Step Running (Attempt 1)
	// 2. Step Running (Attempt 2)
	// 3. Step Success
	broadcaster.On("BroadcastToRedis", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(3)

	start := time.Now()
	engine.RunExecution(context.Background(), execution, workflow)
	elapsed := time.Since(start)

	assert.True(t, elapsed >= 1*time.Second, "should have waited at least 1s for backoff")
	mockAction.AssertExpectations(t)
	broadcaster.AssertExpectations(t)
}

func TestEngine_MaxRetries_Exhausted(t *testing.T) {
	eRepo := new(mockExecRepo)
	sRepo := new(mockStepRepo)
	trx := new(mockTrx)
	l := logger.NewNop()
	broadcaster := new(MockBroadcaster)

	registry := NewRegistry()
	mockAction := new(MockAction)
	registry.Registry("HTTP", mockAction)

	engine := NewExecutionEngine(eRepo, sRepo, trx, l, broadcaster, 5*time.Minute)
	engine.registry = registry

	workflow := &model.Workflow{
		ID:            "wf-exhausted",
		DAGDefinition: json.RawMessage(`[{"id": "step-1", "action": "HTTP", "max_retries": 1}]`),
	}
	execution := &model.Execution{
		ID:      "exec-exhausted",
		Version: 1,
	}

	// Always fails
	mockAction.On("Execute", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("persistent failure")).Times(2)

	sRepo.On("Update", mock.Anything, "se-1", mock.Anything).Return(nil).Maybe()
	sRepo.On("GetByExecutionAndStep", mock.Anything, "exec-exhausted", "step-1").Return(&model.StepExecution{
		ID: "se-1", StepID: "step-1",
	}, nil).Maybe()
	sRepo.On("ListByExecution", mock.Anything, "exec-exhausted").Return([]*model.StepExecution{}, nil).Maybe()
	eRepo.On("Update", mock.Anything, "exec-exhausted", mock.Anything, mock.Anything).Return(nil)

	// Broadcaster expectations:
	// 1. Step Running (A1)
	// 2. Step Running (A2)
	// 3. Step Failed
	broadcaster.On("BroadcastToRedis", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(3)

	engine.RunExecution(context.Background(), execution, workflow)

	mockAction.AssertExpectations(t)
	broadcaster.AssertExpectations(t)
	eRepo.AssertCalled(t, "Update", mock.Anything, "exec-exhausted", mock.Anything, mock.MatchedBy(func(data map[string]any) bool {
		return data["status"] == string(model.StatusExecutionFailed)
	}))
}

func TestEngine_ConditionalSkip(t *testing.T) {
	eRepo := new(mockExecRepo)
	sRepo := new(mockStepRepo)
	trx := new(mockTrx)
	l := logger.NewNop()
	broadcaster := new(MockBroadcaster)

	registry := NewRegistry()
	mockAction := new(MockAction)
	registry.Registry("HTTP", mockAction)

	engine := NewExecutionEngine(eRepo, sRepo, trx, l, broadcaster, 5*time.Minute)
	engine.registry = registry

	workflow := &model.Workflow{
		ID: "wf-skip",
		DAGDefinition: json.RawMessage(`[
			{"id": "step-1", "action": "HTTP", "parameters": {"condition": "some-expression"}},
			{"id": "step-2", "action": "HTTP", "depends_on": ["step-1"]}
		]`),
	}
	execution := &model.Execution{
		ID:      "exec-skip",
		Version: 1,
	}

	mockAction.On("Execute", mock.Anything, mock.MatchedBy(func(params map[string]any) bool {
		return params["condition"] == "some-expression"
	})).Return(map[string]any{"condition_met": false}, nil).Once()

	sRepo.On("Update", mock.Anything, "se-1", mock.Anything).Return(nil).Maybe()
	sRepo.On("GetByExecutionAndStep", mock.Anything, "exec-skip", "step-1").Return(&model.StepExecution{
		ID: "se-1", StepID: "step-1",
	}, nil).Maybe()
	sRepo.On("GetByExecutionAndStep", mock.Anything, "exec-skip", "step-2").Return(&model.StepExecution{
		ID: "se-2", StepID: "step-2",
	}, nil).Maybe()
	sRepo.On("ListByExecution", mock.Anything, "exec-skip").Return([]*model.StepExecution{}, nil).Maybe()
	eRepo.On("Update", mock.Anything, "exec-skip", mock.Anything, mock.Anything).Return(nil)

	// Broadcaster: 1x Running, 1x Success (for step 1)
	broadcaster.On("BroadcastToRedis", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)

	engine.RunExecution(context.Background(), execution, workflow)

	mockAction.AssertExpectations(t)
	broadcaster.AssertExpectations(t)
}

func TestEngine_ParallelExecution(t *testing.T) {
	eRepo := new(mockExecRepo)
	sRepo := new(mockStepRepo)
	trx := new(mockTrx)
	l := logger.NewNop()
	broadcaster := new(MockBroadcaster)

	registry := NewRegistry()
	mockAction := new(MockAction)
	registry.Registry("WAIT", mockAction)

	engine := NewExecutionEngine(eRepo, sRepo, trx, l, broadcaster, 5*time.Minute)
	engine.registry = registry

	workflow := &model.Workflow{
		ID: "wf-parallel",
		DAGDefinition: json.RawMessage(`[
			{"id": "step-1", "action": "WAIT", "parameters": {"duration": "1s"}},
			{"id": "step-2", "action": "WAIT", "parameters": {"duration": "1s"}}
		]`),
	}
	execution := &model.Execution{
		ID:      "exec-parallel",
		Version: 1,
	}

	mockAction.On("Execute", mock.Anything, mock.Anything).Return(nil, nil).Twice()

	sRepo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	sRepo.On("GetByExecutionAndStep", mock.Anything, "exec-parallel", "step-1").Return(&model.StepExecution{
		ID: "se-1", StepID: "step-1",
	}, nil).Maybe()
	sRepo.On("GetByExecutionAndStep", mock.Anything, "exec-parallel", "step-2").Return(&model.StepExecution{
		ID: "se-2", StepID: "step-2",
	}, nil).Maybe()
	sRepo.On("ListByExecution", mock.Anything, "exec-parallel").Return([]*model.StepExecution{}, nil).Maybe()
	eRepo.On("Update", mock.Anything, "exec-parallel", mock.Anything, mock.Anything).Return(nil)

	// Broadcaster: (1x Running + 1x Success) * 2 steps = 4 calls
	broadcaster.On("BroadcastToRedis", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(4)

	start := time.Now()
	engine.RunExecution(context.Background(), execution, workflow)
	elapsed := time.Since(start)

	mockAction.AssertExpectations(t)
	broadcaster.AssertExpectations(t)
	assert.True(t, elapsed < 1*time.Second)
}
