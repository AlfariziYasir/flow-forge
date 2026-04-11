package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"flowforge/internal/broadcaster"
	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/dag"
	"flowforge/pkg/logger"
	"go.uber.org/zap"
)

type ExecutionEngine interface {
	RunExecution(ctx context.Context, execution *model.Execution, workflow *model.Workflow)
}

type engine struct {
	execRepo repository.ExecutionRepository
	stepRepo repository.StepExecutionRepository
	l        *logger.Logger
}

func NewExecutionEngine(execRepo repository.ExecutionRepository, stepRepo repository.StepExecutionRepository, l *logger.Logger) ExecutionEngine {
	return &engine{
		execRepo: execRepo,
		stepRepo: stepRepo,
		l:        l,
	}
}

func (e *engine) RunExecution(ctx context.Context, execution *model.Execution, workflow *model.Workflow) {
	// Parse the DAG definition from workflow
	var steps []model.StepDefinition
	err := json.Unmarshal(workflow.DAGDefinition, &steps)
	if err != nil {
		e.failExecution(ctx, execution, fmt.Sprintf("invalid DAG: %v", err))
		return
	}

	executionPlan, err := dag.BuildExecutionPlan(steps)
	if err != nil {
		e.failExecution(ctx, execution, fmt.Sprintf("DAG validation failed: %v", err))
		return
	}

	// Make sure all steps exist in step_executions table as PENDING
	// (usually done during trigger, but we ensure it here for MVP simplicity if not created)
	// We will skip that out of band and assume they are created in the trigger or we create them now.
	// Actually for simplicity let's just create them here if they don't exist.
	existingSteps, _ := e.stepRepo.ListByExecution(ctx, execution.ID)
	stepStatusMap := make(map[string]*model.StepExecution)
	for _, s := range existingSteps {
		stepStatusMap[s.StepID] = s
	}

	// Update execution status to RUNNING
	err = e.execRepo.Update(ctx, execution.ID, execution.Version, map[string]any{
		"status":     "RUNNING",
		"started_at": time.Now(),
	})
	if err != nil {
		e.l.Error("failed to update execution status to running", zap.Error(err))
		return
	}
	execution.Version++ // locally increment
	execution.Status = "RUNNING"
	broadcastExecutionStatus(execution)

	for _, layer := range executionPlan {
		var wg sync.WaitGroup
		errs := make(chan error, len(layer))

		for _, stepDef := range layer {
			wg.Add(1)
			go func(def model.StepDefinition) {
				defer wg.Done()

				e.executeStepWithRetry(ctx, execution, def, stepStatusMap[def.ID], errs)
			}(stepDef)
		}

		wg.Wait()
		close(errs)

		var layerFailed bool
		for err := range errs {
			if err != nil {
				layerFailed = true
				break
			}
		}

		if layerFailed {
			e.failExecution(ctx, execution, "step execution failed in layer")
			return
		}
	}

	// Success
	e.execRepo.Update(ctx, execution.ID, execution.Version, map[string]any{
		"status":       "SUCCESS",
		"completed_at": time.Now(),
	})
	execution.Status = "SUCCESS"
	broadcastExecutionStatus(execution)
}

func (e *engine) executeStepWithRetry(ctx context.Context, execution *model.Execution, stepDef model.StepDefinition, stepRec *model.StepExecution, errs chan<- error) {

	// Create step record if doesn't exist (simplification for MVP if trigger didn't do it)
	// Or we just update
	if stepRec == nil {
		// dummy id for step model pk if it wasn't saved yet
		e.l.Info("step record not found, assuming dynamic execution")
	}

	maxRetries := stepDef.MaxRetries
	if maxRetries == 0 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		e.updateStepStatus(ctx, stepRec.ID, "RUNNING", "")

		// Simulated execution based on action type
		duration := time.Millisecond * 500
		var stepErr error

		switch stepDef.Action {
		case "HTTP_CALL":
			e.l.Info("executing HTTP_CALL", zap.String("step", stepDef.ID))
			time.Sleep(duration)
		case "WAIT":
			e.l.Info("executing WAIT", zap.String("step", stepDef.ID))
			time.Sleep(duration * 2)
		case "SCRIPT":
			e.l.Info("executing SCRIPT", zap.String("step", stepDef.ID))
			time.Sleep(duration)
		default:
			stepErr = fmt.Errorf("unknown action: %s", stepDef.Action)
		}

		if stepErr == nil {
			e.updateStepStatus(ctx, stepRec.ID, "SUCCESS", "")
			errs <- nil
			return
		}

		lastErr = stepErr
		e.l.Error("step failed, retrying", zap.Error(stepErr), zap.Int("attempt", attempt))
		time.Sleep(time.Millisecond * 100 * time.Duration(attempt+1)) // exp backoff
	}

	e.updateStepStatus(ctx, stepRec.ID, "FAILED", lastErr.Error())
	errs <- lastErr
}

func (e *engine) updateStepStatus(ctx context.Context, id, status, errorLog string) {
	if id == "" {
		return
	} // safety
	e.stepRepo.UpdateStatus(ctx, id, status, errorLog)

	// broadcast step update
	broadcaster.Get().Broadcast(broadcaster.Event{
		Type: "STEP_UPDATE",
		Payload: map[string]string{
			"id":     id,
			"status": status,
		},
	})
}

func (e *engine) failExecution(ctx context.Context, execution *model.Execution, errLog string) {
	e.l.Error("execution failed", zap.String("errLog", errLog))
	e.execRepo.Update(ctx, execution.ID, execution.Version, map[string]any{
		"status":       "FAILED",
		"completed_at": time.Now(),
	})
	execution.Status = "FAILED"
	broadcastExecutionStatus(execution)
}

func broadcastExecutionStatus(exe *model.Execution) {
	broadcaster.Get().Broadcast(broadcaster.Event{
		Type: "EXECUTION_UPDATE",
		Payload: map[string]string{
			"id":     exe.ID,
			"status": exe.Status,
		},
	})
}
