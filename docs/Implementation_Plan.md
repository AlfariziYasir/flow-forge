# Code Review: FlowForge Workflow

Issues found by cross-referencing the codebase against [Project_Requirement_Document.md](file:///home/sigma/golang/flow-forge/docs/Project_Requirement_Document.md) and [ARCHITECTURE.md](file:///home/sigma/golang/flow-forge/docs/ARCHITECTURE.md).

---

## Critical Bugs

### 1. `context.WithTimeout` defer-cancel leak inside retry loop

**File:** [engine.go](file:///home/sigma/golang/flow-forge/internal/worker/service/engine.go#L180-L182)

```go
// Line 181-182: inside a for loop
stepCtx, cancel := context.WithTimeout(ctx, e.stepTimeout)
defer cancel()  // BUG: deferred to function return, not loop iteration
```

`defer cancel()` is scoped to the enclosing **function**, not the loop iteration. On every retry, a new `context.WithTimeout` is created but the previous cancel is never invoked until the function exits. This creates a **goroutine/timer leak** — one leaked context per retry attempt.

> [!CAUTION]
> This is a resource leak that compounds under load. With `maxRetries=3` and thousands of concurrent step executions, this can exhaust memory.

**Fix:** Call `cancel()` explicitly at the end of each iteration, or extract the step execution into a separate function so `defer` scopes correctly.

```diff
-stepCtx, cancel := context.WithTimeout(ctx, e.stepTimeout)
-defer cancel()
-result, err := action.Execute(stepCtx, params)
+stepCtx, cancel := context.WithTimeout(ctx, e.stepTimeout)
+result, err := action.Execute(stepCtx, params)
+cancel() // cancel immediately after use
```

---

### 2. `ScriptAction` registered with nil Docker client

**File:** [engine.go](file:///home/sigma/golang/flow-forge/internal/worker/service/engine.go#L49)

```go
r.Registry("SCRIPT", &ScriptAction{})  // dockerClient is nil!
```

`ScriptAction` has a `NewScriptAction()` constructor that properly initializes the Docker client, but the engine uses a bare struct literal `&ScriptAction{}`. Any workflow using the `SCRIPT` step type will **panic with a nil pointer dereference** on `s.dockerClient.ContainerCreate(...)`.

> [!CAUTION]
> This is a latent crash bug that will trigger the moment any workflow uses the SCRIPT action.

**Fix:** Use the constructor and propagate the error, or lazily initialize the Docker client on first use.

```diff
-r.Registry("SCRIPT", &ScriptAction{})
+scriptAction, err := NewScriptAction()
+if err != nil {
+    l.Warn("SCRIPT action unavailable (Docker not accessible)", zap.Error(err))
+} else {
+    r.Registry("SCRIPT", scriptAction)
+}
```

---

### 3. Worker `AcquireForWorker` lacks `tenant_id` filtering

**File:** [execution.go](file:///home/sigma/golang/flow-forge/internal/repository/execution.go#L170-L193)

Per PRD §7 and Architecture §2.2, the platform enforces **multi-tenant isolation** via RLS. However, `AcquireForWorker` has no `tenant_id` filter or RLS session-variable setup. If RLS is not configured at the connection level, a worker could process executions from **any** tenant.

> [!WARNING]
> This is a multi-tenant isolation concern. While the worker intentionally processes all tenants' jobs, there is no mechanism to set the RLS context (`SET app.current_tenant`), which means RLS policies on the `executions` table will either block all queries (false negatives) or pass unchecked (data leak).

**Fix:** Either:
- Set `app.current_tenant` before the query (if RLS is enforced), or
- Document that the worker operates as a **superuser** role that bypasses RLS by design, and ensure the DB role is configured accordingly.

---

## Design Gaps (vs PRD/Architecture)

### 4. Missing step types: `SWITCH`, `LOOP/FOREACH`, `WAIT_FOR_EVENT`, `COMPENSATION`

**File:** [engine.go](file:///home/sigma/golang/flow-forge/internal/worker/service/engine.go#L44-L49)

Architecture §3 specifies 7 step types. Only 4 are registered:

| Required (Architecture §3) | Registered | Status |
|---|---|---|
| `HTTP` | ✅ | Registered |
| `SCRIPT` | ⚠️ | Registered (but crashes — see #2) |
| `DELAY` → `WAIT` | ✅ | Registered (name mismatch: code uses `WAIT`, arch says `DELAY`) |
| `TRANSFORM` | ✅ | Registered |
| `SWITCH` | ❌ | **Missing** |
| `LOOP` / `FOREACH` | ❌ | **Missing** |
| `WAIT_FOR_EVENT` | ❌ | **Missing** |
| `COMPENSATION` | ❌ | **Missing** |

Additionally, `ConditionAction` exists in [condition.go](file:///home/sigma/golang/flow-forge/internal/worker/service/condition.go) but is **never registered** in the registry.

> [!IMPORTANT]
> The `SWITCH` and `LOOP` types are core PRD requirements (§3 "Advanced Control Flow"). These are not Phase 2 items.

---

### 5. Missing `Idempotency-Key` header on HTTP action

**File:** [http.go](file:///home/sigma/golang/flow-forge/internal/worker/service/http.go#L20-L60)

PRD §6 (REQ-1.3) states:
> *"External network calls must generate and utilize `Idempotency-Key` headers to prevent duplicate data mutations during transient network retries."*

The `HTTPAction.Execute` method sets user-provided headers but never generates or attaches an `Idempotency-Key`. During retries (handled by `executeStepWithRetry`), the same HTTP call is repeated **without idempotency protection**.

**Fix:** Generate a deterministic idempotency key based on `executionID + stepID + attemptNumber` and inject it as a request header.

---

### 6. `StepExecution` model missing `output` column

**File:** [step.go](file:///home/sigma/golang/flow-forge/internal/model/step.go#L5-L16)

The engine writes step output to the DB at [engine.go:238](file:///home/sigma/golang/flow-forge/internal/worker/service/engine.go#L235-L239):

```go
e.stepRepo.Update(ctx, stepExec.ID, map[string]any{
    "output": result,  // writes a map[string]any
})
```

But the `StepExecution` struct has **no `Output` field**, and the `Columns()`/`Values()` methods don't include it. This means:
- The DB `UPDATE SET output=...` may work if the column exists in Postgres (via `SetMap`), but the data is **never read back** when fetching steps.
- The model is incomplete — any code reading step execution results won't have access to outputs.

**Fix:** Add `Output json.RawMessage \`db:"output"\`` to `StepExecution` and update `Columns()`/`Values()`.

---

### 7. `ConditionAction` numeric comparison uses string ordering

**File:** [condition.go](file:///home/sigma/golang/flow-forge/internal/worker/service/condition.go#L17-L36)

Operators `>`, `>=`, `<`, `<=` compare values using `fmt.Sprintf("%v", v1)` which produces **lexicographic string comparison**, not numeric. This means `"9" > "10"` evaluates to `true` because `"9"` > `"1"` lexicographically.

**Fix:** Attempt numeric coercion first, fall back to string comparison.

---

## Code Quality Issues

### 8. Dead/unreachable `sRepoUpdate` method

**File:** [engine.go](file:///home/sigma/golang/flow-forge/internal/worker/service/engine.go#L268-L274)

The method `sRepoUpdate` is defined but **never called anywhere** in the codebase.

**Fix:** Remove it or integrate it where `getStepExecution` + `stepRepo.Update` is used inline.

---

### 9. Typo in `ConditionAction` response key

**File:** [condition.go](file:///home/sigma/golang/flow-forge/internal/worker/service/condition.go#L40)

```go
"evaluted": true,  // should be "evaluated"
```

---

### 10. Suppressed `ToSql()` errors in repositories

**Files:**
- [tenant.go:35](file:///home/sigma/golang/flow-forge/internal/repository/tenant.go#L35) — `query, args, _ := r.sq.Insert(...).ToSql()`
- [tenant.go:63](file:///home/sigma/golang/flow-forge/internal/repository/tenant.go#L63) — `sqlQuery, args, _ := query.ToSql()`
- [user.go:36](file:///home/sigma/golang/flow-forge/internal/repository/user.go#L36) — `query, args, _ := r.sq.Insert(...).ToSql()`
- [user.go:64](file:///home/sigma/golang/flow-forge/internal/repository/user.go#L64) — `sqlQuery, args, _ := query.ToSql()`
- [workflow.go:215](file:///home/sigma/golang/flow-forge/internal/repository/workflow.go#L215) — `query, args, _ := r.sq.Update(...).ToSql()`

These silently discard `ToSql()` errors. If Squirrel encounters a builder error (e.g., invalid column name), the query string will be empty, leading to a confusing Postgres error instead of an early, explanatory failure.

**Fix:** Handle the error consistently as done in other repository methods:

```diff
-query, args, _ := r.sq.Insert(...).ToSql()
+query, args, err := r.sq.Insert(...).ToSql()
+if err != nil {
+    return errorx.NewError(errorx.ErrTypeInternal, "failed to build query", err)
+}
```

---

### 11. `ClaimPendingSteps` not in `StepExecutionRepository` interface

**File:** [step_execution.go](file:///home/sigma/golang/flow-forge/internal/repository/step_execution.go#L15-L21) vs [step_execution.go:184](file:///home/sigma/golang/flow-forge/internal/repository/step_execution.go#L184-L215)

The method `ClaimPendingSteps` is implemented on the struct but is **not declared in the `StepExecutionRepository` interface**. It cannot be called through the interface, making it effectively dead code to any consumer using the interface type.

**Fix:** Add it to the interface:

```diff
 type StepExecutionRepository interface {
     Create(ctx context.Context, step *model.StepExecution) error
     Get(ctx context.Context, filters map[string]any, step *model.StepExecution) error
     ListByExecution(ctx context.Context, executionID string) ([]*model.StepExecution, error)
     List(ctx context.Context, executionID string, limit, offset uint64) ([]*model.StepExecution, int, error)
     Update(ctx context.Context, id string, data map[string]any) error
+    ClaimPendingSteps(ctx context.Context, executionID string, limit int) ([]*model.StepExecution, error)
 }
```

---

### 12. Inconsistent constructor naming: `NewRepository` for `UserRepository`

**File:** [user.go](file:///home/sigma/golang/flow-forge/internal/repository/user.go#L28)

```go
func NewRepository(db postgres.PgxExecutor) UserRepository {  // should be NewUserRepository
```

All other constructor names follow `New<Entity>Repository` convention: `NewTenantRepository`, `NewWorkflowRepository`, `NewExecutionRepository`, `NewStepExecutionRepository`. The user repo breaks this pattern.

---

### 13. `Execution.List` filters apply `ILike` universally — unsafe for non-string fields

**File:** [execution.go](file:///home/sigma/golang/flow-forge/internal/repository/execution.go#L100-L104)

```go
for k, v := range filters {
    query = query.Where(squirrel.ILike{k: fmt.Sprintf("%%%s%%", v)})
}
```

If a caller passes `{"tenant_id": "uuid-value"}`, this generates `WHERE tenant_id ILIKE '%uuid-value%'` instead of `WHERE tenant_id = 'uuid-value'`. This is:
- **Incorrect semantics** — UUID filtering should be exact match
- **Performance issue** — `ILIKE` with leading wildcard cannot use indexes
- **Security concern** — partial tenant_id matching can leak cross-tenant data

The same pattern exists in [tenant.go](file:///home/sigma/golang/flow-forge/internal/repository/tenant.go#L82-L86) and [user.go](file:///home/sigma/golang/flow-forge/internal/repository/user.go#L83-L87).

**Fix:** Separate exact-match filters (IDs, enums) from search filters (names, descriptions) and apply `Eq` vs `ILike` accordingly.

---

### 14. `getStepExecution` is O(n) per step per retry — N+1 query concern

**File:** [engine.go](file:///home/sigma/golang/flow-forge/internal/worker/service/engine.go#L255-L266)

Every call to `getStepExecution` fetches **all** step executions for the entire execution (`ListByExecution`) then scans linearly. This is called once per retry per step. For a workflow with 50 steps and 3 retries each, this means 150 full-table-scans of step_executions.

**Fix:** Add a `GetByExecutionAndStep(ctx, execID, stepID)` method to the `StepExecutionRepository` using a direct `WHERE execution_id = $1 AND step_id = $2` query.

---

## Worker Workflow & Architecture Review

Reviewing the `internal/worker` logic (specifically `job_poller.go` and `engine.go`), the current design operates at the **Execution Level** rather than the **Step Level**. A worker claims a full execution and runs all its steps locally using goroutines.

### 15. Unused `ClaimPendingSteps` and Step-Level Distribution

**File:** [step_execution.go:184](file:///home/sigma/golang/flow-forge/internal/repository/step_execution.go#L184-L215)

The `ClaimPendingSteps` repository method is completely unused. It was designed for a architecture where workers claim individual workflow *steps* from the queue rather than entire executions. Since the current engine evaluates the DAG and runs steps in memory on a single pod, step-level claiming is obsolete.

**Fix:** Remove `ClaimPendingSteps` to clean up the codebase. If step-level horizontal scaling is needed in the future, it requires a complete rewrite of `engine.go` to be stateless between steps.

### 16. Inconsistent Locking Strategies in Job Poller

**File:** [job_poller.go](file:///home/sigma/golang/flow-forge/internal/worker/handler/job_poller.go)

The worker uses two concurrent mechanisms to fetch jobs, with conflicting locking strategies:

1. **DB Poller (`poll`):** Uses **Pessimistic Locking**. `AcquireForWorker` uses `FOR UPDATE SKIP LOCKED`. This is highly efficient and correct for concurrent workers polling a database queue.
2. **Redis Poller (`processExecutionByID`):** Uses **Optimistic Locking** over a transaction. It opens a transaction, uses a standard `Get` (no lock), checks the status, and then relies on `Update(..., exe.Version)` to prevent race conditions. 

> [!WARNING]
> While the optimistic lock in the Redis poller prevents duplicate execution, it is inefficient under high contention. If multiple workers receive the same Redis event (e.g., due to duplicate queue deliveries or overlapping with the DB poller), they will both fetch the execution, but one will fail the update.

**Where to implement Pessimistic vs Optimistic Locking:**
- **Pessimistic Locking (`FOR UPDATE SKIP LOCKED`):** MUST be used for **queue consumption / acquisition**. Specifically, the Redis poller (`processExecutionByID`) should use a new repository method `AcquireByIDForWorker(ctx, id)` that executes `SELECT ... FOR UPDATE SKIP LOCKED`. This ensures that only one worker can claim the execution, reducing DB load and preventing optimistic lock failures during acquisition.
- **Optimistic Locking (`Version` column):** MUST be used for **state transitions during execution** (e.g., updating workflow definitions, marking steps as Success/Failed, or updating an execution's status upon completion). It protects against lost updates (e.g., a user cancelling a workflow while the engine is updating its status) without holding long DB locks.

**Fix:** Align the Redis poller to use pessimistic locking for acquisition. Modify `processExecutionByID` to use a `SELECT ... FOR UPDATE SKIP LOCKED` query, bypassing the need for optimistic version checking during acquisition.

### 17. Context Cancellation and Zombie Goroutines in Engine

**File:** [job_poller.go:148](file:///home/sigma/golang/flow-forge/internal/worker/handler/job_poller.go#L148)

The job poller spawns the engine using `context.Background()`:
`go p.engine.RunExecution(context.Background(), exe, &workflow)`

Because it uses `Background()`, if the worker process is commanded to shut down gracefully (SIGTERM), the running executions have no way to receive a cancellation signal. They will continue running, potentially being forcefully killed by the OS or Kubernetes, leaving the execution in a perpetual `RUNNING` state in the database.

**Fix:** Pass a controlled context derived from the worker's lifecycle context, or implement a graceful shutdown `WaitGroup` that waits for active executions to finish before terminating the worker.

### 18. Architectural Mismatch: Redis Lists instead of Redis Streams

**Files:** [job_poller.go](file:///home/sigma/golang/flow-forge/internal/worker/handler/job_poller.go), [pkg/redis/redis.go](file:///home/sigma/golang/flow-forge/pkg/redis/redis.go)

PRD §5 and Architecture §4 strictly specify:
> "Specifically utilizing **Redis Streams** with Consumer Groups for exactly-once job delivery"

However, the implementation uses Redis Lists (`RPush` in the services, `BLPop` in the job poller).
- `BLPop` is destructive. If a worker pulls a job ID via `BLPop` and then crashes before committing the `RUNNING` status to the DB, the job ID is lost from Redis. (It will eventually be picked up by the DB fallback poller, but it breaks the "exactly-once" delivery guarantee of Redis Streams).
- Redis PubSub implementation (`broadcaster/sse.go`) is working correctly as designed for real-time SSE updates, but the queuing mechanism is fundamentally different from the architecture spec.

**Fix:** Either update the Architecture document to reflect the use of Redis Lists + DB Fallback Poller, OR refactor `pkg/redis/redis.go` to support `XADD`, `XREADGROUP`, and `XACK` and rewrite the job poller to use Redis Streams.

---

## System Reliability & Idempotency Improvements

Based on a deeper review of the worker workflow and PRD requirements, the following architectural improvements must be implemented to ensure system reliability at scale:

### 19. Stale Execution Recovery (Dead Job Detection)
**Context:** When the worker claims an execution, it transitions it to `RUNNING` and executes it in memory. 
**Issue:** If the worker pod crashes (OOMKill, node failure), the execution remains permanently stuck in `RUNNING`. The DB poller only queries for `PENDING` jobs, so these stuck jobs are never recovered.
**Fix:** Implement a "Stale Job Sweeper" mechanism. A CRON task or the DB Poller should query for executions where `status = 'RUNNING'` AND `updated_at < NOW() - INTERVAL '15 minutes'`. These executions should be transitioned back to `PENDING` (or `FAILED` if retry limit exceeded) so they can be re-processed.

### 20. Idempotency Key Generation Strategy
**File:** [http.go](file:///home/sigma/golang/flow-forge/internal/worker/service/http.go)
**Context:** PRD REQ-1.3 requires network calls to generate an `Idempotency-Key` to prevent duplicate data mutations during transient retries.
**Issue:** No idempotency logic exists in the codebase.
**Fix Design:** When injecting the `Idempotency-Key` header into the HTTP action, the key MUST be deterministic across retries. 
- **DO NOT** include the retry attempt count in the hash. 
- Use `hash(ExecutionID + StepID)`. 
This guarantees that if attempt 1 times out but reaches the destination server, attempt 2 will use the exact same key, allowing the destination server to safely deduplicate the request.

---

## Summary Matrix

| # | Severity | File | Issue | Status |
|---|---|---|---|---|
| 1 | 🔴 Critical | engine.go | defer-cancel leak in retry loop | ✅ Resolved |
| 2 | 🔴 Critical | engine.go | ScriptAction nil-pointer crash | ✅ Resolved |
| 3 | 🟡 Warning | execution.go | Worker bypasses multi-tenant RLS | ✅ Resolved (Handled in `RunExecution`) |
| 4 | 🟡 Warning | engine.go | 4 step types missing from Architecture | 🚧 Partial (Added `SWITCH`) |
| 5 | 🟡 Warning | http.go | No idempotency key (PRD REQ-1.3) | ✅ Resolved |
| 6 | 🟡 Warning | step.go | Missing `output` field on StepExecution model | ✅ Resolved |
| 7 | 🟠 Medium | condition.go | Numeric comparison uses string ordering | ✅ Resolved |
| 8 | 🔵 Low | engine.go | Dead `sRepoUpdate` method | ✅ Resolved |
| 9 | 🔵 Low | condition.go | Typo `evaluted` → `evaluated` | ✅ Resolved |
| 10 | 🟠 Medium | tenant/user/workflow repos | Suppressed `ToSql()` errors | ✅ Resolved |
| 11 | 🔵 Low | step_execution.go | Unused `ClaimPendingSteps` method | ✅ Resolved |
| 12 | 🔵 Low | user.go | Inconsistent constructor naming | ✅ Resolved |
| 13 | 🟡 Warning | execution/tenant/user repos | `ILike` on identity columns | ✅ Resolved |
| 14 | 🟠 Medium | engine.go | N+1 query in `getStepExecution` | ✅ Resolved |
| 15 | 🔵 Low | step_execution.go | `ClaimPendingSteps` code is obsolete | ✅ Resolved |
| 16 | 🟠 Medium | job_poller.go | Mixed locking strategies in worker poller | ✅ Resolved |
| 17 | 🟡 Warning | job_poller.go | Zombie goroutines due to `context.Background()` | ✅ Resolved |
| 18 | 🟡 Warning | job_poller.go | Architectural mismatch: Uses Redis Lists instead of Streams | ✅ Resolved (Arch doc updated) |
| 19 | 🔴 Critical | job_poller.go | No recovery mechanism for stuck RUNNING executions | ✅ Resolved |
| 20 | 🟠 Medium | http.go | Idempotency key strategy requires correct hashing | ✅ Resolved |

## Proposed Execution Order

1. **Feature gaps**: #4 (LOOP, WAIT_FOR_EVENT, COMPENSATION)

## Verification Plan

### Automated Tests
- `go test ./internal/worker/service/... -v -count=1` — validate existing engine tests still pass after fixes
- `go test ./internal/repository/... -v` — validate repository changes
- `go vet ./...` — ensure no new issues

### Manual Verification
- Add a unit test for the defer-cancel fix (verify context cancellation timing)
- Add test coverage for `ScriptAction` registration (assert non-nil Docker client or graceful degradation)

---

## Refactoring Plan: Engine State Suspension & Fan-Out

To support the remaining complex control flow requirements (`WAIT_FOR_EVENT` and `LOOP`), the execution engine requires a significant architectural refactor. 

### 1. State Suspension & Rehydration (`WAIT_FOR_EVENT`)
Currently, `RunExecution` expects a workflow to run from start to finish within a single goroutine lifespan. To support pausing for an external event:
* **Suspension:** We will introduce a new sentinel error, `errorx.ErrSuspendExecution`. If `executeStepWithRetry` catches this, it will update the Step and Execution statuses to `SUSPENDED` and gracefully exit the engine goroutine, freeing up the worker.
* **Resumption API:** We will add a `ResumeExecution(ctx, execID, stepID, eventPayload)` method to the engine. This will be callable by the HTTP API when a webhook is received. It will update the target step to `SUCCESS` with the event payload, set the execution back to `PENDING`, and re-queue it in Redis.
* **Rehydration:** When a worker picks up a `PENDING` execution, `RunExecution` will first query the DB for all `SUCCESS` steps belonging to that execution. It will pre-populate the `state` engine with their outputs. During DAG traversal, if it encounters a step that is already `SUCCESS`, it will immediately return and skip it, effectively "resuming" exactly where it left off.

> [!IMPORTANT]
> **Open Question for User:** Do you want the `ResumeExecution` method exposed via a new HTTP/REST endpoint right now (e.g., `POST /api/v1/executions/:id/resume`), or should I just implement the engine capability and leave the API handler for later?

### 2. Dynamic DAG Fan-out (`LOOP`)
True DAG manipulation at runtime is complex and hard to trace. Instead of modifying the DAG, we will implement `LOOP` as a specialized composite step handled natively by `executeStepWithRetry`:
* When `def.Action == "LOOP"`, the engine will parse an `items` array from the parameters.
* It will extract a `sub_action` and `sub_parameters` definition from the step.
* For each item, the engine will inject `{"item": item}` into a local context, evaluate the parameters, and invoke the target Action synchronously (or concurrently via bounded goroutines).
* The results of all iterations will be aggregated into an array and saved as the single output of the `LOOP` step, making it safe and predictable for downstream steps to consume.
