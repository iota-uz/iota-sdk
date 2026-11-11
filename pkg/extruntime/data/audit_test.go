package data_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

const (
	claimID       = ns + ".claim"
	policyExtraID = ns + ".policyExtra"
)

// auditManifest extends the reference package with a claim that refers to a
// policy (whose read rule itself walks a ref path) and an overlay of the
// policy entity.
func auditManifest(t *testing.T) *manifest.Manifest {
	t.Helper()
	m := clone(t, insuranceV1())
	m.Resources = append(m.Resources,
		manifest.Resource{ID: claimID, Kind: manifest.ResEntity, Fields: []manifest.Field{
			{ID: "policy", Type: refT(policyID)},
			{ID: "title", Type: ty(manifest.KindString)},
		}},
		manifest.Resource{ID: policyExtraID, Kind: manifest.ResOverlay, Base: policyID, Fields: []manifest.Field{
			{ID: "tag", Type: opt(ty(manifest.KindString))},
		}},
	)
	return m
}

func TestOverlayReadsRequireTheNativeBaseReadPermission(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	uid := e.newUser("u")
	clientA := e.newClient("Ann")
	full := e.scope("v1", uid, "Client.Read", "Extra.Vip")
	_, err := e.store.PutOverlay(e.ctx, e.tx, full, extraID, fmt.Sprint(clientA), js(map[string]any{"loyaltyTier": "gold", "score": 7}))
	require.NoError(t, err)

	nobody := e.scope("v1", uid) // no Client.Read
	got, err := e.store.Get(e.ctx, e.tx, nobody, extraID, fmt.Sprint(clientA))
	require.NoError(t, err)
	assert.Equal(t, "null", string(got), "overlay row must not be readable without the base read permission")
	list, err := e.store.Find(e.ctx, e.tx, nobody, extraID, spi.FindQuery{})
	require.NoError(t, err)
	assert.Equal(t, "[]", string(list))
	n, err := e.store.Count(e.ctx, e.tx, nobody, extraID, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 0, n)
	filtered, err := e.store.Count(e.ctx, e.tx, nobody, extraID, &manifest.Filter{Fields: map[string]manifest.Condition{
		"score": {Eq: raw(7)},
	}})
	require.NoError(t, err)
	assert.EqualValues(t, 0, filtered, "filtering must not reveal hidden rows either")

	// the holder of the permission still reads it
	seen := e.scope("v1", uid, "Client.Read")
	assert.NotEqual(t, "null", string(mustGet(t, e, seen, extraID, fmt.Sprint(clientA))))

	// native list join: the overlay reads as its default without the permission
	args := data.NewArgs(0)
	tier := data.OverlayFieldRef{Overlay: extraID, Field: "loyaltyTier"}
	j, err := e.store.NativeOverlayJoin(e.ctx, e.tx, nobody.Subject, "crm.client", "c", []data.OverlayFieldRef{tier}, args)
	require.NoError(t, err)
	sel, err := j.Select(tier)
	require.NoError(t, err)
	tenant := args.Add(nobody.Subject.TenantID)
	rows, err := e.tx.Query(e.ctx, fmt.Sprintf(`SELECT %s::text FROM clients c %s WHERE c.tenant_id = %s AND c.id = %d`, sel, j.SQL, tenant, clientA), args.Values()...)
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())
	var v *string
	require.NoError(t, rows.Scan(&v))
	assert.Equal(t, "none", deref(v), "an overlay value must not leak through the join")
}

func TestOverlayOfAnOwnEntityFollowsTheBaseRowVisibility(t *testing.T) {
	e := newEnv(t)
	e.install("v1", auditManifest(t))
	owner := e.newUser("Owner")
	stranger := e.newUser("Stranger")
	admin := e.scope("v1", e.newUser("Adm"), "Policy.ReadAll")
	cust := e.mustInsert(admin, customerID, map[string]any{"name": "C", "owner": ref("core.user", fmt.Sprint(owner))})["key"].(string)
	pol := e.mustInsert(admin, policyID, policyBody(cust, "S-1", nil))["key"].(string)
	_, err := e.store.PutOverlay(e.ctx, e.tx, admin, policyExtraID, pol, js(map[string]any{"tag": "secret-tag"}))
	require.NoError(t, err)

	// the owner sees the policy through the holder rule and so its overlay
	ownerSc := e.scope("v1", owner)
	assert.NotEqual(t, "null", string(mustGet(t, e, ownerSc, policyID, pol)))
	assert.NotEqual(t, "null", string(mustGet(t, e, ownerSc, policyExtraID, pol)))
	assert.Len(t, decodeList(t, mustFind(t, e, ownerSc, policyExtraID, spi.FindQuery{})), 1)

	// a stranger cannot see the policy, hence not its overlay
	strangerSc := e.scope("v1", stranger)
	assert.Equal(t, "null", string(mustGet(t, e, strangerSc, policyID, pol)))
	assert.Equal(t, "null", string(mustGet(t, e, strangerSc, policyExtraID, pol)), "overlay of an invisible base row")
	assert.Empty(t, decodeList(t, mustFind(t, e, strangerSc, policyExtraID, spi.FindQuery{})))
	n, err := e.store.Count(e.ctx, e.tx, strangerSc, policyExtraID, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 0, n)

	// ... and cannot write it either: the base is not found for them
	_, err = e.store.PutOverlay(e.ctx, e.tx, strangerSc, policyExtraID, pol, js(map[string]any{"tag": "x"}))
	require.ErrorIs(t, err, data.ErrNotFound)
}

