//go:build unix

package conformance_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/conformance"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/lifecycle"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/manager"
	"github.com/stretchr/testify/require"
)

// TestConformance runs the full suite against a confined Manager. It skips
// when the host cannot confine a kernel, unless PYKERNEL_CONFORMANCE=required
// (set in CI), where a missing boundary fails the test.
func TestConformance(t *testing.T) {
	required := os.Getenv("PYKERNEL_CONFORMANCE") == "required"
	python, err := exec.LookPath("python3")
	if err != nil {
		if required {
			t.Fatal("python3 is required for the conformance suite")
		}
		t.Skip("python3 not available")
	}

	m, err := manager.New(manager.Config{
		PythonPath:   python,
		Policy:       lifecycle.WarmPool(time.Minute, 64),
		EnvAllowlist: []string{"PATH", "LANG"},
		DisposeGrace: 2 * time.Second,
		Limits: isolation.ResourceLimits{
			AddressSpaceBytes: 1 << 30,
			CPUSeconds:        60,
			FileSizeBytes:     32 << 20,
			ProcHeadroom:      32,
			MaxOpenFiles:      128,
			WorkdirBytes:      64 << 20,
			WallClock:         time.Minute,
		},
	})
	if errors.Is(err, isolation.ErrIsolationUnavailable) && !required {
		t.Skipf("sandbox unavailable on this host: %v", err)
	}
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })

	rep := m.Isolation()
	raw, mErr := json.MarshalIndent(rep, "", "  ")
	require.NoError(t, mErr)
	t.Logf("isolation report:\n%s", raw)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	res := conformance.Run(ctx, conformance.Options{Manager: m, WorkRoot: t.TempDir(), ProbeTimeout: 30 * time.Second})
	for _, r := range res.Results {
		for _, c := range r.Checks {
			t.Logf("%-28s %-5t %s — %s", r.Probe, c.OK, c.Name, c.Detail)
		}
		if r.Error != "" {
			t.Logf("%-28s error: %s", r.Probe, r.Error)
		}
	}
	require.NoError(t, res.Err())
	require.Len(t, res.Results, len(conformance.Probes()))
}

// TestCheckReportsVerifiedBoundaries checks isolation.Check against a real
// child: on a capable host every boundary is enabled and verified.
func TestCheckReportsVerifiedBoundaries(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}
	rep := isolation.Check(context.Background(), isolation.CheckOptions{PythonPath: python})
	if rep.Err() != nil && os.Getenv("PYKERNEL_CONFORMANCE") != "required" {
		t.Skipf("sandbox unavailable on this host: %v", rep.Err())
	}
	require.NoError(t, rep.Err())
	require.True(t, rep.Verified)
	require.GreaterOrEqual(t, rep.LandlockABI, isolation.MinLandlockABI)
	for _, b := range isolation.RequiredBoundaries {
		require.True(t, rep.Enabled(b), b)
	}
}
