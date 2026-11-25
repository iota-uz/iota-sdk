package automations_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/httpapi"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
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

// An HTTP API connection's token only authorizes the connection's own requests: a Code node that declares
// CRM__TOKEN does not get it through the default composition's secret resolver.
func TestCodeNodesNeverReadAnHTTPAPIToken(t *testing.T) {
	src, err := config.Build(static.New(map[string]any{"automation.connections.keys": "k1:" + string(bytes.Repeat([]byte("ab"), 32))}))
	require.NoError(t, err)
	f := itf.Setup(t, itf.WithComponents(modules.Components()...), itf.WithUser(itf.User()), itf.WithSource(src))
	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)
	store, err := composition.Resolve[connectors.Store](f.Container)
	require.NoError(t, err)
	const token = "crm-token-must-stay-hidden"
	_, err = store.Save(f.Ctx, connectors.Connection{TenantID: tenant, Kind: httpapi.Kind, Name: "crm", DisplayName: "CRM"},
		connectors.Credentials{"base_url": "https://crm.example.com", "auth": "basic", "username": "bot", "password": token})
	require.NoError(t, err)

	resolver, err := composition.Resolve[host.SecretResolver](f.Container)
	require.NoError(t, err)
	_, err = resolver.Secret(f.Ctx, spi.Scope{Subject: spi.Subject{TenantID: tenant, UserID: 1}}, "crm.password")
	require.Error(t, err, "the resolver serves no authorization-only field")
	v, err := resolver.Secret(f.Ctx, spi.Scope{Subject: spi.Subject{TenantID: tenant, UserID: 1}}, "crm.username")
	assert.True(t, err != nil || v == "", "a field that is not secret is not served either")

	reg, err := composition.Resolve[catalog.Registry](f.Container)
	require.NoError(t, err)
	nt, ok := reg.Node("code")
	require.True(t, ok)
	res, err := nt.(catalog.CallNode).Execute(f.Ctx, catalog.CallInput{
		TenantID: tenant, RunAs: spi.Subject{TenantID: tenant, UserID: 1, Permissions: []string{"Secrets.Use", "Connections.Use"}},
		Deadline: time.Now().Add(time.Minute), IdempotencyKey: uuid.NewString(),
		Params: map[string]any{"lang": "js", "secrets": []any{"CRM__PASSWORD"}, "outcomes": []any{"done"},
			"source": `export default async (ctx, env) => ({ outcome: "done", output: env.secrets.CRM__PASSWORD })`},
	})
	require.NoError(t, err)
	assert.Equal(t, "failed", res.Outcome)
	assert.Equal(t, "secret_missing", res.Output["error"].(map[string]any)["code"])
	raw, _ := json.Marshal(res.Output)
	assert.NotContains(t, string(raw), token)
}
