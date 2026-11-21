package code

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	kafig "github.com/iota-uz/kafig/kafig-go"
	"github.com/tetratelabs/wazero"
	"golang.org/x/sync/semaphore"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

//go:embed driver.js
var driverJS string

// Engine defaults.
const (
	// DefaultMemoryPages is the wasm memory cap of one sandbox: 2048 pages of 64 KiB, 128 MiB.
	DefaultMemoryPages = 2048
	// DefaultMemoryBudget is the memory the engine plans for all sandboxes together.
	DefaultMemoryBudget = 4 << 30
	wasmPageBytes       = 64 << 10
)

// SecretName is the pattern of a secret's name.
var SecretName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

// EngineOption configures NewEngine.
type EngineOption func(*engineOptions)

type engineOptions struct {
	pages        uint32
	cacheDir     string
	budget       int64
	maxRuns      int
	tenantRuns   int
	logger       *slog.Logger
	closeOnCtxDn bool
}

// WithMemoryPages sets the wasm memory cap of every sandbox in 64 KiB pages (default 2048, 128 MiB).
func WithMemoryPages(n uint32) EngineOption {
	return func(o *engineOptions) {
		if n > 0 {
			o.pages = n
		}
	}
}

// WithCompilationCacheDir keeps wazero's compiled module in dir, so a restart does not compile it again.
func WithCompilationCacheDir(dir string) EngineOption {
	return func(o *engineOptions) { o.cacheDir = dir }
}

// WithMemoryBudget sets the memory all concurrent runs may plan for, in bytes (default 4 GiB). Without
// WithMaxConcurrent the engine runs at most budget / (4 × memory cap) sandboxes at once.
func WithMemoryBudget(bytes int64) EngineOption {
	return func(o *engineOptions) {
		if bytes > 0 {
			o.budget = bytes
		}
	}
}

// WithMaxConcurrent sets how many runs execute at once, overriding the memory budget.
func WithMaxConcurrent(n int) EngineOption {
	return func(o *engineOptions) {
		if n > 0 {
			o.maxRuns = n
		}
	}
}

// WithTenantConcurrency sets how many runs one tenant executes at once (default half of the engine's, at
// least one), so a tenant cannot take every slot.
func WithTenantConcurrency(n int) EngineOption {
	return func(o *engineOptions) {
		if n > 0 {
			o.tenantRuns = n
		}
	}
}

// WithLogger sets the logger of the engine and of the sandbox runtime.
func WithLogger(l *slog.Logger) EngineOption {
	return func(o *engineOptions) {
		if l != nil {
			o.logger = l
		}
	}
}

type engine struct {
	rt        *kafig.Runtime
	cache     wazero.CompilationCache
	jsHeap    uint32
	logger    *slog.Logger
	global    *semaphore.Weighted
	perTenant int64
	closed    atomic.Bool

	mu      sync.Mutex
	tenants map[string]*tenantSlot
}

type tenantSlot struct {
	sem  *semaphore.Weighted
	refs int
}

// NewEngine compiles the sandbox runtime once for the process. Every Run gets a fresh instance of it.
func NewEngine(ctx context.Context, opts ...EngineOption) (Engine, error) {
	const op = serrors.Op("automation.code.NewEngine")
	o := engineOptions{pages: DefaultMemoryPages, budget: DefaultMemoryBudget, logger: slog.Default(), closeOnCtxDn: true}
	for _, opt := range opts {
		opt(&o)
	}
	capBytes := int64(o.pages) * wasmPageBytes
	if o.maxRuns == 0 {
		o.maxRuns = int(o.budget / (4 * capBytes))
		if o.maxRuns < 1 {
			o.maxRuns = 1
		}
	}
	if o.tenantRuns == 0 {
		o.tenantRuns = max(1, o.maxRuns/2)
	}
	o.tenantRuns = min(o.tenantRuns, o.maxRuns)

	var cache wazero.CompilationCache
	rtOpts := []kafig.RuntimeOption{
		kafig.WithPrelude(driverJS),
		kafig.WithWASMMemoryLimitPages(o.pages),
		kafig.WithCloseOnContextDone(o.closeOnCtxDn),
		kafig.WithLogger(o.logger),
	}
	if o.cacheDir != "" {
		c, err := wazero.NewCompilationCacheWithDir(o.cacheDir)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		cache = c
		rtOpts = append(rtOpts, kafig.WithCompilationCache(c))
	}
	rt, err := kafig.New(ctx, rtOpts...)
	if err != nil {
		if cache != nil {
			_ = cache.Close(ctx)
		}
		return nil, serrors.Wrap(op, err)
	}
	return &engine{
		rt:        rt,
		cache:     cache,
		jsHeap:    uint32(capBytes / 8 * 7),
		logger:    o.logger,
		global:    semaphore.NewWeighted(int64(o.maxRuns)),
		perTenant: int64(o.tenantRuns),
		tenants:   map[string]*tenantSlot{},
	}, nil
}

