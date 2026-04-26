package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://forge:forge@localhost:5432/flowforge?sslmode=disable"
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer pool.Close()

	if err := seedData(ctx, pool); err != nil {
		log.Fatalf("seed failed: %v", err)
	}

	log.Println("Seed completed successfully!")
}

func seedData(ctx context.Context, pool *pgxpool.Pool) error {
	var tenantID, userID string
	var tenantExists, userExists bool

	err := pool.QueryRow(ctx, "SELECT id FROM tenants WHERE name = $1", "admin Corp").Scan(&tenantID)
	if err == nil {
		tenantExists = true
		log.Printf("Tenant 'admin Corp' already exists (ID: %s)", tenantID)
	} else {
		tenantID = uuid.New().String()
		_, err = pool.Exec(ctx, "INSERT INTO tenants (id, name, is_active) VALUES ($1, $2, $3)", tenantID, "admin Corp", true)
		if err != nil {
			return fmt.Errorf("failed to create tenant: %w", err)
		}
		log.Printf("Created tenant: admin Corp (ID: %s)", tenantID)
	}

	err = pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", "admin@admin.com").Scan(&userID)
	if err == nil {
		userExists = true
		log.Printf("User 'admin@admin.com' already exists (ID: %s)", userID)
	} else {
		userID = uuid.New().String()
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}

		_, err = pool.Exec(ctx,
			"INSERT INTO users (id, tenant_id, email, password_hash, role) VALUES ($1, $2, $3, $4, $5)",
			userID, tenantID, "admin@admin.com", string(hashedPassword), "admin")
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
		log.Printf("Created user: admin@admin.com (ID: %s)", userID)
	}

	if !tenantExists || !userExists {
		mockDAG := `[
			{
				"id": "step_fetch",
				"action": "HTTP",
				"parameters": {"url": "https://dummyjson.com/products/1", "method": "GET"},
				"depends_on": [],
				"max_retries": 1
			},
			{
				"id": "step_process",
				"action": "TRANSFORM",
				"parameters": {"operation": "UPPERCASE", "input": "{{step_fetch.body}}"},
				"depends_on": ["step_fetch"],
				"max_retries": 0
			}
		]`
		workflowID := uuid.New().String()

		_, err = pool.Exec(ctx, `
			INSERT INTO workflows (id, tenant_id, name, description, dag_definition, version, is_current, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			workflowID, tenantID, "Data Fetch Pipeline", "Fetches data from an API and processes it", mockDAG, 1, true, true,
		)
		if err != nil {
			log.Printf("workflow already seeded or error: %v", err)
		} else {
			log.Printf("Created workflow: Data Fetch Pipeline (ID: %s)", workflowID)
		}
	} else {
		log.Println("Workflow already exists, skipping...")
	}

	fmt.Println("========================================")
	fmt.Println("Login Credentials:")
	fmt.Println("========================================")
	fmt.Printf("Email:    admin@admin.com\n")
	fmt.Printf("Password: admin123\n")
	fmt.Printf("Tenant:   admin Corp\n")
	fmt.Println("========================================")

	return nil
}
