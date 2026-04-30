package services

import (
	"context"
	"flowforge/internal/model"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
)

// MockWorkflowRepository
type MockWorkflowRepository struct {
	mock.Mock
}

func (m *MockWorkflowRepository) Create(ctx context.Context, workflow *model.Workflow) error {
	args := m.Called(ctx, workflow)
	return args.Error(0)
}

func (m *MockWorkflowRepository) Get(ctx context.Context, filters map[string]any, workflow *model.Workflow) error {
	args := m.Called(ctx, filters, workflow)
	if args.Get(0) != nil {
		*workflow = *args.Get(0).(*model.Workflow)
	}
	return args.Error(1)
}

func (m *MockWorkflowRepository) GetForUpdate(ctx context.Context, tenantID, name string, version int, isCurrent bool) (string, error) {
	args := m.Called(ctx, tenantID, name, version, isCurrent)
	return args.String(0), args.Error(1)
}

func (m *MockWorkflowRepository) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.Workflow, int, error) {
	args := m.Called(ctx, limit, offset, filters)
	return args.Get(0).([]*model.Workflow), args.Int(1), args.Error(2)
}

func (m *MockWorkflowRepository) ListVersions(ctx context.Context, tenantID, name string) ([]*model.Workflow, error) {
	args := m.Called(ctx, tenantID, name)
	return args.Get(0).([]*model.Workflow), args.Error(1)
}

func (m *MockWorkflowRepository) Update(ctx context.Context, id string, data map[string]any) error {
	args := m.Called(ctx, id, data)
	return args.Error(0)
}

func (m *MockWorkflowRepository) Delete(ctx context.Context, tenantID, name string) error {
	args := m.Called(ctx, tenantID, name)
	return args.Error(0)
}

// MockExecutionRepository
type MockExecutionRepository struct {
	mock.Mock
}

func (m *MockExecutionRepository) Create(ctx context.Context, execution *model.Execution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockExecutionRepository) Get(ctx context.Context, filters map[string]any, execution *model.Execution) error {
	args := m.Called(ctx, filters, execution)
	if args.Get(0) != nil {
		*execution = *args.Get(0).(*model.Execution)
	}
	return args.Error(1)
}

func (m *MockExecutionRepository) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.Execution, int, error) {
	args := m.Called(ctx, limit, offset, filters)
	return args.Get(0).([]*model.Execution), args.Int(1), args.Error(2)
}

func (m *MockExecutionRepository) Update(ctx context.Context, id string, version int, data map[string]any) error {
	args := m.Called(ctx, id, version, data)
	return args.Error(0)
}

func (m *MockExecutionRepository) AcquireForWorker(ctx context.Context, limit int) ([]*model.Execution, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*model.Execution), args.Error(1)
}

func (m *MockExecutionRepository) AcquireByIDForWorker(ctx context.Context, id string) (*model.Execution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Execution), args.Error(1)
}

func (m *MockExecutionRepository) RecoverStuckJobs(ctx context.Context, timeout time.Duration) (int64, error) {
	args := m.Called(ctx, timeout)
	return int64(args.Int(0)), args.Error(1)
}

// MockStepExecutionRepository
type MockStepExecutionRepository struct {
	mock.Mock
}

func (m *MockStepExecutionRepository) Create(ctx context.Context, step *model.StepExecution) error {
	args := m.Called(ctx, step)
	return args.Error(0)
}

func (m *MockStepExecutionRepository) Get(ctx context.Context, filters map[string]any, step *model.StepExecution) error {
	args := m.Called(ctx, filters, step)
	if args.Get(0) != nil {
		*step = *args.Get(0).(*model.StepExecution)
	}
	return args.Error(1)
}

func (m *MockStepExecutionRepository) ListByExecution(ctx context.Context, executionID string) ([]*model.StepExecution, error) {
	args := m.Called(ctx, executionID)
	return args.Get(0).([]*model.StepExecution), args.Error(1)
}

