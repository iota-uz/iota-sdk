package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/iota-uz/applets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dummyMethod() applets.RPCMethod {
	return applets.RPCMethod{
		Handler: func(_ context.Context, _ json.RawMessage) (any, error) {
			return map[string]any{"ok": true}, nil
		},
	}
}

func TestRegistry_AcceptsNamespacedMethod(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	err := registry.RegisterPublic("bichat", "bichat.ping", dummyMethod(), nil)
	require.NoError(t, err)

	method, exists := registry.Get("bichat.ping")
	require.True(t, exists)
	assert.Equal(t, "bichat", method.AppletName)
	assert.Equal(t, "bichat.ping", method.Name)
	assert.Equal(t, MethodTargetGo, method.Target)
	assert.Equal(t, 1, registry.CountPublic())
}

func TestRegistry_RejectsMismatchedNamespace(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	err := registry.RegisterPublic("bichat", "chat.ping", dummyMethod(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be namespaced")
}

func TestRegistry_RejectsDuplicateMethods(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	err := registry.RegisterPublic("bichat", "bichat.ping", dummyMethod(), nil)
	require.NoError(t, err)

	err = registry.RegisterPublic("bichat", "bichat.ping", dummyMethod(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate method")
}

func TestRegistry_SetPublicTargetForApplet(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublic("bichat", "bichat.ping", dummyMethod(), nil))
	require.NoError(t, registry.RegisterServerOnly("bichat", "bichat.kv.get", dummyMethod(), nil))

	require.NoError(t, registry.SetPublicTargetForApplet("bichat", MethodTargetBun))

	publicMethod, ok := registry.Get("bichat.ping")
	require.True(t, ok)
	assert.Equal(t, MethodTargetBun, publicMethod.Target)

	serverMethod, ok := registry.Get("bichat.kv.get")
	require.True(t, ok)
	assert.Equal(t, MethodTargetGo, serverMethod.Target)
}

func TestRegistry_RejectsBunTargetForServerOnlyMethods(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	err := registry.register(Method{
		AppletName: "bichat",
		Name:       "bichat.kv.get",
		Visibility: visibilityServerOnly,
		Target:     MethodTargetBun,
		Method:     dummyMethod(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for public methods")
}
