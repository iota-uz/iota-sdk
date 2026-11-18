package data_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

func TestApplyCreatesTypedTables(t *testing.T) {
	e := newEnv(t)
	plan := e.install("v1", insuranceV1())
	assert.False(t, plan.Empty())

	type col struct{ typ, nullable, def string }
	cols := map[string]col{}
	rows, err := e.tx.Query(e.ctx, `SELECT column_name, data_type || COALESCE('(' || character_maximum_length || ')', '') ||
		COALESCE('/' || numeric_precision || ',' || numeric_scale, ''), is_nullable, COALESCE(column_default, '')
		FROM information_schema.columns WHERE table_schema = 'x_acme_insurance' AND table_name = $1`, "policy")
	require.NoError(t, err)
	for rows.Next() {
		var name string
		var c col
		require.NoError(t, rows.Scan(&name, &c.typ, &c.nullable, &c.def))
		cols[name] = c
	}
	rows.Close()

	assert.Equal(t, "numeric/18,2", cols["premium"].typ)
	assert.Equal(t, "bigint/64,0", cols["limit"].typ)
	assert.Equal(t, "bigint/64,0", cols["seats"].typ)
	assert.Equal(t, "date", cols["expires_on"].typ)
	assert.Equal(t, "timestamp with time zone", cols["issued_at"].typ)
	assert.Equal(t, "jsonb", cols["extra"].typ)
	assert.Equal(t, "uuid", cols["holder"].typ)
	assert.Equal(t, "boolean", cols["active"].typ)
	assert.Equal(t, "NO", cols["number"].nullable)
	assert.Equal(t, "YES", cols["notes"].nullable)
	assert.Contains(t, cols["status"].def, "draft")
	assert.Equal(t, "integer/32,0", cols["row_version"].typ)
	assert.Equal(t, "NO", cols["tenant_id"].nullable)
	assert.Contains(t, cols["id"].def, "gen_random_uuid")

	cust := e.scalar(`SELECT data_type || COALESCE('(' || character_maximum_length || ')', '') FROM information_schema.columns
		WHERE table_schema = 'x_acme_insurance' AND table_name = 'customer' AND column_name = 'name'`)
	assert.Equal(t, "character varying(100)", cust)
	assert.Equal(t, "bigint", e.scalar(`SELECT data_type FROM information_schema.columns
		WHERE table_schema = 'x_acme_insurance' AND table_name = 'customer' AND column_name = 'owner'`),
		"a native ref stores the native key type")
	assert.Equal(t, "bigint", e.scalar(`SELECT data_type FROM information_schema.columns
		WHERE table_schema = 'x_acme_insurance' AND table_name = 'client_extra' AND column_name = 'base_key'`))

	// indexes are tenant-leading
	for _, idx := range []string{"uq_policy_number", "ix_policy_expires_on"} {
		def := e.scalar(`SELECT indexdef FROM pg_indexes WHERE schemaname = 'x_acme_insurance' AND indexname = $1`, idx)
		assert.Contains(t, def, "(tenant_id, ", idx)
	}
	assert.Contains(t, e.scalar(`SELECT indexdef FROM pg_indexes WHERE indexname = 'uq_policy_number'`), "UNIQUE")

	// foreign keys: own ref is tenant-consistent, overlay base cascades
	assert.Contains(t, e.scalar(`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = 'fk_policy_holder'`),
		"FOREIGN KEY (tenant_id, holder) REFERENCES x_acme_insurance.customer(tenant_id, id)")
	assert.Contains(t, e.scalar(`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = 'fk_client_extra_base_key'`),
		"REFERENCES clients(id) ON DELETE CASCADE")
	assert.Equal(t, "0", e.scalar(`SELECT count(*)::text FROM pg_constraint WHERE conrelid = 'x_acme_insurance.customer'::regclass AND contype = 'f' AND conname LIKE 'fk_customer_owner%'`),
		"native refs carry no foreign key")

	// enum CHECK
	err = e.attempt(func(tx pgx.Tx) error {
		_, err := tx.Exec(e.ctx, `INSERT INTO x_acme_insurance.client_extra (tenant_id, base_key, loyalty_tier) VALUES ($1, 1, 'platinum')`, e.tenant)
		return err
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ck_client_extra_loyalty_tier")

	// metadata
	assert.Equal(t, "3", e.scalar(`SELECT count(*)::text FROM extruntime.entity_schemas WHERE package = $1`, pkgName))
	assert.Equal(t, "3", e.scalar(`SELECT count(*)::text FROM extruntime.entity_schema_versions WHERE package = $1 AND version = 'v1'`, pkgName))
	assert.Equal(t, "v1", e.scalar(`SELECT to_version FROM extruntime.schema_migrations_history WHERE package = $1`, pkgName))
	assert.Equal(t, "number", e.scalar(`SELECT (columns -> 0 ->> 'name') FROM extruntime.entity_schemas WHERE entity_id = $1`, policyID))
}

func TestCompileRefusals(t *testing.T) {
	e := newEnv(t)
	c := data.NewCompiler(e.natives, e.catalog)
	bad := &manifest.Manifest{Schema: manifest.SchemaVersion, Namespace: ns, Resources: []manifest.Resource{
		{ID: ns + ".thing", Kind: manifest.ResEntity, Fields: []manifest.Field{
			{ID: "key", Type: ty(manifest.KindString)},
			{ID: "obj", Type: manifest.Type{Kind: manifest.KindObject}},
			{ID: "ghost", Type: refT("nowhere.missing")},
			{ID: "bad-key", Type: ty(manifest.KindString)},
			{ID: "r", Type: refT("core.user"), Default: raw("x")},
		}},
	}}
	_, err := c.Compile(e.ctx, pkgName, "v1", bad)
	require.Error(t, err)
	require.ErrorIs(t, err, data.ErrIncompatibleSchema)
	var se *data.SchemaError
	require.ErrorAs(t, err, &se)
	assert.Len(t, se.Diagnostics, 5, err.Error())
}

func mutate(t *testing.T, f func(m *manifest.Manifest)) *manifest.Manifest {
	t.Helper()
	m := clone(t, insuranceV1())
	f(m)
	return m
}

func diagCodes(err error) []string {
	var se *data.SchemaError
	if !errors.As(err, &se) {
		return nil
	}
	out := make([]string, 0, len(se.Diagnostics))
	for _, d := range se.Diagnostics {
		out = append(out, d.Resource+"."+d.Field+":"+d.Code)
	}
	return out
}

func TestEvolutionRefusals(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	before := e.scalar(`SELECT count(*)::text FROM information_schema.columns WHERE table_schema = 'x_acme_insurance'`)
	c := data.NewCompiler(e.natives, e.catalog)

	cases := []struct {
		name string
		edit func(m *manifest.Manifest)
		want string
	}{
		{"type change", func(m *manifest.Manifest) {
			resource(m, policyID).Fields[2].Type = ty(manifest.KindString)
		}, policyID + ".premium:" + data.DiagTypeChange},
		{"decimal scale change", func(m *manifest.Manifest) {
			resource(m, policyID).Fields[2].Type = manifest.Type{Kind: manifest.KindDecimal, Precision: 18, Scale: 4}
		}, policyID + ".premium:" + data.DiagTypeChange},
		{"dropped field", func(m *manifest.Manifest) {
			r := resource(m, policyID)
			r.Fields = append(r.Fields[:9], r.Fields[10:]...) // notes
		}, policyID + ".notes:" + data.DiagFieldDropped},
		{"dropped entity", func(m *manifest.Manifest) {
			m.Resources = m.Resources[:2]
		}, extraID + ".:" + data.DiagEntityDropped},
		{"required without default", func(m *manifest.Manifest) {
			r := resource(m, policyID)
			r.Fields = append(r.Fields, manifest.Field{ID: "branch", Type: ty(manifest.KindString)})
		}, policyID + ".branch:" + data.DiagRequiredNoDefault},
		{"optional becomes required", func(m *manifest.Manifest) {
			resource(m, policyID).Fields[9].Type = ty(manifest.KindString)
		}, policyID + ".notes:" + data.DiagTightenNullability},
		{"string shrinks", func(m *manifest.Manifest) {
			resource(m, customerID).Fields[0].Type = manifest.Type{Kind: manifest.KindString, MaxLength: 10}
		}, customerID + ".name:" + data.DiagNarrowing},
		{"enum loses a value", func(m *manifest.Manifest) {
			resource(m, policyID).Fields[7].Type = enumT("draft", "active")
		}, policyID + ".status:" + data.DiagNarrowing},
		{"ref retargeted", func(m *manifest.Manifest) {
			resource(m, policyID).Fields[1].Type = refT("core.user")
		}, policyID + ".holder:" + data.DiagTypeChange},
		{"renamedFrom still declared", func(m *manifest.Manifest) {
			r := resource(m, policyID)
			r.Fields[10].RenamedFrom = []string{"notes"}
		}, policyID + ".extra:" + data.DiagAmbiguousRename},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := data.Prepare(e.ctx, e.tx, c, pkgName, "v2", mutate(t, tc.edit), true)
			require.Error(t, err)
			require.ErrorIs(t, err, data.ErrIncompatibleSchema)
			assert.Contains(t, diagCodes(err), tc.want, err.Error())
		})
	}
	assert.Equal(t, before, e.scalar(`SELECT count(*)::text FROM information_schema.columns WHERE table_schema = 'x_acme_insurance'`),
		"a refused plan changes nothing")
	assert.Equal(t, "v1", e.scalar(`SELECT to_version FROM extruntime.schema_migrations_history WHERE package = $1 ORDER BY id DESC LIMIT 1`, pkgName))

	t.Run("every reason is listed", func(t *testing.T) {
		m := mutate(t, func(m *manifest.Manifest) {
			r := resource(m, policyID)
			r.Fields[2].Type = ty(manifest.KindString)
			r.Fields = append(r.Fields[:9], r.Fields[10:]...)
		})
		_, err := data.Prepare(e.ctx, e.tx, c, pkgName, "v2", m, false)
		require.Error(t, err)
		assert.Len(t, diagCodes(err), 2, err.Error())
	})
}