func (e *engine) Close(ctx context.Context) error {
	const op = serrors.Op("automation.code.Engine.Close")
	if !e.closed.CompareAndSwap(false, true) {
		return nil
	}
	err := e.rt.Close(ctx)
	if e.cache != nil {
		err = errors.Join(err, e.cache.Close(ctx))
	}
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return nil
}

var errClosed = errors.New("the code engine is closed")

// acquire takes a slot of the tenant, then one of the engine; release gives both back.
func (e *engine) acquire(ctx context.Context, tenant string) (func(), error) {
	e.mu.Lock()
	slot := e.tenants[tenant]
	if slot == nil {
		slot = &tenantSlot{sem: semaphore.NewWeighted(e.perTenant)}
		e.tenants[tenant] = slot
	}
	slot.refs++
	e.mu.Unlock()
	drop := func() {
		e.mu.Lock()
		slot.refs--
		if slot.refs == 0 {
			delete(e.tenants, tenant)
		}
		e.mu.Unlock()
	}
	if err := slot.sem.Acquire(ctx, 1); err != nil {
		drop()
		return nil, err
	}
	if err := e.global.Acquire(ctx, 1); err != nil {
		slot.sem.Release(1)
		drop()
		return nil, err
	}
	return func() {
		e.global.Release(1)
		slot.sem.Release(1)
		drop()
	}, nil
}

func (e *engine) Run(ctx context.Context, script *Compiled, in RunInput) (*RunResult, error) {
	const op = serrors.Op("automation.code.Engine.Run")
	if e.closed.Load() {
		return nil, serrors.Wrap(op, errClosed)
	}
	if script == nil || script.Script == "" {
		return nil, serrors.Wrap(op, errors.New("no compiled script"))
	}
	limits := in.Limits.WithDefaults()
	ctxJSON, err := normalizeCtx(in.Ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	names := make([]string, 0, len(in.Secrets))
	for name := range in.Secrets {
		if !SecretName.MatchString(name) {
			return nil, serrors.Wrap(op, fmt.Errorf("invalid secret name %q", name))
		}
		names = append(names, name)
	}
	sort.Strings(names)
	masker := NewMasker(in.Secrets)
	for _, name := range names {
		if in.Secrets[name] == "" {
			return failed(&RunError{Reason: ReasonSecretMissing, Message: "secret " + name + " is not set"}, nil, 0), nil
		}
	}
	secrets := in.Secrets
	if secrets == nil {
		secrets = map[string]string{}
	}
	secretsJSON, err := json.Marshal(secrets)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}

	release, err := e.acquire(ctx, in.Tenant)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer release()
	if e.closed.Load() {
		return nil, serrors.Wrap(op, errClosed)
	}

	start := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, limits.Timeout)
	defer cancel()
	evalCtx, stop := context.WithCancel(runCtx)
	defer stop()
	st := newRunState(in, limits, masker, stop)

	dl, _ := runCtx.Deadline()
	inst, err := e.rt.Instance(evalCtx,
		kafig.WithRouter(st.router()),
		kafig.WithJSMemoryLimit(e.jsHeap),
		// The soft stop: an early, readable error. The hard stop is the context (CloseOnContextDone).
		kafig.WithInterruptCallback(func(uint64, uint64) bool { return !time.Now().Before(dl) }),
	)
	if err != nil {
		if ctx.Err() != nil {
			return nil, serrors.Wrap(op, ctx.Err())
		}
		return nil, serrors.Wrap(op, err)
	}
	var src strings.Builder
	src.Grow(len(script.Script) + len(ctxJSON) + len(secretsJSON) + 64)
	src.WriteString(script.Script)
	src.WriteString("\n;__codeRun(typeof __mod === \"undefined\" ? undefined : __mod,")
	src.Write(quoteJSON(string(ctxJSON)))
	src.WriteString(",")
	src.Write(quoteJSON(string(secretsJSON)))
	src.WriteString(");\n")

	_, evalErr := inst.Eval(evalCtx, src.String(), kafig.WithAsync())
	_ = inst.Close(context.WithoutCancel(ctx))
	duration := time.Since(start)

	res, err := e.result(st, script, evalErr, ctx, runCtx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	res.Duration = duration
	return res, nil
}

func normalizeCtx(raw json.RawMessage) ([]byte, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return []byte("{}"), nil
	}
	if !json.Valid([]byte(trimmed)) || trimmed[0] != '{' {
		return nil, errors.New("RunInput.Ctx must be a JSON object")
	}
	return []byte(trimmed), nil
}

func failed(err *RunError, logs []string, d time.Duration) *RunResult {
	return &RunResult{Outcome: OutcomeFailed, Logs: logs, Duration: d, Err: err}
}

