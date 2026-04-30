-- +goose Up

SET constant.app.current_tenant_id = '00000000-0000-0000-0000-000000000000';

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_tenants ON tenants
    USING (id::text = current_setting('app.current_tenant_id', true));

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_users ON users
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

ALTER TABLE workflows ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_workflows ON workflows
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

ALTER TABLE executions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_executions ON executions
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

ALTER TABLE step_executions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_step_executions ON step_executions
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- +goose Down

DROP POLICY IF EXISTS tenant_isolation_step_executions ON step_executions;
DROP POLICY IF EXISTS tenant_isolation_executions ON executions;
DROP POLICY IF EXISTS tenant_isolation_workflows ON workflows;
DROP POLICY IF EXISTS tenant_isolation_users ON users;
DROP POLICY IF EXISTS tenant_isolation_tenants ON tenants;

ALTER TABLE step_executions DISABLE ROW LEVEL SECURITY;
ALTER TABLE executions DISABLE ROW LEVEL SECURITY;
ALTER TABLE workflows DISABLE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
ALTER TABLE tenants DISABLE ROW LEVEL SECURITY;