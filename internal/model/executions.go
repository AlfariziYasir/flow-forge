package model

import "time"

type StatusExecutions string

const (
	StatusExecutionPending   = "PENDING"
	StatusExecutionSuccess   = "SUCCESS"
	StatusExecutionFailed    = "FAILED"
	StatusExecutionTimeout   = "TIMEOUT"
	StatusExecutionRunning   = "RUNNING"
	StatusExecutionCancelled = "CANCELLED"
	StatusExecutionSkipped   = "SKIPPED"
	StatusExecutionSuspended = "SUSPENDED"
	StatusExecutionCompensating = "COMPENSATING"
)

type Execution struct {
	ID          string     `db:"id"`
	TenantID    string     `db:"tenant_id"`
	WorkflowID  string     `db:"workflow_id"`
	Status      string     `db:"status"`
	TriggerType string     `db:"trigger_type"`
	Version     int        `db:"version"`
	StartedAt   *time.Time `db:"started_at"`
	CompletedAt *time.Time `db:"completed_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

func (t *Execution) Tablename() string {
	return "executions"
}

func (t *Execution) Columns() []string {
	return []string{
		"id", "tenant_id", "workflow_id",
		"status", "trigger_type", "version",
		"started_at", "completed_at", "created_at", "updated_at",
	}
}

func (t *Execution) Values() []any {
	return []any{
		t.ID, t.TenantID, t.WorkflowID,
		t.Status, t.TriggerType, t.Version,
		t.StartedAt, t.CompletedAt, t.CreatedAt, t.UpdatedAt,
	}
}

type ExecutionRequest struct {
	TenantID    string `json:"tenant_id"`
	WorkflowID  string `json:"workflow_id"`
	Status      string `json:"status"`
	TriggerType string `json:"trigger_type"`
}

type ExecutionResponse struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	WorkflowID  string     `json:"workflow_id"`
	Status      string     `json:"status"`
	TriggerType string     `json:"trigger_type"`
	Version     int        `json:"version"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ListExecutionRequest struct {
	PageSize    uint64 `json:"page_size"`
	PageToken   string `json:"page_token"`
	TenantID    string `json:"-"`
	WorkflowID  string `json:"workflow_id"`
	Status      string `json:"status"`
	TriggerType string `json:"trigger_type"`
}