// result turns what the run left into a RunResult. The error is for a cancelled caller only.
func (e *engine) result(st *runState, script *Compiled, evalErr error, parent, runCtx context.Context) (*RunResult, error) {
	st.mu.Lock()
	logs := append([]string(nil), st.logs...)
	st.mu.Unlock()
	timedOut := errors.Is(runCtx.Err(), context.DeadlineExceeded)
	fail := func(err *RunError) (*RunResult, error) {
		err.Message = st.masker.Mask(err.Message)
		err.Stack = st.masker.Mask(err.Stack)
		return failed(err, logs, 0), nil
	}

	if rep := st.reported(); rep != nil {
		switch {
		case rep.Error != nil:
			return fail(e.jsFailure(st, script, rep.Error.Name, rep.Error.Message, rep.Error.Stack, timedOut))
		case rep.Invalid != nil:
			return fail(&RunError{Reason: ReasonOutputInvalid, Message: *rep.Invalid})
		}
		if rep.Outcome == nil || !slices.Contains(st.outcomes, *rep.Outcome) {
			got := "no outcome"
			if rep.Outcome != nil {
				got = fmt.Sprintf("%q", *rep.Outcome)
			}
			return fail(&RunError{Reason: ReasonUnknownOutcome, Message: fmt.Sprintf("the code returned %s; allowed: %s", got, strings.Join(quoteAll(st.outcomes), ", "))})
		}
		res := &RunResult{Outcome: *rep.Outcome, Logs: logs}
		if rep.Output != nil {
			out := []byte(*rep.Output)
			if len(out) > st.limits.MaxOutputBytes {
				return fail(&RunError{Reason: ReasonOutputInvalid, Message: fmt.Sprintf("output is %d bytes, at most %d", len(out), st.limits.MaxOutputBytes)})
			}
			if !json.Valid(out) {
				return fail(&RunError{Reason: ReasonOutputInvalid, Message: "output is not JSON"})
			}
			res.Output = st.masker.MaskJSON(out)
		}
		return res, nil
	}

	if evalErr == nil {
		if timedOut {
			return fail(timeoutError(st))
		}
		return fail(&RunError{Reason: ReasonOutcomeUnknown, Message: "the code finished without a result: a promise never settled"})
	}
	if errors.Is(parent.Err(), context.Canceled) {
		return nil, parent.Err()
	}
	var jsErr *kafig.JsError
	if errors.As(evalErr, &jsErr) {
		stack := ""
		if jsErr.Stack != nil {
			stack = *jsErr.Stack
		}
		return fail(e.jsFailure(st, script, jsErr.Name, jsErr.Message, stack, timedOut))
	}
	var rtErr *kafig.RuntimeError
	if errors.As(evalErr, &rtErr) {
		switch rtErr.Code {
		case kafig.ErrorCodeMemoryLimitExceeded:
			return fail(oomError())
		case kafig.ErrorCodeStackOverflow:
			return fail(&RunError{Reason: ReasonStackOverflow, Message: "maximum call stack size exceeded"})
		}
	}
	if timedOut {
		return fail(timeoutError(st))
	}
	msg := evalErr.Error()
	switch {
	case strings.Contains(msg, "out of memory"), strings.Contains(msg, "memory_limit"):
		return fail(oomError())
	case rtErr != nil && (rtErr.Code == kafig.ErrorCodeCPULimitExceeded || rtErr.Code == kafig.ErrorCodeCanceled):
		return fail(&RunError{Reason: ReasonOutcomeUnknown, Message: "the code was interrupted before it returned a result"})
	}
	e.logger.Warn("automation code: sandbox aborted", slog.String("error", st.masker.Mask(msg)))
	return fail(&RunError{Reason: ReasonException, Message: "the sandbox aborted: " + msg})
}

func timeoutError(st *runState) *RunError {
	return &RunError{Reason: ReasonTimeout, Message: fmt.Sprintf("the code did not finish within %s", st.limits.Timeout)}
}

func oomError() *RunError {
	return &RunError{Reason: ReasonOutOfMemory, Message: "the code ran out of memory"}
}

// jsFailure classifies an exception the code threw or the sandbox raised in it.
func (e *engine) jsFailure(st *runState, script *Compiled, name, message, stack string, timedOut bool) *RunError {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "stack size exceeded"), strings.Contains(lower, "stack overflow"):
		return &RunError{Reason: ReasonStackOverflow, Message: "maximum call stack size exceeded"}
	case name == "InternalError" && strings.Contains(lower, "out of memory"):
		return oomError()
	case name == "InternalError" && lower == "interrupted":
		if timedOut {
			return timeoutError(st)
		}
		return &RunError{Reason: ReasonOutcomeUnknown, Message: "the code was interrupted before it returned a result"}
	}
	reason := ReasonException
	if name == "NetworkBlocked" && st.blocked.Load() {
		reason = ReasonNetworkBlocked
	}
	full := message
	if name != "" {
		full = name + ": " + message
	}
	out := &RunError{Reason: reason, Message: full}
	filename := "code"
	sm := parseSourceMap(script.SourceMap)
	if sm != nil && sm.source != "" {
		filename = sm.source
	}
	out.Stack, out.Line, out.Column = mapStack(stack, sm, filename, strings.Count(script.Script, "\n")+1)
	return out
}

func quoteAll(list []string) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}
