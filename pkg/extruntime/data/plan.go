package data

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// StepKind classifies a DDL step.
type StepKind string

const (
	StepCreateSchema StepKind = "create_schema"
	StepCreateTable  StepKind = "create_table"
	StepAddColumn    StepKind = "add_column"
	StepAlterColumn  StepKind = "alter_column"
	StepConstraint   StepKind = "constraint"
	StepIndex        StepKind = "index"
	StepForeignKey   StepKind = "foreign_key"
)

// Precondition is a query that must return no rows before its step runs.
type Precondition struct {
	SQL     string `json:"sql"`
	Message string `json:"message"`
}

// Step is one DDL statement.
type Step struct {
	Kind        StepKind      `json:"kind"`
	Resource    string        `json:"resource"`
	Description string        `json:"description"`
	SQL         string        `json:"sql"`
	Check       *Precondition `json:"check,omitempty"`
}

// Rename records a metadata-only field key rename: the physical column does
// not change.
type Rename struct {
	Resource string `json:"resource"`
	From     string `json:"from"`
	To       string `json:"to"`
	Column   string `json:"column"`
}

// MigrationPlan is the result of Plan: DDL to run plus the metadata to
// record. It is a plain value; inspect Steps before Apply.
type MigrationPlan struct {
	Package     string
	FromVersion string
	ToVersion   string
	// FromHash identifies the applied physical state the plan was made
	// against; Apply refuses a plan whose base moved.
	FromHash string
	Steps    []Step
	Renames  []Rename
	// Adopted is true when ToVersion was applied before (activating an older
	// version again): no DDL, the recorded view is reused.
	Adopted bool
	// View is the resolved schema of ToVersion with physical columns.
	View *Schema
	// Changed lists the physical tables whose state Apply records.
	Changed []*PhysTable
	// Physical is the physical state after the plan.
	Physical map[string]*PhysTable
}

// Empty reports whether the plan changes nothing physical.
func (p MigrationPlan) Empty() bool { return len(p.Steps) == 0 }

// SQL renders the statements, for review.
func (p MigrationPlan) SQL() string {
	var b strings.Builder
	for _, s := range p.Steps {
		b.WriteString(s.SQL)
		b.WriteString(";\n")
	}
	return b.String()
}

