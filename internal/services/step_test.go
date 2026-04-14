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

func TestStepExecutionService_Get(t *testing.T) {
	sRepo := new(MockStepExecutionRepository)
	svc := NewStepExecutionService(sRepo, logger.NewNop(), nil)

	ctx := context.Background()
	id := uuid.New().String()
	tenantID := uuid.New().String()
	step := &model.StepExecution{ID: id, TenantID: tenantID}

	sRepo.On("Get", ctx, map[string]any{"id": id, "tenant_id": tenantID}, mock.AnythingOfType("*model.StepExecution")).
		Return(step, nil)

	res, err := svc.Get(ctx, tenantID, id)

	assert.NoError(t, err)
	assert.Equal(t, id, res.ID)
}

func TestStepExecutionService_List(t *testing.T) {
	sRepo := new(MockStepExecutionRepository)
	svc := NewStepExecutionService(sRepo, logger.NewNop(), nil)

	ctx := context.Background()
	execID := uuid.New().String()
	steps := []*model.StepExecution{{ID: "s1", ExecutionID: execID}}

	sRepo.On("ListByExecution", ctx, execID).Return(steps, nil)

	res, err := svc.List(ctx, execID)

	assert.NoError(t, err)
	assert.Len(t, res, 1)
}
