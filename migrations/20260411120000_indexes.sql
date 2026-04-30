-- +goose Up

-- Optimization: Compound index for Workflow filtering and sorting by creation date
CREATE INDEX idx_workflows_tenant_name_created_at
    ON workflows(tenant_id, name, created_at DESC)
    WHERE is_current = true AND is_active = true;

-- Optimization: Compound index for Step Executions fetching by execution_id ordered by step_id (used in ClaimPendingSteps)
CREATE INDEX idx_step_executions_pending_ordered
    ON step_executions(execution_id, status, step_id)
    WHERE status = 'PENDING';

-- +goose Down
DROP INDEX idx_step_executions_pending_ordered;
DROP INDEX idx_workflows_tenant_name_created_at;