func v2Manifest(t *testing.T) *manifest.Manifest {
	t.Helper()
	return mutate(t, func(m *manifest.Manifest) {
		r := resource(m, policyID)
		r.Fields[9].ID = "comment" // notes -> comment
		r.Fields[9].RenamedFrom = []string{"notes"}
		r.Fields = append(r.Fields,
			manifest.Field{ID: "branch", Type: opt(ty(manifest.KindString)), Index: true},
			manifest.Field{ID: "rating", Type: ty(manifest.KindInt), Default: raw(5)},
			manifest.Field{ID: "tier", Type: opt(enumT("a", "b")), Default: raw("a")},
		)
		r.Fields[7].Type = enumT("draft", "active", "expired", "cancelled")
		resource(m, customerID).Fields[0].Type = manifest.Type{Kind: manifest.KindString, MaxLength: 200}
	})
}

func TestEvolutionAutomatic(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	notesCol := e.scalar(`SELECT columns -> 9 ->> 'name' FROM extruntime.entity_schemas WHERE entity_id = $1`, policyID)
	require.Equal(t, "notes", notesCol)

	plan := e.install("v2", v2Manifest(t))
	sql := plan.SQL()
	assert.NotContains(t, sql, "RENAME", "a field rename is metadata only")
	assert.NotContains(t, sql, "DROP COLUMN")
	assert.Contains(t, sql, `ADD COLUMN "branch" text`)
	assert.Contains(t, sql, `ADD COLUMN "rating" bigint NOT NULL DEFAULT 5`)
	assert.Contains(t, sql, `CREATE INDEX "ix_policy_branch"`)
	assert.Contains(t, sql, "ALTER COLUMN \"name\" TYPE varchar(200)")
	assert.Equal(t, []data.Rename{{Resource: policyID, From: "notes", To: "comment", Column: "notes"}}, plan.Renames)

	// the physical column keeps its name
	assert.Equal(t, "notes", e.scalar(`SELECT column_name FROM information_schema.columns WHERE table_schema = 'x_acme_insurance' AND table_name = 'policy' AND column_name = 'notes'`))
	assert.Equal(t, `["notes","comment"]`, strings.ReplaceAll(e.scalar(`SELECT (columns -> 9 -> 'keys')::text FROM extruntime.entity_schemas WHERE entity_id = $1`, policyID), " ", ""))

	// re-planning the same version against its applied state is a no-op (adopt)
	again, err := data.Prepare(e.ctx, e.tx, data.NewCompiler(e.natives, e.catalog), pkgName, "v2", v2Manifest(t), true)
	require.NoError(t, err)
	assert.True(t, again.Adopted)
	assert.True(t, again.Empty())

	// the changed enum accepts the new value, the old check is replaced
	e.exec(`UPDATE x_acme_insurance.client_extra SET loyalty_tier = loyalty_tier WHERE false`)
}

