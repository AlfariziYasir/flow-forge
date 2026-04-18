# FlowForge: Architecture & Implementation Blueprint

**Status:** Draft / Work in Progress
**Last Updated:** Phase 1 (Initial Setup)

## 1. System Overview
FlowForge is a self-hosted, scalable workflow automation engine inspired by Zapier. It allows tenants to define, execute, monitor, and troubleshoot automated workflows via a Directed Acyclic Graph (DAG) execution model, augmented by AI capabilities.

## 2. System Architecture
We utilize a Monorepo structure with distinct deployables to decouple I/O-bound web traffic from CPU-bound execution tasks.

* **API Layer (`cmd/server/main.go`):** 
  * Handles HTTP REST/GraphQL traffic, User Authentication (JWT), RBAC, and Webhook ingestion.
  * Streams real-time execution updates to the UI via Server-Sent Events (SSE).
  * Horizontally scalable behind a Load Balancer.
* **Worker Layer (`cmd/worker/main.go`):**
  * Connects to the Database and Message Broker.
  * Polls for pending jobs, parses DAGs, and executes step actions (HTTP, Scripts, Delays).
  * Scaled independently based on queue depth (e.g., via KEDA).

## 3. PubSub & Real-Time Communication
**Broker:** Redis

1. **Job Queuing:** The API enqueues `PENDING` workflow executions into a Redis List/Stream (`flowforge:jobs:queue`).
2. **Event Broadcasting:** As Workers process steps, they publish state changes (`START`, `SUCCESS`, `FAIL`) to a Redis PubSub channel (`flowforge:events:tenant_{id}`).
3. **SSE Delivery:** The API layer subscribes to these tenant-specific Redis channels and pushes live updates to the React UI via SSE.

## 4. Test-Driven Development (TDD) Strategy
Quality is enforced via a strict testing pyramid before code reaches production:
* **Unit Tests (70%):** Core business logic (DAG topological sorting, cycle detection, AI prompt formatting, JWT claims).
* **Integration Tests (20%):** Database repositories (using `testcontainers` for Postgres) and API handlers.
* **E2E Tests (10%):** Simulated end-to-end runs (API -> Redis -> Worker -> DB).

## 5. AI Integration (Agent Services)
Located in `internal/services/ai_service.go`, leveraging an LLM API.
* **Natural Language to DAG:** Users describe workflows in plain English. The LLM generates a JSON DAG, validated immediately by our internal parser with a self-correction retry loop.
* **Intelligent Diagnostics:** On step failure, the payload and error response are asynchronously sent to the LLM to generate an actionable fix recommendation for the user.

## 6. Log Management & Data Store
* **Primary DB:** PostgreSQL (Stores Tenants, Users, Workflows, Execution Metadata).
* **Application Logs:** Structured JSON logging (e.g., `slog` mapped to Datadog/ELK) sent to `stdout`.
* **Execution Logs (Data Volume):** 
  * *MVP:* Stored in a partitioned Postgres table (`step_executions`).
  * *Scale:* Planned migration to an append-only NoSQL datastore (DynamoDB/Elasticsearch) for heavy I/O payloads.
  * *Pruning:* Automated CRON worker to hard-delete free-tier execution payloads older than 30 days.

## 7. SDLC & CI/CD Pipeline
* **Local:** Git hooks (`husky`, `golangci-lint`) to enforce formatting and prevent bad commits.
* **CI (Pull Requests):** Linting, Unit/Integration Tests, 80% coverage enforcement.
* **CD (Main Branch):** 
  * Build Vite UI assets and Go binaries via multi-stage Dockerfile.
  * Push to Container Registry.
  * Apply safe database migrations automatically.
  * Trigger rolling deployment to cluster.

---
**[Future Expansion Areas - To Be Defined]**
* *Database Schema Details (ERD)*
* *Specific API Endpoints & Rate Limiting Strategy*
* *Security & Secrets Management (Vault/KMS)*