func (a *Applied) stateHash() (string, error) {
	if a == nil || len(a.Tables) == 0 {
		return "", nil
	}
	ids := make([]string, 0, len(a.Tables))
	for id := range a.Tables {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	h := sha256.New()
	for _, id := range ids {
		t := *a.Tables[id]
		t.Version = "" // version bumps alone do not move the physical base
		b, err := json.Marshal(t)
		if err != nil {
			return "", err
		}
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type phases struct {
	schemas, creates, adds, alters, checks, indexes, fks []Step
}

func (p *phases) flatten() []Step {
	var out []Step
	for _, l := range [][]Step{p.schemas, p.creates, p.adds, p.alters, p.checks, p.indexes, p.fks} {
		out = append(out, l...)
	}
	return out
}

// Plan compares the applied state of a package with the desired schema of a
// new version. Field key renames (renamedFrom) are metadata-only, optional
// or defaulted fields and indexes are added automatically, and anything that
// could lose or invalidate data (type change, dropped field or entity, new
// required field without default, optional becoming required) is refused with
// a *SchemaError listing every reason. Tables are never dropped.
func Plan(applied *Applied, desired *Schema) (MigrationPlan, error) {
	const op = "extruntime.data.Plan"
	if applied == nil {
		applied = &Applied{Package: desired.Package, Tables: map[string]*PhysTable{}, Views: map[string]*Schema{}}
	}
	fromHash, err := applied.stateHash()
	if err != nil {
		return MigrationPlan{}, serrors.Wrap(op, err)
	}
	plan := MigrationPlan{
		Package: desired.Package, ToVersion: desired.Version,
		FromVersion: applied.Current, FromHash: fromHash, Physical: map[string]*PhysTable{},
	}
	var bag diagBag

	if v, ok := applied.Views[desired.Version]; ok {
		viewHash, err := v.logicalHash()
		if err != nil {
			return plan, serrors.Wrap(op, err)
		}
		desiredHash, err := desired.logicalHash()
		if err != nil {
			return plan, serrors.Wrap(op, err)
		}
		if viewHash != desiredHash {
			bag.add(desired.Package, "", DiagStructureChange, "version %s was already applied with a different schema", desired.Version)
			return plan, bag.err()
		}
		if err := CompatibleWithPinned(applied, v); err != nil {
			return plan, err
		}
		plan.Adopted, plan.View = true, v
		for id, t := range applied.Tables {
			plan.Physical[id] = t
		}
		return plan, nil
	}

	for id, t := range applied.Tables {
		plan.Physical[id] = t
		if _, ok := desired.Table(id); !ok {
			bag.add(id, "", DiagEntityDropped, "entity is not declared by version %s; tables are never dropped automatically, an explicit reviewed migration is required", desired.Version)
		}
	}

	var ph phases
	schemasSeen := map[string]bool{}
	view := &Schema{Package: desired.Package, Version: desired.Version}
	for _, dt := range desired.Tables {
		pt, exists := applied.Tables[dt.ID]
		var next *PhysTable
		var vt Table
		if !exists {
			next, vt = planNewTable(dt, &ph, schemasSeen)
		} else {
			next, vt = planExistingTable(pt, dt, &ph, &plan, &bag)
		}
		view.Tables = append(view.Tables, vt)
		if next == nil {
			continue
		}
		next.Version = desired.Version
		plan.Physical[dt.ID] = next
		if !exists || !reflect.DeepEqual(stripVersion(pt), stripVersion(next)) {
			plan.Changed = append(plan.Changed, next)
		}
	}
	if err := bag.err(); err != nil {
		return MigrationPlan{}, err
	}
	plan.Steps = ph.flatten()
	plan.View = view
	_ = op
	return plan, nil
}

func stripVersion(t *PhysTable) PhysTable {
	c := *t
	c.Version = ""
	return c
}

func (p *phases) ensureSchema(schema string, seen map[string]bool) {
	if seen[schema] {
		return
	}
	seen[schema] = true
	p.schemas = append(p.schemas, Step{
		Kind: StepCreateSchema, Resource: schema, Description: "create schema " + schema,
		SQL: "CREATE SCHEMA IF NOT EXISTS " + quoteIdent(schema),
	})
}

func physFromField(col string, f FieldSpec) PhysColumn {
	return PhysColumn{
		Name: col, Keys: []string{f.Key}, Type: f.Type, SQLType: f.SQLType, NotNull: f.NotNull,
		DefaultSQL: f.DefaultSQL, Unique: f.Unique, Index: f.Index, Ref: f.Ref,
	}
}

func planNewTable(dt Table, ph *phases, schemasSeen map[string]bool) (*PhysTable, Table) {
	ph.ensureSchema(dt.Schema, schemasSeen)
	pt := &PhysTable{ID: dt.ID, Kind: dt.Kind, Schema: dt.Schema, Name: dt.Name, Base: dt.Base}
	vt := dt
	vt.Fields = make([]FieldSpec, len(dt.Fields))
	taken := map[string]bool{}
	for i, f := range dt.Fields {
		col := assignColumn(f.Key, taken)
		taken[col] = true
		f.Column = col
		vt.Fields[i] = f
		pt.Columns = append(pt.Columns, physFromField(col, f))
	}
	ph.creates = append(ph.creates, Step{
		Kind: StepCreateTable, Resource: dt.ID, Description: "create table " + dt.Schema + "." + dt.Name,
		SQL: createTableSQL(pt),
	})
	for i := range pt.Columns {
		c := &pt.Columns[i]
		ph.indexes = append(ph.indexes, indexSteps(pt, c, indexNone, wantIndex(c))...)
		if fk := columnFK(pt, c); fk != nil {
			ph.fks = append(ph.fks, *fk)
		}
	}
	if fk := baseFK(pt); fk != nil {
		ph.fks = append(ph.fks, *fk)
	}
	return pt, vt
}

type indexKind int

const (
	indexNone indexKind = iota
	indexPlain
	indexUnique
)

func wantIndex(c *PhysColumn) indexKind {
	switch {
	case c.Unique:
		return indexUnique
	case c.Index:
		return indexPlain
	}
	return indexNone
}

func indexName(t *PhysTable, c *PhysColumn, k indexKind) string {
	if k == indexUnique {
		return shortName("uq", t.Name, c.Name)
	}
	return shortName("ix", t.Name, c.Name)
}

// indexSteps moves a column from index kind have to want.
func indexSteps(t *PhysTable, c *PhysColumn, have, want indexKind) []Step {
	if have == want {
		return nil
	}
	var out []Step
	tbl := qualified(t.Schema, t.Name)
	if have != indexNone {
		out = append(out, Step{
			Kind: StepIndex, Resource: t.ID, Description: "drop index on " + c.Name,
			SQL: "DROP INDEX " + qualified(t.Schema, indexName(t, c, have)),
		})
	}
	switch want {
	case indexUnique:
		step := Step{
			Kind: StepIndex, Resource: t.ID, Description: "unique index on " + c.Name,
			SQL: fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (tenant_id, %s)", quoteIdent(indexName(t, c, want)), tbl, quoteIdent(c.Name)),
		}
		if have == indexNone || have == indexPlain {
			step.Check = &Precondition{
				SQL:     fmt.Sprintf("SELECT 1 FROM %s WHERE %s IS NOT NULL GROUP BY tenant_id, %s HAVING count(*) > 1 LIMIT 1", tbl, quoteIdent(c.Name), quoteIdent(c.Name)),
				Message: fmt.Sprintf("%s.%s: existing rows already hold duplicate values, cannot make the field unique", t.ID, c.currentKey()),
			}
		}
		out = append(out, step)
	case indexPlain:
		out = append(out, Step{
			Kind: StepIndex, Resource: t.ID, Description: "index on " + c.Name,
			SQL: fmt.Sprintf("CREATE INDEX %s ON %s (tenant_id, %s)", quoteIdent(indexName(t, c, want)), tbl, quoteIdent(c.Name)),
		})
	case indexNone:
		// Only the drop above applies.
	}
	return out
}

func columnFK(t *PhysTable, c *PhysColumn) *Step {
	if c.Ref == nil || c.Ref.Native {
		return nil
	}
	return &Step{
		Kind: StepForeignKey, Resource: t.ID, Description: "foreign key " + c.Name + " -> " + c.Ref.Entity,
		SQL: fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (tenant_id, %s) REFERENCES %s (tenant_id, %s)",
			qualified(t.Schema, t.Name), quoteIdent(shortName("fk", t.Name, c.Name)), quoteIdent(c.Name),
			qualified(c.Ref.Schema, c.Ref.Table), quoteIdent(c.Ref.KeyColumn)),
	}
}

func baseFK(t *PhysTable) *Step {
	if t.Base == nil {
		return nil
	}
	name := quoteIdent(shortName("fk", t.Name, "base_key"))
	tbl := qualified(t.Schema, t.Name)
	b := t.Base
	sql := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (base_key) REFERENCES %s (%s) ON DELETE CASCADE",
		tbl, name, qualified(b.Schema, b.Table), quoteIdent(b.KeyColumn))
	if !b.Native {
		sql = fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (tenant_id, base_key) REFERENCES %s (tenant_id, %s) ON DELETE CASCADE",
			tbl, name, qualified(b.Schema, b.Table), quoteIdent(b.KeyColumn))
	}
	return &Step{Kind: StepForeignKey, Resource: t.ID, Description: "overlay base foreign key", SQL: sql}
}

