package data

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Querier is the read side shared by pgx.Tx and pools.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// LoadApplied reads everything recorded for pkg. It returns an empty Applied
// (no tables, no views) for a package that was never applied.
func LoadApplied(ctx context.Context, q Querier, pkg string) (*Applied, error) {
	const op = serrors.Op("extruntime.data.LoadApplied")
	a := &Applied{Package: pkg, Tables: map[string]*PhysTable{}, Views: map[string]*Schema{}}

	rows, err := q.Query(ctx, `SELECT entity_id, kind, schema_name, table_name, base, columns, version
		FROM extruntime.entity_schemas WHERE package = $1`, pkg)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	for rows.Next() {
		var t PhysTable
		var kind string
		var base, cols []byte
		if err := rows.Scan(&t.ID, &kind, &t.Schema, &t.Name, &base, &cols, &t.Version); err != nil {
			rows.Close()
			return nil, serrors.FromDB(op, err)
		}
		t.Kind = manifest.ResourceKind(kind)
		if len(base) > 0 {
			if err := json.Unmarshal(base, &t.Base); err != nil {
				rows.Close()
				return nil, serrors.Wrap(op, err)
			}
		}
		if err := json.Unmarshal(cols, &t.Columns); err != nil {
			rows.Close()
			return nil, serrors.Wrap(op, err)
		}
		a.Tables[t.ID] = &t
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, serrors.FromDB(op, err)
	}

	vrows, err := q.Query(ctx, `SELECT version, entity_id, kind, schema_name, table_name, base, fields
		FROM extruntime.entity_schema_versions WHERE package = $1 ORDER BY applied_at, entity_id`, pkg)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer vrows.Close()
	for vrows.Next() {
		var version, kind string
		var t Table
		var base, fields []byte
		if err := vrows.Scan(&version, &t.ID, &kind, &t.Schema, &t.Name, &base, &fields); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		t.Kind = manifest.ResourceKind(kind)
		if len(base) > 0 {
			if err := json.Unmarshal(base, &t.Base); err != nil {
				return nil, serrors.Wrap(op, err)
			}
		}
		if err := json.Unmarshal(fields, &t.Fields); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		v := a.Views[version]
		if v == nil {
			v = &Schema{Package: pkg, Version: version}
			a.Views[version] = v
		}
		v.Tables = append(v.Tables, t)
	}
	if err := vrows.Err(); err != nil {
		return nil, serrors.FromDB(op, err)
	}
	vrows.Close()
	rows2, err := q.Query(ctx, `SELECT to_version FROM extruntime.schema_migrations_history
		WHERE package = $1 ORDER BY id DESC LIMIT 1`, pkg)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows2.Close()
	if rows2.Next() {
		if err := rows2.Scan(&a.Current); err != nil {
			return nil, serrors.FromDB(op, err)
		}
	}
	return a, rows2.Err()
}

