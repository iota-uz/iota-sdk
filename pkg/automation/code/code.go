// Package code runs the JavaScript and TypeScript of Code nodes in an isolated sandbox.
//
// A tenant administrator writes `export default async function run(ctx, env)`. The [Compiler] transpiles it
// with esbuild into a self-contained script; the [Engine] runs that script inside QuickJS-ng compiled to
// WebAssembly (github.com/iota-uz/kafig on wazero): no CGO, no Node, a fresh instance per run, a hard memory
// cap (wasm pages) and a hard deadline (the context). The sandbox has no filesystem, environment, stdout or
// module loader; the network is reachable only through env.fetch and a [Fetcher], and the code sees only the
// secrets the node declared. Everything that leaves the package — output, logs, error messages and stacks — is
// masked first ([Masker]).
//
// The trust boundary is the Go router of host calls: user code can call the sandbox's host functions directly
// with any parameters, so every parameter is validated in Go, fetch counters are kept in Go and the result is
// accepted once.
//
// [Declarations] writes the .d.ts the editor needs to type `ctx` and `env`.
package code

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Lang is the language of a Code node's source.
type Lang string

const (
	// LangTS is TypeScript. Types are stripped, never checked.
	LangTS Lang = "ts"
	// LangJS is JavaScript.
	LangJS Lang = "js"
)

// MaxSourceChars is the longest source a Compiler accepts, in characters.
const MaxSourceChars = 100_000

// Source is the code of a Code node.
type Source struct {
	Code     string // at most MaxSourceChars characters
	Filename string // shown in stacks and compile errors; "code.ts" or "code.js" when empty
	Lang     Lang
}

// Compiled is a Source transpiled into a script the Engine runs. Cached values are shared: never modify one.
type Compiled struct {
	Script    string // an IIFE assigning the global __mod
	SourceMap []byte // source map v3 of Script
	Hash      string // sha256 of the source, its filename and language, and the compiler version
}

// CompileError is a syntax error, a forbidden import or a source that is too long. Line and Column are 1-based
// (Column counts UTF-16 code units, as editors do); both are 0 when the error has no position.
type CompileError struct {
	Message      string
	Line, Column int
}

func (e *CompileError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%d:%d: %s", e.Line, e.Column, e.Message)
	}
	return e.Message
}

// RunError is the failure a run reports when the node cannot be compiled: the Code node's step fails with
// ReasonCompileFailed and the error's position.
func (e *CompileError) RunError() *RunError {
	return &RunError{Reason: ReasonCompileFailed, Message: e.Message, Line: e.Line, Column: e.Column}
}

// Compiler transpiles Code node sources.
type Compiler interface {
	// Compile transpiles the source and checks its syntax without checking types. A static import or
	// require is a *CompileError, as are syntax errors and a source longer than MaxSourceChars.
	Compile(ctx context.Context, src Source) (*Compiled, error)
}

// Engine runs compiled Code nodes.
type Engine interface {
	// Run runs the script once in a fresh sandbox. The error is only for a failure of the host (a closed
	// engine, invalid input, a cancelled context); every failure of the code is in RunResult.Err.
	Run(ctx context.Context, script *Compiled, in RunInput) (*RunResult, error)
	// Close waits for nothing: runs in flight fail. Close releases the compiled runtime and its cache.
	Close(ctx context.Context) error
}

// RunInput is what one run receives.
type RunInput struct {
	// Ctx is the node's context, a JSON object. The sandbox freezes it deeply. Integers beyond 2^53 lose
	// precision in JavaScript: callers pass IDs as strings.
	Ctx json.RawMessage
	// Secrets are the secrets the node declared, by name (^[A-Z][A-Z0-9_]{0,63}$). A declared secret whose
	// value is empty is missing: the run fails with ReasonSecretMissing before the code starts.
	Secrets map[string]string
	// Outcomes are the outcomes the code may return. "failed" is the node's own outcome for every RunError.
	Outcomes []string
	Limits   Limits
	// Fetcher serves env.fetch. With nil every fetch fails with NetworkBlocked.
	Fetcher Fetcher
	// Logger receives env.log lines as they are written, already masked and truncated. It may be nil.
	Logger LogSink
	// Tenant keys the per-tenant concurrency limit; runs with an empty Tenant share one key.
	Tenant string
}

// Limits bound one run. Zero values take the defaults.
type Limits struct {
	Timeout              time.Duration // default 30s, at most 5m
	MaxLogLines          int           // default 1000
	MaxLogLineChars      int           // default 8000
	MaxOutputBytes       int           // default 1 MiB
	MaxFetches           int           // default 50 per run
	MaxConcurrentFetches int           // default 6
}

