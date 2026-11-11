package data_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corepermissions "github.com/iota-uz/iota-sdk/modules/core/permissions"
	crmpermissions "github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

func TestNativeDescriptorsMatchTheDatabase(t *testing.T) {
	e := newEnv(t)
	kinds := map[manifest.TypeKind][]string{
		manifest.KindString:   {"character varying", "text"},
		manifest.KindDate:     {"date"},
		manifest.KindDateTime: {"timestamp with time zone"},
	}
	for _, n := range e.natives.List() {
		keyType := e.scalar(`SELECT data_type FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 AND column_name = $3`,
			"public", n.Table, n.KeyColumn)
		assert.Equal(t, n.KeyType.SQL(), keyType, "%s key type", n.ID)
		assert.NotEmpty(t, e.scalar(`SELECT data_type FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`, n.Table, n.TenantColumn), n.ID)
		for _, f := range n.Fields {
			typ := e.scalar(`SELECT data_type FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`, n.Table, f.Column)
			require.NotEmpty(t, typ, "%s.%s: column %s missing", n.ID, f.ID, f.Column)
			assert.Contains(t, kinds[f.Type.Kind], typ, "%s.%s", n.ID, f.ID)
		}
	}
	assert.Equal(t, corepermissions.UserRead.Name(), data.CoreUser().ReadPermission)
	assert.Equal(t, crmpermissions.ClientRead.Name(), data.CRMClient().ReadPermission)
}

func TestNativeReadOnly(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	uid := e.newUser("Reader")
	other := e.newUser("Zed")
	e.exec(`INSERT INTO users (tenant_id, type, first_name, last_name, email, ui_language) VALUES (NULL, 'system', 'Sys', 'Tem', $1, 'en')`, "sys-"+fmt.Sprint(uid)+"@x.io")
	sc := e.scope("v1", uid, "User.Read")

	got := decode(t, mustGet(t, e, sc, "core.user", fmt.Sprint(uid)))
	assert.Equal(t, "Reader", got["firstName"])
	assert.Equal(t, fmt.Sprint(uid), got["key"])
	assert.NotContains(t, got, "password")
	assert.NotContains(t, got, "rowVersion")

	n, err := e.store.Count(e.ctx, e.tx, sc, "core.user", nil)
	require.NoError(t, err)
	var inTenant int64
	require.NoError(t, e.tx.QueryRow(e.ctx, `SELECT count(*) FROM users WHERE tenant_id = $1`, e.tenant).Scan(&inTenant))
	assert.Equal(t, inTenant, n, "tenant-scoped; rows of other tenants are invisible")

	list := decodeList(t, mustFind(t, e, sc, "core.user", spi.FindQuery{
		Where:   &manifest.Filter{Fields: map[string]manifest.Condition{"firstName": {In: []json_{raw("Reader"), raw("Zed")}}}},
		OrderBy: []spi.Order{{Field: "firstName", Desc: true}},
	}))
	require.Len(t, list, 2)
	assert.Equal(t, "Zed", list[0]["firstName"])
	assert.Equal(t, fmt.Sprint(other), list[0]["key"])

	none := e.scope("v1", uid)
	_, err = e.store.Get(e.ctx, e.tx, none, "core.user", fmt.Sprint(uid))
	require.ErrorIs(t, err, data.ErrDenied)
	_, err = e.store.Count(e.ctx, e.tx, none, "core.user", nil)
	require.ErrorIs(t, err, data.ErrDenied)

	bad := manifest.Filter{Fields: map[string]manifest.Condition{"password": {Eq: raw("x")}}}
	_, err = e.store.Count(e.ctx, e.tx, sc, "core.user", &bad)
	require.ErrorIs(t, err, data.ErrUnknownField, "columns outside the descriptor are not addressable")

	for name, f := range map[string]func() error{
		"insert": func() error {
			_, err := e.store.Insert(e.ctx, e.tx, sc, "core.user", js(map[string]any{"firstName": "x"}))
			return err
		},
		"update": func() error {
			_, err := e.store.Update(e.ctx, e.tx, sc, "core.user", fmt.Sprint(uid), js(map[string]any{"firstName": "x"}))
			return err
		},
		"delete": func() error { return e.store.Delete(e.ctx, e.tx, sc, "core.user", fmt.Sprint(uid)) },
		"overlay": func() error {
			_, err := e.store.PutOverlay(e.ctx, e.tx, sc, "core.user", fmt.Sprint(uid), js(map[string]any{}))
			return err
		},
		"client": func() error {
			_, err := e.store.Insert(e.ctx, e.tx, sc, "crm.client", js(map[string]any{"firstName": "x"}))
			return err
		},
	} {
		require.ErrorIs(t, f(), data.ErrNativeWrite, name)
	}
	assert.Equal(t, "Reader", e.scalar(`SELECT first_name FROM users WHERE id = $1`, uid), "nothing was written")
}

