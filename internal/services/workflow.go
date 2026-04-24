package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/dag"
	"flowforge/pkg/errorx"
	"flowforge/pkg/logger"
	"flowforge/pkg/postgres"
	"flowforge/pkg/redis"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type WorkflowService interface {
	Create(ctx context.Context, tenantID string, req *model.WorkflowRequest) (*model.WorkflowResponse, error)
	Get(ctx context.Context, tenantID, id string) (*model.WorkflowResponse, error)
	List(ctx context.Context, req model.ListWorkflowRequest) ([]*model.WorkflowResponse, int, string, error)
	ListVersions(ctx context.Context, tenantID, name string) ([]*model.WorkflowVersion, error)
	Update(ctx context.Context, req *model.WorkflowRequest) error
	Rollback(ctx context.Context, req *model.WorkflowRollbackRequest) error
	Delete(ctx context.Context, tenantID, name string) error
	Trigger(ctx context.Context, tenantID, workflowID, triggerType string) (*model.Execution, error)
}

type workflowService struct {
	wRepo repository.WorkflowRepository
	eRepo repository.ExecutionRepository
	sRepo repository.StepExecutionRepository
	trx   postgres.Trx
	cache redis.Cache
	log   *logger.Logger
}

func NewWorkflowService(
	wRepo repository.WorkflowRepository,
	eRepo repository.ExecutionRepository,
	sRepo repository.StepExecutionRepository,
	trx postgres.Trx,
	cache redis.Cache,
	l *logger.Logger,
) WorkflowService {
	return &workflowService{
		wRepo: wRepo,
		eRepo: eRepo,
		sRepo: sRepo,
		trx:   trx,
		cache: cache,
		log:   l,
	}
}

func (s *workflowService) Create(ctx context.Context, tenantID string, req *model.WorkflowRequest) (*model.WorkflowResponse, error) {
	var steps []model.StepDefinition
	if err := json.Unmarshal(req.DAGDefinition, &steps); err != nil {
		s.log.Error("failed unmarshal request", zap.Error(err))
		return nil, errorx.NewError(errorx.ErrTypeValidation, "failed unmarshal request", err)
	}

	_, err := dag.BuildExecutionPlan(steps)
	if err != nil {
		s.log.Error("failed to build dag workflow", zap.Error(err))
		return nil, err
	}

	workflow := model.Workflow{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		Name:          req.Name,
		Description:   req.Description,
		DAGDefinition: req.DAGDefinition,
		CreatedAt:     time.Now(),
		Version:       1,
	}

	err = s.wRepo.Create(ctx, &workflow)
	if err != nil {
		s.log.Error("failed to create workflow", zap.Error(err))
		return nil, err
	}

	return &model.WorkflowResponse{
		WorkflowID:    workflow.ID,
		TenantID:      workflow.TenantID,
		Name:          workflow.Name,
		Description:   workflow.Description,
		DAGDefinition: steps,
		CreatedAt:     workflow.CreatedAt,
		Version:       workflow.Version,
		IsActive:      workflow.IsActive,
		UpdatedAt:     workflow.UpdatedAt,
	}, nil
}

func (s *workflowService) Get(ctx context.Context, tenantID, id string) (*model.WorkflowResponse, error) {
	var wf model.Workflow
	err := s.wRepo.Get(ctx, map[string]any{"id": id, "tenant_id": tenantID}, &wf)
	if err != nil {
		return nil, err
	}

	var steps []model.StepDefinition
	if err := json.Unmarshal(wf.DAGDefinition, &steps); err != nil {
		s.log.Error("failed to unmarshal dag definition", zap.Error(err), zap.String("id", wf.ID))
		return nil, errorx.NewError(errorx.ErrTypeValidation, "invalid dag definition", err)
	}

	return &model.WorkflowResponse{
		WorkflowID:    wf.ID,
		TenantID:      wf.TenantID,
		Name:          wf.Name,
		Description:   wf.Description,
		DAGDefinition: steps,
		Version:       wf.Version,
		IsActive:      wf.IsActive,
		CreatedAt:     wf.CreatedAt,
		UpdatedAt:     wf.UpdatedAt,
	}, nil
}

