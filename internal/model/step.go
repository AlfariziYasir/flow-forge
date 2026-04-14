package model

import "time"

type StepExecution struct {
	ID          string     `db:"id"`
	TenantID    string     `db:"tenant_id"`
	ExecutionID string     `db:"execution_id"`
	StepID      string     `db:"step_id"`
	Action      string     `db:"action"`
	Status      string     `db:"status"`
	RetryCount  int        `db:"retry_count"`
	ErrorLog    string     `db:"error_log"`
	StartedAt   *time.Time `db:"started_at"`
	CompletedAt *time.Time `db:"completed_at"`
}

func (t *StepExecution) Tablename() string {
	return "step_executions"
}

func (t *StepExecution) Columns() []string {
	return []string{"id", "tenant_id", "execution_id", "step_id", "action", "status", "retry_count", "error_log", "started_at", "completed_at"}
}

func (t *StepExecution) Values() []any {
	return []any{t.ID, t.TenantID, t.ExecutionID, t.StepID, t.Action, t.Status, t.RetryCount, t.ErrorLog, t.StartedAt, t.CompletedAt}
}

type ListStepExecutionRequest struct {
	PageSize    uint64 `json:"page_size"`
	PageToken   string `json:"page_token"`
	TenantID    string `json:"-"`
	ExecutionID string `json:"execution_id"`
	Status      string `json:"status"`
	Action      string `json:"action"`
}

type StepExecutionResponse struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	ExecutionID string     `json:"execution_id"`
	StepID      string     `json:"step_id"`
	Action      string     `json:"action"`
	Status      string     `json:"status"`
	RetryCount  int        `json:"retry_count"`
	ErrorLog    string     `json:"error_log"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
