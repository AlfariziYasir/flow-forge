# FlowForge: Architecture & Implementation Blueprint

**Status:** Draft / Work in Progress
**Last Updated:** Phase 1 (Initial Setup)

## 1. System Overview
FlowForge is a self-hosted, scalable workflow automation engine inspired by Zapier and n8n. It allows tenants to define, execute, monitor, and troubleshoot automated workflows via a Directed Acyclic Graph (DAG) execution model, augmented by AI capabilities. The platform enforces strict Role-Based Access Control (RBAC) at the workspace (tenant) level.

### 1.1 User Roles
- **Admin (Workspace Owner):** Full administrative privileges. Can invite/remove users, configure global workspace variables and OAuth connections (Credential Vault), and manage all workflows.
- **Editor (Workflow Builder):** Can create, edit, delete, and manually trigger workflows. Can perform Local Step Testing and view execution logs.
- **Viewer (Read-Only):** Can view workflow configurations, execution histories, and logs. Cannot modify workflows or trigger runs.

## 2. System Architecture
We utilize a Monorepo structure with distinct deployables to decouple I/O-bound web traffic from CPU-bound execution tasks.

* **API Layer (`cmd/server/main.go`):** 
  * Handles HTTP REST traffic, User Authentication (JWT), RBAC, and Webhook ingestion.
  * Streams real-time execution updates to the UI via Server-Sent Events (SSE).
  * Horizontally scalable behind a Load Balancer.
* **Worker Layer (`cmd/worker/main.go`):**
  * Connects to the Database and Message Broker.
  * Polls for pending jobs, parses DAGs, and executes step actions.
  * Scaled independently based on queue depth (e.g., via KEDA).

### 2.1 Software Design Patterns

| Pattern | Implementation |
|---------|---------------|
| **Strategy** | `StepExecutor` interface abstracts step execution logic. New step types (HTTP, Script, Delay, Switch, Loop) added without modifying engine. |
| **Worker Pool** | Bounded goroutine pools per pod prevent memory exhaustion during traffic spikes. |
| **Saga (Compensation)** | For workflows bridging multiple external APIs, compensation steps run automatically on failure to rollback/cleanup. |

### 2.2 Multi-Tenant Isolation
- **Approach:** Shared Database / Shared Schema with PostgreSQL Row-Level Security (RLS)
- **Enforcement:** RLS policies enforce strict `tenant_id` boundaries on all tenant-scoped tables
- **Credential Vault:** Centralized, encrypted storage for OAuth tokens and API keys

## 3. Step Types & Control Flow
The engine supports the following step types via the Strategy pattern:

| Step Type | Description |
|-----------|-------------|
| `HTTP` | External API calls with idempotency key support |
| `SCRIPT` | Custom script execution (future: Wasm sandbox) |
| `DELAY` | Wait for specified duration |
| `SWITCH` | Conditional branching (if/else logic) |
| `LOOP` / `FOREACH` | Iteration over arrays |
| `WAIT_FOR_EVENT` | Stateful webhook - pauses execution until external event |
| `COMPENSATION` | Rollback action for saga pattern |

### 3.1 Data Interpolation Engine
- Supports Go Templates and JSONata for dynamic variable injection
- Syntax: `{{ step_A.user.email }}` to reference upstream step outputs
- Variables scoped to execution context

## 4. PubSub & Real-Time Communication
**Broker:** Redis

1. **Job Queuing:** The API enqueues `PENDING` workflow executions into a Redis List (`flowforge:jobs:queue`). Workers use `BLPop` for low-latency processing, complemented by a periodic Database Poller for reliability and recovery of missed jobs.
2. **Event Broadcasting:** As Workers process steps, they publish state changes (`START`, `SUCCESS`, `FAIL`) to a Redis PubSub channel (`flowforge:events:tenant_{id}`).
3. **SSE Delivery:** The API layer subscribes to these tenant-specific Redis channels and pushes live updates to the React UI via SSE.

### 4.1 Cron Scheduler
- Utilizes Redis distributed locks (`SETNX`) to prevent duplicate job enqueueing across horizontally scaled API pods

## 5. Test-Driven Development (TDD) Strategy
Quality is enforced via a strict testing pyramid before code reaches production:
* **Unit Tests (70%):** Core business logic (DAG topological sorting, cycle detection, AI prompt formatting, JWT claims).
* **Integration Tests (20%):** Database repositories (using `testcontainers` for Postgres) and API handlers.
* **E2E Tests (10%):** Simulated end-to-end runs (API -> Redis -> Worker -> DB).
* **Coverage Target:** Minimum 80% for core engine packages and AI validation loops.

## 6. AI Integration (Agent Services)
Located in `internal/services/ai_service.go`, leveraging Go AI SDK (langchaingo or Google Gen AI SDK).

* **Natural Language to DAG:** Users describe workflows in plain English. The LLM generates a JSON DAG, validated immediately by our internal parser.
* **Self-Correction Loop:** If DAG parser detects an error (e.g., cycle), the Go service immediately queries the LLM to correct the JSON (max 3 retries).
* **Intelligent Diagnostics:** On step failure, the payload and error response are asynchronously sent to the LLM to generate actionable fix recommendations.

## 7. Log Management & Data Store
* **Primary DB:** PostgreSQL (Stores Tenants, Users, Workflows, Execution Metadata).
  * Uses JSONB and GIN indexes for flexible payload storage.
  * `step_executions` table uses native Table Partitioning by month for high-volume logs.
* **Application Logs:** Structured JSON logging (e.g., `slog` mapped to Datadog/ELK) sent to `stdout`.
* **Execution Logs (Data Volume):** 
  * *MVP:* Stored in a partitioned Postgres table (`step_executions`).
  * *Scale:* Planned migration to an append-only NoSQL datastore (DynamoDB/Elasticsearch) for heavy I/O payloads.
  * *Pruning:* Automated CRON worker to hard-delete free-tier execution payloads older than 30 days.

## 8. Non-Functional Requirements
* **Scalability:** API and Worker containers must be completely stateless (aside from Redis/DB connections) for horizontal scaling.
* **Idempotency:** External network calls generate `Idempotency-Key` headers to prevent duplicate mutations during retries.

## 9. SDLC & CI/CD Pipeline
* **Local:** Git hooks (`golangci-lint`) to enforce formatting and prevent bad commits.
* **CI (Pull Requests):** Linting, Unit/Integration Tests, 80% coverage enforcement.
* **CD (Main Branch):** 
  * Build Vite UI assets and Go binaries via multi-stage Dockerfile.
  * Push to Container Registry.
  * Apply safe database migrations automatically.
  * Trigger rolling deployment to cluster.

---

## 10. Out of Scope (Phase 2)
* Migrating inter-service JSON payloads to Protocol Buffers (Protobuf)
* PostgreSQL Tablespaces for physical I/O isolation
* Custom user scripts in WebAssembly (Wasm) sandboxes using `wazero`
* OpenTelemetry for distributed tracing