func columnDef(c *PhysColumn) string {
	def := quoteIdent(c.Name) + " " + c.SQLType
	if c.NotNull {
		def += " NOT NULL"
	}
	if c.DefaultSQL != "" {
		def += " DEFAULT " + c.DefaultSQL
	}
	return def
}

func enumCheck(t *PhysTable, c *PhysColumn) string {
	vals := make([]string, len(c.Type.Values))
	for i, v := range c.Type.Values {
		vals[i] = quoteLiteral(v)
	}
	return fmt.Sprintf("CONSTRAINT %s CHECK (%s IN (%s))", quoteIdent(shortName("ck", t.Name, c.Name)), quoteIdent(c.Name), strings.Join(vals, ", "))
}

func createTableSQL(t *PhysTable) string {
	lines := make([]string, 0, len(t.Columns)*2+8)
	lines = append(lines, "tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE")
	if t.Kind == manifest.ResOverlay {
		lines = append(lines, "base_key "+t.Base.KeyType.SQL()+" NOT NULL")
	} else {
		lines = append(lines, "id uuid NOT NULL DEFAULT gen_random_uuid()")
	}
	for i := range t.Columns {
		lines = append(lines, columnDef(&t.Columns[i]))
	}
	if t.Kind == manifest.ResEntity {
		lines = append(lines,
			"created_at timestamptz NOT NULL DEFAULT now()",
			"updated_at timestamptz NOT NULL DEFAULT now()",
			"row_version integer NOT NULL DEFAULT 1",
			"PRIMARY KEY (id)",
			"CONSTRAINT "+quoteIdent(shortName("uq", t.Name, "tenant_id_id"))+" UNIQUE (tenant_id, id)",
		)
	} else {
		lines = append(lines, "PRIMARY KEY (tenant_id, base_key)")
	}
	for i := range t.Columns {
		if t.Columns[i].Type.Kind == manifest.KindEnum {
			lines = append(lines, enumCheck(t, &t.Columns[i]))
		}
	}
	return "CREATE TABLE " + qualified(t.Schema, t.Name) + " (\n    " + strings.Join(lines, ",\n    ") + "\n)"
}

