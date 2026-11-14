package extension_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/crm/extension"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

func TestResourcesDeclareTheTypedRouteAndTheSlot(t *testing.T) {
	set := native.NewSet()
	require.NoError(t, set.Add(extension.Items("/crm/clients/")...))
	route, ok := set.Resource(extension.RouteClientView)
	require.True(t, ok)
	slot, ok := set.Resource(extension.SlotClientSidebar)
	require.True(t, ok)

	assert.Equal(t, extension.RouteClientView, route.ID)
	assert.Equal(t, manifest.ResRoute, route.Kind)
	assert.Equal(t, "/crm/clients/:id", route.Path, "the base path decides the pattern")
	require.NotNil(t, route.Params)
	assert.Equal(t, "id", route.Params.Properties[0].Name)
	assert.Equal(t, manifest.KindInt, route.Params.Properties[0].Type.Kind)
	assert.Equal(t, "Client.Read", route.Permission, "a package page replacing the screen inherits the native requirement")

	assert.Equal(t, extension.SlotClientSidebar, slot.ID)
	assert.Equal(t, manifest.ResSlot, slot.Kind)
	assert.Empty(t, slot.Target, "a declaration, not a contribution")
	require.NotNil(t, slot.Props)
	assert.Equal(t, "clientId", slot.Props.Properties[0].Name)
	assert.Equal(t, "Client.Read", slot.Permission)
}
