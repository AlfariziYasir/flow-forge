package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"time"

	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/errorx"
	"flowforge/pkg/logger"
	"flowforge/pkg/postgres"
	"flowforge/pkg/redis"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ExecutionService interface {
	Get(ctx context.Context, tenantID, id string) (*model.ExecutionResponse, error)
	List(ctx context.Context, req model.ListExecutionRequest) ([]*model.ExecutionResponse, int, string, error)
	Retry(ctx context.Context, tenantID, executionID string) error
	Cancel(ctx context.Context, tenantID, executionID string) error
}

type executionService struct {
	eRepo repository.ExecutionRepository
	wRepo repository.WorkflowRepository
	sRepo repository.StepExecutionRepository
	trx   postgres.Trx
	cache redis.Cache
	log   *logger.Logger
}

func NewExecutionService(eRepo repository.ExecutionRepository, sRepo repository.StepExecutionRepository, wRepo repository.WorkflowRepository, log *logger.Logger, trx postgres.Trx, cache redis.Cache) ExecutionService {
	return &executionService{
		eRepo: eRepo,
		wRepo: wRepo,
		sRepo: sRepo,
		log:   log,
		trx:   trx,
		cache: cache,
	}
}

func (s *executionService) Get(ctx context.Context, tenantID, id string) (*model.ExecutionResponse, error) {
	var exec model.Execution
	err := s.eRepo.Get(ctx, map[string]any{"id": id, "tenant_id": tenantID}, &exec)
	if err != nil {
		s.log.Error("failed to get execution by id", zap.Error(err))
		return nil, err
	}

	return &model.ExecutionResponse{
		ID:          exec.ID,
		TenantID:    exec.TenantID,
		WorkflowID:  exec.WorkflowID,
		Status:      exec.Status,
		TriggerType: exec.TriggerType,
		CreatedAt:   exec.CreatedAt,
		StartedAt:   exec.StartedAt,
		CompletedAt: exec.CompletedAt,
	}, nil
}

func (s *executionService) List(ctx context.Context, req model.ListExecutionRequest) ([]*model.ExecutionResponse, int, string, error) {
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

	filters := map[string]any{
		"tenant_id": req.TenantID,
	}
	if req.WorkflowID != "" {
		filters["workflow_id"] = req.WorkflowID
	}

	if req.Status != "" {
		filters["status"] = req.Status
	}

	if req.TriggerType != "" {
		filters["trigger_type"] = req.TriggerType
	}

	executions, count, err := s.eRepo.List(ctx, uint64(req.PageSize), offset, filters)
	if err != nil {
		s.log.Error("failed to get list executions", zap.Error(err))
		return nil, 0, "", err
	}

	nextPageToken := ""
	if count == int(req.PageSize) {
		nextOffset := offset + uint64(req.PageSize)
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(nextOffset, 10)))
	}

	res := make([]*model.ExecutionResponse, 0, len(executions))
	for _, exec := range executions {
		res = append(res, &model.ExecutionResponse{
			ID:          exec.ID,
			WorkflowID:  exec.WorkflowID,
			TenantID:    exec.TenantID,
			Status:      exec.Status,
			TriggerType: exec.TriggerType,
			Version:     exec.Version,
			CreatedAt:   exec.CreatedAt,
			StartedAt:   exec.StartedAt,
			CompletedAt: exec.CompletedAt,
		})
	}

	return res, count, nextPageToken, nil
}

func (s *executionService) Retry(ctx context.Context, tenantID, executionID string) error {
	var oldExec model.Execution
	err := s.eRepo.Get(ctx, map[string]any{"id": executionID, "tenant_id": tenantID}, &oldExec)
	if err != nil {
		s.log.Error("failed to get execution by id", zap.Error(err))
		return err
	}

	if oldExec.Status != model.StatusExecutionFailed {
		s.log.Error("execution is not in failed state")
		return errorx.NewError(errorx.ErrTypeConflict, "execution is not in failed state", nil)
	}

	var workflow model.Workflow
	err = s.wRepo.Get(ctx, map[string]any{
		"id":        oldExec.WorkflowID,
		"version":   oldExec.Version,
		"tenant_id": tenantID,
	}, &workflow)
	if err != nil {
		s.log.Error("failed to get workflow", zap.Error(err))
		return err
	}

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		return err
	}
	defer s.trx.Rollback(txCtx)

	now := time.Now()
	execution := &model.Execution{
		ID:          uuid.New().String(),
		TenantID:    oldExec.TenantID,
		WorkflowID:  oldExec.WorkflowID,
		Status:      string(model.StatusExecutionPending),
		TriggerType: "MANUAL",
		Version:     oldExec.Version,
		CreatedAt:   now,
	}

	err = s.eRepo.Create(txCtx, execution)
	if err != nil {
		s.log.Error("failed to create execution", zap.Error(err))
		return err
	}

	var steps []model.StepDefinition
	if err := json.Unmarshal(workflow.DAGDefinition, &steps); err != nil {
		s.log.Error("failed to unmarshal dag definition", zap.Error(err), zap.String("id", workflow.ID))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to unmarshal workflow definition", err)
	}

	for _, stepDef := range steps {
		stepRec := &model.StepExecution{
			ID:          uuid.New().String(),
			TenantID:    tenantID,
			ExecutionID: execution.ID,
			StepID:      stepDef.ID,
			Action:      stepDef.Action,
			Status:      string(model.StatusExecutionPending),
			Version:     1,
			RetryCount:  0,
		}
		err = s.sRepo.Create(txCtx, stepRec)
		if err != nil {
			s.log.Error("failed to create step execution", zap.Error(err))
			return err
		}
	}

	if err := s.trx.Commit(txCtx); err != nil {
		return err
	}

	if s.cache != nil {
		_ = s.cache.RPush(ctx, "flowforge:jobs:queue", execution.ID)
	}

	return nil
}

func (s *executionService) Cancel(ctx context.Context, tenantID, executionID string) error {
	var exec model.Execution
	err := s.eRepo.Get(ctx, map[string]any{"id": executionID, "tenant_id": tenantID}, &exec)
	if err != nil {
		s.log.Error("failed to get execution by id", zap.Error(err))
		return err
	}

	if exec.Status != model.StatusExecutionRunning {
		s.log.Error("execution is not in running state")
		return errorx.NewError(errorx.ErrTypeConflict, "execution is not in running state", nil)
	}

	err = s.eRepo.Update(ctx, exec.ID, exec.Version, map[string]any{"status": model.StatusExecutionCancelled})
	if err != nil {
		s.log.Error("failed to update execution", zap.Error(err))
		return err
	}

	return nil
}
