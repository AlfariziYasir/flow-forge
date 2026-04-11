-- +goose Up

CREATE TABLE tenants (
    id         UUID PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    is_active  BOOLEAN      NOT NULL DEFAULT true
);
CREATE INDEX idx_tenants_name ON tenants(name);

CREATE TABLE users (
    id            UUID PRIMARY KEY,
    tenant_id     UUID         REFERENCES tenants(id) ON DELETE CASCADE,
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(20)  NOT NULL CHECK (role IN ('admin', 'editor', 'viewer')),
    version       INT          NOT NULL DEFAULT 1,
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP,
    is_active     BOOLEAN      NOT NULL DEFAULT true
);
CREATE INDEX idx_users_tenant_id ON users(tenant_id);
CREATE INDEX idx_users_email ON users(email);

CREATE TABLE workflows (
    id             UUID    PRIMARY KEY,
    tenant_id      UUID    REFERENCES tenants(id) ON DELETE CASCADE,
    name           VARCHAR(255) NOT NULL,
    description    TEXT,
    dag_definition JSONB   NOT NULL,
    version        INT     NOT NULL DEFAULT 1,
    is_current     BOOLEAN NOT NULL DEFAULT false,
    is_active      BOOLEAN NOT NULL DEFAULT true,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP,

    CONSTRAINT unique_workflow_name_tenant_version UNIQUE (tenant_id, name, version)
);
CREATE UNIQUE INDEX idx_workflows_one_current
    ON workflows(tenant_id, name)
    WHERE is_current = true;
CREATE INDEX idx_workflows_tenant_current ON workflows(tenant_id, is_current, is_active);
CREATE INDEX idx_workflows_history ON workflows(tenant_id, name, version DESC);

CREATE TABLE executions (
    id           UUID PRIMARY KEY,
    tenant_id    UUID        REFERENCES tenants(id)   ON DELETE CASCADE,
    workflow_id  UUID        REFERENCES workflows(id) ON DELETE CASCADE,
    status       VARCHAR(20) NOT NULL CHECK (status IN ('PENDING', 'RUNNING', 'SUCCESS', 'FAILED', 'TIMEOUT')),
    trigger_type VARCHAR(20) NOT NULL CHECK (trigger_type IN ('MANUAL', 'CRON', 'WEBHOOK')),
    version      INT         NOT NULL DEFAULT 1,
    started_at   TIMESTAMP,
    completed_at TIMESTAMP,
    created_at   TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_executions_tenant_status ON executions(tenant_id, status);
CREATE INDEX idx_executions_pending ON executions(status) WHERE status = 'PENDING';

CREATE TABLE step_executions (
    id           UUID PRIMARY KEY,
    tenant_id    UUID         REFERENCES tenants(id)    ON DELETE CASCADE,
    execution_id UUID         REFERENCES executions(id) ON DELETE CASCADE,
    step_id      VARCHAR(100) NOT NULL,
    action       VARCHAR(50)  NOT NULL,
    status       VARCHAR(20)  NOT NULL CHECK (status IN ('PENDING', 'RUNNING', 'SUCCESS', 'FAILED')),
    retry_count  INT          NOT NULL DEFAULT 0,
    error_log    TEXT,
    started_at   TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE INDEX idx_step_executions_execution ON step_executions(execution_id, status);
CREATE INDEX idx_step_executions_pending ON step_executions(status) WHERE status = 'PENDING';

-- +goose Down
DROP TABLE IF EXISTS step_executions;
DROP TABLE IF EXISTS executions;
DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;