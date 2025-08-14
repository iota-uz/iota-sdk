# Phase 6: Advanced Features & Polish (3 days)

## Overview
This final phase focuses on implementing the advanced execution modes (cron jobs and HTTP endpoints), hardening the system with performance and security enhancements, and adding essential monitoring capabilities. This will transform the script runner from a manual tool into a robust automation platform.

## Background
- Cron scheduling requires a reliable, persistent scheduler that can survive application restarts.
- Dynamic HTTP endpoints must be integrated securely into the existing routing layer.
- Performance optimizations like caching are critical for a multi-tenant environment.
- Monitoring is essential for maintaining the stability and reliability of the script execution engine.

## Task 6.1: Cron Job Scheduler (Day 1)

### Objectives
- Implement a scheduler that executes scripts based on their `cron_expression`.
- Ensure the scheduler is robust and handles application restarts gracefully.
- Provide UI elements to manage the schedule of a script.

### Detailed Steps

#### 1. Choose and Integrate a Cron Library
- Integrate a popular Go cron library like `github.com/robfig/cron/v3`. This library supports the standard cron format and allows for dynamic job management.

#### 2. Create a Scheduler Service
- `modules/scripts/infrastructure/scheduler/cron_scheduler.go`:
  - `CronScheduler` struct that holds a `cron.Cron` instance.
  - `Start()`: On application startup, this method will fetch all active cron scripts from the database and add them as jobs to the cron instance.
  - `AddJob(script.Script)`: A function to add a new script to the scheduler. It will use the script's ID as the job ID.
  - `RemoveJob(scriptID uuid.UUID)`: Removes a job from the scheduler.
  - `UpdateJob(script.Script)`: Removes the old job and adds a new one.
  - The job function itself will call `ExecutionService.Execute(...)` with the appropriate script ID and context.

#### 3. Integrate with ScriptService
- Modify `ScriptService` to interact with the `CronScheduler`:
  - When a script is created or updated with `type=cron`, call `CronScheduler.AddJob` or `CronScheduler.UpdateJob`.
  - When a cron script is deleted or disabled, call `CronScheduler.RemoveJob`.

#### 4. Update UI
- In `edit.templ`, when the script type is "Cron", display an input field for the `cron_expression`.
- Add validation for the cron expression on both the client and server side.

### Testing Requirements
- Integration Test: Create a cron script that runs every second. Start the scheduler and verify that the script is executed multiple times.
- Integration Test: Update the cron expression for a script and verify that the scheduler picks up the new schedule.
- Integration Test: Disable a cron script and verify that it stops executing.

## Task 6.2: Dynamic HTTP Endpoints (Day 2)

### Objectives
- Allow scripts to define and serve custom HTTP endpoints.
- Integrate these dynamic routes into the main application router (`chi`).
- Ensure requests are properly authenticated, authorized, and tenant-scoped.

### Detailed Steps

#### 1. Create an Endpoint Router
- `modules/scripts/infrastructure/scheduler/endpoint_router.go`:
  - `EndpointRouter` struct that will manage the dynamic routes.
  - It will have a generic handler function that can be registered with `chi`.
  - `LoadEndpoints()`: On startup, fetches all active `endpoint` scripts and stores them in a map, keyed by method and path.
  - `AddOrUpdateEndpoint(script.Script)` and `RemoveEndpoint(script.Script)` to manage endpoints at runtime.

#### 2. Implement the Generic Handler
- The generic handler will be registered with `chi` for a wildcard path, e.g., `/api/scripts/*`.
- Inside the handler:
  1.  Extract the HTTP method and path from the request.
  2.  Look up the corresponding script in the `EndpointRouter`'s map.
  3.  If a script is found:
      a.  Perform authentication and authorization checks.
      b.  Create the appropriate execution context.
      c.  Call `ExecutionService.Execute(...)`, passing the request details (headers, body, query params) in the execution parameters.
      d.  The script is expected to return an object with `status`, `headers`, and `body`.
      e.  Use the returned object to construct and send the HTTP response.
  4.  If no script is found, return a 404 Not Found error.

#### 3. Integrate with ScriptService
- Similar to the cron scheduler, `ScriptService` will call the `EndpointRouter`'s methods when endpoint scripts are created, updated, or deleted.

#### 4. Update UI
- In `edit.templ`, when the script type is "Endpoint", display input fields for the `endpoint_path` and `endpoint_method` (dropdown: GET, POST, etc.).

### Testing Requirements
- Integration Test: Create an endpoint script (e.g., `GET /api/scripts/hello`).
- Use an HTTP client to make a request to the endpoint and verify that it returns the expected response from the script.
- Test that requests to non-existent script endpoints return a 404.
- Test that unauthenticated requests are rejected.

## Task 6.3: Performance, Security & Monitoring (Day 3)

### Objectives
- Implement a script compilation cache to improve performance.
- Harden the sandbox with stricter resource limits.
- Add Prometheus metrics for monitoring script execution.

### Detailed Steps

#### 1. Implement Compilation Cache
- `infrastructure/runtime/runtime_manager.go`:
  - Add a `sync.Map` to the `RuntimeManager` to store compiled `goja.Program` objects.
  - The cache key should be the script ID and version.
  - Before executing a script, check if a compiled version exists in the cache. If so, use `vm.RunProgram()`. If not, compile the script using `goja.Compile()`, store it in the cache, and then run it.
  - Invalidate the cache entry when a script's content is updated.

#### 2. Harden the Sandbox
- In `infrastructure/runtime/sandbox.go`:
  - Use `vm.SetMemoryLimit()` to enforce a hard memory cap on each VM instance. This prevents scripts from causing out-of-memory errors.
  - Implement a mechanism to recycle VMs after a certain number of executions to prevent slow memory leaks.

#### 3. Add Prometheus Metrics
- `modules/scripts/metrics/metrics.go`:
  - Define Prometheus counters, gauges, and histograms:
    - `script_executions_total`: Counter for total executions, with labels for `script_id`, `type`, and `status` (success/failed).
    - `script_execution_duration_seconds`: Histogram to track execution latency.
    - `active_scripts_gauge`: Gauge for the number of active scripts.
    - `active_vms_gauge`: Gauge for the number of VMs currently in use from the pool.
- Instrument the `ExecutionService` to update these metrics during the script lifecycle.

### Testing Requirements
- Performance Test: Execute the same script multiple times and verify that the execution time decreases after the first run (due to the compilation cache).
- Security Test: Write a script designed to consume a large amount of memory (e.g., creating a large array in a loop). Verify that the execution is terminated by the memory limit.
- Monitoring Test: Execute several scripts and then scrape the `/metrics` endpoint to verify that the script-related metrics are present and have the correct values.

### Deliverables Checklist
- [ ] Cron scheduler for recurring script execution.
- [ ] Dynamic HTTP endpoint router.
- [ ] UI elements for managing schedules and endpoints.
- [ ] Script compilation cache is implemented and working.
- [ ] Memory limits are enforced on the VM sandbox.
- [ ] Prometheus metrics for executions, performance, and errors.
- [ ] Comprehensive integration tests for all advanced features.
- [ ] Documentation for cron jobs, HTTP endpoints, and monitoring.