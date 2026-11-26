package mcpwiring_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/mcpwiring"
)

type allow struct{}

func (allow) Authorize(ctx context.Context, _ mcp.Principal, _ mcp.Policy) (context.Context, error) {
	return ctx, nil
}

type auditor struct{}

func (auditor) Record(context.Context, mcp.AuditEntry) {}

func (auditor) RecordConfirmed(context.Context, mcp.AuditEntry) error { return nil }

// The component is on when the application gives it a registry, and a registry without the rest of the
// required configuration is an error that names what is missing.
func TestOptionsEnablement(t *testing.T) {
	t.Parallel()
	off := mcpwiring.Options{}
	assert.False(t, off.Enabled())
	require.NoError(t, off.Validate(), "without a registry nothing is required")

	partial := mcpwiring.Options{Registry: mcp.NewRegistry(), Authorizer: allow{}}
	assert.True(t, partial.Enabled())
	err := partial.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Auditor")
	assert.Contains(t, err.Error(), "ApprovalURL")
	assert.NotContains(t, err.Error(), "Authorizer")

	full := mcpwiring.Options{Registry: mcp.NewRegistry(), Authorizer: allow{}, Auditor: auditor{},
		ApprovalURL: func(id string) string { return "/approvals/" + id }}
	require.NoError(t, full.Validate())
}