func planExistingTable(pt *PhysTable, dt Table, ph *phases, plan *MigrationPlan, bag *diagBag) (*PhysTable, Table) {
	if pt.Kind != dt.Kind || pt.Schema != dt.Schema || pt.Name != dt.Name {
		bag.add(dt.ID, "", DiagStructureChange, "entity kind or location changed (%s %s.%s -> %s %s.%s)", pt.Kind, pt.Schema, pt.Name, dt.Kind, dt.Schema, dt.Name)
		return nil, dt
	}
	if !reflect.DeepEqual(pt.Base, dt.Base) {
		bag.add(dt.ID, "", DiagStructureChange, "overlay base changed; an overlay cannot move to another base entity")
		return nil, dt
	}
	next := pt.clone()
	current := map[string]*PhysColumn{}
	for i := range next.Columns {
		current[next.Columns[i].currentKey()] = &next.Columns[i]
	}
	declared := map[string]bool{}
	for _, f := range dt.Fields {
		declared[f.Key] = true
	}

	matched := map[string]string{} // physical column -> desired key
	vt := dt
	vt.Fields = make([]FieldSpec, len(dt.Fields))
	type pair struct {
		pc *PhysColumn
		f  FieldSpec
		i  int
	}
	pairs := make([]pair, 0, len(dt.Fields))
	var added []int
	for i, f := range dt.Fields {
		var pc *PhysColumn
		for _, a := range f.Aliases {
			if declared[a] {
				bag.add(dt.ID, f.Key, DiagAmbiguousRename, "renamedFrom %q, but %q is still declared as its own field", a, a)
			}
		}
		if c, ok := current[f.Key]; ok {
			pc = c
		} else {
			for _, a := range f.Aliases {
				if declared[a] {
					continue
				}
				if c, ok := current[a]; ok {
					pc = c
					plan.Renames = append(plan.Renames, Rename{Resource: dt.ID, From: a, To: f.Key, Column: c.Name})
					break
				}
			}
		}
		if pc == nil {
			added = append(added, i)
			continue
		}
		if other, dup := matched[pc.Name]; dup {
			bag.add(dt.ID, f.Key, DiagAmbiguousRename, "maps to the same physical column as %q", other)
			continue
		}
		matched[pc.Name] = f.Key
		pairs = append(pairs, pair{pc, f, i})
	}
	for i := range next.Columns {
		c := &next.Columns[i]
		if _, ok := matched[c.Name]; !ok {
			bag.add(dt.ID, c.currentKey(), DiagFieldDropped, "field is not declared by the new version; columns are never dropped automatically, an explicit reviewed migration is required")
		}
	}

	for _, p := range pairs {
		f := p.f
		diffColumn(next, p.pc, f, ph, bag)
		if p.pc.currentKey() != f.Key {
			p.pc.Keys = append(p.pc.Keys, f.Key)
		}
		f.Column = p.pc.Name
		vt.Fields[p.i] = f
	}
	taken := map[string]bool{}
	for _, c := range next.Columns {
		taken[c.Name] = true
	}
	for _, i := range added {
		f := dt.Fields[i]
		if f.NotNull && f.DefaultSQL == "" {
			bag.add(dt.ID, f.Key, DiagRequiredNoDefault, "new field is required but has no default; existing rows could not be filled. Make it optional or give it a default")
			continue
		}
		col := assignColumn(f.Key, taken)
		taken[col] = true
		f.Column = col
		vt.Fields[i] = f
		pc := physFromField(col, f)
		next.Columns = append(next.Columns, pc)
		c := &next.Columns[len(next.Columns)-1]
		ph.adds = append(ph.adds, Step{
			Kind: StepAddColumn, Resource: dt.ID, Description: "add column " + col,
			SQL: "ALTER TABLE " + qualified(next.Schema, next.Name) + " ADD COLUMN " + columnDef(c),
		})
		if c.Type.Kind == manifest.KindEnum {
			ph.checks = append(ph.checks, Step{
				Kind: StepConstraint, Resource: dt.ID, Description: "enum check on " + col,
				SQL: "ALTER TABLE " + qualified(next.Schema, next.Name) + " ADD " + enumCheck(next, c),
			})
		}
		ph.indexes = append(ph.indexes, indexSteps(next, c, indexNone, wantIndex(c))...)
		if fk := columnFK(next, c); fk != nil {
			ph.fks = append(ph.fks, *fk)
		}
	}
	return next, vt
}

