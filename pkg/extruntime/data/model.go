package data

import (
	"context"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type modelKind int

const (
	kindOwn modelKind = iota
	kindOverlay
	kindNative
)

// fieldModel is one field key as the calling package version sees it.
type fieldModel struct {
	Key        string
	Column     string
	Type       manifest.Type
	Nullable   bool // may hold NULL: optional or nullable
	NotNull    bool
	Default    []byte // canonical wire default, nil if none
	Unique     bool
	RefKeyType KeyType
	Meta       bool // pseudo field (key, createdAt, ...)
}

// model is the resolved shape of an entity, overlay or native entity for one
// call: physical location, the caller's field keys and the read policy.
type model struct {
	ID   string
	Kind modelKind
	// Owner is the namespace of the package that declares the entity/overlay.
	Owner     string
	Schema    string
	Table     string
	TenantCol string
	KeyCol    string
	KeyType   KeyType
	Fields    []*fieldModel
	byKey     map[string]*fieldModel
	meta      map[string]*fieldModel
	hasMeta   bool
	// Base is the overlay's base target.
	Base *Target

	// read policy
	rules    []manifest.ReadRule
	policy   *model // model of the active version, used to compile rules
	perms    map[string][]string
	readPerm string // native
	uniques  map[string]string
}

func (m *model) qualified() string { return qualified(m.Schema, m.Table) }

// lookup resolves a field key (or alias, or pseudo-field).
func (m *model) lookup(key string) (*fieldModel, bool) {
	if f, ok := m.byKey[key]; ok {
		return f, true
	}
	if f, ok := m.meta[key]; ok {
		return f, true
	}
	return nil, false
}

func (m *model) hidden(sub spi.Subject, f *fieldModel) bool {
	for _, p := range m.perms[f.Column] {
		if !sub.Has(p) {
			return true
		}
	}
	return false
}

func newModel(t *Table, id string) *model {
	m := &model{ID: id, Schema: t.Schema, Table: t.Name, TenantCol: "tenant_id",
		byKey: map[string]*fieldModel{}, meta: map[string]*fieldModel{}, uniques: map[string]string{}}
	switch t.Kind {
	case manifest.ResOverlay:
		m.Kind, m.KeyCol, m.KeyType, m.Base = kindOverlay, "base_key", t.Base.KeyType, t.Base
	case manifest.ResEntity, manifest.ResQuery, manifest.ResCommand, manifest.ResOperation,
		manifest.ResEvent, manifest.ResWorkflow, manifest.ResNotice, manifest.ResRoute,
		manifest.ResPermission, manifest.ResIssue, manifest.ResDomainError, manifest.ResColumn,
		manifest.ResPage, manifest.ResNavigation, manifest.ResSlot, manifest.ResCell,
		manifest.ResShell, manifest.ResContactResolver, manifest.ResDeliveryPolicy:
		fallthrough
	default:
		m.Kind, m.KeyCol, m.KeyType, m.hasMeta = kindOwn, "id", KeyUUID, true
	}
	for _, f := range t.Fields {
		fm := &fieldModel{
			Key: f.Key, Column: f.Column, Type: f.Type, NotNull: f.NotNull,
			Nullable: !f.NotNull, Unique: f.Unique, Default: f.Default,
		}
		if f.Ref != nil {
			fm.RefKeyType = f.Ref.KeyType
		}
		m.Fields = append(m.Fields, fm)
		m.byKey[f.Key] = fm
		for _, a := range f.Aliases {
			if _, taken := m.byKey[a]; !taken {
				m.byKey[a] = fm
			}
		}
		if f.Unique {
			m.uniques[shortName("uq", t.Name, f.Column)] = f.Key
		}
	}
	m.addMeta()
	return m
}

func (m *model) addMeta() {
	m.meta["key"] = &fieldModel{Key: "key", Column: m.KeyCol, Type: m.KeyType.wireType(), NotNull: true, Meta: true}
	if m.hasMeta {
		m.meta["createdAt"] = &fieldModel{Key: "createdAt", Column: "created_at", Type: manifest.Type{Kind: manifest.KindDateTime}, NotNull: true, Meta: true}
		m.meta["updatedAt"] = &fieldModel{Key: "updatedAt", Column: "updated_at", Type: manifest.Type{Kind: manifest.KindDateTime}, NotNull: true, Meta: true}
		m.meta["rowVersion"] = &fieldModel{Key: "rowVersion", Column: "row_version", Type: manifest.Type{Kind: manifest.KindInt}, NotNull: true, Meta: true}
	}
}

func newNativeModel(n NativeEntity) *model {
	m := &model{ID: n.ID, Kind: kindNative, Schema: n.schema(), Table: n.Table, TenantCol: n.TenantColumn,
		KeyCol: n.KeyColumn, KeyType: n.KeyType, byKey: map[string]*fieldModel{}, meta: map[string]*fieldModel{},
		readPerm: n.ReadPermission, perms: map[string][]string{}}
	for _, f := range n.Fields {
		fm := &fieldModel{Key: f.ID, Column: f.Column, Type: f.Type, Nullable: f.Type.Optional || f.Type.Nullable}
		fm.NotNull = !fm.Nullable
		if f.Type.Kind == manifest.KindRef {
			fm.RefKeyType = KeyText
		}
		m.Fields = append(m.Fields, fm)
		m.byKey[f.ID] = fm
		if p := n.FieldPermissions[f.ID]; len(p) > 0 {
			m.perms[f.Column] = p
		}
	}
	m.addMeta()
	return m
}

// SchemaSource reads the applied view of a package version.
type SchemaSource interface {
	Table(ctx context.Context, q Querier, pkg, version, entityID string) (*Table, error)
}

type dbSchemas struct{}

func (dbSchemas) Table(ctx context.Context, q Querier, pkg, version, entityID string) (*Table, error) {
	const op = serrors.Op("extruntime.data.SchemaSource.Table")
	rows, err := q.Query(ctx, `SELECT kind, schema_name, table_name, base, fields
		FROM extruntime.entity_schema_versions WHERE package = $1 AND version = $2 AND entity_id = $3`, pkg, version, entityID)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	t := &Table{ID: entityID}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		return nil, fail(op, ErrUnknownEntity, "%s has no applied schema in %s@%s", entityID, pkg, version)
	}
	var kind string
	var base, fields []byte
	if err := rows.Scan(&kind, &t.Schema, &t.Name, &base, &fields); err != nil {
		return nil, serrors.FromDB(op, err)
	}
	t.Kind = manifest.ResourceKind(kind)
	if err := unmarshalTable(t, base, fields); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return t, nil
}

