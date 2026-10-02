# Correlated invocations

The built-in manager also implements the optional `RunController` interface.
Existing `Manager` implementations and `RunTask(name)` callers remain compatible.

```go
control := manager.(periodics.RunController)
receipt, err := control.RunTaskWithReceipt("recover_insurance_sales")
if err != nil {
    // A *periodics.RunError with Code "busy" includes CurrentRunID.
    return err
}
result, err := control.WaitRun(ctx, receipt.ID)
if err != nil {
    return err // Waiting cancelled, or receipt unknown/evicted.
}
if result.Status != "succeeded" {
    return fmt.Errorf("invocation %s: %s", receipt.ID, result.Error)
}
```

`GetRun(id)` returns a snapshot of that invocation. Status is `running`,
`succeeded`, `failed`, or `cancelled`; a terminal result includes `finishedAt`.
Receipts are process-local. The manager retains the most recent 256 completed
receipts plus every active receipt. Unknown or evicted IDs return `run_not_found`.

Manual invocations reserve the task atomically and exclude scheduled execution
until completion. Scheduled `EnableSkipIfRunning=false` still permits overlap
among scheduled invocations, but cannot overlap an exclusive manual invocation.
Busy errors include the owning receipt ID; wait for it and explicitly request
a new invocation when testing a subsequent sweep. A completion counter or an
unrelated scheduled success does not establish that your requested sweep ran.

Retries and metrics run inside the owned invocation. `MaxRetries` remains the
total attempt count; non-positive counts and invalid timeouts are rejected with
`invalid_task_config`, rather than reporting success without execution. The
configured timeout is the invocation's context deadline, including retry waits.
An execution that ignores cancellation stays `running` and holds its reservation
until `Execute` exits. Cancelling `WaitRun` cancels only the wait. `Stop(ctx)`
cancels and joins scheduled, startup, and manual executions; if its context
expires, it returns that error and retains ownership of executions still alive.

`RunError.Code` values are `busy`, `task_not_found`, `run_not_found`,
`manager_stopped`, and `invalid_task_config`. Task failure is a terminal
`RunResult`, rather than an error from the wait operation.