func (s *workflowService) List(ctx context.Context, req model.ListWorkflowRequest) ([]*model.WorkflowResponse, int, string, error) {
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
	    req.PageSize = 20 // default
	}
	if req.PageSize > 100 {
	    return nil, 0, "", errorx.NewValidationError(map[string]string{
	        "page_size": "max page size is 100",
	    })
	}

	filters := map[string]any{
		"tenant_id": req.TenantID,
	}
	if req.WorkflowName != "" {
		filters["name"] = req.WorkflowName
	}

	workflows, count, err := s.wRepo.List(ctx, uint64(req.PageSize), offset, filters)
	if err != nil {
		s.log.Error("failed to get list workflow", zap.Error(err))
		return nil, 0, "", err
	}

	nextPageToken := ""
	if count == int(req.PageSize) {
		nextOffset := offset + uint64(req.PageSize)
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(nextOffset, 10)))
	}

	res := make([]*model.WorkflowResponse, 0, len(workflows))
	for _, wf := range workflows {
		var steps []model.StepDefinition
		if err := json.Unmarshal(wf.DAGDefinition, &steps); err != nil {
			s.log.Error("failed to unmarshal dag definition", zap.Error(err), zap.String("id", wf.ID))
			return nil, 0, "", errorx.NewError(errorx.ErrTypeValidation, "invalid dag definition", err)
		}
		res = append(res, &model.WorkflowResponse{
			WorkflowID:    wf.ID,
			TenantID:      wf.TenantID,
			Name:          wf.Name,
			Description:   wf.Description,
			DAGDefinition: steps,
			Version:       wf.Version,
			IsActive:      wf.IsActive,
			CreatedAt:     wf.CreatedAt,
			UpdatedAt:     wf.UpdatedAt,
		})
	}

	return res, count, nextPageToken, nil
}

func (s *workflowService) ListVersions(ctx context.Context, tenantID, name string) ([]*model.WorkflowVersion, error) {
	workflows, err := s.wRepo.ListVersions(ctx, tenantID, name)
	if err != nil {
		s.log.Error("failed to get list versions workflow", zap.Error(err))
		return nil, err
	}

	res := slices.Grow([]*model.WorkflowVersion{}, len(workflows))
	for _, wf := range workflows {
		var steps []model.StepDefinition
		if err := json.Unmarshal(wf.DAGDefinition, &steps); err != nil {
			s.log.Error("failed to unmarshal dag definition", zap.Error(err), zap.String("id", wf.ID))
		}
		res = append(res, &model.WorkflowVersion{
			WorkflowID:    wf.ID,
			TenantID:      wf.TenantID,
			Name:          wf.Name,
			Description:   wf.Description,
			DAGDefinition: steps,
			Version:       wf.Version,
			IsCurrent:     wf.IsCurrent,
			CreatedAt:     wf.CreatedAt,
		})
	}

	return res, nil
}

