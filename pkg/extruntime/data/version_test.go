package data_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// A workflow pinned to v1 keeps reading and writing through v1's field keys
// after v2 (a rename plus new fields) is the active version.
func TestCallerVersionResolvesKeysThroughItsOwnMap(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	e.install("v2", v2Manifest(t))
	uid := e.newUser("u")
	v1 := e.scope("v1", uid, "Policy.ReadAll")
	v2 := e.scope("v2", uid, "Policy.ReadAll", "Policy.Secret")
	cust := e.mustInsert(v2, customerID, map[string]any{"name": "C"})["key"].(string)

	pol := e.mustInsert(v1, policyID, policyBody(cust, "V-1", map[string]any{"notes": "n1"}))
	assert.Equal(t, "n1", pol["notes"])
	assert.NotContains(t, pol, "comment")
	assert.NotContains(t, pol, "rating", "v1 does not know fields added later")
	key := pol["key"].(string)
	// v2 (which holds the permission) fills the secret column; v1 cannot read
	// it, and cannot write it either: field access follows the column.
	_, err := e.store.Update(e.ctx, e.tx, v2, policyID, key, js(map[string]any{"secret": "s"}))
	require.NoError(t, err)
	assert.NotContains(t, decode(t, mustGet(t, e, v1, policyID, key)), "secret", "field access follows the column, whatever the key")
	_, err = e.store.Update(e.ctx, e.tx, v1, policyID, key, js(map[string]any{"secret": "t"}))
	require.ErrorIs(t, err, data.ErrFieldNotAccessible)

	seen := decode(t, mustGet(t, e, v2, policyID, key))
	assert.Equal(t, "n1", seen["comment"], "the renamed key reads the same column")
	assert.EqualValues(t, 5, seen["rating"], "the database default filled a field v1 never wrote")
	assert.Equal(t, "a", seen["tier"], "optional field default applies only through the schema default")
	assert.NotContains(t, seen, "notes")

	_, err = e.store.Update(e.ctx, e.tx, v2, policyID, key, js(map[string]any{"comment": "n2", "branch": "B"}))
	require.NoError(t, err)
	back := decode(t, mustGet(t, e, v1, policyID, key))
	assert.Equal(t, "n2", back["notes"])
	assert.NotContains(t, back, "branch")

	// filters resolve per caller; the renamed-from key is an alias in v2
	for _, c := range []struct {
		sc  spi.Scope
		key string
	}{{v1, "notes"}, {v2, "comment"}, {v2, "notes"}} {
		n, err := e.store.Count(e.ctx, e.tx, c.sc, policyID, &manifest.Filter{Fields: map[string]manifest.Condition{c.key: {Eq: raw("n2")}}})
		require.NoError(t, err, c.key)
		assert.EqualValues(t, 1, n, c.key)
	}
	_, err = e.store.Count(e.ctx, e.tx, v1, policyID, &manifest.Filter{Fields: map[string]manifest.Condition{"comment": {Eq: raw("n2")}}})
	require.ErrorIs(t, err, data.ErrUnknownField, "v1 cannot address v2's key")

	// a value v2 added to an enum is not writable through v1's contract
	_, err = e.store.Update(e.ctx, e.tx, v1, policyID, key, js(map[string]any{"status": "cancelled"}))
	require.ErrorIs(t, err, data.ErrInvalidValue)
	_, err = e.store.Update(e.ctx, e.tx, v2, policyID, key, js(map[string]any{"status": "cancelled"}))
	require.NoError(t, err)

	// callers from another package see the active version
	foreign := spi.Scope{Subject: v2.Subject, Caller: spi.Caller{Package: "@other/pkg", Version: "9"}}
	assert.Contains(t, decode(t, mustGet(t, e, foreign, policyID, key)), "comment")

	// a version that was never applied cannot call
	_, err = e.store.Get(e.ctx, e.tx, e.scope("v0", uid), policyID, key)
	require.ErrorIs(t, err, data.ErrUnknownEntity)

	// the cached source serves the same views
	cached := data.NewStore(e.catalog, e.natives, nil, data.WithSchemaSource(&data.CachedSchemas{}))
	out, err := cached.Get(e.ctx, e.tx, v1, policyID, key)
	require.NoError(t, err)
	assert.Equal(t, "n2", decode(t, out)["notes"])
	out, err = cached.Get(e.ctx, e.tx, v1, policyID, key)
	require.NoError(t, err)
	assert.Equal(t, "n2", decode(t, out)["notes"])
}
