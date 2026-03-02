package safety

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsForceEnabled_EnvAndFlag(t *testing.T) {
	t.Setenv("SEED_FORCE", "")
	assert.False(t, IsForceEnabled(RunOptions{}))
	assert.True(t, IsForceEnabled(RunOptions{Force: true}))

	t.Setenv("SEED_FORCE", "1")
	assert.True(t, IsForceEnabled(RunOptions{}))
	t.Setenv("SEED_FORCE", "true")
	assert.True(t, IsForceEnabled(RunOptions{}))
}

func TestEnforceSafety_DestructiveRequiresForce(t *testing.T) {
	t.Setenv("SEED_FORCE", "")
	err := EnforceSafety(RunOptions{}, PreflightResult{
		Operation:     OperationE2EDrop,
		IsDestructive: true,
	}, bytes.NewBufferString("yes\n"), &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force")
}

func TestEnforceSafety_HighRiskRequiresYesOnNonTTY(t *testing.T) {
	t.Setenv("SEED_FORCE", "1")
	stdin := os.Stdin
	defer func() { os.Stdin = stdin }()

	f, err := os.CreateTemp(t.TempDir(), "stdin-*")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	os.Stdin = f

	err = EnforceSafety(RunOptions{}, PreflightResult{
		Risks: []Risk{{Code: "remote_db", Severity: "high", Message: "remote"}},
	}, bytes.NewBufferString("yes\n"), &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestEnforceSafety_YesBypassesHighRiskPrompt(t *testing.T) {
	t.Setenv("SEED_FORCE", "1")
	err := EnforceSafety(RunOptions{Yes: true}, PreflightResult{
		Risks: []Risk{{Code: "remote_db", Severity: "high", Message: "remote"}},
	}, bytes.NewBufferString("no\n"), &bytes.Buffer{})
	require.NoError(t, err)
}
