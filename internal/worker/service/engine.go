package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/dag"
	"flowforge/pkg/errorx"
	"flowforge/pkg/jwt"
	"flowforge/pkg/logger"
	"flowforge/pkg/postgres"

	"go.uber.org/zap"
)

type Broadcaster interface {
	BroadcastToRedis(ctx context.Context, tenantID string, event any) error
}

type ExecutionEngine interface {
	RunExecution(ctx context.Context, execution *model.Execution, workflow *model.Workflow)
	ResumeExecution(ctx context.Context, executionID string, stepID string, payload map[string]any) error
}

type engine struct {
	execRepo    repository.ExecutionRepository
	stepRepo    repository.StepExecutionRepository
	trx         postgres.Trx
	l           *logger.Logger
	registry    *Registry
	broadcast   Broadcaster
	stepTimeout time.Duration
	version     int
	versionMu   sync.Mutex
}

func NewExecutionEngine(
	execRepo repository.ExecutionRepository,
	stepRepo repository.StepExecutionRepository,
	trx postgres.Trx,
	l *logger.Logger,
	broadcast Broadcaster,
	stepTimeout time.Duration,
) *engine {
	r := NewRegistry()
	r.Registry("HTTP", NewHTTPAction())
	r.Registry("WAIT", NewWaitAction())
	r.Registry("TRANSFORM", NewTransformAction())
	r.Registry("SWITCH", &ConditionAction{})
	r.Registry("WAIT_FOR_EVENT", NewWaitForEventAction())
	scriptAction, err := NewScriptAction()
	if err != nil {
		l.Warn("SCRIPT action unavailable (Docker not accessible)", zap.Error(err))
	} else {
		r.Registry("SCRIPT", scriptAction)
	}

	if stepTimeout <= 0 {
		stepTimeout = 5 * time.Minute
	}

	return &engine{
		execRepo:    execRepo,
		stepRepo:    stepRepo,
		trx:         trx,
		l:           l,
		registry:    r,
		broadcast:   broadcast,
		stepTimeout: stepTimeout,
	}
}

func (e *engine) nextVersion() int {
	e.versionMu.Lock()
	defer e.versionMu.Unlock()
	e.version++
	return e.version
}

