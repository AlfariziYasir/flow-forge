package services

import (
	"context"
	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/logger"
	"flowforge/pkg/postgres"
	"slices"

	"go.uber.org/zap"
)

type StepExecutionService interface {
	Get(ctx context.Context, tenantID, id string) (*model.StepExecutionResponse, error)
	List(ctx context.Context, executionID string) ([]*model.StepExecutionResponse, error)
}

type stepExecutionService struct {
	sRepo repository.StepExecutionRepository
	log   *logger.Logger
	trx   postgres.Trx
}

func NewStepExecutionService(sRepo repository.StepExecutionRepository, log *logger.Logger, trx postgres.Trx) StepExecutionService {
	return &stepExecutionService{
		sRepo: sRepo,
		log:   log,
		trx:   trx,
	}
}

func (s *stepExecutionService) Get(ctx context.Context, tenantID, id string) (*model.StepExecutionResponse, error) {
	var step model.StepExecution
	err := s.sRepo.Get(ctx, map[string]any{"id": id, "tenant_id": tenantID}, &step)
	if err != nil {
		s.log.Error("failed to get execution by id", zap.Error(err))
		return nil, err
	}

	return &model.StepExecutionResponse{
		ID:          step.ID,
		TenantID:    step.TenantID,
		ExecutionID: step.ExecutionID,
		StepID:      step.StepID,
		Action:      step.Action,
		Status:      step.Status,
		RetryCount:  step.RetryCount,
		ErrorLog:    step.ErrorLog,
		StartedAt:   step.StartedAt,
		CompletedAt: step.CompletedAt,
	}, nil
}

func (s *stepExecutionService) List(ctx context.Context, executionID string) ([]*model.StepExecutionResponse, error) {
	steps, err := s.sRepo.ListByExecution(ctx, executionID)
	if err != nil {
		s.log.Error("failed to get step execution by execution id", zap.Error(err))
		return nil, err
	}

	res := slices.Grow([]*model.StepExecutionResponse{}, len(steps))
	for _, step := range steps {
		res = append(res, &model.StepExecutionResponse{
			ID:          step.ID,
			TenantID:    step.TenantID,
			ExecutionID: step.ExecutionID,
			StepID:      step.StepID,
			Action:      step.Action,
			Status:      step.Status,
			RetryCount:  step.RetryCount,
			ErrorLog:    step.ErrorLog,
			StartedAt:   step.StartedAt,
			CompletedAt: step.CompletedAt,
		})
	}

	return res, nil
}
