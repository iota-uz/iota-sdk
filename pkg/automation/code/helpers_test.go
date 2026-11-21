package code

import (
	"context"
	"sync"
	"testing"
	"time"
)

var (
	sharedOnce   sync.Once
	sharedEngine Engine
	errShared    error
)

// testEngine is one engine for the package's tests, as in production: one runtime, fresh instances.
func testEngine(tb testing.TB) Engine {
	tb.Helper()
	sharedOnce.Do(func() {
		sharedEngine, errShared = NewEngine(context.Background(), WithMaxConcurrent(16), WithTenantConcurrency(16))
	})
	if errShared != nil {
		tb.Fatalf("NewEngine: %v", errShared)
	}
	return sharedEngine
}

func mustCompile(tb testing.TB, lang Lang, code string) *Compiled {
	tb.Helper()
	c, err := NewCompiler().Compile(context.Background(), Source{Code: code, Lang: lang})
	if err != nil {
		tb.Fatalf("Compile: %v", err)
	}
	return c
}

// runCode compiles and runs code (TypeScript) with in; Outcomes default to ["done"].
func runCode(tb testing.TB, code string, in RunInput) *RunResult {
	tb.Helper()
	if in.Outcomes == nil {
		in.Outcomes = []string{"done"}
	}
	res, err := testEngine(tb).Run(context.Background(), mustCompile(tb, LangTS, code), in)
	if err != nil {
		tb.Fatalf("Run: %v", err)
	}
	return res
}

func wantReason(tb testing.TB, res *RunResult, want FailureReason) {
	tb.Helper()
	if res.Err == nil {
		tb.Fatalf("want %s, got outcome %q output %s", want, res.Outcome, res.Output)
	}
	if res.Err.Reason != want {
		tb.Fatalf("want %s, got %s: %s\n%s", want, res.Err.Reason, res.Err.Message, res.Err.Stack)
	}
	if res.Outcome != OutcomeFailed {
		tb.Fatalf("failed run has outcome %q", res.Outcome)
	}
}

// within fails when a run took longer than its deadline plus the 50 ms the sandbox may overshoot.
func within(tb testing.TB, took, deadline time.Duration) {
	tb.Helper()
	if took > deadline+50*time.Millisecond {
		tb.Fatalf("took %s, deadline %s", took, deadline)
	}
}
