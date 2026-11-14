package listext_test

import (
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/crm"
	"github.com/iota-uz/iota-sdk/modules/logging"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/listext/listexttest"
)

const list = "crm.lists.clients"

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

type env struct {
	*itf.TestEnvironment
	t      *testing.T
	tenant uuid.UUID
}

func setup(t *testing.T) *env {
	t.Helper()
	e := itf.Setup(t, itf.WithComponents(
		logging.NewComponent(),
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		crm.NewComponent(),
	))
	return &env{TestEnvironment: e, t: t, tenant: e.Tenant.ID}
}

func (e *env) client(first string) string {
	e.t.Helper()
	var id int64
	require.NoError(e.t, e.Tx.QueryRow(e.Ctx,
		`INSERT INTO clients (tenant_id, first_name, last_name) VALUES ($1, $2, 'Doe') RETURNING id`,
		e.tenant, first).Scan(&id))
	return fmt.Sprint(id)
}

func (e *env) sub(perms ...string) spi.Subject {
	return spi.Subject{TenantID: e.tenant, UserID: 1, Kind: spi.SubjectInitiator, Permissions: perms}
}

// page runs the clients list the way a native list does and returns the
// first names in order, the count and the plan.
func (e *env) page(t *testing.T, eng *listext.Engine, sub spi.Subject, req listext.Request, limit, offset int) ([]string, int64, *listext.Plan) {
	t.Helper()
	plan, err := eng.Compile(e.Ctx, e.Tx, sub, listext.Input{List: list, Native: "crm.client", BaseAlias: "c", Request: req, UsedArgs: 1})
	require.NoError(t, err)
	where := "c.tenant_id = $1"
	for _, w := range plan.Where {
		where += " AND " + w
	}
	order := "c.id"
	if len(plan.OrderBy) > 0 {
		order = plan.OrderBy[0] + ", c.id"
	}
	args := plan.Args([]any{e.tenant})
	var count int64
	require.NoError(t, e.Tx.QueryRow(e.Ctx, "SELECT count(*) FROM clients c "+plan.Join+" WHERE "+where, args...).Scan(&count))
	rows, err := e.Tx.Query(e.Ctx, fmt.Sprintf("SELECT c.first_name FROM clients c %s WHERE %s ORDER BY %s LIMIT %d OFFSET %d", plan.Join, where, order, limit, offset), args...)
	require.NoError(t, err)
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	return names, count, plan
}

func TestAbsentPackageContributesNothing(t *testing.T) {
	e := setup(t)
	f := listexttest.Install(t, e.Ctx, e.Tx, list, listexttest.Manifest(nil), false)
	e.client("Ann")
	names, count, plan := e.page(t, f.Engine, e.sub("Client.Read"), listext.Request{}, 10, 0)
	assert.Equal(t, []string{"Ann"}, names)
	assert.EqualValues(t, 1, count)
	assert.Empty(t, plan.Columns)
	assert.Empty(t, plan.Join)
	assert.Nil(t, listext.NewView(plan, listext.Request{}, "en"), "no columns, no view")
}

