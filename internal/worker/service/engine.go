package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/dag"
	"flowforge/pkg/logger"
	"flowforge/pkg/postgres"

	"go.uber.org/zap"
)

type Broadcaster interface {
	BroadcastToRedis(ctx context.Context, tenantID string, event any) error
}

type ExecutionEngine interface {
	RunExecution(ctx context.Context, execution *model.Execution, workflow *model.Workflow)
}

type engine struct {
	execRepo  repository.ExecutionRepository
	stepRepo  repository.StepExecutionRepository
	uow       postgres.Trx
	l         *logger.Logger
	registry  *Registry
	broadcast Broadcaster
}

func NewExecutionEngine(
	execRepo repository.ExecutionRepository,
	stepRepo repository.StepExecutionRepository,
	uow postgres.Trx,
	l *logger.Logger,
	broadcast Broadcaster,
) *engine {
	r := NewRegistry()
	// Register default actions
	r.Registry("HTTP", NewHTTPAction())
	r.Registry("WAIT", NewWaitAction())
	r.Registry("TRANSFORM", NewTransformAction())
	r.Registry("SCRIPT", &ScriptAction{})

	return &engine{
		execRepo:  execRepo,
		stepRepo:  stepRepo,
		uow:       uow,
		l:         l,
		registry:  r,
		broadcast: broadcast,
	}
}

func (e *engine) RunExecution(ctx context.Context, execution *model.Execution, workflow *model.Workflow) {
	e.l.Info("starting execution", zap.String("execution_id", execution.ID), zap.String("workflow_id", workflow.ID))

	// Parse DAG
	var steps []model.StepDefinition
	if err := workflow.GetSteps(&steps); err != nil {
		e.l.Error("failed to parse workflow steps", zap.Error(err))
		e.markExecutionFailed(ctx, execution, fmt.Errorf("invalid workflow steps: %v", err))
		return
	}

	execPlan, err := dag.BuildExecutionPlan(steps)
	if err != nil {
		e.l.Error("failed to build execution plan", zap.Error(err))
		e.markExecutionFailed(ctx, execution, fmt.Errorf("invalid DAG: %v", err))
		return
	}

	state := newState()
	skipMap := &sync.Map{}

	for _, layer := range execPlan {
		var wg sync.WaitGroup
		errs := make(chan error, len(layer))
		for _, stepDef := range layer {
			wg.Add(1)
			go func(def model.StepDefinition) {
				defer wg.Done()

				// Check dependencies skipped
				for _, dep := range def.DependsOn {
					if _, skipped := skipMap.Load(dep); skipped {
						skipMap.Store(def.ID, true)
						e.l.Info("skipping step because dependency was skipped", zap.String("step_id", def.ID), zap.String("dep_id", dep))
						return
					}
				}

				err := e.executeStepWithRetry(ctx, execution, def, state, skipMap)
				if err != nil {
					errs <- err
				}
			}(stepDef)
		}
		wg.Wait()
		close(errs)

		for err := range errs {
			if err != nil {
				e.l.Error("execution stopped due to step failure", zap.Error(err))
				e.markExecutionFailed(ctx, execution, err)
				return
			}
		}
	}

	// Update execution to SUCCESS
	err = e.execRepo.Update(ctx, execution.ID, execution.Version+1, map[string]any{
		"status":       string(model.StatusExecutionSuccess),
		"completed_at": time.Now(),
	})
	if err != nil {
		e.l.Error("failed to mark execution as success", zap.Error(err))
	}

	e.l.Info("execution completed successfully", zap.String("execution_id", execution.ID))
}

func (e *engine) executeStepWithRetry(
	ctx context.Context,
	execution *model.Execution,
	def model.StepDefinition,
	state *State,
	skipMap *sync.Map,
) error {
	maxRetries := def.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	} else {
		maxRetries += 1
	}

	for i := 0; i < maxRetries; i++ {
		updateMap := map[string]any{"status": string(model.StatusExecutionRunning)}
		if i == 0 {
			updateMap["started_at"] = time.Now()
		}

		err := e.execRepo.Update(ctx, execution.ID, execution.Version, updateMap)
		if err == nil {
			execution.Version++
		}

		// Broadcast step running
		e.broadcast.BroadcastToRedis(ctx, execution.TenantID, map[string]any{
			"execution_id": execution.ID,
			"step_id":      def.ID,
			"status":       "RUNNING",
		})

		// Prepare params with interpolation
		params := state.Resolve(def.Parameters)
		action, err := e.registry.Get(def.Action)
		if err != nil {
			return err
		}

		result, err := action.Execute(ctx, params)
		if err != nil {
			e.l.Warn("step execution failed", zap.String("step_id", def.ID), zap.Int("attempt", i+1), zap.Error(err))
			if i < maxRetries-1 {
				backoff := time.Duration(1<<uint(i)) * time.Second
				time.Sleep(backoff)
				continue
			}

			// Broadcast step failure
			e.broadcast.BroadcastToRedis(ctx, execution.TenantID, map[string]any{
				"execution_id": execution.ID,
				"step_id":      def.ID,
				"status":       "FAILED",
				"error":        err.Error(),
			})
			return fmt.Errorf("step %s failed after %d attempts: %v", def.ID, maxRetries, err)
		}

		// Success
		state.Set(def.ID, result)

		// Check skip condition
		if cond, ok := def.Parameters["condition"].(string); ok && cond != "" {
			// Basic condition evaluation (placeholder for now)
			if result["condition_met"] == false {
				skipMap.Store(def.ID, true)
			}
		}

		// Update step status to SUCCESS
		err = e.sRepoUpdate(ctx, execution.ID, def.ID, map[string]any{
			"status":       string(model.StatusExecutionSuccess),
			"completed_at": time.Now(),
			"output":       result,
		})

		// Broadcast step success
		e.broadcast.BroadcastToRedis(ctx, execution.TenantID, map[string]any{
			"execution_id": execution.ID,
			"step_id":      def.ID,
			"status":       "SUCCESS",
			"output":       result,
		})

		return err
	}

	return nil
}

func (e *engine) sRepoUpdate(ctx context.Context, execID, stepID string, data map[string]any) error {
	steps, err := e.stepRepo.ListByExecution(ctx, execID)
	if err != nil {
		return err
	}
	for _, s := range steps {
		if s.StepID == stepID {
			return e.stepRepo.Update(ctx, s.ID, data)
		}
	}
	return fmt.Errorf("step execution not found for step %s", stepID)
}

func (e *engine) markExecutionFailed(ctx context.Context, execution *model.Execution, err error) {
	e.execRepo.Update(ctx, execution.ID, execution.Version+1, map[string]any{
		"status":        string(model.StatusExecutionFailed),
		"error_message": err.Error(),
		"completed_at":  time.Now(),
	})
}