// CachedSchemas memoizes a SchemaSource by (package, version, entity). Use it
// only when versions are applied in their own committed transactions: an
// entry read from an uncommitted apply would outlive a rollback.
type CachedSchemas struct {
	Inner SchemaSource
	cache sync.Map
}

func (c *CachedSchemas) Table(ctx context.Context, q Querier, pkg, version, entityID string) (*Table, error) {
	k := pkg + "\x00" + version + "\x00" + entityID
	if v, ok := c.cache.Load(k); ok {
		return v.(*Table), nil
	}
	inner := c.Inner
	if inner == nil {
		inner = dbSchemas{}
	}
	t, err := inner.Table(ctx, q, pkg, version, entityID)
	if err != nil {
		return nil, err
	}
	c.cache.Store(k, t)
	return t, nil
}

// load resolves an entity, overlay or native entity for one call.
func (s *Store) load(ctx context.Context, q Querier, sc spi.Scope, id string) (*model, error) {
	const op = serrors.Op("extruntime.data.Store.load")
	if n, ok := s.natives.Get(id); ok {
		return newNativeModel(n), nil
	}
	if s.catalog == nil {
		return nil, fail(op, ErrUnknownEntity, "%s", id)
	}
	rr, ok, err := s.catalog.Resolve(ctx, id)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if !ok {
		return nil, fail(op, ErrUnknownEntity, "%s", id)
	}
	if rr.Native {
		return nil, fail(op, ErrUnknownEntity, "native entity %s has no data descriptor", id)
	}
	if rr.Resource.Kind != manifest.ResEntity && rr.Resource.Kind != manifest.ResOverlay {
		return nil, fail(op, ErrUnknownEntity, "%s is a %s, not an entity", id, rr.Resource.Kind)
	}
	if rr.Version == "" {
		return nil, fail(op, ErrUnknownEntity, "%s has no active package version", id)
	}
	viewVersion := rr.Version
	if sc.Caller.Package == rr.Package && sc.Caller.Version != "" {
		viewVersion = sc.Caller.Version
	}
	table, err := s.schemas.Table(ctx, q, rr.Package, viewVersion, id)
	if err != nil {
		return nil, err
	}
	m := newModel(table, id)
	m.Owner = rr.Package
	active := table
	if viewVersion != rr.Version {
		if active, err = s.schemas.Table(ctx, q, rr.Package, rr.Version, id); err != nil {
			return nil, err
		}
	}
	pm := m
	if active != table {
		pm = newModel(active, id)
	}
	m.policy = pm
	if acc := rr.Resource.Access; acc != nil {
		m.rules = acc.Read
		m.perms = map[string][]string{}
		for key, perms := range acc.Fields {
			if f, ok := pm.byKey[key]; ok && len(perms) > 0 {
				m.perms[f.Column] = perms
			}
		}
	}
	return m, nil
}

func unsupported(op serrors.Op, format string, args ...any) error {
	return fail(op, ErrUnsupported, format, args...)
}