func (s *workflowService) Update(ctx context.Context, req *model.WorkflowRequest) error {
	var steps []model.StepDefinition
	if err := json.Unmarshal(req.DAGDefinition, &steps); err != nil {
		s.log.Error("failed unmarshal request", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeValidation, "failed unmarshal request", err)
	}

	_, err := dag.BuildExecutionPlan(steps)
	if err != nil {
		s.log.Error("failed to build dag workflow", zap.Error(err))
		return err
	}

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transaction", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	lockedID, err := s.wRepo.GetForUpdate(txCtx, req.TenantID, req.Name, req.CurrentVersion, true)
	if err != nil {
		s.log.Error("failed to get workflow", zap.Error(err))
		return err
	}

	workflow := model.Workflow{
		ID:            uuid.New().String(),
		TenantID:      req.TenantID,
		Name:          req.Name,
		Description:   req.Description,
		DAGDefinition: req.DAGDefinition,
		CreatedAt:     time.Now(),
		Version:       req.CurrentVersion + 1,
	}
	err = s.wRepo.Create(txCtx, &workflow)
	if err != nil {
		s.log.Error("failed to create workflow", zap.Error(err))
		return err
	}

	err = s.wRepo.Update(txCtx, lockedID, map[string]any{
		"is_current": false,
		"updated_at": time.Now(),
	})
	if err != nil {
		s.log.Error("failed to update workflow", zap.Error(err))
		return err
	}

	if err := s.trx.Commit(txCtx); err != nil {
		s.log.Error("failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (s *workflowService) Rollback(ctx context.Context, req *model.WorkflowRollbackRequest) error {
	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transaction", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	currentID, err := s.wRepo.GetForUpdate(txCtx, req.TenantID, req.Name, req.CurrentVersion, true)
	if err != nil {
		s.log.Error("failed to get workflow by current version", zap.Error(err))
		return err
	}

	targetID, err := s.wRepo.GetForUpdate(txCtx, req.TenantID, req.Name, req.TargetVersion, true)
	if err != nil {
		s.log.Error("failed to get workflow by target version", zap.Error(err))
		return err
	}

	err = s.wRepo.Update(txCtx, currentID, map[string]any{
		"is_current": false,
		"updated_at": time.Now(),
	})
	if err != nil {
		s.log.Error("failed to update workflow", zap.Error(err))
		return err
	}

	err = s.wRepo.Update(txCtx, targetID, map[string]any{
		"is_current": true,
		"updated_at": time.Now(),
	})
	if err != nil {
		s.log.Error("failed to update workflow", zap.Error(err))
		return err
	}

	if err := s.trx.Commit(txCtx); err != nil {
		s.log.Error("failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (s *workflowService) Delete(ctx context.Context, tenantID, name string) error {
	err := s.wRepo.Delete(ctx, tenantID, name)
	if err != nil {
		s.log.Error("failed to delete workflow", zap.Error(err))
		return err
	}

	return nil
}

func (s *workflowService) Trigger(ctx context.Context, tenantID, workflowID, triggerType string) (*model.Execution, error) {
	if triggerType == "" {
		triggerType = "MANUAL"
	}

	var wf model.Workflow
	err := s.wRepo.Get(ctx, map[string]any{
		"id": workflowID,
		"tenant_id": tenantID,
		"is_current": true,
	}, &wf)
	if err != nil {
		return nil, err
	}

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer s.trx.Rollback(txCtx)

	now := time.Now()
	execution := &model.Execution{
		ID:          uuid.New().String(),
		TenantID:    wf.TenantID,
		WorkflowID:  wf.ID,
		Status:      string(model.StatusExecutionPending),
		TriggerType: triggerType,
		Version:     wf.Version,
		CreatedAt:   now,
	}

	err = s.eRepo.Create(txCtx, execution)
	if err != nil {
		s.log.Error("failed to create execution", zap.Error(err))
		return nil, err
	}

	var steps []model.StepDefinition
	if err := json.Unmarshal(wf.DAGDefinition, &steps); err != nil {
		s.log.Error("failed to unmarshal dag definition", zap.Error(err), zap.String("id", wf.ID))
		return nil, errorx.NewError(errorx.ErrTypeInternal, "failed to unmarshal workflow definition", err)
	}

	for _, stepDef := range steps {
		stepRec := &model.StepExecution{
			ID:          uuid.New().String(),
			TenantID:    wf.TenantID,
			ExecutionID: execution.ID,
			StepID:      stepDef.ID,
			Action:      stepDef.Action,
			Status:      string(model.StatusExecutionPending),
			RetryCount:  0,
		}
		err = s.sRepo.Create(txCtx, stepRec)
		if err != nil {
			s.log.Error("failed to create step execution", zap.Error(err))
			return nil, err
		}
	}

	if err := s.trx.Commit(txCtx); err != nil {
		return nil, err
	}

	if s.cache != nil {
		_ = s.cache.RPush(ctx, "flowforge:jobs:queue", execution.ID)
	}

	return execution, nil
}
