package composition

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/appletengine/handlers"
	"github.com/stretchr/testify/require"
)

type fakeKVStore struct{}

func (fakeKVStore) Get(context.Context, string) (any, error)      { return map[string]any{}, nil }
func (fakeKVStore) Set(context.Context, string, any, *int) error  { return nil }
func (fakeKVStore) Delete(context.Context, string) (bool, error)  { return false, nil }
func (fakeKVStore) MGet(context.Context, []string) ([]any, error) { return []any{}, nil }

func TestBackendRegistryValidateAndBuildAreSeparated(t *testing.T) {
	registry := NewBackendRegistry()
	validateCalls := 0
	buildCalls := 0

	err := registry.KV.Register("memory", BackendFactory[handlers.KVStore]{
		Validate: func(BuildContext) error {
			validateCalls++
			return nil
		},
		Build: func(*Container) (handlers.KVStore, error) {
			buildCalls++
			return fakeKVStore{}, nil
		},
	})
	require.NoError(t, err)

	factory, ok := registry.KV.Lookup("memory")
	require.True(t, ok)
	require.NotNil(t, factory.Build)

	err = registry.KV.Validate("memory", BuildContext{})
	require.NoError(t, err)
	require.Equal(t, 1, validateCalls)
	require.Equal(t, 0, buildCalls)

	store, err := registry.KV.Build("memory", nil)
	require.NoError(t, err)
	require.IsType(t, fakeKVStore{}, store)
	require.Equal(t, 1, validateCalls)
	require.Equal(t, 1, buildCalls)
}