func (m *MockStepExecutionRepository) List(ctx context.Context, executionID string, limit, offset uint64) ([]*model.StepExecution, int, error) {
	args := m.Called(ctx, executionID, limit, offset)
	return args.Get(0).([]*model.StepExecution), args.Int(1), args.Error(2)
}

func (m *MockStepExecutionRepository) Update(ctx context.Context, id string, currentVersion int, data map[string]any) error {
	args := m.Called(ctx, id, currentVersion, data)
	return args.Error(0)
}

func (m *MockStepExecutionRepository) GetByExecutionAndStep(ctx context.Context, execID, stepID string) (*model.StepExecution, error) {
	args := m.Called(ctx, execID, stepID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.StepExecution), args.Error(1)
}

// MockUserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Get(ctx context.Context, filters map[string]any, isActive bool, user *model.User) error {
	args := m.Called(ctx, filters, isActive, user)
	if args.Get(0) != nil {
		*user = *args.Get(0).(*model.User)
	}
	return args.Error(1)
}

func (m *MockUserRepository) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.User, int, error) {
	args := m.Called(ctx, limit, offset, filters)
	return args.Get(0).([]*model.User), args.Int(1), args.Error(2)
}

func (m *MockUserRepository) Update(ctx context.Context, id string, version int, data map[string]any) error {
	args := m.Called(ctx, id, version, data)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(ctx context.Context, id string, version int) error {
	args := m.Called(ctx, id, version)
	return args.Error(0)
}

// MockTenantRepository
type MockTenantRepository struct {
	mock.Mock
}

func (m *MockTenantRepository) Create(ctx context.Context, tenant *model.Tenant) error {
	args := m.Called(ctx, tenant)
	return args.Error(0)
}

func (m *MockTenantRepository) Get(ctx context.Context, filters map[string]any, isActive bool, tenant *model.Tenant) error {
	args := m.Called(ctx, filters, isActive, tenant)
	if args.Get(0) != nil {
		*tenant = *args.Get(0).(*model.Tenant)
	}
	return args.Error(1)
}

func (m *MockTenantRepository) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.Tenant, int, error) {
	args := m.Called(ctx, limit, offset, filters)
	return args.Get(0).([]*model.Tenant), args.Int(1), args.Error(2)
}

func (m *MockTenantRepository) Update(ctx context.Context, id string, data map[string]any) error {
	args := m.Called(ctx, id, data)
	return args.Error(0)
}

func (m *MockTenantRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockTrx
type MockTrx struct {
	mock.Mock
}

func (m *MockTrx) Begin(ctx context.Context) (context.Context, error) {
	args := m.Called(ctx)
	return args.Get(0).(context.Context), args.Error(1)
}

func (m *MockTrx) Commit(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTrx) Rollback(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockCache
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	args := m.Called(ctx, key, value, expiration)
	return args.Error(0)
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockCache) GetStruct(ctx context.Context, key string, dest interface{}) error {
	args := m.Called(ctx, key, dest)
	return args.Error(0)
}

func (m *MockCache) Delete(ctx context.Context, keys ...string) error {
	args := m.Called(ctx, keys)
	return args.Error(0)
}

func (m *MockCache) DeleteByPattern(ctx context.Context, pattern string) error {
	args := m.Called(ctx, pattern)
	return args.Error(0)
}

func (m *MockCache) Client() redis.UniversalClient {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(redis.UniversalClient)
}

func (m *MockCache) Publish(ctx context.Context, channel string, message any) error {
	args := m.Called(ctx, channel, message)
	return args.Error(0)
}

func (m *MockCache) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	args := m.Called(ctx, channel)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*redis.PubSub)
}

func (m *MockCache) RPush(ctx context.Context, key string, values ...any) error {
	args := m.Called(ctx, key, values)
	return args.Error(0)
}

func (m *MockCache) BLPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	args := m.Called(ctx, timeout, keys)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockCache) Allow(ctx context.Context, key string, limit int, rate int) (bool, error) {
	args := m.Called(ctx, key, limit, rate)
	return args.Bool(0), args.Error(1)
}

func (m *MockCache) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, key, value, ttl)
	return args.Bool(0), args.Error(1)
}
