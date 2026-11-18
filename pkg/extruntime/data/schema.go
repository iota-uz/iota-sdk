package data

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// Target locates the table and key a ref or overlay points at.
type Target struct {
	Entity    string  `json:"entity"`
	Native    bool    `json:"native,omitempty"`
	Schema    string  `json:"schema"`
	Table     string  `json:"table"`
	KeyColumn string  `json:"keyColumn"`
	KeyType   KeyType `json:"keyType"`
}

// FieldSpec is one field key of a package version, resolved to SQL. Column is
// the immutable physical column the key maps to; it is empty until Plan
// assigns it.
type FieldSpec struct {
	Key        string          `json:"key"`
	Column     string          `json:"column,omitempty"`
	Aliases    []string        `json:"aliases,omitempty"` // renamedFrom keys, resolvable to Column
	Type       manifest.Type   `json:"type"`
	Unique     bool            `json:"unique,omitempty"`
	Index      bool            `json:"index,omitempty"`
	Default    json.RawMessage `json:"default,omitempty"`
	SQLType    string          `json:"sqlType"`
	NotNull    bool            `json:"notNull,omitempty"`
	DefaultSQL string          `json:"defaultSql,omitempty"`
	Ref        *Target         `json:"ref,omitempty"`
}

// Table is one entity or overlay as seen by a package version.
type Table struct {
	ID     string                `json:"id"`
	Kind   manifest.ResourceKind `json:"kind"`
	Schema string                `json:"schema"`
	Name   string                `json:"name"`
	Base   *Target               `json:"base,omitempty"` // overlay base
	Fields []FieldSpec           `json:"fields"`
}

// Schema is the view of one package version: field keys, types and the
// physical columns they map to.
type Schema struct {
	Package string  `json:"package"`
	Version string  `json:"version"`
	Tables  []Table `json:"tables"`
}

// Table returns the table of a resource ID.
func (s *Schema) Table(id string) (*Table, bool) {
	for i := range s.Tables {
		if s.Tables[i].ID == id {
			return &s.Tables[i], true
		}
	}
	return nil, false
}