func TestRefPathFilterIntoAnEntityWhoseReadRuleUsesARefPath(t *testing.T) {
	e := newEnv(t)
	e.install("v1", auditManifest(t))
	owner := e.newUser("Owner")
	other := e.newUser("Other")
	admin := e.scope("v1", e.newUser("Adm"), "Policy.ReadAll")
	mine := e.mustInsert(admin, customerID, map[string]any{"name": "Mine", "owner": ref("core.user", fmt.Sprint(owner))})["key"].(string)
	theirs := e.mustInsert(admin, customerID, map[string]any{"name": "Theirs", "owner": ref("core.user", fmt.Sprint(other))})["key"].(string)
	p1 := e.mustInsert(admin, policyID, policyBody(mine, "P-1", nil))["key"].(string)
	p2 := e.mustInsert(admin, policyID, policyBody(theirs, "P-2", nil))["key"].(string)
	e.mustInsert(admin, claimID, map[string]any{"policy": ref(policyID, p1), "title": "c1"})
	e.mustInsert(admin, claimID, map[string]any{"policy": ref(policyID, p2), "title": "c2"})

	// policy's rule ("holder.owner" = user) needs a join of its own while the
	// claim filter walks "policy.number" and "policy.holder.name".
	sc := e.scope("v1", owner)
	where := &manifest.Filter{Fields: map[string]manifest.Condition{
		"policy.number":      {Like: ptr("P-%")},
		"policy.holder.name": {Ne: raw("zzz")},
	}}
	list := decodeList(t, mustFind(t, e, sc, claimID, spi.FindQuery{Where: where, OrderBy: []spi.Order{{Field: "policy.number"}}}))
	// the claim of an invisible policy joins to NULL and so fails the filter
	require.Len(t, list, 1)
	assert.Equal(t, "c1", list[0]["title"])
	n, err := e.store.Count(e.ctx, e.tx, sc, claimID, where)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)

	// a subject with the global rule sees both
	n, err = e.store.Count(e.ctx, e.tx, admin, claimID, where)
	require.NoError(t, err)
	assert.EqualValues(t, 2, n)
}

func TestHiddenFieldsCannotBeWrittenOrProbed(t *testing.T) {
	e := newEnv(t)
	e.install("v1", auditManifest(t))
	admin := e.scope("v1", e.newUser("Adm"), "Policy.ReadAll", "Policy.Secret")
	cust := e.mustInsert(admin, customerID, map[string]any{"name": "C"})["key"].(string)
	pol := e.mustInsert(admin, policyID, policyBody(cust, "S-1", map[string]any{"secret": "classified-token"}))
	key := pol["key"].(string)
	reader := e.scope("v1", admin.Subject.UserID, "Policy.ReadAll") // no Policy.Secret

	// the equality oracle: right and wrong guesses must both be refused,
	// with no difference an attacker can observe and no value in the error
	for _, guess := range []string{"classified-token", "guess"} {
		_, err := e.store.Update(e.ctx, e.tx, reader, policyID, key, js(map[string]any{"secret": guess}))
		require.ErrorIs(t, err, data.ErrFieldNotAccessible, guess)
		assert.NotContains(t, err.Error(), "classified-token")
	}
	_, err := e.store.Update(e.ctx, e.tx, reader, policyID, key, js(map[string]any{"secret": nil}))
	require.ErrorIs(t, err, data.ErrFieldNotAccessible, "nor may it be cleared")
	_, err = e.store.Insert(e.ctx, e.tx, reader, policyID, js(policyBody(cust, "S-2", map[string]any{"secret": "mine"})))
	require.ErrorIs(t, err, data.ErrFieldNotAccessible, "nor set on insert")

	// the stored value is untouched
	assert.Equal(t, "classified-token", decode(t, mustGet(t, e, admin, policyID, key))["secret"])

	// a patch of visible fields still works for the same subject
	up, err := e.store.Update(e.ctx, e.tx, reader, policyID, key, js(map[string]any{"notes": "ok"}))
	require.NoError(t, err)
	assert.Equal(t, "ok", decode(t, up)["notes"])
	assert.NotContains(t, decode(t, up), "secret")

	// the holder of the permission writes it
	_, err = e.store.Update(e.ctx, e.tx, admin, policyID, key, js(map[string]any{"secret": "rotated"}))
	require.NoError(t, err)
	assert.Equal(t, "rotated", decode(t, mustGet(t, e, admin, policyID, key))["secret"])
}

func TestHiddenOverlayFieldsCannotBeWritten(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	uid := e.newUser("u")
	client := fmt.Sprint(e.newClient("Ann"))
	full := e.scope("v1", uid, "Client.Read", "Extra.Vip")
	_, err := e.store.PutOverlay(e.ctx, e.tx, full, extraID, client, js(map[string]any{"vip": "yes"}))
	require.NoError(t, err)

	noVip := e.scope("v1", uid, "Client.Read")
	for _, guess := range []string{"yes", "no"} {
		_, err = e.store.PutOverlay(e.ctx, e.tx, noVip, extraID, client, js(map[string]any{"vip": guess}))
		require.ErrorIs(t, err, data.ErrFieldNotAccessible, guess)
	}
	assert.Equal(t, "yes", decode(t, mustGet(t, e, full, extraID, client))["vip"])
}