func TestPresentOverlayReadsDefaultsAndFiltersAndSorts(t *testing.T) {
	e := setup(t)
	f := listexttest.Install(t, e.Ctx, e.Tx, list, listexttest.Manifest(nil), true)
	ann, bob, cid := e.client("Ann"), e.client("Bob"), e.client("Cid")
	_ = bob
	full := e.sub("Client.Read", listexttest.VipPermission)
	f.Put(t, e.Ctx, e.Tx, full, ann, map[string]any{"loyaltyTier": "gold", "score": 30})
	f.Put(t, e.Ctx, e.Tx, full, cid, map[string]any{"loyaltyTier": "silver", "score": 10})

	// Bob has no overlay row: tier reads as the declared default, score as NULL.
	view := func(req listext.Request) (*listext.View, []string) {
		names, _, plan := e.page(t, f.Engine, full, req, 10, 0)
		return listext.NewView(plan, req, "en"), names
	}
	_, names := view(listext.Request{})
	assert.Equal(t, []string{"Ann", "Bob", "Cid"}, names)

	plan, err := f.Engine.Compile(e.Ctx, e.Tx, full, listext.Input{List: list, Native: "crm.client", BaseAlias: "c", UsedArgs: 1})
	require.NoError(t, err)
	rows, err := e.Tx.Query(e.Ctx, "SELECT c.first_name"+plan.SelectList()+" FROM clients c "+plan.Join+" WHERE c.tenant_id = $1 ORDER BY c.id", plan.Args([]any{e.tenant})...)
	require.NoError(t, err)
	got := map[string][]string{}
	for rows.Next() {
		var n string
		vals, err := plan.Scan(rows, &n)
		require.NoError(t, err)
		for _, v := range vals {
			got[n] = append(got[n], fmt.Sprintf("%q/%v", v.Text(), v.Null()))
		}
	}
	rows.Close()
	require.Len(t, plan.Columns, 4)
	titles := []string{}
	for _, c := range plan.Columns {
		titles = append(titles, c.Title("ru"))
	}
	assert.Equal(t, []string{"Уровень", "Баллы", "VIP", "Заметки"}, titles, "columns keep the source order and are localized")
	assert.Equal(t, []string{`"gold"/false`, `"30"/false`, `""/true`, `""/true`}, got["Ann"])
	assert.Equal(t, []string{`"none"/false`, `""/true`, `""/true`, `""/true`}, got["Bob"], "no overlay row reads the default")

	// filter sees the default-read value
	_, names = view(listext.Request{Filters: []listext.Filter{{Column: listexttest.TierCol, Op: listext.OpEq, Value: "none"}}})
	assert.Equal(t, []string{"Bob"}, names)
	// numeric range filter and descending sort
	_, names = view(listext.Request{Filters: []listext.Filter{{Column: listexttest.ScoreCol, Op: listext.OpGte, Value: "10"}}, Sort: &listext.Sort{Column: listexttest.ScoreCol, Desc: true}})
	assert.Equal(t, []string{"Ann", "Cid"}, names)
	// invalid values and unknown columns are ignored, never injected
	_, _, p := e.page(t, f.Engine, full, listext.Request{Filters: []listext.Filter{
		{Column: listexttest.ScoreCol, Op: listext.OpEq, Value: "1; DROP TABLE clients"},
		{Column: "acme.insurance.nope", Op: listext.OpEq, Value: "x"},
		{Column: listexttest.NotesCol, Op: listext.OpEq, Value: "x"},
	}}, 10, 0)
	assert.Len(t, p.Ignored, 3)
	assert.Empty(t, p.Where)
}

