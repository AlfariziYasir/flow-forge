package handler

import (
	"context"
	"time"

	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/internal/worker/service"
	"flowforge/pkg/logger"
	"flowforge/pkg/postgres"
	"go.uber.org/zap"
)

type JobPoller struct {
	execRepo   repository.ExecutionRepository
	workRepo   repository.WorkflowRepository
	uow        postgres.Trx
	engine     service.ExecutionEngine
	l          *logger.Logger
	pollPeriod time.Duration
}

func NewJobPoller(
	execRepo repository.ExecutionRepository,
	workRepo repository.WorkflowRepository,
	uow postgres.Trx,
	engine service.ExecutionEngine,
	l *logger.Logger,
	pollPeriod time.Duration,
) *JobPoller {
	return &JobPoller{
		execRepo:   execRepo,
		workRepo:   workRepo,
		uow:        uow,
		engine:     engine,
		l:          l,
		pollPeriod: pollPeriod,
	}
}

func (p *JobPoller) Start(ctx context.Context) {
	p.l.Info("starting job poller", zap.Duration("period", p.pollPeriod))
	ticker := time.NewTicker(p.pollPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.l.Info("stopping job poller")
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *JobPoller) poll(ctx context.Context) {
	txCtx, err := p.uow.Begin(ctx)
	if err != nil {
		p.l.Error("failed to begin transaction for polling", zap.Error(err))
		return
	}
	defer p.uow.Rollback(txCtx) // automatically ignored if committed

	// Acquire up to 5 executions per tick globally (tenant="" skips where tenant_id)
	executions, err := p.execRepo.AcquireForWorker(txCtx, 5)
	if err != nil {
		p.l.Error("failed to acquire executions", zap.Error(err))
		return
	}

	if len(executions) == 0 {
		return // nothing to do
	}

	// Update statuses to RUNNING to lock them out completely from other potential txs
	// though SKIP LOCKED already does that locally
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

	if err := p.uow.Commit(txCtx); err != nil {
		p.l.Error("failed to commit acquired executions", zap.Error(err))
		return
	}

	// Now that they are committed as RUNNING, we can spawn goroutines to execute them
	for _, exe := range executions {
		// Fetch full workflow
		var workflow model.Workflow
		err = p.workRepo.Get(ctx, map[string]any{"id": exe.WorkflowID}, &workflow)
		if err != nil {
			p.l.Error("failed to fetch workflow for execution", zap.Error(err), zap.String("workflow_id", exe.WorkflowID))
			// mark execution as failed
			p.execRepo.Update(ctx, exe.ID, exe.Version+1, map[string]any{
				"status": "FAILED",
			})
			continue
		}

		// Execute asynchronously
		go p.engine.RunExecution(context.Background(), exe, &workflow)
	}
}
