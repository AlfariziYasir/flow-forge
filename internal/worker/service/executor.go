package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"flowforge/internal/broadcaster"
	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/dag"
	"flowforge/pkg/logger"
	"flowforge/pkg/postgres"

	"go.uber.org/zap"
)

type ExecutionEngine interface {
	RunExecution(ctx context.Context, execution *model.Execution, workflow *model.Workflow)
}

type engine struct {
	eRepo repository.ExecutionRepository
	sRepo repository.StepExecutionRepository
	trx   postgres.Trx
	log   *logger.Logger
}

func NewExecutionEngine(eRepo repository.ExecutionRepository, sRepo repository.StepExecutionRepository, trx postgres.Trx, log *logger.Logger) ExecutionEngine {
	return &engine{
		eRepo: eRepo,
		sRepo: sRepo,
		trx:   trx,
		log:   log,
	}
}

func (e *engine) RunExecution(ctx context.Context, execution *model.Execution, workflow *model.Workflow) {
	// Global 5 minutes timeout for the entire DAG
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

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

	existingSteps, _ := e.sRepo.ListByExecution(ctx, execution.ID)
	stepStatusMap := make(map[string]*model.StepExecution)
	for _, s := range existingSteps {
		stepStatusMap[s.StepID] = s
	}

	err = e.eRepo.Update(ctx, execution.ID, execution.Version, map[string]any{
		"started_at": time.Now(),
	})
	if err != nil {
		e.log.Error("failed to update execution started_at", zap.Error(err))
	} else {
		execution.Version++
	}
	execution.Status = model.StatusExecutionRunning
	broadcastExecutionStatus(execution)

	for _, layer := range executionPlan {
		var wg sync.WaitGroup
		errs := make(chan error, len(layer))

		for _, stepDef := range layer {
			wg.Add(1)
			go func(def model.StepDefinition) {
				defer wg.Done()

				e.executeStepWithRetry(ctx, def, stepStatusMap[def.ID], errs)
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
	err = e.eRepo.Update(ctx, execution.ID, execution.Version, map[string]any{
		"status":       model.StatusExecutionSuccess,
		"completed_at": time.Now(),
	})
	if err != nil {
		e.log.Error("failed to update execution status to success", zap.Error(err))
		return
	}
	execution.Status = model.StatusExecutionSuccess
	broadcastExecutionStatus(execution)
}

func (e *engine) executeStepWithRetry(ctx context.Context, stepDef model.StepDefinition, stepRec *model.StepExecution, errs chan<- error) {
	if stepRec == nil {
		e.log.Info("step record not found, assuming dynamic execution")
		stepRec = &model.StepExecution{ID: ""} // Dummy so we don't nil panic
	}

	maxRetries := stepDef.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	} else {
		maxRetries += 1 // attempt + retries
	}

	var lastErr error
	var errLog string
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Update status to running with started_at
		updateMap := map[string]any{"status": model.StatusExecutionRunning}
		if attempt == 0 {
			updateMap["started_at"] = time.Now()
		}

		if err := e.updateStepState(ctx, stepRec.ID, updateMap); err != nil {
			e.log.Error("failed to update step status to running", zap.Error(err))
			errs <- err
			return
		}

		stepErr, output := e.executeAction(ctx, stepDef)

		if stepErr == nil {
			if err := e.updateStepState(ctx, stepRec.ID, map[string]any{
				"status":       model.StatusExecutionSuccess,
				"completed_at": time.Now(),
				"error_log":    output,
				"retry_count":  attempt,
			}); err != nil {
				e.log.Error("failed to update step status to success", zap.Error(err))
				errs <- err
				return
			}
			errs <- nil
			return
		}

		lastErr = stepErr
		errLog = fmt.Sprintf("Error: %v\nOutput: %s", stepErr, output)
		e.log.Error("step failed, retrying", zap.Error(stepErr), zap.Int("attempt", attempt))
		
		if attempt < maxRetries-1 {
			// Exponential backoff
			backoff := time.Second * time.Duration(1<<attempt)
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			
			select {
			case <-ctx.Done():
				lastErr = ctx.Err()
				errLog = "timeout or cancelled"
				goto Fail
			case <-time.After(backoff):
			}
		}
	}

Fail:
	if err := e.updateStepState(ctx, stepRec.ID, map[string]any{
		"status":       model.StatusExecutionFailed,
		"completed_at": time.Now(),
		"error_log":    errLog,
		"retry_count":  maxRetries - 1,
	}); err != nil {
		e.log.Error("failed to update step status to failed", zap.Error(err))
	}
	errs <- lastErr
}

func (e *engine) executeAction(ctx context.Context, stepDef model.StepDefinition) (error, string) {
	switch stepDef.Action {
	case "HTTP_CALL":
		return e.executeHTTPCall(ctx, stepDef.Parameters)
	case "SCRIPT":
		return e.executeScript(ctx, stepDef.Parameters)
	case "WAIT":
		return e.executeWait(ctx, stepDef.Parameters)
	case "CONDITIONAL":
		cond, ok := stepDef.Parameters["condition"].(bool)
		if ok && cond {
			return nil, "condition met"
		} else if !ok {
			return fmt.Errorf("condition parameter missing or not a boolean"), ""
		}
		return fmt.Errorf("condition failed"), "condition evaluated to false"
	default:
		return fmt.Errorf("unknown action: %s", stepDef.Action), ""
	}
}

func (e *engine) executeHTTPCall(ctx context.Context, params map[string]any) (error, string) {
	url, _ := params["url"].(string)
	if url == "" {
		return fmt.Errorf("HTTP_CALL missing 'url' parameter"), ""
	}

	method, _ := params["method"].(string)
	if method == "" {
		method = "GET"
	}

	var reqBody io.Reader
	if body, ok := params["body"]; ok {
		switch b := body.(type) {
		case string:
			reqBody = bytes.NewBufferString(b)
		default:
			jsonBytes, _ := json.Marshal(body)
			reqBody = bytes.NewBuffer(jsonBytes)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return err, ""
	}

	if headers, ok := params["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}
	
	if reqBody != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err, ""
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status), string(respBody)
	}

	return nil, string(respBody)
}

func (e *engine) executeScript(ctx context.Context, params map[string]any) (error, string) {
	script, _ := params["script"].(string)
	if script == "" {
		return fmt.Errorf("SCRIPT missing 'script' parameter"), ""
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return err, out.String()
	}

	return nil, out.String()
}

func (e *engine) executeWait(ctx context.Context, params map[string]any) (error, string) {
	durationMs, _ := params["duration_ms"].(float64)
	if durationMs <= 0 {
		durationMs = 1000
	}

	waitDur := time.Duration(durationMs) * time.Millisecond
	
	select {
	case <-ctx.Done():
		return ctx.Err(), "wait cancelled"
	case <-time.After(waitDur):
		return nil, fmt.Sprintf("waited for %v", waitDur)
	}
}

func (e *engine) updateStepState(ctx context.Context, id string, data map[string]any) error {
	if id == "" {
		return nil
	}

	txCtx, err := e.trx.Begin(ctx)
	if err != nil {
		return err
	}
	defer e.trx.Rollback(txCtx)

	err = e.sRepo.Update(txCtx, id, data)
	if err != nil {
		return err
	}

	if err := e.trx.Commit(txCtx); err != nil {
		return err
	}

	if status, ok := data["status"].(string); ok {
		broadcaster.Get().Broadcast(broadcaster.Event{
			Type: "STEP_UPDATE",
			Payload: map[string]string{
				"id":     id,
				"status": status,
			},
		})
	}

	return nil
}

func (e *engine) failExecution(ctx context.Context, execution *model.Execution, errLog string) {
	e.log.Error("execution failed", zap.String("errLog", errLog))
	err := e.eRepo.Update(ctx, execution.ID, execution.Version, map[string]any{
		"status":       model.StatusExecutionFailed,
		"completed_at": time.Now(),
	})
	if err != nil {
		e.log.Error("failed to update execution status to failed", zap.Error(err))
	}
	execution.Status = model.StatusExecutionFailed
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