func TestOverlayRoundTripAndAbsence(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	uid := e.newUser("u")
	clientA, clientB := e.newClient("Ann"), e.newClient("Bob")
	sc := e.scope("v1", uid, "Client.Read")
	full := e.scope("v1", uid, "Client.Read", "Extra.Vip")

	assert.Equal(t, "null", string(mustGet(t, e, sc, extraID, fmt.Sprint(clientA))), "absent overlay row is null")

	put, err := e.store.PutOverlay(e.ctx, e.tx, full, extraID, fmt.Sprint(clientA), js(map[string]any{"loyaltyTier": "gold", "score": 7, "vip": "yes"}))
	require.NoError(t, err)
	got := decode(t, put)
	assert.Equal(t, "gold", got["loyaltyTier"])
	assert.Equal(t, fmt.Sprint(clientA), got["key"])
	assert.Equal(t, "yes", got["vip"])

	again := decode(t, mustGet(t, e, full, extraID, fmt.Sprint(clientA)))
	assert.Equal(t, got, again)
	assert.NotContains(t, decode(t, mustGet(t, e, sc, extraID, fmt.Sprint(clientA))), "vip", "field access applies to overlays")

	// partial update keeps the rest; guard saw insert then update
	e.guard.calls = nil
	_, err = e.store.PutOverlay(e.ctx, e.tx, sc, extraID, fmt.Sprint(clientA), js(map[string]any{"score": 9}))
	require.NoError(t, err)
	require.Len(t, e.guard.calls, 1)
	assert.Equal(t, "update", e.guard.calls[0].Op)
	assert.Equal(t, []string{"score"}, e.guard.calls[0].Changed)
	assert.Equal(t, "gold", decode(t, mustGet(t, e, sc, extraID, fmt.Sprint(clientA)))["loyaltyTier"])

	e.guard.calls = nil
	created, err := e.store.PutOverlay(e.ctx, e.tx, sc, extraID, fmt.Sprint(clientB), js(map[string]any{"score": 1}))
	require.NoError(t, err)
	assert.Equal(t, "insert", e.guard.calls[0].Op)
	assert.Equal(t, "none", decode(t, created)["loyaltyTier"], "stored default")

	_, err = e.store.PutOverlay(e.ctx, e.tx, sc, extraID, "999999", js(map[string]any{"score": 1}))
	require.ErrorIs(t, err, data.ErrNotFound, "the base record must exist in the tenant")
	_, err = e.store.PutOverlay(e.ctx, e.tx, e.scope("v1", uid), extraID, fmt.Sprint(clientA), js(map[string]any{"score": 1}))
	require.ErrorIs(t, err, data.ErrDenied, "the base entity read permission is required")
	_, err = e.store.PutOverlay(e.ctx, e.tx, sc, extraID, fmt.Sprint(clientA), js(map[string]any{"loyaltyTier": "platinum"}))
	require.ErrorIs(t, err, data.ErrInvalidValue)
	_, err = e.store.Insert(e.ctx, e.tx, sc, extraID, js(map[string]any{"score": 1}))
	require.ErrorIs(t, err, data.ErrUnsupported, "overlays are written with PutOverlay")

	list := decodeList(t, mustFind(t, e, sc, extraID, spi.FindQuery{OrderBy: []spi.Order{{Field: "score", Desc: true}}}))
	require.Len(t, list, 2)
	assert.Equal(t, fmt.Sprint(clientA), list[0]["key"])

	// deleting the base cascades
	e.exec(`DELETE FROM clients WHERE id = $1`, clientA)
	assert.Equal(t, "null", string(mustGet(t, e, sc, extraID, fmt.Sprint(clientA))))
}

