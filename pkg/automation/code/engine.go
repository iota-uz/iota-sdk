package code

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
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

	lru "github.com/hashicorp/golang-lru/v2"
	kafig "github.com/iota-uz/kafig/kafig-go"
	"github.com/tetratelabs/wazero"
	"golang.org/x/sync/semaphore"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

//go:embed driver.js
var driverJS string

// driverLines is how many lines the driver takes in front of a script in the program a run evaluates.
var driverLines = strings.Count(driverJS, "\n")

// bytecodeCacheSize is how many compiled programs (driver + script) the engine keeps.
const bytecodeCacheSize = 64

// compileTimeout bounds compiling a program to bytecode; it runs apart from any one run's deadline.
const compileTimeout = 10 * time.Second

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
	queueTimeout time.Duration
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

// WithTenantConcurrency sets how many runs one tenant executes at once (default a quarter of the engine's,
// at least one), so a tenant cannot take every slot.
func WithTenantConcurrency(n int) EngineOption {
	return func(o *engineOptions) {
		if n > 0 {
			o.tenantRuns = n
		}
	}
}

// DefaultQueueTimeout is how long a run waits for a free slot before Run returns ErrBusy.
const DefaultQueueTimeout = time.Minute

// ErrBusy is returned by Run when no slot of the engine or the tenant freed up within the queue timeout.
var ErrBusy = errors.New("the code engine is busy")

