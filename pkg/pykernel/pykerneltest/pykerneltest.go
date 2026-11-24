// Package pykerneltest helps tests run real kernels on hosts with and without
// the OS sandbox.
package pykerneltest

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

// EnvConformance set to "required" makes a missing sandbox fail the test
// instead of falling back (CI sets it).
const EnvConformance = "PYKERNEL_CONFORMANCE"

var (
	once   sync.Once
	report isolation.Report
)

// Sandbox returns the profile a test Manager should use: confined when this
// host can confine a python3 kernel, otherwise the insecure development mode
// (logged). With PYKERNEL_CONFORMANCE=required a host that cannot confine
// fails the test.
func Sandbox(tb testing.TB) isolation.Profile {
	tb.Helper()
	once.Do(func() {
		python, err := exec.LookPath("python3")
		if err != nil {
			return
		}
		report = isolation.Check(context.Background(), isolation.CheckOptions{PythonPath: python})
	})
	if err := report.Err(); err != nil {
		if os.Getenv(EnvConformance) == "required" {
			tb.Fatalf("pykernel sandbox required but unavailable: %v", err)
		}
		tb.Logf("pykernel sandbox unavailable, running unconfined: %v", err)
		return isolation.Profile{Mode: isolation.ModeInsecureDisabled}
	}
	return isolation.Profile{Mode: isolation.ModeRequired}
}
