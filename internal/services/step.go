package services

import (
	"context"
	"encoding/base64"
	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/errorx"
	"flowforge/pkg/logger"
	"flowforge/pkg/postgres"
	"strconv"

	"go.uber.org/zap"
)

type StepExecutionService interface {
	Get(ctx context.Context, tenantID, id string) (*model.StepExecutionResponse, error)
	List(ctx context.Context, req model.ListStepExecutionRequest) ([]*model.StepExecutionResponse, int, string, error)
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
		Version:     step.Version,
		RetryCount:  step.RetryCount,
		ErrorLog:    step.ErrorLog,
		StartedAt:   step.StartedAt,
		CompletedAt: step.CompletedAt,
	}, nil
}

func (s *stepExecutionService) List(ctx context.Context, req model.ListStepExecutionRequest) ([]*model.StepExecutionResponse, int, string, error) {
	var offset uint64 = 0
	if req.PageToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.PageToken)
		if err != nil {
			return nil, 0, "", errorx.NewError(errorx.ErrTypeValidation, "invalid page token", err)
		}
		offset, err = strconv.ParseUint(string(decoded), 10, 64)
		if err != nil {
			return nil, 0, "", errorx.NewError(errorx.ErrTypeValidation, "invalid page token", err)
		}
	}

	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		return nil, 0, "", errorx.NewValidationError(map[string]string{
			"page_size": "max page size is 100",
		})
	}

	steps, count, err := s.sRepo.List(ctx, req.ExecutionID, uint64(req.PageSize), offset)
	if err != nil {
		s.log.Error("failed to get step execution by execution id", zap.Error(err))
		return nil, 0, "", err
	}

	nextPageToken := ""
	if count == int(req.PageSize) {
		nextOffset := offset + uint64(req.PageSize)
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(nextOffset, 10)))
	}

	res := make([]*model.StepExecutionResponse, 0, len(steps))
	for _, step := range steps {
		res = append(res, &model.StepExecutionResponse{
			ID:          step.ID,
			TenantID:    step.TenantID,
			ExecutionID: step.ExecutionID,
			StepID:      step.StepID,
			Action:      step.Action,
			Status:      step.Status,
			Version:     step.Version,
			RetryCount:  step.RetryCount,
			ErrorLog:    step.ErrorLog,
			StartedAt:   step.StartedAt,
			CompletedAt: step.CompletedAt,
		})
	}

	return res, count, nextPageToken, nil
}