func TestExportPlanKeepsOnlyExportableColumns(t *testing.T) {
	e := setup(t)
	f := listexttest.Install(t, e.Ctx, e.Tx, list, listexttest.Manifest(nil), true)
	plan, err := f.Engine.Compile(e.Ctx, e.Tx, e.sub("Client.Read", listexttest.VipPermission), listext.Input{List: list, Native: "crm.client", BaseAlias: "c", Purpose: listext.PurposeExport})
	require.NoError(t, err)
	ids := make([]string, 0, len(plan.Columns))
	for _, c := range plan.Columns {
		ids = append(ids, c.ID)
	}
	assert.Equal(t, []string{listexttest.ScoreCol, listexttest.TierCol, listexttest.VipCol}, sortedCopy(ids))
	assert.Len(t, plan.Select, 3)
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func TestFieldAccessHidesColumnAndRejectsItsFilter(t *testing.T) {
	e := setup(t)
	f := listexttest.Install(t, e.Ctx, e.Tx, list, listexttest.Manifest(nil), true)
	ann := e.client("Ann")
	e.client("Bob")
	f.Put(t, e.Ctx, e.Tx, e.sub("Client.Read", listexttest.VipPermission), ann, map[string]any{"vip": "platinum"})

	limited := e.sub("Client.Read")
	names, _, plan := e.page(t, f.Engine, limited, listext.Request{Filters: []listext.Filter{{Column: listexttest.VipCol, Op: listext.OpEq, Value: "platinum"}}, Sort: &listext.Sort{Column: listexttest.VipCol}}, 10, 0)
	require.Len(t, plan.Hidden, 1)
	assert.Equal(t, listexttest.VipCol, plan.Hidden[0].ID)
	for _, c := range plan.Columns {
		assert.NotEqual(t, listexttest.VipCol, c.ID, "hidden field is not selected")
	}
	assert.Len(t, plan.Ignored, 2, "its filter and sort are ignored, they do not widen or narrow the list")
	assert.Equal(t, []string{"Ann", "Bob"}, names)

	names, _, plan = e.page(t, f.Engine, e.sub("Client.Read", listexttest.VipPermission), listext.Request{Filters: []listext.Filter{{Column: listexttest.VipCol, Op: listext.OpEq, Value: "platinum"}}}, 10, 0)
	assert.Equal(t, []string{"Ann"}, names)
	assert.Empty(t, plan.Hidden)
}

func TestRowAccessAppliesBeforePaginationAndCount(t *testing.T) {
	e := setup(t)
	// Gold overlay rows are readable only with Extra.Tiers.
	access := &manifest.Access{Read: []manifest.ReadRule{
		{Permission: "Extra.Tiers", All: true},
		{Rows: &manifest.Filter{Fields: map[string]manifest.Condition{"loyaltyTier": {Ne: []byte(`"gold"`)}}}},
	}}
	f := listexttest.Install(t, e.Ctx, e.Tx, list, listexttest.Manifest(access), true)
	full := e.sub("Client.Read", "Extra.Tiers", listexttest.VipPermission)
	ids := map[string]string{}
	for i, name := range []string{"Ann", "Bob", "Cid", "Dan", "Eve"} {
		id := e.client(name)
		ids[name] = id
		tier := "gold"
		if i%2 == 1 {
			tier = "silver"
		}
		f.Put(t, e.Ctx, e.Tx, full, id, map[string]any{"loyaltyTier": tier, "score": i})
	}
	gold := listext.Request{Filters: []listext.Filter{{Column: listexttest.TierCol, Op: listext.OpEq, Value: "gold"}}}

	names, count, _ := e.page(t, f.Engine, full, gold, 10, 0)
	assert.Equal(t, []string{"Ann", "Cid", "Eve"}, names)
	assert.EqualValues(t, 3, count)

	limited := e.sub("Client.Read", listexttest.VipPermission)
	names, count, _ = e.page(t, f.Engine, limited, gold, 10, 0)
	assert.Empty(t, names, "gold rows are hidden from the join, so a gold filter matches none")
	assert.EqualValues(t, 0, count, "the count honors row access too")

	// Hidden rows read as the default, and page boundaries are computed over
	// the access-filtered list, not over the raw rows.
	none := listext.Request{Filters: []listext.Filter{{Column: listexttest.TierCol, Op: listext.OpEq, Value: "none"}}}
	names, count, _ = e.page(t, f.Engine, limited, none, 2, 0)
	assert.Equal(t, []string{"Ann", "Cid"}, names)
	assert.EqualValues(t, 3, count)
	names, _, _ = e.page(t, f.Engine, limited, none, 2, 2)
	assert.Equal(t, []string{"Eve"}, names)
}

func TestParseQuery(t *testing.T) {
	v := url.Values{}
	v.Set("ext.acme.insurance.clientTier", "gold")
	v.Set("extfrom.acme.insurance.clientScore", "5")
	v.Set("extto.acme.insurance.clientScore", "9")
	v.Set("ext.acme.insurance.clientVip", "  ")
	v.Set("ext.sort", "acme.insurance.clientScore")
	v.Set("ext.dir", "desc")
	v.Set("Search", "x")
	r := listext.ParseQuery(v)
	assert.ElementsMatch(t, []listext.Filter{
		{Column: "acme.insurance.clientTier", Op: listext.OpContains, Value: "gold"},
		{Column: "acme.insurance.clientScore", Op: listext.OpGte, Value: "5"},
		{Column: "acme.insurance.clientScore", Op: listext.OpLte, Value: "9"},
	}, r.Filters)
	assert.Equal(t, &listext.Sort{Column: "acme.insurance.clientScore", Desc: true}, r.Sort)
	assert.True(t, listext.ParseQuery(url.Values{}).Empty())
}

func TestLabelFor(t *testing.T) {
	l := manifest.Label{"en": "Tier", "ru": "Уровень"}
	assert.Equal(t, "Уровень", listext.LabelFor(l, "ru", "x"))
	assert.Equal(t, "Уровень", listext.LabelFor(l, "ru-RU", "x"))
	assert.Equal(t, "Tier", listext.LabelFor(l, "uz", "x"))
	assert.Equal(t, "x", listext.LabelFor(nil, "en", "x"))
}
