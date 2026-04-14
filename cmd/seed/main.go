package main

import (
	"context"
	"fmt"
	"log"

	"flowforge/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := "postgres://forge:forge@localhost:5432/flowforge?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer pool.Close()

	tenantID := uuid.New().String()
	userID := uuid.New().String()

	// 1. Create Tenant
	_, err = pool.Exec(ctx, "INSERT INTO tenants (id, name, is_active) VALUES ($1, $2, $3)", tenantID, "Acme Corp", true)
	if err != nil {
		log.Printf("tenant already seeded or error: %v", err)
	}

	// 2. Create User
	// Normally we would hash the password, string comparison is only for MVP
	_, err = pool.Exec(ctx, "INSERT INTO users (id, tenant_id, email, password_hash, role) VALUES ($1, $2, $3, $4, $5)",
		userID, tenantID, "admin@acme.com", "admin123", "admin")
	if err != nil {
		log.Printf("user already seeded or error: %v", err)
	}

	// 3. Create Example Workflow DAG
	mockDAG := `[
		{
			"id": "step_fetch",
			"action": "HTTP_CALL",
			"parameters": {"url": "https://dummyjson.com/products/1", "method": "GET"},
			"depends_on": [],
			"max_retries": 1
		},
		{
			"id": "step_process",
			"action": "SCRIPT",
			"parameters": {"script": "return data;"},
			"depends_on": ["step_fetch"],
			"max_retries": 0
		}
	]`
	workflowID := uuid.New().String()
	
	wf := &model.Workflow{
		ID:            workflowID,
		TenantID:      tenantID,
		Name:          "Data Fetch Pipeline",
		Description:   "Fetches data from an API and processes it",
		DAGDefinition: []byte(mockDAG),
		Version:       1,
		IsCurrent:     true,
		IsActive:      true,
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO workflows (id, tenant_id, name, description, dag_definition, version, is_current, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		wf.ID, wf.TenantID, wf.Name, wf.Description, string(wf.DAGDefinition), wf.Version, wf.IsCurrent, wf.IsActive,
	)
	if err != nil {
		log.Printf("workflow already seeded or error: %v", err)
	}

	fmt.Println("Seed complete!")
	fmt.Printf("Login Email: admin@acme.com\nPassword: admin123\nTenant ID: %s\n", tenantID)
}
