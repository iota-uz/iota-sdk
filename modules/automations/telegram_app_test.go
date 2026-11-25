package automations_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/graphs"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/integrations"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// The application's automations module registers the Telegram bot connector itself (#1439): a tenant finds it
// among the integrations, and once a bot is connected its nodes and trigger are in the editor's catalog.
func TestTelegramIsInTheCatalogOfTheApplication(t *testing.T) {
	s, container := app(t, allAutomations...)
	dir, err := composition.Resolve[connectors.Directory](container)
	require.NoError(t, err)
	conn, ok := dir.Get(telegram.Kind)
	require.True(t, ok, "the connector is registered")
	assert.IsType(t, &telegram.Connector{}, conn)

	var list integrations.ListResponse
	require.Nil(t, call(t, s, integrations.DefaultPath+"/rpc", integrations.MethodList, map[string]any{}, &list).Error)
	kinds := map[string]bool{}
	for _, c := range list.Connectors {
		kinds[c.Kind] = true
	}
	assert.True(t, kinds[string(telegram.Kind)], "the integrations screen offers it: %v", kinds)

	// Once the tenant connected a bot, the editor offers its nodes and its trigger.
	store, err := composition.Resolve[connectors.ConnectionStore](container)
	require.NoError(t, err)
	_, err = store.Save(context.Background(), connectors.Connection{TenantID: s.Env().Tenant.ID, Kind: telegram.Kind, Name: "bot", DisplayName: "@tenant_bot", Status: connectors.StatusConnected},
		connectors.Credentials{"token": "123456:ABCDEFGHIJKLMNOPQRSTUVWXYZ"})
	require.NoError(t, err)
	var cat graphs.CatalogResponse
	require.Nil(t, rpc(t, s, graphs.MethodCatalog, map[string]any{}, &cat).Error)
	types := map[string]bool{}
	for _, n := range cat.Catalog.Nodes {
		types[n.Type] = true
	}
	events := map[string]bool{}
	for _, e := range cat.Catalog.Events {
		events[e.Key] = true
	}
	for _, typ := range []string{"telegram.send", "telegram.ask", "telegram.read"} {
		assert.True(t, types[typ], "the editor's catalog has %s", typ)
	}
	assert.True(t, events["telegram.message_received.v1"], "and the trigger")
}

// The editor's pickers of fields with x-options-source ask the step's connection (#1439): an author gets what its
// connector offers, a connection that is not the tenant's is not found, and a platform credential offers none.
func TestTheEditorSearchesTheOptionsOfAConnection(t *testing.T) {
	s, container := app(t, allAutomations...)
	store, err := composition.Resolve[connectors.ConnectionStore](container)
	require.NoError(t, err)
	conn, err := store.Save(context.Background(), connectors.Connection{TenantID: s.Env().Tenant.ID, Kind: hookyKind, Name: "main", DisplayName: "Main", Status: connectors.StatusConnected},
		connectors.Credentials{"secret": "s3cret"})
	require.NoError(t, err)

	var res graphs.SearchOptionsResponse
	require.Nil(t, rpc(t, s, graphs.MethodSearchOptions, map[string]any{"source": "hooky.things", "query": "al", "connection": conn.ID.String()}, &res).Error)
	assert.Equal(t, []graphs.OptionView{{Value: "42", Label: "Thing al", Group: "private"}}, res.Options)

	require.Nil(t, rpc(t, s, graphs.MethodSearchOptions, map[string]any{"source": "other", "query": "", "connection": conn.ID.String()}, &res).Error)
	assert.Empty(t, res.Options, "a source the connector does not know has no options")
	require.Nil(t, rpc(t, s, graphs.MethodSearchOptions, map[string]any{"source": "hooky.things", "query": "", "connection": "platform:" + hookyKind}, &res).Error)
	assert.Empty(t, res.Options, "a platform credential offers none")
	reply := rpc(t, s, graphs.MethodSearchOptions, map[string]any{"source": "hooky.things", "query": "", "connection": uuid.NewString()}, &res)
	require.NotNil(t, reply.Error, "a connection that is not the tenant's is not found")

	reader, _ := app(t, permissions.AutomationsRead)
	reply = rpc(t, reader, graphs.MethodSearchOptions, map[string]any{"source": "hooky.things", "query": "", "connection": conn.ID.String()}, nil)
	require.NotNil(t, reply.Error, "authoring is required")
}
