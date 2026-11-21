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
	sharedErr    error
)

// testEngine is one engine for the package's tests, as in production: one runtime, fresh instances.
func testEngine(t testing.TB) Engine {
	t.Helper()
	sharedOnce.Do(func() {
		sharedEngine, sharedErr = NewEngine(context.Background(), WithMaxConcurrent(16), WithTenantConcurrency(16))
	})
	if sharedErr != nil {
		t.Fatalf("NewEngine: %v", sharedErr)
	}
	return sharedEngine
}

func mustCompile(t testing.TB, lang Lang, code string) *Compiled {
	t.Helper()
	c, err := NewCompiler().Compile(context.Background(), Source{Code: code, Lang: lang})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return c
}

// runCode compiles and runs code (TypeScript) with in; Outcomes default to ["done"].
func runCode(t testing.TB, code string, in RunInput) *RunResult {
	t.Helper()
	if in.Outcomes == nil {
		in.Outcomes = []string{"done"}
	}
	res, err := testEngine(t).Run(context.Background(), mustCompile(t, LangTS, code), in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res
}

func wantReason(t testing.TB, res *RunResult, want FailureReason) {
	t.Helper()
	if res.Err == nil {
		t.Fatalf("want %s, got outcome %q output %s", want, res.Outcome, res.Output)
	}
	if res.Err.Reason != want {
		t.Fatalf("want %s, got %s: %s\n%s", want, res.Err.Reason, res.Err.Message, res.Err.Stack)
	}
	if res.Outcome != OutcomeFailed {
		t.Fatalf("failed run has outcome %q", res.Outcome)
	}
}

// within fails when a run took longer than its deadline plus the 50 ms the sandbox may overshoot.
func within(t testing.TB, took, deadline time.Duration) {
	t.Helper()
	if took > deadline+50*time.Millisecond {
		t.Fatalf("took %s, deadline %s", took, deadline)
	}
}