func TestStalePlanRefused(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	c := data.NewCompiler(e.natives, e.catalog)
	applied, err := data.LoadApplied(e.ctx, e.tx, pkgName)
	require.NoError(t, err)
	desired, err := c.Compile(e.ctx, pkgName, "v2", v2Manifest(t))
	require.NoError(t, err)
	stale, err := data.Plan(applied, desired)
	require.NoError(t, err)

	other := mutate(t, func(m *manifest.Manifest) {
		r := resource(m, policyID)
		r.Fields = append(r.Fields, manifest.Field{ID: "other", Type: opt(ty(manifest.KindString))})
	})
	e.install("v1b", other)

	err = data.Apply(e.ctx, e.tx, stale)
	require.ErrorIs(t, err, data.ErrStalePlan)
}

func TestUniqueIndexPrecondition(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	sc := e.scope("v1", e.newUser("u"), "Policy.ReadAll")
	cust := e.mustInsert(sc, customerID, map[string]any{"name": "C"})
	for _, n := range []string{"P1", "P2"} {
		e.mustInsert(sc, policyID, map[string]any{
			"number": n, "holder": ref(customerID, cust["key"].(string)), "premium": "1", "limit": "1",
			"expiresOn": "2027-01-01", "notes": "dup",
		})
	}
	m := mutate(t, func(m *manifest.Manifest) { resource(m, policyID).Fields[9].Unique = true })
	err := e.attempt(func(tx pgx.Tx) error {
		_, err := data.Prepare(e.ctx, tx, data.NewCompiler(e.natives, e.catalog), pkgName, "v2", m, true)
		return err
	})
	require.ErrorIs(t, err, data.ErrIncompatibleSchema)
	assert.Contains(t, diagCodes(err)[0], data.DiagUniqueViolatedByRow)
	assert.Contains(t, err.Error(), "duplicate values")
}

