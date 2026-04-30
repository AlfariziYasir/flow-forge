package handler

import (
	"context"
	"time"

	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/internal/worker/service"
	"flowforge/pkg/logger"
	"flowforge/pkg/postgres"
	"flowforge/pkg/redis"

	"go.uber.org/zap"
)

type JobPoller struct {
	execRepo   repository.ExecutionRepository
	workRepo   repository.WorkflowRepository
	trx        postgres.Trx
	cache      redis.Cache
	engine     service.ExecutionEngine
	l          *logger.Logger
	pollPeriod time.Duration
}

func NewJobPoller(
	execRepo repository.ExecutionRepository,
	workRepo repository.WorkflowRepository,
	trx postgres.Trx,
	cache redis.Cache,
	engine service.ExecutionEngine,
	l *logger.Logger,
	pollPeriod time.Duration,
) *JobPoller {
	return &JobPoller{
		execRepo:   execRepo,
		workRepo:   workRepo,
		trx:        trx,
		cache:      cache,
		engine:     engine,
		l:          l,
		pollPeriod: pollPeriod,
	}
}

func (p *JobPoller) Start(ctx context.Context) {
	p.l.Info("starting job poller", zap.Duration("period", p.pollPeriod))

	go func() {
		ticker := time.NewTicker(p.pollPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.poll(ctx)
			}
		}
	}()

	go func() {
		// Recover stuck jobs every 5 minutes
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.recoverStuckJobs(ctx)
			}
		}
	}()

	if p.cache != nil {
		go p.redisLoop(ctx)
	}

	<-ctx.Done()
	p.l.Info("stopping job poller")
}

func (p *JobPoller) redisLoop(ctx context.Context) {
	p.l.Info("starting redis job poller")
	queueKey := "flowforge:jobs:queue"
	for {
		select {
		case <-ctx.Done():
			return
		default:
			res, err := p.cache.BLPop(ctx, 5*time.Second, queueKey)
			if err != nil {
				if err.Error() == "redis: nil" {
					continue
				}
				p.l.Warn("redis blpop error", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			if len(res) > 1 {
				executionID := res[1]
				p.l.Debug("job received from redis", zap.String("execution_id", executionID))
				p.processExecutionByID(ctx, executionID)
			}
		}
	}
}

func (p *JobPoller) processExecutionByID(ctx context.Context, id string) {
	txCtx, err := p.trx.Begin(ctx)
	if err != nil {
		p.l.Error("failed to begin transaction for redis job", zap.Error(err))
		return
	}
	defer p.trx.Rollback(txCtx)

	exe, err := p.execRepo.AcquireByIDForWorker(txCtx, id)
	if err != nil {
		p.l.Debug("failed to acquire execution for redis job (already claimed or not pending)", zap.String("id", id), zap.Error(err))
		return
	}

	err = p.execRepo.Update(txCtx, exe.ID, exe.Version, map[string]any{
		"status": string(model.StatusExecutionRunning),
	})
	if err != nil {
		p.l.Error("failed to update execution to RUNNING for redis job", zap.Error(err))
		return
	}
	exe.Version++

	if err := p.trx.Commit(txCtx); err != nil {
		p.l.Error("failed to commit redis job acquisition", zap.Error(err))
		return
	}

	p.executeOne(ctx, exe)
}

func (p *JobPoller) executeOne(ctx context.Context, exe *model.Execution) {
	var workflow model.Workflow
	err := p.workRepo.Get(ctx, map[string]any{"id": exe.WorkflowID}, &workflow)
	if err != nil {
		p.l.Error("failed to fetch workflow for execution", zap.Error(err), zap.String("workflow_id", exe.WorkflowID))
		if err := p.execRepo.Update(ctx, exe.ID, exe.Version, map[string]any{
			"status": "FAILED",
		}); err != nil {
			p.l.Error("failed to mark execution as failed", zap.Error(err))
		}
		return
	}

	go p.engine.RunExecution(ctx, exe, &workflow)
}

func (p *JobPoller) poll(ctx context.Context) {
	txCtx, err := p.trx.Begin(ctx)
	if err != nil {
		p.l.Error("failed to begin transaction for polling", zap.Error(err))
		return
	}
	defer p.trx.Rollback(txCtx)

	executions, err := p.execRepo.AcquireForWorker(txCtx, 5)
	if err != nil {
		p.l.Error("failed to acquire executions", zap.Error(err))
		return
	}

	if len(executions) == 0 {
		return
	}

	for _, exe := range executions {
		err = p.execRepo.Update(txCtx, exe.ID, exe.Version, map[string]any{
			"status": "RUNNING",
		})
		if err != nil {
			p.l.Error("failed to update execution to RUNNING in tx", zap.Error(err))
			continue
		}
		exe.Version++
	}

	if err := p.trx.Commit(txCtx); err != nil {
		p.l.Error("failed to commit acquired executions", zap.Error(err))
		return
	}

for _, exe := range executions {

		var workflow model.Workflow
		err = p.workRepo.Get(ctx, map[string]any{"id": exe.WorkflowID}, &workflow)
		if err != nil {
			p.l.Error("failed to fetch workflow for execution", zap.Error(err), zap.String("workflow_id", exe.WorkflowID))

			if err := p.execRepo.Update(ctx, exe.ID, exe.Version, map[string]any{
				"status": "FAILED",
			}); err != nil {
				p.l.Error("failed to mark execution as failed", zap.Error(err))
			}
			continue
		}

		go p.engine.RunExecution(ctx, exe, &workflow)
	}
}

func (p *JobPoller) recoverStuckJobs(ctx context.Context) {
	count, err := p.execRepo.RecoverStuckJobs(ctx, 15*time.Minute)
	if err != nil {
		p.l.Error("failed to recover stuck jobs", zap.Error(err))
		return
	}
	if count > 0 {
		p.l.Info("recovered stuck jobs", zap.Int64("count", count))
	}
}