func TestNativeListOverlayJoin(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	uid := e.newUser("u")
	a, b, c := e.newClient("Ann"), e.newClient("Bob"), e.newClient("Cid")
	full := e.scope("v1", uid, "Client.Read", "Extra.Vip")
	_, err := e.store.PutOverlay(e.ctx, e.tx, full, extraID, fmt.Sprint(a), js(map[string]any{"loyaltyTier": "gold", "score": 30, "vip": "yes"}))
	require.NoError(t, err)
	_, err = e.store.PutOverlay(e.ctx, e.tx, full, extraID, fmt.Sprint(c), js(map[string]any{"loyaltyTier": "silver", "score": 10}))
	require.NoError(t, err)
	// b has no overlay row: reads as the declared default / NULL

	tier := data.OverlayFieldRef{Overlay: extraID, Field: "loyaltyTier"}
	score := data.OverlayFieldRef{Overlay: extraID, Field: "score"}
	vip := data.OverlayFieldRef{Overlay: extraID, Field: "vip"}

	tt := t
	run := func(sub spi.Subject, cond *manifest.Condition, order string) ([]string, *data.OverlayJoin) {
		args := data.NewArgs(0)
		tenant := args.Add(sub.TenantID)
		j, err := e.store.NativeOverlayJoin(e.ctx, e.tx, sub, "crm.client", "c", []data.OverlayFieldRef{tier, score, vip}, args)
		require.NoError(t, err)
		selTier, err := j.Select(tier)
		require.NoError(t, err)
		selScore, err := j.Select(score)
		require.NoError(t, err)
		where := "c.tenant_id = " + tenant
		if cond != nil {
			f, err := j.Filter(tier, *cond, sub, args)
			require.NoError(t, err)
			where += " AND " + f
		}
		ord := "c.id"
		switch order {
		case "score desc":
			ord, err = j.OrderBy(score, true)
			require.NoError(t, err)
			ord += ", c.id"
		case "tier asc":
			ord, err = j.OrderBy(tier, false)
			require.NoError(t, err)
			ord += ", c.id"
		}
		rows, err := e.tx.Query(e.ctx, fmt.Sprintf(`SELECT c.first_name, %s::text, COALESCE(%s::text, '-') FROM clients c %s WHERE %s ORDER BY %s`,
			selTier, selScore, j.SQL, where, ord), args.Values()...)
		require.NoError(t, err)
		defer rows.Close()
		var out []string
		for rows.Next() {
			var n, tv, s string
			require.NoError(tt, rows.Scan(&n, &tv, &s))
			out = append(out, n+":"+tv+":"+s)
		}
		return out, j
	}
	_ = b

	got, j := run(full.Subject, nil, "")
	assert.Equal(t, []string{"Ann:gold:30", "Bob:none:-", "Cid:silver:10"}, got, "missing row reads as default, NULL without one")
	assert.Empty(t, j.Hidden)

	got, _ = run(full.Subject, &manifest.Condition{Eq: raw("none")}, "")
	assert.Equal(t, []string{"Bob:none:-"}, got, "filter sees the default-read value")
	got, _ = run(full.Subject, &manifest.Condition{In: []json_{raw("gold"), raw("silver")}}, "score desc")
	assert.Equal(t, []string{"Ann:gold:30", "Cid:silver:10"}, got)
	got, _ = run(full.Subject, nil, "tier asc")
	assert.Equal(t, []string{"Ann:gold:30", "Bob:none:-", "Cid:silver:10"}, got)

	// field access: vip is hidden without Extra.Vip and cannot be used
	_, jj := run(e.scope("v1", uid, "Client.Read").Subject, nil, "")
	assert.Equal(t, []data.OverlayFieldRef{vip}, jj.Hidden)
	assert.False(t, jj.Visible(vip))
	_, err = jj.Select(vip)
	require.ErrorIs(t, err, data.ErrFieldNotAccessible)
	_, err = jj.Filter(vip, manifest.Condition{Eq: raw("yes")}, full.Subject, data.NewArgs(0))
	require.ErrorIs(t, err, data.ErrFieldNotAccessible)
	_, err = jj.OrderBy(vip, false)
	require.ErrorIs(t, err, data.ErrFieldNotAccessible)
	typ, ok := j.Type(tier)
	require.True(t, ok)
	assert.Equal(t, manifest.KindEnum, typ.Kind)

	_, err = e.store.NativeOverlayJoin(e.ctx, e.tx, full.Subject, "core.user", "u", []data.OverlayFieldRef{tier}, data.NewArgs(0))
	require.ErrorIs(t, err, data.ErrInvalidQuery, "an overlay joins only its own base")
	_, err = e.store.NativeOverlayJoin(e.ctx, e.tx, full.Subject, "crm.client", "c; drop", []data.OverlayFieldRef{tier}, data.NewArgs(0))
	require.ErrorIs(t, err, data.ErrInvalidQuery)
	assert.NotContains(t, j.SQL, ";")
}

