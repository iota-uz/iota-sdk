package automations_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/mcp"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The module's engine knows the connectors' descriptors: a call's output is masked by the secret
// fields of its connection only, so an MCP tool that echoes the server URL or the auth kind keeps
// the output its pinned schema checked.
func TestEngineMasksOnlyTheSecretFieldsOfAConnection(t *testing.T) {
	src, err := config.Build(static.New(map[string]any{}))
	require.NoError(t, err)
	components := []composition.Component{}
	for _, c := range modules.Components() {
		if c.Descriptor().Name != "automations" {
			components = append(components, c)
		}
	}
	components = append(components, automations.NewComponent())
	f := itf.Setup(t, itf.WithComponents(components...), itf.WithUser(itf.User()), itf.WithSource(src))

	dir := automations.EngineConnectors(f.Container)
	require.NotNil(t, dir, "the engine is given the connector directory")
	conn, ok := dir.Get(mcp.Kind)
	require.True(t, ok)
	secret := connectors.SecretFields(conn.Descriptor())
	require.True(t, secret[mcp.KeyToken])
	require.True(t, secret[mcp.KeyAccessToken])
	require.False(t, secret[mcp.KeyServerURL])
	require.False(t, secret[mcp.KeyAuth])
}
