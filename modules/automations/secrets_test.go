package automations_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// A Code node's declared secret MAIN__KEY is the secret field key of the tenant's connection main,
// through the module's own composition: no application resolver is needed.
func TestCodeSecretsComeFromTheTenantsConnections(t *testing.T) {
	src, err := config.Build(static.New(map[string]any{"automation.connections.keys": "k1:" + string(bytes.Repeat([]byte("ab"), 32))}))
	require.NoError(t, err)
	components := []composition.Component{}
	for _, c := range modules.Components() {
		if c.Descriptor().Name != "automations" {
			components = append(components, c)
		}
	}
	components = append(components, automations.NewComponent(automations.WithConnectors(noop{})))
	f := itf.Setup(t, itf.WithComponents(components...), itf.WithUser(itf.User()), itf.WithSource(src))
	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)
	store, err := composition.Resolve[connectors.Store](f.Container)
	require.NoError(t, err)
	_, err = store.Save(f.Ctx, connectors.Connection{TenantID: tenant, Kind: "noop", Name: "main", DisplayName: "Main"}, connectors.Credentials{"key": "sk-composed-31"})
	require.NoError(t, err)
	// The store keeps rows in the fixture's transaction: the node reads them through it.
	ctx := f.Ctx

	reg, err := composition.Resolve[catalog.Registry](f.Container)
	require.NoError(t, err)
	nt, ok := reg.Node("code")
	require.True(t, ok)
	res, err := nt.(catalog.CallNode).Execute(ctx, catalog.CallInput{
		TenantID: tenant, RunAs: spi.Subject{TenantID: tenant, UserID: 1}, Deadline: time.Now().Add(time.Minute), IdempotencyKey: uuid.NewString(),
		Params: map[string]any{"lang": "js", "secrets": []any{"MAIN__KEY"}, "outcomes": []any{"done"},
			"source": `export default async (ctx, env) => ({ outcome: "done", output: { length: env.secrets.MAIN__KEY.length, value: env.secrets.MAIN__KEY } })`},
	})
	require.NoError(t, err)
	require.Equal(t, "done", res.Outcome, "%v", res.Output)
	out := res.Output["output"].(map[string]any)
	assert.InDelta(t, len("sk-composed-31"), out["length"], 0)
	assert.Equal(t, "***", out["value"], "the secret is masked in the output")
	_ = context.Background
}