// logicalHash identifies the schema ignoring physical column assignment.
func (s *Schema) logicalHash() (string, error) {
	cp := Schema{Package: s.Package, Version: s.Version, Tables: make([]Table, len(s.Tables))}
	for i, t := range s.Tables {
		t.Fields = append([]FieldSpec(nil), t.Fields...)
		for j := range t.Fields {
			t.Fields[j].Column = ""
		}
		cp.Tables[i] = t
	}
	b, err := json.Marshal(cp)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// PhysColumn is a column of a physical table. Keys lists every field key that
// ever mapped to it; the last one is current. The column name never changes.
type PhysColumn struct {
	Name       string        `json:"name"`
	Keys       []string      `json:"keys"`
	Type       manifest.Type `json:"type"`
	SQLType    string        `json:"sqlType"`
	NotNull    bool          `json:"notNull,omitempty"`
	DefaultSQL string        `json:"defaultSql,omitempty"`
	Unique     bool          `json:"unique,omitempty"`
	Index      bool          `json:"index,omitempty"`
	Ref        *Target       `json:"ref,omitempty"`
}

func (c *PhysColumn) currentKey() string { return c.Keys[len(c.Keys)-1] }

// PhysTable is the applied physical state of one entity or overlay.
type PhysTable struct {
	ID      string                `json:"id"`
	Kind    manifest.ResourceKind `json:"kind"`
	Schema  string                `json:"schema"`
	Name    string                `json:"name"`
	Base    *Target               `json:"base,omitempty"`
	Columns []PhysColumn          `json:"columns"`
	Version string                `json:"version"`
}

func (t *PhysTable) column(name string) (*PhysColumn, bool) {
	for i := range t.Columns {
		if t.Columns[i].Name == name {
			return &t.Columns[i], true
		}
	}
	return nil, false
}

func (t *PhysTable) clone() *PhysTable {
	cp := *t
	cp.Columns = make([]PhysColumn, len(t.Columns))
	for i, c := range t.Columns {
		c.Keys = append([]string(nil), c.Keys...)
		cp.Columns[i] = c
	}
	return &cp
}

// Applied is everything recorded for a package: physical tables plus the
// per-version views. A nil *Applied means nothing was applied yet.
type Applied struct {
	Package string
	// Current is the version applied most recently.
	Current string
	Tables  map[string]*PhysTable
	Views   map[string]*Schema
}

// Compiler turns manifests into logical schemas. Ref and overlay targets are
// resolved against the manifest itself, the native registry and the catalog.
type Compiler struct {
	natives *NativeRegistry
	catalog spi.Catalog
}

func NewCompiler(natives *NativeRegistry, catalog spi.Catalog) *Compiler {
	if natives == nil {
		natives = NewNativeRegistry()
	}
	return &Compiler{natives: natives, catalog: catalog}
}

type diagBag struct{ list []Diagnostic }

func (d *diagBag) add(res, field, code, format string, args ...any) {
	d.list = append(d.list, Diagnostic{Resource: res, Field: field, Code: code, Message: fmt.Sprintf(format, args...)})
}

func (d *diagBag) err() error {
	if len(d.list) == 0 {
		return nil
	}
	return &SchemaError{Diagnostics: d.list}
}

// Compile maps the entity and overlay resources of m to tables. Physical
// columns are assigned later by Plan.
func (c *Compiler) Compile(ctx context.Context, pkg, version string, m *manifest.Manifest) (*Schema, error) {
	const op = "extruntime.data.Compiler.Compile"
	if err := m.Validate(); err != nil {
		return nil, fail(op, ErrIncompatibleSchema, "%v", err)
	}
	var bag diagBag
	out := &Schema{Package: pkg, Version: version}
	names := map[string]string{}
	for _, r := range m.Resources {
		if r.Kind != manifest.ResEntity && r.Kind != manifest.ResOverlay {
			continue
		}
		schema, table, err := entityLocation(r.ID)
		if err != nil {
			bag.add(r.ID, "", DiagInvalidResource, "%v", err)
			continue
		}
		if prev, dup := names[table]; dup {
			bag.add(r.ID, "", DiagInvalidResource, "table %s.%s collides with %s", schema, table, prev)
			continue
		}
		names[table] = r.ID
		t := Table{ID: r.ID, Kind: r.Kind, Schema: schema, Name: table}
		if r.Kind == manifest.ResOverlay {
			base, err := c.resolveTarget(ctx, m, r.Base)
			if err != nil {
				bag.add(r.ID, "", DiagUnresolvedRef, "overlay base %q: %v", r.Base, err)
				continue
			}
			t.Base = base
		}
		seen := map[string]bool{}
		for _, f := range r.Fields {
			if seen[f.ID] {
				bag.add(r.ID, f.ID, DiagInvalidField, "declared twice")
				continue
			}
			seen[f.ID] = true
			spec, err := c.compileField(ctx, m, f)
			if err != nil {
				bag.add(r.ID, f.ID, DiagInvalidField, "%v", err)
				continue
			}
			t.Fields = append(t.Fields, spec)
		}
		out.Tables = append(out.Tables, t)
	}
	if err := bag.err(); err != nil {
		return nil, err
	}
	sort.Slice(out.Tables, func(i, j int) bool { return out.Tables[i].ID < out.Tables[j].ID })
	return out, nil
}

func (c *Compiler) resolveTarget(ctx context.Context, m *manifest.Manifest, id string) (*Target, error) {
	if id == "" {
		return nil, fmt.Errorf("target entity is empty")
	}
	if r, ok := m.Find(id); ok {
		if r.Kind != manifest.ResEntity {
			return nil, fmt.Errorf("%s is %s, not an entity", id, r.Kind)
		}
		return ownTarget(id)
	}
	if n, ok := c.natives.Get(id); ok {
		return &Target{Entity: id, Native: true, Schema: n.schema(), Table: n.Table, KeyColumn: n.KeyColumn, KeyType: n.KeyType}, nil
	}
	if c.catalog != nil {
		rr, ok, err := c.catalog.Resolve(ctx, id)
		if err != nil {
			return nil, err
		}
		if ok {
			if rr.Native {
				return nil, fmt.Errorf("native entity %s has no data descriptor", id)
			}
			if rr.Resource.Kind != manifest.ResEntity {
				return nil, fmt.Errorf("%s is %s, not an entity", id, rr.Resource.Kind)
			}
			return ownTarget(id)
		}
	}
	return nil, fmt.Errorf("entity %s is not declared, native or in the catalog", id)
}

func ownTarget(id string) (*Target, error) {
	schema, table, err := entityLocation(id)
	if err != nil {
		return nil, err
	}
	return &Target{Entity: id, Schema: schema, Table: table, KeyColumn: "id", KeyType: KeyUUID}, nil
}

func (c *Compiler) compileField(ctx context.Context, m *manifest.Manifest, f manifest.Field) (FieldSpec, error) {
	if !fieldKeyRe.MatchString(f.ID) || len(f.ID) > 48 {
		return FieldSpec{}, fmt.Errorf("key must match %s and be at most 48 characters", fieldKeyRe)
	}
	if reservedKeys[f.ID] {
		return FieldSpec{}, fmt.Errorf("%q is reserved by the record wire form", f.ID)
	}
	spec := FieldSpec{
		Key: f.ID, Type: f.Type, Unique: f.Unique, Index: f.Index,
		NotNull: !f.Type.Optional && !f.Type.Nullable,
	}
	for _, a := range f.RenamedFrom {
		if !fieldKeyRe.MatchString(a) {
			return FieldSpec{}, fmt.Errorf("renamedFrom %q is not a valid field key", a)
		}
		if a == f.ID {
			return FieldSpec{}, fmt.Errorf("renamedFrom lists the field itself")
		}
		spec.Aliases = append(spec.Aliases, a)
	}
	switch f.Type.Kind {
	case manifest.KindObject, manifest.KindArray:
		return FieldSpec{}, fmt.Errorf("%s fields are not stored; declare a json field", f.Type.Kind)
	case manifest.KindRef:
		target, err := c.resolveTarget(ctx, m, f.Type.Entity)
		if err != nil {
			return FieldSpec{}, fmt.Errorf("ref target: %v", err)
		}
		spec.Ref = target
		spec.SQLType = target.KeyType.SQL()
	case manifest.KindString, manifest.KindInt, manifest.KindInt64, manifest.KindDecimal,
		manifest.KindBool, manifest.KindDate, manifest.KindDateTime, manifest.KindUUID,
		manifest.KindEnum, manifest.KindJSON:
		fallthrough
	default:
		t, err := sqlType(f.Type)
		if err != nil {
			return FieldSpec{}, err
		}
		spec.SQLType = t
	}
	if (f.Unique || f.Index) && f.Type.Kind == manifest.KindJSON {
		return FieldSpec{}, fmt.Errorf("json fields cannot be unique or indexed")
	}
	if len(f.Default) > 0 && string(f.Default) != "null" {
		if f.Type.Kind == manifest.KindRef {
			return FieldSpec{}, fmt.Errorf("ref fields cannot have a default")
		}
		if err := manifest.ValidateValue(withoutNull(f.Type), f.Default); err != nil {
			return FieldSpec{}, fmt.Errorf("default: %v", err)
		}
		canon, err := canonicalValue(f.Type, f.Default)
		if err != nil {
			return FieldSpec{}, fmt.Errorf("default: %v", err)
		}
		spec.Default = canon
		lit, err := defaultLiteral(f.Type, canon)
		if err != nil {
			return FieldSpec{}, fmt.Errorf("default: %v", err)
		}
		spec.DefaultSQL = lit
	}
	return spec, nil
}

func withoutNull(t manifest.Type) manifest.Type {
	t.Nullable = false
	return t
}

func sqlType(t manifest.Type) (string, error) {
	switch t.Kind {
	case manifest.KindString:
		if t.MaxLength > 0 {
			return "varchar(" + strconv.Itoa(t.MaxLength) + ")", nil
		}
		return "text", nil
	case manifest.KindInt, manifest.KindInt64:
		return "bigint", nil
	case manifest.KindDecimal:
		if t.Precision <= 0 {
			if t.Scale > 0 {
				return "", fmt.Errorf("decimal scale requires a precision")
			}
			return "numeric", nil
		}
		if t.Scale < 0 || t.Scale > t.Precision || t.Precision > 1000 {
			return "", fmt.Errorf("invalid decimal(%d,%d)", t.Precision, t.Scale)
		}
		return fmt.Sprintf("numeric(%d,%d)", t.Precision, t.Scale), nil
	case manifest.KindBool:
		return "boolean", nil
	case manifest.KindDate:
		return "date", nil
	case manifest.KindDateTime:
		return "timestamptz", nil
	case manifest.KindUUID:
		return "uuid", nil
	case manifest.KindEnum:
		if len(t.Values) == 0 {
			return "", fmt.Errorf("enum without values")
		}
		seen := map[string]bool{}
		for _, v := range t.Values {
			if v == "" || seen[v] {
				return "", fmt.Errorf("enum values must be unique and non-empty")
			}
			seen[v] = true
		}
		return "text", nil
	case manifest.KindJSON:
		return "jsonb", nil
	case manifest.KindRef, manifest.KindObject, manifest.KindArray:
		fallthrough
	default:
		return "", fmt.Errorf("type %q cannot be stored", t.Kind)
	}
}

func defaultLiteral(t manifest.Type, canon json.RawMessage) (string, error) {
	switch t.Kind {
	case manifest.KindString, manifest.KindEnum:
		var s string
		if err := json.Unmarshal(canon, &s); err != nil {
			return "", err
		}
		return quoteLiteral(s), nil
	case manifest.KindInt:
		return strings.TrimSpace(string(canon)), nil
	case manifest.KindInt64, manifest.KindDecimal:
		var s string
		if err := json.Unmarshal(canon, &s); err != nil {
			return "", err
		}
		return "'" + s + "'::" + map[manifest.TypeKind]string{manifest.KindInt64: "bigint", manifest.KindDecimal: "numeric"}[t.Kind], nil
	case manifest.KindBool:
		return strings.TrimSpace(string(canon)), nil
	case manifest.KindDate, manifest.KindDateTime, manifest.KindUUID:
		var s string
		if err := json.Unmarshal(canon, &s); err != nil {
			return "", err
		}
		return quoteLiteral(s) + "::" + map[manifest.TypeKind]string{manifest.KindDate: "date", manifest.KindDateTime: "timestamptz", manifest.KindUUID: "uuid"}[t.Kind], nil
	case manifest.KindJSON:
		return quoteLiteral(string(canon)) + "::jsonb", nil
	case manifest.KindRef, manifest.KindObject, manifest.KindArray:
		// Refs and composites cannot carry a default; reported below.
	}
	return "", fmt.Errorf("type %q cannot have a default", t.Kind)
}