// Limit defaults and caps.
const (
	DefaultTimeout              = 30 * time.Second
	MaxTimeout                  = 5 * time.Minute
	DefaultMaxLogLines          = 1000
	DefaultMaxLogLineChars      = 8000
	DefaultMaxOutputBytes       = 1 << 20
	DefaultMaxFetches           = 50
	DefaultMaxConcurrentFetches = 6
)

// WithDefaults fills zero and negative limits with the defaults and caps the timeout at MaxTimeout.
func (l Limits) WithDefaults() Limits {
	if l.Timeout <= 0 {
		l.Timeout = DefaultTimeout
	}
	if l.Timeout > MaxTimeout {
		l.Timeout = MaxTimeout
	}
	if l.MaxLogLines <= 0 {
		l.MaxLogLines = DefaultMaxLogLines
	}
	if l.MaxLogLineChars <= 0 {
		l.MaxLogLineChars = DefaultMaxLogLineChars
	}
	if l.MaxOutputBytes <= 0 {
		l.MaxOutputBytes = DefaultMaxOutputBytes
	}
	if l.MaxFetches <= 0 {
		l.MaxFetches = DefaultMaxFetches
	}
	if l.MaxConcurrentFetches <= 0 {
		l.MaxConcurrentFetches = DefaultMaxConcurrentFetches
	}
	return l
}

// OutcomeFailed is the outcome of every run that ends with a RunError.
const OutcomeFailed = "failed"

// RunResult is the result of one run.
type RunResult struct {
	Outcome  string          // one of RunInput.Outcomes, or OutcomeFailed when Err is set
	Output   json.RawMessage // already masked; nil when the code returned no output or failed
	Logs     []string        // already masked
	Duration time.Duration
	Err      *RunError
}

// FailureReason classifies a failed run.
type FailureReason string

const (
	ReasonException      FailureReason = "exception"
	ReasonNetworkBlocked FailureReason = "network_blocked"
	ReasonUnknownOutcome FailureReason = "unknown_outcome"
	ReasonOutputInvalid  FailureReason = "output_invalid" // not JSON, a BigInt, a cycle, over MaxOutputBytes
	ReasonTimeout        FailureReason = "timeout"
	ReasonOutOfMemory    FailureReason = "out_of_memory"
	ReasonStackOverflow  FailureReason = "stack_overflow"
	ReasonCompileFailed  FailureReason = "compile_failed"
	ReasonSecretMissing  FailureReason = "secret_missing"
	ReasonOutcomeUnknown FailureReason = "outcome_unknown" // the run ended without returning a result
)

// RunError is a failure of the code.
type RunError struct {
	Reason       FailureReason
	Message      string // masked
	Stack        string // masked, positions mapped to the source through the source map
	Line, Column int    // 1-based position in the source of the innermost frame of the code; 0 when unknown
}

func (e *RunError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s at %d:%d: %s", e.Reason, e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Reason, e.Message)
}

// Fetcher performs the HTTP requests of env.fetch.
type Fetcher interface {
	// Fetch performs one request, following redirects as req.Redirect says. A request the network policy
	// refuses fails with a *NetworkBlockedError.
	Fetch(ctx context.Context, req FetchRequest) (*FetchResponse, error)
}

// FetchRequest is one env.fetch call.
type FetchRequest struct {
	URL, Method string
	Header      http.Header
	Body        []byte
	Redirect    string // "follow" (default) | "manual" | "error"
}

// FetchResponse is the response env.fetch resolves with.
type FetchResponse struct {
	Status     int
	StatusText string
	Header     http.Header
	Body       []byte // at most 10 MiB after decoding
	URL        string // the URL of the final response
	Redirected bool
}

// NetworkBlockedError is a request the network policy refused.
type NetworkBlockedError struct {
	Host, Address string
	// Reason is "scheme" | "name" | "private_address" | "port" | "redirect_limit", or "disabled" when the
	// run has no Fetcher.
	Reason string
}

func (e *NetworkBlockedError) Error() string {
	target := e.Host
	if e.Address != "" && e.Address != e.Host {
		target += " (" + e.Address + ")"
	}
	switch e.Reason {
	case "scheme":
		return "network blocked: only https URLs are allowed: " + target
	case "name":
		return "network blocked: host name is not allowed: " + target
	case "private_address":
		return "network blocked: host resolves to a private address: " + target
	case "port":
		return "network blocked: port is not allowed: " + target
	case "redirect_limit":
		return "network blocked: too many redirects: " + target
	case "disabled":
		return "network blocked: this node has no network access"
	}
	return "network blocked: " + e.Reason + ": " + target
}

// LogSink receives the lines a run logs.
type LogSink interface{ Log(level, line string) }
