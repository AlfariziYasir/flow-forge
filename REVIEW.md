# Code Review Feedback

**File/Snippet Reviewed:** Simulated Flawed Workflow Processor

```go
// --- Flawed Snippet ---
func ProcessAllWorkflows(wfs []Workflow) {
	for i := 0; i < len(wfs); i++ {
		go func() {
			val := wfs[i]
			db.Save(&val)
			for j := 0; j < len(val.Steps); j++ {
				go runStep(val.Steps[j])
			}
		}()
	}
}
```

## Feedback Details

Hi there! Thanks for putting this processor together. I was reviewing the code and spotted a few things that could cause some tricky bugs in production, particularly around concurrency. Please see my notes below:

### 1. Loop Variable Capture in Goroutines (Prior to Go 1.22)
By using `i` and `j` inside the `go func()` bodies, you're capturing the loop variables by reference. Because the loop iterates faster than the goroutines spin up, when `wfs[i]` is evaluated, `i` has likely advanced, leading to skipping elements or producing out-of-bounds panics (`panic: runtime error: index out of range`).
**Fix:** Pass the loop variables as arguments into the goroutine closure, or assign them to block-scoped variables.

```go
func ProcessAllWorkflows(wfs []Workflow) {
	for i := range wfs {
		wf := wfs[i] // local scope cap
		go func(w Workflow) {
			db.Save(&w)
			for _, step := range w.Steps {
				s := step
				go runStep(s)
			}
		}(wf)
	}
}
```

### 2. Uncontrolled Goroutine Spawning
You are spawning a goroutine for every single workflow, and another for every single step. In an environment with thousands of workflows, this will cause memory to balloon and overwhelm our database connection pool.
**Fix:** Consider using a worker pool pattern or a semaphore channel (`make(chan struct{}, maxWorkers)`) to bound the concurrency. 

### 3. Lack of Synchronization / WaitGroups
The function `ProcessAllWorkflows` kicks off goroutines and returns immediately. If the main program expects the processing to complete or needs to shut down gracefully, it has no way of waiting.
**Fix:** Utilize a `sync.WaitGroup` to track completion.

### 4. Lack of context.Context and Error Handling
Transactions like `db.Save(&w)` and `runStep(s)` can fail. Currently, errors are completely swallowed, and there's no way to cancel hanging processes.
**Fix:** Pass down `context.Context` from the caller so that operations can respect timeouts/cancellations, and handle your `db.Save` errors by at least logging them or pushing them to an error channel.

Let me know if you want to pair on implementing the worker pool!
