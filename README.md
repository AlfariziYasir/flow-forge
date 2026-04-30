# FlowForge

FlowForge is a simplified, self-hosted execution engine for automated workflows. It handles DAG-based workflow execution, parallel tasks, scheduling, and logging with rich multi-tenant support.

## Project Structure (Clean Architecture)
```text
.
├── cmd/
│   ├── seed/main.go       # DB Seeder script
│   └── server/main.go     # Main HTTP REST & Background Worker entrypoint
├── internal/
│   ├── broadcaster/       # SSE / Real-time PubSub engine
│   ├── handler/           # API Handlers (RESTful routes, WebSockets/SSE)
│   ├── model/             # Core domain models (DB structs, JSON payloads)
│   ├── repository/        # PostgreSQL squirrel implementations
│   ├── services/          # Business logic (Workflow parsing, triggers, AI proxy)
│   └── worker/            # Background DAG executor engine & poller
├── pkg/
│   ├── dag/               # Core algorithm for topological sort and parallel layers
│   ├── errorx/            # Standardized application errors
│   ├── jwt/               # Auth token manager
│   ├── logger/            # Zap structured logger
│   └── postgres/          # DB context and transactions (Unit of Work)
├── migrations/            # Goose migration files
└── ui/                    # React Vite Frontend (to be implemented)
```

## Setup & Running

1. **Spin up Infrastructure:**
   ```bash
   docker-compose up -d db
   ```
   *Wait a few seconds for Postgres to initialize.*

2. **Run Migrations (assuming goose installed locally):**
   ```bash
   export GOOSE_DRIVER=postgres
   export GOOSE_DBSTRING="postgres://forge:forge@localhost:5432/flowforge?sslmode=disable"
   goose -dir migrations up
   ```

3. **Seed Database:**
   ```bash
   go run cmd/seed/main.go
   ```

4. **Start Application:**
   ```bash
   go run cmd/server/main.go
   ```

## Architecture Overview
The application is purely monolithic to reduce ops overhead for an MVP, but horizontally separated into modular domains. The `handler` layer exposes Chi routes wrapped under JWT authentication. The `service` layer manages validation.
For the execution engine, `cmd/server/main.go` runs the API thread and crucially runs the `job_poller` goroutine which behaves as a rudimentary cron polling Postgres. 
It uses Pessimistic Locking (`SELECT FOR UPDATE SKIP LOCKED`) natively in PostgreSQL (`internal/repository/execution.go`) allowing infinite seamless horizontal scaling of workers across multiple pod replicas without causing race conditions.

## API Reference
- `POST /api/v1/auth/login` - Exchange credentials for JWT.
- `GET /api/v1/monitor/stream` - SSE endpoint for live Step updates.
- `GET/POST /api/v1/workflows` - Workflow management.
- `POST /api/v1/workflows/{id}/trigger` - Initiates an execution asynchronously.

## Trade-offs & Future Improvements
1. **Queueing System**: We are using PostgreSQL as a queue mechanism with `SKIP LOCKED`. While this yields a fantastic ROI for MVPs and prevents having to deploy complex brokers like RabbitMQ or Kafka, it doesn't perform well past roughly 10,000 requests per minute. **Improvement**: Migrate the pub/sub event bus to RabbitMQ and the worker queue to Redis/Asynq.
2. **Log Store**: High volume logs (`error_log`) are presently in the primary operational table `step_executions`. We optimized this using compound indexes, but text columns are heavy. **Improvement**: Pipe logs append-only directly to an ELK stack or Grafana Loki.
3. **SSE vs WebSockets**: I picked SSE for one-way status streams. **Improvement**: Moving to WebSockets would enable bi-directional triggers from the dashboard without needing polling REST requests in return.