func (e *engine) RunExecution(ctx context.Context, execution *model.Execution, workflow *model.Workflow) {
	e.version = execution.Version
	ctx = jwt.SetContext(ctx, jwt.TenantKey, execution.TenantID)

	e.l.Info("starting execution", zap.String("execution_id", execution.ID), zap.String("workflow_id", workflow.ID), zap.String("tenant_id", execution.TenantID))

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

	stepsDb, err := e.stepRepo.ListByExecution(ctx, execution.ID)
	if err == nil {
		for _, s := range stepsDb {
			if s.Status == string(model.StatusExecutionSuccess) && s.Output != nil {
				var output map[string]any
				if err := json.Unmarshal(s.Output, &output); err == nil {
					state.Set(s.StepID, output)
				}
			}
		}
	}

	for _, layer := range execPlan {
		var wg sync.WaitGroup
		errs := make(chan error, len(layer))

		layerCtx, cancelLayer := context.WithCancel(ctx)

		for _, stepDef := range layer {
			wg.Add(1)
			go func(def model.StepDefinition) {
				defer wg.Done()

				for _, dep := range def.DependsOn {
					if _, skipped := skipMap.Load(dep); skipped {
						skipMap.Store(def.ID, true)
						e.l.Info("skipping step because dependency was skipped", zap.String("step_id", def.ID), zap.String("dep_id", dep))
						e.broadcastStepStatus(execution.TenantID, def.ID, "SKIPPED", nil)
						return
					}
				}

				stepState := state.Copy()
				err := e.executeStepWithRetry(layerCtx, execution, def, stepState, skipMap)
				if err != nil {
					errs <- err
					cancelLayer()
				}
			}(stepDef)
		}
		wg.Wait()
		cancelLayer()
		close(errs)

		var suspended bool
		for err := range errs {
			if err != nil {
				if errors.Is(err, errorx.ErrSuspendExecution) {
					e.l.Info("execution suspended", zap.String("execution_id", execution.ID))
					ver := e.nextVersion()
					e.execRepo.Update(ctx, execution.ID, ver, map[string]any{
						"status":     string(model.StatusExecutionSuspended),
						"updated_at": time.Now(),
					})
					suspended = true
				} else {
					e.l.Error("execution stopped due to step failure", zap.Error(err))

					e.runCompensation(ctx, execution, steps, state)

					e.markExecutionFailed(ctx, execution, err)
					return
				}
			}
		}
		if suspended {
			return
		}
	}

	ver := e.nextVersion()
	err = e.execRepo.Update(ctx, execution.ID, ver, map[string]any{
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
		stepExec, err := e.stepRepo.GetByExecutionAndStep(ctx, execution.ID, def.ID)
		if err != nil {
			e.markExecutionFailed(ctx, execution, err)
			return fmt.Errorf("failed to get step execution: %w", err)
		}

		if stepExec.Status == string(model.StatusExecutionSuccess) {
			return nil
		}

		updateMap := map[string]any{"status": string(model.StatusExecutionRunning)}
		if i == 0 {
			updateMap["started_at"] = time.Now()
		}

		err = e.stepRepo.Update(ctx, stepExec.ID, stepExec.Version, updateMap)
		if err != nil {
			e.l.Warn(
				"optimistic lock failed: aborting step execution to prevent duplication",
				zap.String("execution_id", execution.ID),
				zap.String("step_id", def.ID),
				zap.Error(err),
			)
			return err
		}

		e.nextVersion()

		go e.broadcast.BroadcastToRedis(ctx, execution.TenantID, map[string]any{
			"execution_id": execution.ID,
			"step_id":      def.ID,
			"status":       "RUNNING",
		})

		params := state.Resolve(def.Parameters)
		params["_execution_id"] = execution.ID
		params["_step_id"] = def.ID

		var result map[string]any

		if def.Action == "LOOP" {
			itemsRaw, ok := params["items"]
			if !ok {
				err = fmt.Errorf("missing 'items' parameter for LOOP action")
				e.markExecutionFailed(ctx, execution, err)
				return err
			}

			items, ok := itemsRaw.([]any)
			if !ok {
				err = fmt.Errorf("'items' parameter must be an array")
				e.markExecutionFailed(ctx, execution, err)
				return err
			}

			subActionName, _ := def.Parameters["action"].(string)
			subActionParams, _ := def.Parameters["parameters"].(map[string]any)

			subAction, err := e.registry.Get(subActionName)
			if err != nil {
				e.markExecutionFailed(ctx, execution, err)
				return err
			}

			var results []any
			for idx, item := range items {
				localState := state.Copy()
				localState.Set("item", item)
				localState.Set("index", idx)

				localParams := localState.Resolve(subActionParams)
				localParams["_execution_id"] = execution.ID
				localParams["_step_id"] = fmt.Sprintf("%s_%d", def.ID, idx)

				stepCtx, cancel := context.WithTimeout(ctx, e.stepTimeout)
				subResult, subErr := subAction.Execute(stepCtx, localParams)
				cancel()

				if subErr != nil {
					err = fmt.Errorf("loop iteration %d failed: %w", idx, subErr)
					break
				}
				results = append(results, subResult)
			}

			if err == nil {
				result = map[string]any{"results": results}
			}
		} else {
			action, actionErr := e.registry.Get(def.Action)
			if actionErr != nil {
				e.markExecutionFailed(ctx, execution, actionErr)
				return actionErr
			}

			stepCtx, cancel := context.WithTimeout(ctx, e.stepTimeout)
			result, err = action.Execute(stepCtx, params)
			cancel()
		}

		if err != nil {
			if errors.Is(err, errorx.ErrSuspendExecution) {
				if err := e.stepRepo.Update(ctx, stepExec.ID, stepExec.Version, map[string]any{
					"status": string(model.StatusExecutionSuspended),
					"output": result,
				}); err != nil {
					e.l.Error("failed to update step to SUSPENDED", zap.Error(err))
				}
				go e.broadcast.BroadcastToRedis(ctx, execution.TenantID, map[string]any{
					"execution_id": execution.ID,
					"step_id":      def.ID,
					"status":       "SUSPENDED",
					"output":       result,
				})
				return err
			}
			e.l.Warn("step execution failed", zap.String("step_id", def.ID), zap.Int("attempt", i+1), zap.Error(err))

			retryErr := e.stepRepo.Update(ctx, stepExec.ID, stepExec.Version, map[string]any{
				"retry_count": stepExec.RetryCount + 1,
				"error_log":   err.Error(),
			})
			if retryErr != nil {
				e.l.Warn("failed to update retry count", zap.Error(retryErr))
			}

			if i < maxRetries-1 {
				backoff := time.Duration(1<<uint(i)) * time.Second
				time.Sleep(backoff)
				continue
			}

			failErr := e.stepRepo.Update(ctx, stepExec.ID, stepExec.Version, map[string]any{
				"status":       string(model.StatusExecutionFailed),
				"completed_at": time.Now(),
				"error_log":    err.Error(),
			})
			if failErr != nil {
				e.l.Error("failed to update step to FAILED", zap.Error(failErr))
			}

			go e.broadcast.BroadcastToRedis(ctx, execution.TenantID, map[string]any{
				"execution_id": execution.ID,
				"step_id":      def.ID,
				"status":       "FAILED",
				"error":        err.Error(),
			})
			return fmt.Errorf("step %s failed after %d attempts: %v", def.ID, maxRetries, err)
		}

		state.Set(def.ID, result)

		if cond, ok := def.Parameters["condition"].(string); ok && cond != "" {
			if result["condition_met"] == false {
				skipMap.Store(def.ID, true)
			}
		}

		err = e.stepRepo.Update(ctx, stepExec.ID, stepExec.Version, map[string]any{
			"status":       string(model.StatusExecutionSuccess),
			"completed_at": time.Now(),
			"output":       result,
		})

		go e.broadcast.BroadcastToRedis(ctx, execution.TenantID, map[string]any{
			"execution_id": execution.ID,
			"step_id":      def.ID,
			"status":       "SUCCESS",
			"output":       result,
		})

		return err
	}

	return nil
}

func (e *engine) ResumeExecution(ctx context.Context, executionID string, stepID string, payload map[string]any) error {
	stepExec, err := e.stepRepo.GetByExecutionAndStep(ctx, executionID, stepID)
	if err != nil {
		return err
	}

	if stepExec.Status != string(model.StatusExecutionSuspended) {
		return fmt.Errorf("step is not suspended")
	}

	err = e.stepRepo.Update(ctx, stepExec.ID, stepExec.Version, map[string]any{
		"status":       string(model.StatusExecutionSuccess),
		"output":       payload,
		"completed_at": time.Now(),
	})
	if err != nil {
		return err
	}

	var exe model.Execution
	err = e.execRepo.Get(ctx, map[string]any{"id": executionID}, &exe)
	if err != nil {
		return err
	}

	err = e.execRepo.Update(ctx, exe.ID, exe.Version, map[string]any{
		"status":     string(model.StatusExecutionPending),
		"updated_at": time.Now(),
	})
	if err != nil {
		return err
	}

	e.l.Info("execution resumed", zap.String("execution_id", executionID))
	return nil
}

func (e *engine) runCompensation(ctx context.Context, execution *model.Execution, steps []model.StepDefinition, state *State) {
	e.l.Info("initiating compensation sequence", zap.String("execution_id", execution.ID))

	stepsDb, err := e.stepRepo.ListByExecution(ctx, execution.ID)
	if err != nil {
		e.l.Error("failed to list steps for compensation", zap.Error(err))
		return
	}

	successSteps := make(map[string]bool)
	for _, s := range stepsDb {
		if s.Status == string(model.StatusExecutionSuccess) {
			successSteps[s.StepID] = true
		}
	}

	// Update execution status to COMPENSATING
	ver := e.nextVersion()
	err = e.execRepo.Update(ctx, execution.ID, ver, map[string]any{
		"status":     string(model.StatusExecutionCompensating),
		"updated_at": time.Now(),
	})

	for _, def := range steps {
		if def.Action == "COMPENSATION" {
			target, _ := def.Parameters["compensates_for"].(string)
			if successSteps[target] {
				// Find the StepExecution record
				var stepExec *model.StepExecution
				for _, s := range stepsDb {
					if s.StepID == def.ID {
						stepExec = s
						break
					}
				}

				if stepExec == nil {
					e.l.Error("step execution record not found for compensation step", zap.String("step_id", def.ID))
					continue
				}

				e.l.Info("running compensation step", zap.String("step_id", def.ID), zap.String("target_step", target))

				// Update to RUNNING
				if err := e.stepRepo.Update(ctx, stepExec.ID, stepExec.Version, map[string]any{
					"status":     string(model.StatusExecutionRunning),
					"started_at": time.Now(),
				}); err != nil {
					e.l.Error("failed to update compensation step to RUNNING", zap.Error(err))
				}
				go e.broadcast.BroadcastToRedis(ctx, execution.TenantID, map[string]any{
					"execution_id": execution.ID,
					"step_id":      def.ID,
					"status":       "RUNNING",
				})

				subActionName, _ := def.Parameters["action"].(string)
				if subActionName == "" {
					continue
				}

				subAction, err := e.registry.Get(subActionName)
				if err != nil {
					e.l.Error("compensation action not found", zap.String("action", subActionName))
					continue
				}

				var subParams map[string]any
				if pRaw, ok := def.Parameters["parameters"]; ok {
					if pMap, ok := pRaw.(map[string]any); ok {
						subParams = state.Resolve(pMap)
					}
				}
				if subParams == nil {
					subParams = make(map[string]any)
				}

				subParams["_execution_id"] = execution.ID
				subParams["_step_id"] = def.ID

				stepCtx, cancel := context.WithTimeout(ctx, e.stepTimeout)
				result, subErr := subAction.Execute(stepCtx, subParams)
				cancel()

				if subErr != nil {
					e.l.Error("compensation step failed", zap.String("step_id", def.ID), zap.Error(subErr))
					if err := e.stepRepo.Update(ctx, stepExec.ID, stepExec.Version, map[string]any{
						"status":       string(model.StatusExecutionFailed),
						"completed_at": time.Now(),
						"error_log":    subErr.Error(),
					}); err != nil {
						e.l.Error("failed to update compensation step to FAILED", zap.Error(err))
					}
					go e.broadcast.BroadcastToRedis(ctx, execution.TenantID, map[string]any{
						"execution_id": execution.ID,
						"step_id":      def.ID,
						"status":       "FAILED",
						"error":        subErr.Error(),
					})
				} else {
					e.l.Info("compensation step successful", zap.String("step_id", def.ID))
					if err := e.stepRepo.Update(ctx, stepExec.ID, stepExec.Version, map[string]any{
						"status":       string(model.StatusExecutionSuccess),
						"completed_at": time.Now(),
						"output":       result,
					}); err != nil {
						e.l.Error("failed to update compensation step to SUCCESS", zap.Error(err))
					}
					go e.broadcast.BroadcastToRedis(ctx, execution.TenantID, map[string]any{
						"execution_id": execution.ID,
						"step_id":      def.ID,
						"status":       "SUCCESS",
						"output":       result,
					})
				}
			}
		}
	}
}

func (e *engine) broadcastStepStatus(tenantID, stepID, status string, output map[string]any) {
	go e.broadcast.BroadcastToRedis(context.Background(), tenantID, map[string]any{
		"execution_id": "",
		"step_id":      stepID,
		"status":       status,
		"output":       output,
	})
}

func (e *engine) markExecutionFailed(ctx context.Context, execution *model.Execution, err error) {
	ver := e.nextVersion()
	e.execRepo.Update(ctx, execution.ID, ver, map[string]any{
		"status":        string(model.StatusExecutionFailed),
		"error_message": err.Error(),
		"completed_at":  time.Now(),
	})
}
