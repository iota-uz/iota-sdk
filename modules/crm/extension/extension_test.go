package extension_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/crm/extension"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

func TestResourcesDeclareTheTypedRouteAndTheSlot(t *testing.T) {
	res := extension.Resources("/crm/clients/")
	require.Len(t, res, 2)

	route := res[0]
	assert.Equal(t, extension.RouteClientView, route.ID)
	assert.Equal(t, manifest.ResRoute, route.Kind)
	assert.Equal(t, "/crm/clients/:id", route.Path, "the base path decides the pattern")
	require.NotNil(t, route.Params)
	assert.Equal(t, "id", route.Params.Properties[0].Name)
	assert.Equal(t, manifest.KindInt, route.Params.Properties[0].Type.Kind)
	assert.Equal(t, "Client.Read", route.Permission, "a package page replacing the screen inherits the native requirement")

	slot := res[1]
	assert.Equal(t, extension.SlotClientSidebar, slot.ID)
	assert.Equal(t, manifest.ResSlot, slot.Kind)
	assert.Empty(t, slot.Target, "a declaration, not a contribution")
	require.NotNil(t, slot.Props)
	assert.Equal(t, "clientId", slot.Props.Properties[0].Name)
	assert.Equal(t, "Client.Read", slot.Permission)
}
