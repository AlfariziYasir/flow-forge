package model

import (
	"encoding/json"
	"time"
)

type Workflow struct {
	ID            string          `db:"id"`
	TenantID      string          `db:"tenant_id"`
	Name          string          `db:"name"`
	Description   string          `db:"description"`
	DAGDefinition json.RawMessage `db:"dag_definition"`
	Version       int             `db:"version"`
	IsCurrent     bool            `db:"is_current"`
	IsActive      bool            `db:"is_active"`
	CreatedAt     time.Time       `db:"created_at"`
	UpdatedAt     *time.Time      `db:"updated_at"`
}

func (w *Workflow) NextVersion() int {
	return w.Version + 1
}

func (w *Workflow) Tablename() string {
	return "workflows"
}

func (w *Workflow) Columns() []string {
	return []string{
		"id", "tenant_id", "name", "description",
		"dag_definition", "version", "is_current", "is_active",
		"created_at", "updated_at",
	}
}

func (w *Workflow) Values() []any {
	return []any{
		w.ID, w.TenantID, w.Name, w.Description,
		w.DAGDefinition, w.Version, w.IsCurrent, w.IsActive,
		w.CreatedAt, w.UpdatedAt,
	}
}

type WorkflowRequest struct {
	TenantID      string          `json:"tenant_id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	DAGDefinition json.RawMessage `json:"dag_definition"`
}

type WorkflowUpdateRequest struct {
	TenantID       string          `json:"tenant_id"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	DAGDefinition  json.RawMessage `json:"dag_definition"`
	CurrentVersion int             `json:"current_version"`
}

type WorkflowRollbackRequest struct {
	TenantID       string `json:"tenant_id"`
	Name           string `json:"name"`
	TargetVersion  int    `json:"target_version"`
	CurrentVersion int    `json:"current_version"`
}

type WorkflowResponse struct {
	WorkflowID    string           `json:"workflow_id"`
	TenantID      string           `json:"tenant_id"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	DAGDefinition []StepDefinition `json:"dag_definition"`
	Version       int              `json:"version"`
	IsActive      bool             `json:"is_active"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     *time.Time       `json:"updated_at,omitempty"`
	// History holds older versions, ordered newest-first.
	History []WorkflowVersion `json:"history,omitempty"`
}

type WorkflowVersion struct {
	WorkflowID    string           `json:"workflow_id"`
	TenantID      string           `json:"tenant_id"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	DAGDefinition []StepDefinition `json:"dag_definition"`
	Version       int              `json:"version"`
	IsCurrent     bool             `json:"is_current"`
	CreatedAt     time.Time        `json:"created_at"`
}

type StepDefinition struct {
	ID         string         `json:"id"`
	Action     string         `json:"action"`
	Parameters map[string]any `json:"parameters"`
	DependsOn  []string       `json:"depends_on"`
	MaxRetries int            `json:"max_retries"`
}
