package testenv

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JobRequest is passed to a consumer job and its conditions. The job context
// carries the same Correlation so downstream records and fakes stay scoped.
type JobRequest struct {
	ExecutionID string               `json:"executionId"`
	ScopeID     string               `json:"scopeId"`
	Job         string               `json:"job"`
	Refs        map[string]EntityRef `json:"refs"`
}

// JobFunc runs the real handler or worker; it must not fake its result.
type JobFunc func(context.Context, JobRequest) error

// ConditionFunc observes durable state produced by a job. Query with an
// explicit asOf (Registry clock) where temporal semantics are checked.
type ConditionFunc func(context.Context, JobRequest) (bool, error)

// JobDefinition declares a consumer job. Dedicated jobs touch environment-wide
// state (periodic tasks, settings) and are refused in shared environments.
type JobDefinition struct {
	Name      string `json:"name"`
	Dedicated bool   `json:"dedicated"`
}

// ConditionCompleted is reached when the execution itself succeeded.
const ConditionCompleted = "completed"

const (
	MinWaitTimeoutMs = 1
	MaxWaitTimeoutMs = 120000
)

// ExecutionStatus is the observed state of a job execution.
type ExecutionStatus struct {
	ExecutionID string     `json:"executionId"`
	ScopeID     string     `json:"scopeId"`
	Job         string     `json:"job"`
	State       string     `json:"state"` // running|succeeded|failed|canceled
	Error       string     `json:"error,omitempty"`
	StartedAt   time.Time  `json:"startedAt"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
}

// WaitResult reports whether a condition was reached, with diagnostics.
type WaitResult struct {
	Outcome            string          `json:"outcome"` // reached|failed|timeout|canceled
	Condition          string          `json:"condition"`
	Execution          ExecutionStatus `json:"execution"`
	Checks             int             `json:"checks"`
	LastConditionError string          `json:"lastConditionError,omitempty"`
	ElapsedMs          int64           `json:"elapsedMs"`
}

type execution struct {
	request JobRequest
	cancel  context.CancelFunc
	done    chan struct{}
	status  ExecutionStatus
}

// Jobs runs consumer-registered jobs inside owned scopes and waits for
// consumer-registered conditions. It is attached to the registry, so scope
// disposal cancels and joins the scope's executions.
type Jobs struct {
	registry   *Registry
	mu         sync.Mutex
	jobs       map[string]registeredJob
	conditions map[string]ConditionFunc
	executions map[string]*execution
	// PollInterval bounds how often conditions over async state are re-read.
	PollInterval time.Duration
}

type registeredJob struct {
	definition JobDefinition
	run        JobFunc
}

// NewJobs attaches a job controller to r.
func NewJobs(r *Registry) (*Jobs, error) {
	j := &Jobs{registry: r, jobs: map[string]registeredJob{}, conditions: map[string]ConditionFunc{}, executions: map[string]*execution{}, PollInterval: 10 * time.Millisecond}
	if err := r.Attach("jobs", j); err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.jobs = j
	r.mu.Unlock()
	return j, nil
}

// Register adds a job handler.
func (j *Jobs) Register(definition JobDefinition, run JobFunc) error {
	if definition.Name == "" || run == nil {
		return failure("invalid_input", "job name and handler required")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, exists := j.jobs[definition.Name]; exists {
		return failure("scope_conflict", "job already registered")
	}
	j.jobs[definition.Name] = registeredJob{definition: definition, run: run}
	return nil
}

// RegisterCondition adds a named condition usable with Wait.
func (j *Jobs) RegisterCondition(name string, condition ConditionFunc) error {
	if name == "" || name == ConditionCompleted || condition == nil {
		return failure("invalid_input", "condition name and function required")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, exists := j.conditions[name]; exists {
		return failure("scope_conflict", "condition already registered")
	}
	j.conditions[name] = condition
	return nil
}

// Definitions lists registered jobs and conditions.
func (j *Jobs) Definitions() ([]JobDefinition, []string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	jobs := make([]JobDefinition, 0, len(j.jobs))
	for _, job := range j.jobs {
		jobs = append(jobs, job.definition)
	}
	sort.Slice(jobs, func(a, b int) bool { return jobs[a].Name < jobs[b].Name })
	conditions := []string{ConditionCompleted}
	for name := range j.conditions {
		conditions = append(conditions, name)
	}
	sort.Strings(conditions[1:])
	return jobs, conditions
}

// Run starts job in scopeID and returns its execution ID. The execution
// outlives ctx; it ends when the job returns or the scope is disposed.
func (j *Jobs) Run(ctx context.Context, scopeID, job string, refs map[string]EntityRef) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", failure("timeout", err.Error())
	}
	if err := j.registry.owned(scopeID); err != nil {
		return "", err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	registered, ok := j.jobs[job]
	if !ok {
		return "", failure("unknown_scenario", "unknown job: "+job)
	}
	if registered.definition.Dedicated && !j.registry.dedicated {
		return "", failure("missing_capability", "job requires a dedicated environment")
	}
	copied := make(map[string]EntityRef, len(refs))
	for name, ref := range refs {
		copied[name] = ref
	}
	request := JobRequest{ExecutionID: uuid.NewString(), ScopeID: scopeID, Job: job, Refs: copied}
	runCtx, cancel := context.WithCancel(WithCorrelation(context.Background(), Correlation{ScopeID: scopeID, ExecutionID: request.ExecutionID}))
	e := &execution{request: request, cancel: cancel, done: make(chan struct{}), status: ExecutionStatus{ExecutionID: request.ExecutionID, ScopeID: scopeID, Job: job, State: "running", StartedAt: time.Now().UTC()}}
	j.executions[request.ExecutionID] = e
	go func() {
		err := runJob(runCtx, registered.run, request)
		finished := time.Now().UTC()
		j.mu.Lock()
		e.status.FinishedAt = &finished
		switch {
		case err == nil:
			e.status.State = "succeeded"
		case runCtx.Err() != nil:
			e.status.State, e.status.Error = "canceled", err.Error()
		default:
			e.status.State, e.status.Error = "failed", err.Error()
		}
		j.mu.Unlock()
		cancel()
		close(e.done)
	}()
	return request.ExecutionID, nil
}

func runJob(ctx context.Context, run JobFunc, request JobRequest) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.New("job panicked")
		}
	}()
	return run(ctx, request)
}

// Status returns the state of an execution owned by scopeID.
func (j *Jobs) Status(scopeID, executionID string) (ExecutionStatus, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	e := j.executions[executionID]
	if e == nil || e.request.ScopeID != scopeID {
		return ExecutionStatus{}, failure("scope_conflict", "execution is not owned by this scope")
	}
	return e.status, nil
}

// Wait blocks until condition is reached, the execution fails, timeoutMs
// passes (1..120000) or ctx is canceled. Reached returns a nil error; failed
// and timeout return a typed error whose Details carry the WaitResult.
// Cancellation stops waiting and reports the execution state without error.
func (j *Jobs) Wait(ctx context.Context, scopeID, executionID, condition string, timeoutMs int) (WaitResult, error) {
	if timeoutMs < MinWaitTimeoutMs || timeoutMs > MaxWaitTimeoutMs {
		return WaitResult{}, failure("invalid_input", "timeoutMs must be an integer from 1 to 120000")
	}
	if condition == "" {
		condition = ConditionCompleted
	}
	j.mu.Lock()
	e := j.executions[executionID]
	check := j.conditions[condition]
	poll := j.PollInterval
	j.mu.Unlock()
	if e == nil || e.request.ScopeID != scopeID {
		return WaitResult{}, failure("scope_conflict", "execution is not owned by this scope")
	}
	if condition != ConditionCompleted && check == nil {
		return WaitResult{}, failure("unknown_scenario", "unknown condition: "+condition)
	}
	if poll <= 0 {
		poll = 10 * time.Millisecond
	}
	started := time.Now()
	deadline := time.NewTimer(time.Duration(timeoutMs) * time.Millisecond)
	defer deadline.Stop()
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	result := WaitResult{Condition: condition}
	finish := func(outcome string) (WaitResult, error) {
		result.Outcome = outcome
		result.ElapsedMs = time.Since(started).Milliseconds()
		j.mu.Lock()
		result.Execution = e.status
		j.mu.Unlock()
		switch outcome {
		case "failed":
			return result, &Error{Code: "execution_failed", Message: "condition " + condition + " failed", Details: result}
		case "timeout":
			return result, &Error{Code: "timeout", Message: "condition " + condition + " not reached", Details: result}
		}
		return result, nil
	}
	done := e.done
	for {
		finished := false
		select {
		case <-done:
			finished = true
		default:
		}
		state := j.state(e)
		if state == "failed" || state == "canceled" {
			return finish("failed")
		}
		if condition == ConditionCompleted {
			if state == "succeeded" {
				return finish("reached")
			}
		} else {
			result.Checks++
			reached, err := check(WithCorrelation(ctx, Correlation{ScopeID: scopeID, ExecutionID: executionID}), e.request)
			if err != nil {
				if ctx.Err() != nil {
					return finish("canceled")
				}
				result.LastConditionError = err.Error()
				return finish("failed")
			}
			if reached {
				return finish("reached")
			}
		}
		if finished {
			// The job is done; downstream async work may still converge, so
			// keep observing until the deadline rather than failing early.
			done = nil
		}
		select {
		case <-ctx.Done():
			return finish("canceled")
		case <-deadline.C:
			return finish("timeout")
		case <-done:
		case <-ticker.C:
		}
	}
}

func (j *Jobs) state(e *execution) string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return e.status.State
}

// ForgetScope cancels the scope's executions and waits for them to exit.
func (j *Jobs) ForgetScope(ctx context.Context, scopeID string) error {
	j.mu.Lock()
	var owned []*execution
	for _, e := range j.executions {
		if e.request.ScopeID == scopeID {
			owned = append(owned, e)
			e.cancel()
		}
	}
	j.mu.Unlock()
	for _, e := range owned {
		select {
		case <-e.done:
		case <-ctx.Done():
			return failure("cleanup_failed", "job "+e.request.Job+" did not stop: "+ctx.Err().Error())
		}
	}
	j.mu.Lock()
	for _, e := range owned {
		delete(j.executions, e.request.ExecutionID)
	}
	j.mu.Unlock()
	return nil
}
