package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/errorx"
	"flowforge/pkg/logger"
	"flowforge/pkg/postgres"
	"flowforge/pkg/redis"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type CronService interface {
	Start(ctx context.Context) error
	Stop() error
	ScheduleWorkflow(ctx context.Context, workflowID, cronExpr string) error
	UnscheduleWorkflow(ctx context.Context, workflowID string) error
}

type cronService struct {
	cache     redis.Cache
	eRepo     repository.ExecutionRepository
	sRepo     repository.StepExecutionRepository
	wRepo     repository.WorkflowRepository
	trx       postgres.Trx
	log       *logger.Logger
	cron      *cron.Cron
	queueName string
}

func NewCronService(
	cache redis.Cache,
	eRepo repository.ExecutionRepository,
	sRepo repository.StepExecutionRepository,
	wRepo repository.WorkflowRepository,
	trx postgres.Trx,
	l *logger.Logger,
) CronService {
	return &cronService{
		cache:     cache,
		eRepo:     eRepo,
		sRepo:     sRepo,
		wRepo:     wRepo,
		trx:       trx,
		log:       l,
		cron:      cron.New(),
		queueName: "flowforge:jobs:queue",
	}
}

func (s *cronService) Start(ctx context.Context) error {
	s.log.Info("starting cron service")
	s.cron.Start()
	return nil
}

func (s *cronService) Stop() error {
	ctx := s.cron.Stop()
	<-ctx.Done()
	return nil
}

func (s *cronService) ScheduleWorkflow(ctx context.Context, workflowID, cronExpr string) error {
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		if err := s.enqueueWorkflowExecution(context.Background(), workflowID); err != nil {
			s.log.Error("failed to enqueue cron workflow execution", zap.Error(err))
		}
	})
	if err != nil {
		s.log.Error("failed to schedule cron job", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to schedule cron job", err)
	}

	lockKey := fmt.Sprintf("cron:lock:%s", workflowID)
	acquired, err := s.cache.SetNX(ctx, lockKey, int(entryID), 24*time.Hour)
	if err != nil {
		s.log.Error("failed to acquire cron lock", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to acquire cron lock", err)
	}
	if !acquired {
		s.log.Error("could not acquire cron lock for workflow", zap.String("workflow_id", workflowID))
		return errorx.NewError(errorx.ErrTypeConflict, fmt.Sprintf("could not acquire cron lock for workflow %s", workflowID), nil)
	}
	defer s.cron.Remove(entryID)

	return nil
}

func (s *cronService) UnscheduleWorkflow(ctx context.Context, workflowID string) error {
	lockKey := fmt.Sprintf("cron:lock:%s", workflowID)

	val, err := s.cache.Get(ctx, lockKey)
	if err != nil {
		s.log.Error("failed to get cron lock", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to get cron lock", err)
	}
	if val == "" {
		s.log.Error("cron lock is empty")
		return errorx.NewError(errorx.ErrTypeUnauthorized, "token invalid", errors.New("cron lock is empty"))
	}

	var entryID int
	if err := json.Unmarshal([]byte(val), &entryID); err != nil {
		s.log.Error("failed to unmarshal cache value", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "internal server error", err)
	}

	s.cron.Remove(cron.EntryID(entryID))
	if err := s.cache.Delete(ctx, lockKey); err != nil {
		s.log.Error("failed to delete cron lock", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to delete cron lock", err)
	}

	return nil
}

func (s *cronService) enqueueWorkflowExecution(ctx context.Context, workflowID string) error {
	lockKey := fmt.Sprintf("cron:execution:%s", workflowID)

	acquired, err := s.cache.SetNX(ctx, lockKey, "1", 10*time.Second)
	if err != nil {
		s.log.Error("failed to acquire execution lock", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to acquire execution lock", err)
	}
	if !acquired {
		s.log.Warn("could not acquire execution lock for workflow", zap.String("workflow_id", workflowID))
		return nil
	}
	defer s.cache.Delete(ctx, lockKey)

	var wf model.Workflow
	err = s.wRepo.Get(ctx, map[string]any{
		"id": workflowID,
	}, &wf)
	if err != nil {
		s.log.Error("failed to get workflow", zap.Error(err))
		return err
	}

	now := time.Now()
	execution := &model.Execution{
		ID:          uuid.New().String(),
		TenantID:    wf.TenantID,
		WorkflowID:  wf.ID,
		Status:      string(model.StatusExecutionPending),
		TriggerType: "CRON",
		Version:     wf.Version,
		CreatedAt:   now,
	}

	err = s.eRepo.Create(ctx, execution)
	if err != nil {
		s.log.Error("failed to create execution", zap.Error(err))
		return err
	}

	var steps []model.StepDefinition
	if err := wf.GetSteps(&steps); err != nil {
		s.log.Error("failed to parse workflow steps", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to parse workflow steps", err)
	}

	for _, stepDef := range steps {
		stepRec := &model.StepExecution{
			ID:          uuid.New().String(),
			TenantID:    wf.TenantID,
			ExecutionID: execution.ID,
			StepID:      stepDef.ID,
			Action:      stepDef.Action,
			Status:      string(model.StatusExecutionPending),
			Version:     1,
			RetryCount:  0,
		}

		err = s.sRepo.Create(ctx, stepRec)
		if err != nil {
			s.log.Error("failed to create step execution", zap.Error(err))
			return err
		}
	}

	err = s.cache.RPush(ctx, s.queueName, execution.ID)
	if err != nil {
		s.log.Error("failed to enqueue job", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to enqueue job", err)
	}

	return nil
}