// LoadView reads the schema view one package version was applied with.
func LoadView(ctx context.Context, q Querier, pkg, version string) (*Schema, bool, error) {
	const op = serrors.Op("extruntime.data.LoadView")
	rows, err := q.Query(ctx, `SELECT entity_id, kind, schema_name, table_name, base, fields
		FROM extruntime.entity_schema_versions WHERE package = $1 AND version = $2 ORDER BY entity_id`, pkg, version)
	if err != nil {
		return nil, false, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var s *Schema
	for rows.Next() {
		var t Table
		var kind string
		var base, fields []byte
		if err := rows.Scan(&t.ID, &kind, &t.Schema, &t.Name, &base, &fields); err != nil {
			return nil, false, serrors.FromDB(op, err)
		}
		t.Kind = manifest.ResourceKind(kind)
		if len(base) > 0 {
			if err := json.Unmarshal(base, &t.Base); err != nil {
				return nil, false, serrors.Wrap(op, err)
			}
		}
		if err := json.Unmarshal(fields, &t.Fields); err != nil {
			return nil, false, serrors.Wrap(op, err)
		}
		if s == nil {
			s = &Schema{Package: pkg, Version: version}
		}
		s.Tables = append(s.Tables, t)
	}
	if err := rows.Err(); err != nil {
		return nil, false, serrors.FromDB(op, err)
	}
	return s, s != nil, nil
}

// Apply executes a plan inside tx and records the physical state and the
// version view. It serializes concurrent applies of the same package, refuses
// a plan whose base state moved (ErrStalePlan) and checks preconditions
// (for example duplicate values before a unique index). Any error leaves tx
// to be rolled back by the caller. The record of a plan with no DDL (for
// example an older version adopted again, or a pure rename) still lands.
func Apply(ctx context.Context, tx pgx.Tx, plan MigrationPlan) error {
	const op = serrors.Op("extruntime.data.Apply")
	if plan.View == nil {
		return fail(op, ErrStalePlan, "plan has no target schema")
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "extruntime.schema:"+plan.Package); err != nil {
		return serrors.FromDB(op, err)
	}
	current, err := LoadApplied(ctx, tx, plan.Package)
	if err != nil {
		return err
	}
	currentHash, err := current.stateHash()
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if currentHash != plan.FromHash {
		return fail(op, ErrStalePlan, "applied schema of %s changed since the plan was made", plan.Package)
	}
	for _, s := range plan.Steps {
		if s.Check != nil {
			var exists bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS ("+s.Check.SQL+")").Scan(&exists); err != nil {
				return serrors.FromDB(op, err)
			}
			if exists {
				return &SchemaError{Diagnostics: []Diagnostic{{Resource: s.Resource, Code: DiagUniqueViolatedByRow, Message: s.Check.Message}}}
			}
		}
		if _, err := tx.Exec(ctx, s.SQL); err != nil {
			return serrors.WrapContext(op, serrors.FromDB("", err), s.Description)
		}
	}
	for _, t := range plan.Changed {
		cols, err := json.Marshal(t.Columns)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		var base any
		if t.Base != nil {
			b, err := json.Marshal(t.Base)
			if err != nil {
				return serrors.Wrap(op, err)
			}
			base = string(b)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO extruntime.entity_schemas
			(package, entity_id, kind, schema_name, table_name, base, columns, version)
			VALUES ($1, $2, $3, $4, $5, $6::text::jsonb, $7::text::jsonb, $8)
			ON CONFLICT (package, entity_id) DO UPDATE
			SET columns = EXCLUDED.columns, version = EXCLUDED.version, applied_at = now()`,
			plan.Package, t.ID, string(t.Kind), t.Schema, t.Name, base, string(cols), plan.ToVersion); err != nil {
			return serrors.FromDB(op, err)
		}
	}
	if !plan.Adopted {
		for _, t := range plan.View.Tables {
			fields, err := json.Marshal(t.Fields)
			if err != nil {
				return serrors.Wrap(op, err)
			}
			var base any
			if t.Base != nil {
				b, err := json.Marshal(t.Base)
				if err != nil {
					return serrors.Wrap(op, err)
				}
				base = string(b)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO extruntime.entity_schema_versions
				(package, version, entity_id, kind, schema_name, table_name, base, fields)
				VALUES ($1, $2, $3, $4, $5, $6, $7::text::jsonb, $8::text::jsonb)`,
				plan.Package, plan.ToVersion, t.ID, string(t.Kind), t.Schema, t.Name, base, string(fields)); err != nil {
				return serrors.FromDB(op, err)
			}
		}
	}
	stmts := make([]string, len(plan.Steps))
	for i, s := range plan.Steps {
		stmts[i] = s.SQL
	}
	sj, err := json.Marshal(stmts)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	rj, err := json.Marshal(plan.Renames)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO extruntime.schema_migrations_history
		(package, from_version, to_version, adopted, statements, renames)
		VALUES ($1, $2, $3, $4, $5::text::jsonb, $6::text::jsonb)`,
		plan.Package, plan.FromVersion, plan.ToVersion, plan.Adopted, string(sj), string(rj)); err != nil {
		return serrors.FromDB(op, err)
	}
	return nil
}

// Prepare is the usual activation sequence for one package version: load the
// applied state, compile the manifest, plan, and (when apply is true) apply.
// It returns the plan even when it is refused by *SchemaError for review.
func Prepare(ctx context.Context, tx pgx.Tx, c *Compiler, pkg, version string, m *manifest.Manifest, apply bool) (MigrationPlan, error) {
	applied, err := LoadApplied(ctx, tx, pkg)
	if err != nil {
		return MigrationPlan{}, err
	}
	desired, err := c.Compile(ctx, pkg, version, m)
	if err != nil {
		return MigrationPlan{}, err
	}
	plan, err := Plan(applied, desired)
	if err != nil {
		return plan, err
	}
	if apply {
		if err := Apply(ctx, tx, plan); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

func unmarshalTable(t *Table, base, fields []byte) error {
	if len(base) > 0 {
		if err := json.Unmarshal(base, &t.Base); err != nil {
			return err
		}
	}
	return json.Unmarshal(fields, &t.Fields)
}