// WithQueueTimeout sets how long a run waits for a slot (default one minute).
func WithQueueTimeout(d time.Duration) EngineOption {
	return func(o *engineOptions) {
		if d > 0 {
			o.queueTimeout = d
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
	queueWait time.Duration
	closed    atomic.Bool

	mu      sync.Mutex
	tenants map[string]*tenantSlot

	// bytecode keeps the QuickJS bytecode of programs this runtime compiled from source, by sha256 of the
	// program; kafig seals it so no other bytecode is ever evaluated.
	bytecode  *lru.Cache[string, []byte]
	compileMu sync.Mutex
	compiling map[string]*compileCall
}

type tenantSlot struct {
	sem  *semaphore.Weighted
	refs int
}

// NewEngine compiles the sandbox runtime once for the process. Every Run gets a fresh instance of it.
func NewEngine(ctx context.Context, opts ...EngineOption) (Engine, error) {
	const op = serrors.Op("automation.code.NewEngine")
	o := engineOptions{
		pages: DefaultMemoryPages, budget: DefaultMemoryBudget, logger: slog.Default(), closeOnCtxDn: true,
		queueTimeout: DefaultQueueTimeout,
	}
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
		o.tenantRuns = max(1, o.maxRuns/4)
	}
	o.tenantRuns = min(o.tenantRuns, o.maxRuns)

	var cache wazero.CompilationCache
	rtOpts := []kafig.RuntimeOption{
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
	bytecode, err := lru.New[string, []byte](bytecodeCacheSize)
	if err != nil {
		_ = rt.Close(ctx)
		return nil, serrors.Wrap(op, err)
	}
	return &engine{
		bytecode:  bytecode,
		rt:        rt,
		cache:     cache,
		jsHeap:    uint32(capBytes / 8 * 7),
		logger:    o.logger,
		global:    semaphore.NewWeighted(int64(o.maxRuns)),
		perTenant: int64(o.tenantRuns),
		queueWait: o.queueTimeout,
		tenants:   map[string]*tenantSlot{},
		compiling: map[string]*compileCall{},
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
	if slices.Contains(in.Outcomes, OutcomeFailed) {
		return nil, serrors.Wrap(op, errors.New(`"failed" is the node's own outcome, not one the code returns`))
	}
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

	queueCtx, queueCancel := context.WithTimeout(ctx, e.queueWait)
	release, err := e.acquire(queueCtx, in.Tenant)
	queueCancel()
	if err != nil {
		if ctx.Err() == nil {
			err = ErrBusy
		}
		return nil, serrors.Wrap(op, err)
	}
	// A compile this run started and left unfinished keeps the run's slots until it ends: the compile holds
	// a sandbox of its own, which the engine's memory budget must still count.
	var keepSlots <-chan struct{}
	defer func() {
		if keepSlots == nil {
			release()
			return
		}
		go func() {
			<-keepSlots
			release()
		}()
	}()
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
	st.timeout = dl.Sub(start) // the parent's deadline may be the earlier one
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
	program := driverJS + script.Script
	var evalErr error
	bc, unfinished := e.compiled(evalCtx, program)
	keepSlots = unfinished
	if bc != nil {
		_, evalErr = inst.EvalCompiled(evalCtx, bc, kafig.WithAsync())
	} else {
		_, evalErr = inst.Eval(evalCtx, program, kafig.WithAsync())
	}
	if evalErr == nil {
		var call strings.Builder
		call.Grow(len(ctxJSON) + len(secretsJSON) + 64)
		call.WriteString("__codeRun(typeof __mod === \"undefined\" ? undefined : __mod,")
		call.Write(quoteJSON(string(ctxJSON)))
		call.WriteString(",")
		call.Write(quoteJSON(string(secretsJSON)))
		call.WriteString(",")
		call.Write(quoteJSON(string(driverLimits(limits, in.Secrets))))
		call.WriteString(")")
		_, evalErr = inst.Eval(evalCtx, call.String(), kafig.WithAsync())
	}
	_ = inst.Close(context.WithoutCancel(ctx))
	duration := time.Since(start)
	if e.closed.Load() {
		return nil, serrors.Wrap(op, errClosed)
	}

	res, err := e.result(st, script, evalErr, ctx, runCtx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	res.Duration = duration
	return res, nil
}

// compileCall is one compile in progress; done closes when it ends, with bc set on success.
type compileCall struct {
	done chan struct{}
	bc   []byte
}

// compiled returns the sealed bytecode of a program, compiling it on first use. The compile has its own
// deadline and is shared by every run waiting for the same program; each run stops waiting at its own
// deadline, and then evaluates the program from source. When the run that started the compile stops
// waiting before it ends, the second result is the compile's done channel: that run keeps its slots until then.
func (e *engine) compiled(ctx context.Context, program string) ([]byte, <-chan struct{}) {
	sum := sha256.Sum256([]byte(program))
	key := hex.EncodeToString(sum[:])
	if bc, ok := e.bytecode.Get(key); ok {
		return bc, nil
	}
	e.compileMu.Lock()
	call, running := e.compiling[key]
	if !running {
		call = &compileCall{done: make(chan struct{})}
		e.compiling[key] = call
		go func() {
			cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), compileTimeout)
			defer cancel()
			if out, err := e.rt.Compile(cctx, program); err == nil {
				call.bc = out
				e.bytecode.Add(key, out)
			}
			e.compileMu.Lock()
			delete(e.compiling, key)
			e.compileMu.Unlock()
			close(call.done)
		}()
	}
	e.compileMu.Unlock()
	select {
	case <-call.done:
		return call.bc, nil
	case <-ctx.Done():
		if running {
			return nil, nil
		}
		return nil, call.done
	}
}

// driverLimits are the bounds the driver applies before a call reaches the host. A log line keeps room for
// the longest escaped form of a secret, so masking never sees a secret cut in half.
func driverLimits(l Limits, secrets map[string]string) []byte {
	longest := 0
	for _, v := range secrets {
		longest = max(longest, len(v))
	}
	return fmt.Appendf(nil, `{"maxFetches":%d,"maxLogLines":%d,"maxLogChars":%d,"maxOutputBytes":%d}`,
		l.MaxFetches, l.MaxLogLines, l.MaxLogLineChars+12*longest+16, l.MaxOutputBytes)
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
	dl, _ := runCtx.Deadline()
	// The interrupt callback can stop the code a moment before the context's timer fires.
	timedOut := errors.Is(runCtx.Err(), context.DeadlineExceeded) || !time.Now().Before(dl)
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
				got, _ = truncateRunes(string(quoteJSON(*rep.Outcome)), 200)
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
			masked := st.masker.MaskJSON(out)
			// Masking can lengthen the output (a short secret becomes "***"): the cap applies to what leaves.
			if len(masked) > st.limits.MaxOutputBytes {
				return fail(&RunError{Reason: ReasonOutputInvalid, Message: fmt.Sprintf("masked output is %d bytes, at most %d", len(masked), st.limits.MaxOutputBytes)})
			}
			res.Output = masked
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
		case kafig.ErrorCodeCPULimitExceeded, kafig.ErrorCodeRuntimeError, kafig.ErrorCodeDeadlineExceeded, kafig.ErrorCodeCanceled:
			// classified below, with the deadline
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
	return &RunError{Reason: ReasonTimeout, Message: fmt.Sprintf("the code did not finish within %s", st.timeout.Round(time.Millisecond))}
}

func oomError() *RunError {
	return &RunError{Reason: ReasonOutOfMemory, Message: "the code ran out of memory"}
}

// jsFailure classifies an exception the code threw or the sandbox raised in it.
func (e *engine) jsFailure(st *runState, script *Compiled, name, message, stack string, timedOut bool) *RunError {
	lower := strings.ToLower(message)
	switch {
	case (name == "RangeError" || name == "InternalError") &&
		(lower == "maximum call stack size exceeded" || lower == "stack overflow"):
		return &RunError{Reason: ReasonStackOverflow, Message: "maximum call stack size exceeded"}
	case name == "InternalError" && lower == "out of memory":
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
	out.Stack, out.Line, out.Column = mapStack(stack, sm, filename, driverLines, strings.Count(script.Script, "\n")+1)
	return out
}

func quoteAll(list []string) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = string(quoteJSON(s))
	}
	return out
}