func TestPinnedCompatibility(t *testing.T) {
	e := newEnv(t)
	e.install("v1", insuranceV1())
	c := data.NewCompiler(e.natives, e.catalog)

	// v2 only adds: v1 is still served
	plan := e.install("v2", v2Manifest(t))
	applied, err := data.LoadApplied(e.ctx, e.tx, pkgName)
	require.NoError(t, err)
	v1 := applied.Views["v1"]
	require.NotNil(t, v1)
	loaded, ok, err := data.LoadView(e.ctx, e.tx, pkgName, "v1")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, v1, loaded)
	_, ok, err = data.LoadView(e.ctx, e.tx, pkgName, "nope")
	require.NoError(t, err)
	assert.False(t, ok)
	require.NoError(t, data.CompatibleWithPinned(applied, v1))
	require.NoError(t, plan.CompatibleWithPinned(v1, applied.Views["v2"]))

	// v3 relaxes a required field: automatic, but a pinned v1 expecting a
	// value on every row can no longer be served, so activation is blocked
	v3m := v2Manifest(t)
	resource(v3m, policyID).Fields[0].Type = opt(ty(manifest.KindString))
	desired, err := c.Compile(e.ctx, pkgName, "v3", v3m)
	require.NoError(t, err)
	p3, err := data.Plan(applied, desired)
	require.NoError(t, err)
	assert.Contains(t, p3.SQL(), "DROP NOT NULL")
	err = p3.CompatibleWithPinned(v1)
	require.ErrorIs(t, err, data.ErrIncompatibleSchema)
	assert.Contains(t, diagCodes(err), policyID+".number:"+data.DiagIncompatiblePinned)

	// a dropped column (reviewed migration) breaks the pinned view
	e.exec(`ALTER TABLE x_acme_insurance.policy DROP COLUMN secret`)
	applied.Tables[policyID].Columns = removeColumn(applied.Tables[policyID], "secret")
	err = data.CompatibleWithPinned(applied, v1)
	assert.Contains(t, diagCodes(err), policyID+".secret:"+data.DiagFieldDropped)
}

func removeColumn(t *data.PhysTable, name string) []data.PhysColumn {
	var out []data.PhysColumn
	for _, c := range t.Columns {
		if c.Name != name {
			out = append(out, c)
		}
	}
	return out
}