func TestOverlayRowAccessFiltersTheJoin(t *testing.T) {
	e := newEnv(t)
	m := clone(t, insuranceV1())
	ov := resource(m, extraID)
	ov.Access.Read = []manifest.ReadRule{{Permission: "Extra.Tiers", All: true}, {Rows: &manifest.Filter{Fields: map[string]manifest.Condition{
		"loyaltyTier": {Ne: raw("gold")},
	}}}}
	e.install("v1", m)
	uid := e.newUser("u")
	a, b := e.newClient("Ann"), e.newClient("Bob")
	sc := e.scope("v1", uid, "Client.Read", "Extra.Tiers")
	for id, tier := range map[int64]string{a: "gold", b: "silver"} {
		_, err := e.store.PutOverlay(e.ctx, e.tx, sc, extraID, fmt.Sprint(id), js(map[string]any{"loyaltyTier": tier}))
		require.NoError(t, err)
	}
	limited := e.scope("v1", uid, "Client.Read")
	assert.Equal(t, "null", string(mustGet(t, e, limited, extraID, fmt.Sprint(a))), "rule hides the gold row")
	assert.NotEqual(t, "null", string(mustGet(t, e, limited, extraID, fmt.Sprint(b))))

	args := data.NewArgs(0)
	tenant := args.Add(limited.Subject.TenantID)
	tier := data.OverlayFieldRef{Overlay: extraID, Field: "loyaltyTier"}
	j, err := e.store.NativeOverlayJoin(e.ctx, e.tx, limited.Subject, "crm.client", "c", []data.OverlayFieldRef{tier}, args)
	require.NoError(t, err)
	sel, _ := j.Select(tier)
	rows, err := e.tx.Query(e.ctx, fmt.Sprintf(`SELECT c.first_name, %s::text FROM clients c %s WHERE c.tenant_id = %s ORDER BY c.id`, sel, j.SQL, tenant), args.Values()...)
	require.NoError(t, err)
	defer rows.Close()
	var got []string
	for rows.Next() {
		var n string
		var tv *string
		require.NoError(t, rows.Scan(&n, &tv))
		got = append(got, fmt.Sprint(n, ":", deref(tv)))
	}
	assert.Equal(t, []string{"Ann:none", "Bob:silver"}, got, "hidden rows read as the default")
}

func deref(s *string) string {
	if s == nil {
		return "<null>"
	}
	return *s
}
