package data_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func policyBody(holder string, number string, extra map[string]any) map[string]any {
	b := map[string]any{
		"number": number, "holder": ref(customerID, holder), "premium": "10", "limit": "1", "expiresOn": "2027-01-01",
	}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

func TestCRUDExactWireTypes(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	owner := e.newUser("Owner")
	sc := e.scope("v1", owner, "Policy.ReadAll", "Policy.Secret")

	cust := e.mustInsert(sc, customerID, map[string]any{"name": "Acme", "owner": ref("core.user", fmt.Sprint(owner))})
	custKey := cust["key"].(string)
	assert.Equal(t, ref("core.user", fmt.Sprint(owner)), toRef(cust["owner"]))
	assert.EqualValues(t, 1, cust["rowVersion"])

	pol := e.mustInsert(sc, policyID, policyBody(custKey, "P-1", map[string]any{
		"premium": "12.5", "limit": "9007199254740993", "seats": 3, "issuedAt": "2026-11-11T10:00:00+05:00",
		"extra": map[string]any{"b": 1, "a": []int{1, 2}}, "secret": "s3", "active": true, "notes": "n",
	}))
	assert.Equal(t, "12.50", pol["premium"], "decimal keeps its scale as a string")
	assert.Equal(t, "9007199254740993", pol["limit"], "int64 beyond 2^53 stays exact")
	assert.EqualValues(t, 3, pol["seats"])
	assert.Equal(t, "2027-01-01", pol["expiresOn"])
	assert.Equal(t, "2026-11-11T05:00:00Z", pol["issuedAt"], "datetime is RFC3339")
	assert.Equal(t, "draft", pol["status"], "declared default applies")
	assert.Equal(t, ref(customerID, custKey), toRef(pol["holder"]))
	assert.Equal(t, map[string]any{"a": []any{1.0, 2.0}, "b": 1.0}, pol["extra"])
	assert.Equal(t, true, pol["active"])
	assert.EqualValues(t, 1, pol["rowVersion"])
	assert.NotContains(t, decode(t, mustGet(t, e, sc, policyID, pol["key"].(string))), "unknownKey")

	got := decode(t, mustGet(t, e, sc, policyID, pol["key"].(string)))
	assert.Equal(t, pol, got)

	// optimistic version + patch
	up, err := e.store.Update(e.ctx, e.tx, sc, policyID, pol["key"].(string), js(map[string]any{"premium": "1000000.5", "status": "active", "rowVersion": 1}))
	require.NoError(t, err)
	upd := decode(t, up)
	assert.Equal(t, "1000000.50", upd["premium"])
	assert.EqualValues(t, 2, upd["rowVersion"])
	assert.Equal(t, "n", upd["notes"], "untouched fields stay")

	_, err = e.store.Update(e.ctx, e.tx, sc, policyID, pol["key"].(string), js(map[string]any{"notes": "x", "rowVersion": 1}))
	require.ErrorIs(t, err, data.ErrVersionConflict)

	// an update that changes nothing writes nothing (same row version)
	same, err := e.store.Update(e.ctx, e.tx, sc, policyID, pol["key"].(string), js(map[string]any{"premium": "1000000.5"}))
	require.NoError(t, err)
	assert.EqualValues(t, 2, decode(t, same)["rowVersion"])

	// clearing an optional field
	cleared, err := e.store.Update(e.ctx, e.tx, sc, policyID, pol["key"].(string), js(map[string]any{"notes": nil}))
	require.NoError(t, err)
	assert.Contains(t, decode(t, cleared), "notes")
	assert.Nil(t, decode(t, cleared)["notes"], "a nullable field reads as null")

	for name, body := range map[string]map[string]any{
		"decimal scale":    policyBody(custKey, "P-2", map[string]any{"premium": "1.234"}),
		"int64 as number":  policyBody(custKey, "P-3", map[string]any{"limit": 5}),
		"bad enum":         policyBody(custKey, "P-4", map[string]any{"status": "gone"}),
		"unknown field":    policyBody(custKey, "P-5", map[string]any{"nope": 1}),
		"wrong ref entity": policyBody(custKey, "P-6", map[string]any{"holder": ref("core.user", "1")}),
		"null required":    policyBody(custKey, "P-7", map[string]any{"premium": nil}),
		"meta field":       policyBody(custKey, "P-8", map[string]any{"rowVersion": 7}),
		"bad date":         policyBody(custKey, "P-9", map[string]any{"expiresOn": "2027-13-01"}),
	} {
		_, err := e.store.Insert(e.ctx, e.tx, sc, policyID, js(body))
		require.Error(t, err, name)
		assert.Equal(t, serrors.Invalid, serrors.CodeOf(err), name)
	}
	_, err = e.store.Insert(e.ctx, e.tx, sc, policyID, js(map[string]any{"number": "P-10"}))
	require.ErrorIs(t, err, data.ErrInvalidValue, "required fields must be present")

	t.Run("unique", func(t *testing.T) {
		err := e.attempt(func(tx pgx.Tx) error {
			_, err := e.store.Insert(e.ctx, tx, sc, policyID, js(policyBody(custKey, "P-1", nil)))
			return err
		})
		require.ErrorIs(t, err, data.ErrUniqueViolation)
		assert.Contains(t, err.Error(), `"number"`)
	})
	t.Run("reference", func(t *testing.T) {
		err := e.attempt(func(tx pgx.Tx) error {
			_, err := e.store.Insert(e.ctx, tx, sc, policyID, js(policyBody("00000000-0000-0000-0000-000000000001", "P-11", nil)))
			return err
		})
		require.ErrorIs(t, err, data.ErrReferenceViolation)
		err = e.attempt(func(tx pgx.Tx) error { return e.store.Delete(e.ctx, tx, sc, customerID, custKey) })
		require.ErrorIs(t, err, data.ErrReferenceViolation)
		_, err = e.store.Insert(e.ctx, e.tx, sc, customerID, js(map[string]any{"name": "x", "owner": ref("core.user", "987654")}))
		require.ErrorIs(t, err, data.ErrReferenceViolation, "refs to native rows must exist in the tenant")
	})

	require.NoError(t, e.store.Delete(e.ctx, e.tx, sc, policyID, pol["key"].(string)))
	assert.Equal(t, "null", string(mustGet(t, e, sc, policyID, pol["key"].(string))))
	require.ErrorIs(t, e.store.Delete(e.ctx, e.tx, sc, policyID, pol["key"].(string)), data.ErrNotFound)

	assert.Equal(t, "null", string(mustGet(t, e, sc, policyID, "0b7ce1fe-0000-4000-8000-000000000000")))
	_, err = e.store.Get(e.ctx, e.tx, sc, policyID, "not-a-uuid")
	require.ErrorIs(t, err, data.ErrInvalidValue)
	_, err = e.store.Get(e.ctx, e.tx, sc, "acme.insurance.nothing", "x")
	require.ErrorIs(t, err, data.ErrUnknownEntity)
}

func toRef(v any) map[string]string {
	m := v.(map[string]any)
	return map[string]string{"entity": m["entity"].(string), "key": m["key"].(string)}
}

func mustGet(t *testing.T, e *env, sc spi.Scope, entity, key string) []byte {
	t.Helper()
	out, err := e.store.Get(e.ctx, e.tx, sc, entity, key)
	require.NoError(t, err)
	return out
}

func TestFilterOperators(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	sc := e.scope("v1", e.newUser("u"), "Policy.ReadAll", "Policy.Secret")
	cust := e.mustInsert(sc, customerID, map[string]any{"name": "C"})["key"].(string)
	for i, p := range []map[string]any{
		{"premium": "10", "status": "draft", "notes": "alpha"},
		{"premium": "20.5", "status": "active", "notes": "beta"},
		{"premium": "30", "status": "active"},
		{"premium": "40", "status": "expired", "notes": "alphabet", "seats": 7},
	} {
		e.mustInsert(sc, policyID, policyBody(cust, fmt.Sprintf("N-%d", i), p))
	}
	count := func(f manifest.Filter) int64 {
		n, err := e.store.Count(e.ctx, e.tx, sc, policyID, &f)
		require.NoError(t, err)
		return n
	}
	cond := func(k string, c manifest.Condition) manifest.Filter {
		return manifest.Filter{Fields: map[string]manifest.Condition{k: c}}
	}
	assert.EqualValues(t, 4, count(manifest.Filter{}))
	assert.EqualValues(t, 1, count(cond("premium", manifest.Condition{Gt: raw("30")})))
	assert.EqualValues(t, 3, count(cond("premium", manifest.Condition{Gte: raw("20.50")})))
	assert.EqualValues(t, 2, count(cond("status", manifest.Condition{Eq: raw("active")})))
	assert.EqualValues(t, 3, count(cond("status", manifest.Condition{In: []json_{raw("active"), raw("expired")}})))
	assert.EqualValues(t, 2, count(cond("notes", manifest.Condition{Like: ptr("alpha%")})))
	assert.EqualValues(t, 1, count(cond("notes", manifest.Condition{IsNull: ptr(true)})))
	assert.EqualValues(t, 3, count(cond("notes", manifest.Condition{IsNull: ptr(false)})))
	assert.EqualValues(t, 3, count(cond("seats", manifest.Condition{Ne: raw(7)})), "ne includes nulls")
	assert.EqualValues(t, 1, count(cond("seats", manifest.Condition{Eq: raw(7)})))
	assert.EqualValues(t, 3, count(cond("limit", manifest.Condition{Eq: raw(nil)}))+3, "eq null is IS NULL")
	assert.EqualValues(t, 0, count(cond("status", manifest.Condition{In: []json_{}})))
	assert.EqualValues(t, 3, count(manifest.Filter{Or: []manifest.Filter{
		cond("status", manifest.Condition{Eq: raw("draft")}),
		cond("premium", manifest.Condition{Gte: raw("30")}),
	}}))
	assert.EqualValues(t, 1, count(manifest.Filter{
		Fields: map[string]manifest.Condition{"status": {Eq: raw("active")}},
		Or:     []manifest.Filter{cond("premium", manifest.Condition{Lt: raw("25")})},
	}))
	assert.EqualValues(t, 4, count(cond("key", manifest.Condition{IsNull: ptr(false)})), "key is filterable")

	for name, f := range map[string]manifest.Filter{
		"unknown":      cond("zzz", manifest.Condition{Eq: raw(1)}),
		"bool compare": cond("active", manifest.Condition{Gt: raw(true)}),
		"like on int":  cond("seats", manifest.Condition{Like: ptr("1%")}),
		"empty":        cond("notes", manifest.Condition{}),
		"bad value":    cond("premium", manifest.Condition{Eq: raw("1.234")}),
		"path on leaf": cond("notes.x", manifest.Condition{Eq: raw("a")}),
		"placeholder":  cond("holder", manifest.Condition{Eq: raw(map[string]string{"$": "tenant"})}),
	} {
		_, err := e.store.Count(e.ctx, e.tx, sc, policyID, &f)
		want := data.ErrInvalidQuery
		if name == "unknown" || name == "path on leaf" {
			want = data.ErrUnknownField
			if name == "path on leaf" {
				want = data.ErrInvalidQuery
			}
		}
		require.ErrorIs(t, err, want, name)
	}

	out, err := e.store.Find(e.ctx, e.tx, sc, policyID, spi.FindQuery{
		OrderBy: []spi.Order{{Field: "premium", Desc: true}}, Limit: 2, Offset: 1,
	})
	require.NoError(t, err)
	got := decodeList(t, out)
	require.Len(t, got, 2)
	assert.Equal(t, []string{"30.00", "20.50"}, []string{got[0]["premium"].(string), got[1]["premium"].(string)})

	_, err = e.store.Find(e.ctx, e.tx, sc, policyID, spi.FindQuery{Limit: -1})
	require.ErrorIs(t, err, data.ErrInvalidQuery)
	_, err = e.store.Find(e.ctx, e.tx, sc, policyID, spi.FindQuery{OrderBy: []spi.Order{{Field: "extra"}}})
	require.ErrorIs(t, err, data.ErrInvalidQuery)
}

type json_ = json.RawMessage

func TestAccessAppliesBeforePagination(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	a, b := e.newUser("A"), e.newUser("B")
	admin := e.scope("v1", e.newUser("Adm"), "Policy.ReadAll")
	mk := func(name string, owner any) string {
		body := map[string]any{"name": name}
		if owner != nil {
			body["owner"] = ref("core.user", fmt.Sprint(owner))
		}
		return e.mustInsert(admin, customerID, body)["key"].(string)
	}
	cA, cB, cN := mk("cA", a), mk("cB", b), mk("cN", nil)
	// interleave so hidden rows sit between visible ones
	for i := 0; i < 6; i++ {
		e.mustInsert(admin, policyID, policyBody(cA, fmt.Sprintf("A%d", i), nil))
		e.mustInsert(admin, policyID, policyBody(cB, fmt.Sprintf("B%d", i%3), map[string]any{"number": fmt.Sprintf("B%d", i)}))
		if i < 2 {
			e.mustInsert(admin, policyID, policyBody(cN, fmt.Sprintf("N%d", i), nil))
		}
	}
	subA := e.scope("v1", a) // no Policy.ReadAll: only rows whose holder is owned by A
	order := []spi.Order{{Field: "number"}}

	n, err := e.store.Count(e.ctx, e.tx, subA, policyID, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 6, n, "count sees only visible rows")
	n, err = e.store.Count(e.ctx, e.tx, admin, policyID, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 14, n)

	numbers := func(sc spi.Scope, q spi.FindQuery) []string {
		out, err := e.store.Find(e.ctx, e.tx, sc, policyID, q)
		require.NoError(t, err)
		var ns []string
		for _, r := range decodeList(t, out) {
			ns = append(ns, r["number"].(string))
		}
		return ns
	}
	assert.Equal(t, []string{"A0", "A1", "A2", "A3"}, numbers(subA, spi.FindQuery{OrderBy: order, Limit: 4}))
	assert.Equal(t, []string{"A4", "A5"}, numbers(subA, spi.FindQuery{OrderBy: order, Limit: 4, Offset: 4}),
		"hidden rows do not shift pages")
	assert.Empty(t, numbers(subA, spi.FindQuery{OrderBy: order, Limit: 4, Offset: 6}))
	assert.Len(t, numbers(e.scope("v1", b), spi.FindQuery{}), 6)
	assert.Empty(t, numbers(e.scope("v1", e.newUser("none")), spi.FindQuery{}))

	// a user filter cannot widen the visible set
	f := manifest.Filter{Fields: map[string]manifest.Condition{"number": {Like: ptr("B%")}}}
	n, err = e.store.Count(e.ctx, e.tx, subA, policyID, &f)
	require.NoError(t, err)
	assert.Zero(t, n)

	// Get respects the same predicate
	all, err := e.store.Find(e.ctx, e.tx, admin, policyID, spi.FindQuery{OrderBy: order})
	require.NoError(t, err)
	for _, r := range decodeList(t, all) {
		got := string(mustGet(t, e, subA, policyID, r["key"].(string)))
		if strings.HasPrefix(r["number"].(string), "A") {
			assert.NotEqual(t, "null", got)
		} else {
			assert.Equal(t, "null", got)
		}
	}

	// dotted filter keys: through an own ref, and on to a native entity
	byOwner := manifest.Filter{Fields: map[string]manifest.Condition{"holder.owner": {Eq: raw(ref("core.user", fmt.Sprint(b)))}}}
	n, err = e.store.Count(e.ctx, e.tx, admin, policyID, &byOwner)
	require.NoError(t, err)
	assert.EqualValues(t, 6, n)
	byName := manifest.Filter{Fields: map[string]manifest.Condition{"holder.owner.firstName": {Eq: raw("A")}}}
	withUsers := e.scope("v1", e.newUser("r"), "Policy.ReadAll", "User.Read")
	n, err = e.store.Count(e.ctx, e.tx, withUsers, policyID, &byName)
	require.NoError(t, err)
	assert.EqualValues(t, 6, n)
	_, err = e.store.Count(e.ctx, e.tx, admin, policyID, &byName)
	require.ErrorIs(t, err, data.ErrDenied, "the joined native entity needs its read permission")
	byHolder := manifest.Filter{Fields: map[string]manifest.Condition{"holder.name": {Like: ptr("c%")}}}
	n, err = e.store.Count(e.ctx, e.tx, admin, policyID, &byHolder)
	require.NoError(t, err)
	assert.EqualValues(t, 14, n)
	out, err := e.store.Find(e.ctx, e.tx, admin, policyID, spi.FindQuery{OrderBy: []spi.Order{{Field: "holder.name", Desc: true}, {Field: "number"}}, Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, "N0", decodeList(t, out)[0]["number"], "order through a ref path")

	// writes are gated on read access: a hidden row looks absent
	for _, r := range decodeList(t, all) {
		key := r["key"].(string)
		_, err := e.store.Update(e.ctx, e.tx, subA, policyID, key, raw(map[string]any{"notes": "x"}))
		delErr := e.store.Delete(e.ctx, e.tx, subA, policyID, key)
		if strings.HasPrefix(r["number"].(string), "A") {
			require.NoError(t, err)
			require.NoError(t, delErr)
		} else {
			require.ErrorIs(t, err, data.ErrNotFound)
			require.ErrorIs(t, delErr, data.ErrNotFound)
		}
	}
}

func TestFieldAccess(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	admin := e.scope("v1", e.newUser("Adm"), "Policy.ReadAll", "Policy.Secret")
	cust := e.mustInsert(admin, customerID, map[string]any{"name": "C"})["key"].(string)
	pol := e.mustInsert(admin, policyID, policyBody(cust, "S-1", map[string]any{"secret": "classified-token"}))
	assert.Equal(t, "classified-token", pol["secret"])

	reader := e.scope("v1", admin.Subject.UserID, "Policy.ReadAll") // no Policy.Secret
	got := decode(t, mustGet(t, e, reader, policyID, pol["key"].(string)))
	assert.NotContains(t, got, "secret")
	assert.Contains(t, got, "number")
	list := decodeList(t, mustFind(t, e, reader, policyID, spi.FindQuery{}))
	assert.NotContains(t, list[0], "secret")

	// writes return the same projection
	up, err := e.store.Update(e.ctx, e.tx, reader, policyID, pol["key"].(string), js(map[string]any{"notes": "x"}))
	require.NoError(t, err)
	assert.NotContains(t, decode(t, up), "secret")

	hidden := manifest.Filter{Fields: map[string]manifest.Condition{"secret": {Eq: raw("classified-token")}}}
	_, err = e.store.Count(e.ctx, e.tx, reader, policyID, &hidden)
	require.ErrorIs(t, err, data.ErrFieldNotAccessible)
	assert.NotContains(t, err.Error(), "classified-token", "errors never carry values")
	_, err = e.store.Find(e.ctx, e.tx, reader, policyID, spi.FindQuery{OrderBy: []spi.Order{{Field: "secret"}}})
	require.ErrorIs(t, err, data.ErrFieldNotAccessible)
	_, err = e.store.Find(e.ctx, e.tx, reader, policyID, spi.FindQuery{Where: &manifest.Filter{
		Or: []manifest.Filter{hidden},
	}})
	require.ErrorIs(t, err, data.ErrFieldNotAccessible, "also inside or-groups")
	n, err := e.store.Count(e.ctx, e.tx, admin, policyID, &hidden)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n, "allowed with the permission")
}

func mustFind(t *testing.T, e *env, sc spi.Scope, entity string, q spi.FindQuery) []byte {
	t.Helper()
	out, err := e.store.Find(e.ctx, e.tx, sc, entity, q)
	require.NoError(t, err)
	return out
}

func TestWriteGuard(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	sc := e.scope("v1", e.newUser("u"), "Policy.ReadAll", "Policy.Secret")
	cust := e.mustInsert(sc, customerID, map[string]any{"name": "C"})["key"].(string)
	e.guard.calls = nil

	pol := e.mustInsert(sc, policyID, policyBody(cust, "G-1", map[string]any{"notes": "one"}))
	require.Len(t, e.guard.calls, 1)
	c := e.guard.calls[0]
	assert.Equal(t, policyID, c.Entity)
	assert.Equal(t, "insert", c.Op)
	assert.Equal(t, pol["key"], c.Key)
	assert.Equal(t, "null", string(c.Existing))
	prop := decode(t, c.Proposed)
	assert.Equal(t, "G-1", prop["number"])
	assert.Equal(t, "draft", prop["status"], "proposed includes defaults")
	assert.Equal(t, "10.00", prop["premium"], "proposed is canonical wire")
	assert.ElementsMatch(t, []string{"number", "holder", "premium", "limit", "expiresOn", "notes"}, c.Changed)

	e.guard.calls = nil
	_, err := e.store.Update(e.ctx, e.tx, sc, policyID, pol["key"].(string), js(map[string]any{"notes": "two", "status": "draft"}))
	require.NoError(t, err)
	require.Len(t, e.guard.calls, 1)
	c = e.guard.calls[0]
	assert.Equal(t, "update", c.Op)
	assert.Equal(t, "one", decode(t, c.Existing)["notes"])
	assert.Equal(t, "two", decode(t, c.Proposed)["notes"])
	assert.Equal(t, "G-1", decode(t, c.Proposed)["number"], "proposed is the whole record")
	assert.Equal(t, []string{"notes"}, c.Changed, "unchanged values are not reported")

	e.guard.calls = nil
	_, err = e.store.Update(e.ctx, e.tx, sc, policyID, pol["key"].(string), js(map[string]any{"notes": "two"}))
	require.NoError(t, err)
	assert.Empty(t, e.guard.calls, "a no-op update does not reach the guard")

	// a rejecting guard stops the write; the surrounding savepoint takes
	// earlier writes of the same unit with it
	boom := serrors.New(serrors.FailedPrecondition, "rule says no")
	e.guard.fail = func(c spi.WriteChange) error {
		if c.Op == "delete" || strings.Contains(string(c.Proposed), "REJECT") {
			return boom
		}
		return nil
	}
	err = e.attempt(func(tx pgx.Tx) error {
		_, err := e.store.Insert(e.ctx, tx, sc, policyID, js(policyBody(cust, "G-2", nil)))
		require.NoError(t, err)
		_, err = e.store.Insert(e.ctx, tx, sc, policyID, js(policyBody(cust, "G-3", map[string]any{"notes": "REJECT"})))
		return err
	})
	require.ErrorIs(t, err, boom)
	n, err := e.store.Count(e.ctx, e.tx, sc, policyID, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n, "the aborted unit left nothing behind")

	e.guard.calls = nil
	require.ErrorIs(t, e.store.Delete(e.ctx, e.tx, sc, policyID, pol["key"].(string)), boom)
	require.Len(t, e.guard.calls, 1)
	assert.Equal(t, "delete", e.guard.calls[0].Op)
	assert.Equal(t, "null", string(e.guard.calls[0].Proposed))
	assert.Equal(t, "two", decode(t, e.guard.calls[0].Existing)["notes"])
	assert.NotEqual(t, "null", string(mustGet(t, e, sc, policyID, pol["key"].(string))), "rejected delete kept the row")
}