func diffColumn(t *PhysTable, pc *PhysColumn, f FieldSpec, ph *phases, bag *diagBag) {
	tbl := qualified(t.Schema, t.Name)
	col := quoteIdent(pc.Name)
	reject := func(code, format string, args ...any) { bag.add(t.ID, f.Key, code, format, args...) }

	if pc.Type.Kind != f.Type.Kind {
		reject(DiagTypeChange, "type changes from %s to %s; an incompatible change requires an explicit reviewed migration", pc.Type.Kind, f.Type.Kind)
		return
	}
	if (pc.Ref == nil) != (f.Ref == nil) || (pc.Ref != nil && pc.Ref.Entity != f.Ref.Entity) {
		reject(DiagTypeChange, "ref target changes from %s to %s; an incompatible change requires an explicit reviewed migration", refName(pc.Ref), refName(f.Ref))
		return
	}
	alterType := false
	switch f.Type.Kind {
	case manifest.KindString:
		switch {
		case pc.Type.MaxLength == f.Type.MaxLength:
		case pc.Type.MaxLength > 0 && (f.Type.MaxLength == 0 || f.Type.MaxLength > pc.Type.MaxLength):
			alterType = true
		default:
			reject(DiagNarrowing, "maxLength changes from %d to %d; shrinking could invalidate stored values", pc.Type.MaxLength, f.Type.MaxLength)
			return
		}
	case manifest.KindDecimal:
		ps, pp, ns, np := pc.Type.Scale, pc.Type.Precision, f.Type.Scale, f.Type.Precision
		switch {
		case ps == ns && pp == np:
		case pp > 0 && (np == 0 || (ns == ps && np > pp)):
			alterType = true
		default:
			reject(DiagTypeChange, "decimal(%d,%d) changes to decimal(%d,%d); only widening precision at the same scale is automatic", pp, ps, np, ns)
			return
		}
	case manifest.KindEnum:
		have := map[string]bool{}
		for _, v := range f.Type.Values {
			have[v] = true
		}
		var removed []string
		for _, v := range pc.Type.Values {
			if !have[v] {
				removed = append(removed, v)
			}
		}
		if len(removed) > 0 {
			reject(DiagNarrowing, "enum values %s were removed; stored rows may still hold them", strings.Join(removed, ", "))
			return
		}
		if len(pc.Type.Values) != len(f.Type.Values) {
			name := quoteIdent(shortName("ck", t.Name, pc.Name))
			next := physFromField(pc.Name, f)
			ph.checks = append(ph.checks,
				Step{Kind: StepConstraint, Resource: t.ID, Description: "drop enum check on " + pc.Name, SQL: "ALTER TABLE " + tbl + " DROP CONSTRAINT " + name},
				Step{Kind: StepConstraint, Resource: t.ID, Description: "enum check on " + pc.Name, SQL: "ALTER TABLE " + tbl + " ADD " + enumCheck(t, &next)},
			)
		}
	case manifest.KindInt, manifest.KindInt64, manifest.KindBool, manifest.KindDate,
		manifest.KindDateTime, manifest.KindUUID, manifest.KindJSON, manifest.KindRef,
		manifest.KindObject, manifest.KindArray:
		// Nothing beyond the kind to reconcile.
	}
	if alterType {
		ph.alters = append(ph.alters, Step{
			Kind: StepAlterColumn, Resource: t.ID, Description: "widen " + pc.Name,
			SQL: "ALTER TABLE " + tbl + " ALTER COLUMN " + col + " TYPE " + f.SQLType,
		})
	}
	switch {
	case pc.NotNull && !f.NotNull:
		ph.alters = append(ph.alters, Step{
			Kind: StepAlterColumn, Resource: t.ID, Description: "drop not null on " + pc.Name,
			SQL: "ALTER TABLE " + tbl + " ALTER COLUMN " + col + " DROP NOT NULL",
		})
	case !pc.NotNull && f.NotNull:
		reject(DiagTightenNullability, "field becomes required but existing rows may hold null; an explicit reviewed migration is required")
		return
	}
	if pc.DefaultSQL != f.DefaultSQL {
		sql := "ALTER TABLE " + tbl + " ALTER COLUMN " + col + " DROP DEFAULT"
		if f.DefaultSQL != "" {
			sql = "ALTER TABLE " + tbl + " ALTER COLUMN " + col + " SET DEFAULT " + f.DefaultSQL
		}
		ph.alters = append(ph.alters, Step{Kind: StepAlterColumn, Resource: t.ID, Description: "default of " + pc.Name, SQL: sql})
	}
	ph.indexes = append(ph.indexes, indexSteps(t, pc, wantIndex(pc), wantIndex(&PhysColumn{Name: pc.Name, Unique: f.Unique, Index: f.Index, Keys: []string{f.Key}}))...)

	pc.Type, pc.SQLType, pc.NotNull, pc.DefaultSQL = f.Type, f.SQLType, f.NotNull, f.DefaultSQL
	pc.Unique, pc.Index = f.Unique, f.Index
}

func refName(r *Target) string {
	if r == nil {
		return "none"
	}
	return r.Entity
}
