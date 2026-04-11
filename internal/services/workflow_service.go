package services

import (
	"context"
	"encoding/json"
	"time"

	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/dag"
	"flowforge/pkg/errorx"
	"flowforge/pkg/logger"
	"flowforge/pkg/postgres"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type WorkflowService interface {
	Create(ctx context.Context, tenantID string, req *model.WorkflowRequest) (*model.Workflow, error)
	Get(ctx context.Context, tenantID, id string) (*model.WorkflowResponse, error)
	List(ctx context.Context, tenantID string, limit, offset uint64, name string) ([]*model.WorkflowResponse, int, error)
	Trigger(ctx context.Context, tenantID, workflowID string) (*model.Execution, error)
}

type workflowService struct {
	wRepo repository.WorkflowRepository
	eRepo repository.ExecutionRepository
	sRepo repository.StepExecutionRepository
	uow   postgres.Trx
	l     *logger.Logger
}

func NewWorkflowService(
	wRepo repository.WorkflowRepository,
	eRepo repository.ExecutionRepository,
	sRepo repository.StepExecutionRepository,
	uow postgres.Trx,
	l *logger.Logger,
) WorkflowService {
	return &workflowService{
		wRepo: wRepo,
		eRepo: eRepo,
		sRepo: sRepo,
		uow:   uow,
		l:     l,
	}
}

func (s *workflowService) Create(ctx context.Context, tenantID string, req *model.WorkflowRequest) (*model.Workflow, error) {
	var steps []model.StepDefinition
	if err := json.Unmarshal(req.DAGDefinition, &steps); err != nil {
		s.l.Logger.Error("failed unmarshal request", zap.Error(err))
		return nil, errorx.NewError(errorx.ErrTypeValidation, "failed unmarshal request", err)
	}

	_, err := dag.BuildExecutionPlan(steps)
	if err != nil {
		return nil, err
	}

	wf := &model.Workflow{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		Name:          req.Name,
		Description:   req.Description,
		DAGDefinition: req.DAGDefinition,
		CreatedAt:     time.Now(),
	}

	err = s.wRepo.Create(ctx, wf)
	if err != nil {
		s.l.Error("failed to create workflow", zap.Error(err))
		return nil, err
	}

	return wf, nil
}

func (s *workflowService) Get(ctx context.Context, tenantID, id string) (*model.WorkflowResponse, error) {
	var wf model.Workflow
	err := s.wRepo.Get(ctx, map[string]any{"id": id, "tenant_id": tenantID}, &wf)
	if err != nil {
		return nil, err
	}

	var steps []model.StepDefinition
	_ = json.Unmarshal(wf.DAGDefinition, &steps)

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

func (s *workflowService) List(ctx context.Context, tenantID string, limit, offset uint64, name string) ([]*model.WorkflowResponse, int, error) {
	filters := map[string]any{"tenant_id": tenantID}
	if name != "" {
		filters["name"] = name
	}

	wfs, count, err := s.wRepo.List(ctx, limit, offset, filters)
	if err != nil {
		return nil, 0, err
	}

	var dtos []*model.WorkflowResponse
	for _, wf := range wfs {
		var steps []model.StepDefinition
		_ = json.Unmarshal(wf.DAGDefinition, &steps)
		dtos = append(dtos, &model.WorkflowResponse{
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

	return dtos, count, nil
}

func (s *workflowService) Trigger(ctx context.Context, tenantID, workflowID string) (*model.Execution, error) {
	var wf model.Workflow
	err := s.wRepo.Get(ctx, map[string]any{"id": workflowID, "tenant_id": tenantID, "is_current": true}, &wf)
	if err != nil {
		return nil, err
	}

	txCtx, err := s.uow.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer s.uow.Rollback(txCtx)

	now := time.Now()
	execution := &model.Execution{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		WorkflowID:  wf.ID,
		Status:      "PENDING",
		TriggerType: "MANUAL",
		Version:     1,
		CreatedAt:   now,
	}

	err = s.eRepo.Create(txCtx, execution)
	if err != nil {
		s.l.Error("failed to create execution", zap.Error(err))
		return nil, err
	}

	var steps []model.StepDefinition
	_ = json.Unmarshal(wf.DAGDefinition, &steps)

	for _, stepDef := range steps {
		stepRec := &model.StepExecution{
			ID:          uuid.New().String(),
			TenantID:    tenantID,
			ExecutionID: execution.ID,
			StepID:      stepDef.ID,
			Action:      stepDef.Action,
			Status:      "PENDING",
			RetryCount:  0,
		}
		err = s.sRepo.Create(txCtx, stepRec)
		if err != nil {
			s.l.Error("failed to create step execution", zap.Error(err))
			return nil, err
		}
	}

	if err := s.uow.Commit(txCtx); err != nil {
		return nil, err
	}

	return execution, nil
}
