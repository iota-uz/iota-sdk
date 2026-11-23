package isolation_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseChildReport_ListsMissingBoundaries(t *testing.T) {
	t.Parallel()
	rep, err := isolation.ParseChildReport([]byte(`{"landlock_abi": 5, "boundaries": {
		"no_new_privs": {"enabled": true}, "no_capabilities": {"enabled": true},
		"landlock_fs": {"enabled": false, "detail": "landlock ABI 5 < required 6"},
		"seccomp": {"enabled": true}, "rlimits": {"enabled": true}, "nproc": {"enabled": true}}}`))
	require.NoError(t, err)
	assert.True(t, rep.Verified)
	assert.Equal(t, 5, rep.LandlockABI)
	assert.True(t, rep.Enabled(isolation.BoundarySeccomp))
	assert.False(t, rep.Complete())

	missing := make([]isolation.Boundary, 0, len(rep.Missing()))
	for _, m := range rep.Missing() {
		missing = append(missing, m.Boundary)
	}
	assert.Equal(t, []isolation.Boundary{
		isolation.BoundaryLandlockFS, isolation.BoundaryLandlockNet, isolation.BoundaryLandlockScope, isolation.BoundaryWorkdirQuota,
	}, missing)

	err = rep.Err()
	require.Error(t, err)
	require.ErrorIs(t, err, isolation.ErrIsolationUnavailable)
	assert.Contains(t, err.Error(), "landlock ABI 5 < required 6")
}

func TestReport_ErrRequiresAVerifiedChild(t *testing.T) {
	t.Parallel()
	rep := isolation.Report{}
	for _, b := range isolation.RequiredBoundaries {
		rep.Boundaries = append(rep.Boundaries, isolation.BoundaryStatus{Boundary: b, Enabled: true})
	}
	assert.True(t, rep.Complete())
	require.ErrorIs(t, rep.Err(), isolation.ErrIsolationUnavailable, "host support alone is not proof")
	rep.Verified = true
	require.NoError(t, rep.Err())
}

func TestLimits_WithDefaultsFillsOnlyZeroFields(t *testing.T) {
	t.Parallel()
	l := isolation.ResourceLimits{CPUSeconds: 5, MaxProcs: 10}.WithDefaults()
	d := isolation.DefaultLimits()
	assert.Equal(t, uint64(5), l.CPUSeconds)
	assert.Equal(t, uint64(10), l.MaxProcs)
	assert.Zero(t, l.ProcHeadroom, "an explicit MaxProcs wins over the headroom")
	assert.Equal(t, d.AddressSpaceBytes, l.AddressSpaceBytes)
	assert.Equal(t, d.WorkdirBytes, l.WorkdirBytes)
	assert.Equal(t, d.WallClock, l.WallClock)
}
