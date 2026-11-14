package data

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Store implements spi.DataStore over the tables created by Apply and the
// registered native entities.
type Store struct {
	catalog spi.Catalog
	natives *NativeRegistry
	guard   spi.WriteGuard
	schemas SchemaSource
}

var _ spi.DataStore = (*Store)(nil)

// StoreOption customizes NewStore.
type StoreOption func(*Store)

// WithSchemaSource replaces how package version views are read (default:
// the extruntime.entity_schema_versions table, one query per call). Wrap it in
// *CachedSchemas to memoize.
func WithSchemaSource(src SchemaSource) StoreOption {
	return func(s *Store) { s.schemas = src }
}

// NewStore wires the store. natives may be nil (no native entities); guard may
// be nil (no entity write rules).
func NewStore(catalog spi.Catalog, natives *NativeRegistry, guard spi.WriteGuard, opts ...StoreOption) *Store {
	if natives == nil {
		natives = NewNativeRegistry()
	}
	s := &Store{catalog: catalog, natives: natives, guard: guard, schemas: dbSchemas{}}
	for _, o := range opts {
		o(s)
	}
	return s
}

func visibleTo(m *model, sub spi.Subject) func(*fieldModel) bool {
	return func(f *fieldModel) bool { return !m.hidden(sub, f) }
}

func allFields(*fieldModel) bool { return true }

// Get returns the record, or JSON null when it does not exist or the subject
// may not see it.
func (s *Store) Get(ctx context.Context, tx pgx.Tx, sc spi.Scope, entity, key string) (json.RawMessage, error) {
	const op = serrors.Op("extruntime.data.Store.Get")
	m, err := s.load(ctx, tx, sc, entity)
	if err != nil {
		return nil, err
	}
	canon, err := m.KeyType.Canonical(key)
	if err != nil {
		return nil, fail(op, ErrInvalidValue, "%v", err)
	}
	b := s.newBuilder(ctx, tx, sc, m)
	keyExpr := m.KeyType.bind(canon, b.args)
	where, err := b.where(sc.Subject, nil, rootAlias+"."+quoteIdent(m.KeyCol)+" = "+keyExpr)
	if err != nil {
		return nil, err
	}
	p := m.project(rootAlias, visibleTo(m, sc.Subject))
	rows, err := tx.Query(ctx, "SELECT "+p.selectList()+b.from()+where, b.args.Values()...)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	recs, err := p.collect(rows)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	if len(recs) == 0 {
		return json.RawMessage("null"), nil
	}
	return p.marshal(recs[0]), nil
}

// Find returns a JSON array of visible records.
func (s *Store) Find(ctx context.Context, tx pgx.Tx, sc spi.Scope, entity string, q spi.FindQuery) (json.RawMessage, error) {
	const op = serrors.Op("extruntime.data.Store.Find")
	m, err := s.load(ctx, tx, sc, entity)
	if err != nil {
		return nil, err
	}
	limit, offset, err := clampPaging(q.Limit, q.Offset)
	if err != nil {
		return nil, err
	}
	b := s.newBuilder(ctx, tx, sc, m)
	where, err := b.where(sc.Subject, q.Where)
	if err != nil {
		return nil, err
	}
	order, err := b.orderBy(q.OrderBy)
	if err != nil {
		return nil, err
	}
	p := m.project(rootAlias, visibleTo(m, sc.Subject))
	sql := "SELECT " + p.selectList() + b.from() + where + order +
		" LIMIT " + b.args.Add(limit) + " OFFSET " + b.args.Add(offset)
	rows, err := tx.Query(ctx, sql, b.args.Values()...)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	recs, err := p.collect(rows)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	return marshalList(p, recs), nil
}

// Count counts visible records matching where, ignoring paging.
func (s *Store) Count(ctx context.Context, tx pgx.Tx, sc spi.Scope, entity string, where *manifest.Filter) (int64, error) {
	const op = serrors.Op("extruntime.data.Store.Count")
	m, err := s.load(ctx, tx, sc, entity)
	if err != nil {
		return 0, err
	}
	b := s.newBuilder(ctx, tx, sc, m)
	w, err := b.where(sc.Subject, where)
	if err != nil {
		return 0, err
	}
	var n int64
	if err := tx.QueryRow(ctx, "SELECT count(*)"+b.from()+w, b.args.Values()...).Scan(&n); err != nil {
		return 0, s.dbErr(op, m, err)
	}
	return n, nil
}

func (s *Store) writable(ctx context.Context, tx pgx.Tx, sc spi.Scope, op serrors.Op, entity string, want modelKind) (*model, error) {
	m, err := s.load(ctx, tx, sc, entity)
	if err != nil {
		return nil, err
	}
	switch {
	case m.Kind == kindNative:
		return nil, fail(op, ErrNativeWrite, "%s is native; use a published operation", entity)
	case sc.Caller.Package != "" && m.Owner != sc.Caller.Package:
		return nil, fail(op, ErrForeignWrite, "%s belongs to %s; use its published operations", entity, m.Owner)
	case m.Kind != want && want == kindOwn:
		return nil, unsupported(op, "%s is an overlay; use PutOverlay", entity)
	case m.Kind != want:
		return nil, unsupported(op, "%s is an entity; use Insert/Update", entity)
	}
	return m, nil
}

func parseObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, fmt.Errorf("expected a JSON object")
	}
	return obj, nil
}

// colWrite is one validated field assignment.
type colWrite struct {
	f     *fieldModel
	canon json.RawMessage
}

// parseWrites validates and canonicalizes an object of field values. Unknown
// keys, meta keys and two keys for one column are refused, and so is a field
// the subject may not read: writing it would let the subject overwrite what it
// cannot see, and the "nothing changed" short cut would answer whether a guess
// equals the hidden value.
func parseWrites(op serrors.Op, sub spi.Subject, m *model, obj map[string]json.RawMessage) ([]colWrite, error) {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	seen := map[string]string{}
	out := make([]colWrite, 0, len(keys))
	for _, k := range keys {
		f, ok := m.byKey[k]
		if !ok {
			return nil, fail(op, ErrUnknownField, "%q on %s", k, m.ID)
		}
		if m.hidden(sub, f) {
			return nil, fail(op, ErrFieldNotAccessible, "%q on %s", k, m.ID)
		}
		if prev, dup := seen[f.Column]; dup {
			return nil, fail(op, ErrInvalidValue, "%q and %q address the same field", prev, k)
		}
		seen[f.Column] = k
		raw := obj[k]
		if isNullRaw(raw) {
			if !f.Nullable {
				return nil, fail(op, ErrInvalidValue, "field %q is required and cannot be null", k)
			}
			out = append(out, colWrite{f, json.RawMessage("null")})
			continue
		}
		if err := validateWire(f.Type, raw); err != nil {
			return nil, fail(op, ErrInvalidValue, "field %q: %v", k, err)
		}
		canon, err := canonicalValue(f.Type, raw)
		if err != nil {
			return nil, fail(op, ErrInvalidValue, "field %q: %v", k, err)
		}
		if f.Type.Kind == manifest.KindRef {
			key, err := refKey(raw, f.RefKeyType)
			if err != nil {
				return nil, fail(op, ErrInvalidValue, "field %q: %v", k, err)
			}
			canon = encodeJSON(manifest.RefValue{Entity: f.Type.Entity, Key: key})
		}
		out = append(out, colWrite{f, canon})
	}
	return out, nil
}

func (w colWrite) expr(a *Args) (string, error) {
	if isNullRaw(w.canon) {
		return "NULL", nil
	}
	return bindValue(w.f.Type, w.f.RefKeyType, w.canon, a)
}

// verifyNativeRefs checks that refs to native entities exist in the tenant;
// refs to own entities are enforced by tenant-qualified foreign keys.
func (s *Store) verifyNativeRefs(ctx context.Context, tx pgx.Tx, op serrors.Op, sub spi.Subject, writes []colWrite) error {
	for _, w := range writes {
		if w.f.Type.Kind != manifest.KindRef || isNullRaw(w.canon) {
			continue
		}
		n, ok := s.natives.Get(w.f.Type.Entity)
		if !ok {
			continue
		}
		var r manifest.RefValue
		_ = json.Unmarshal(w.canon, &r)
		a := NewArgs(0)
		sql := fmt.Sprintf("SELECT 1 FROM %s WHERE %s = %s AND %s = %s", qualified(n.schema(), n.Table),
			quoteIdent(n.TenantColumn), a.Add(sub.TenantID), quoteIdent(n.KeyColumn), n.KeyType.bind(r.Key, a))
		rows, err := tx.Query(ctx, sql, a.Values()...)
		if err != nil {
			return serrors.FromDB(op, err)
		}
		found := rows.Next()
		rows.Close()
		if err := rows.Err(); err != nil {
			return serrors.FromDB(op, err)
		}
		if !found {
			return failField(op, ErrReferenceViolation, w.f.Key, "field %q refers to a missing %s", w.f.Key, n.ID)
		}
	}
	return nil
}

// readable reports whether the subject's read rules let it see the row; a
// row it cannot read cannot be updated or deleted either (reported as not
// found, so hidden rows do not leak through write errors).
func (s *Store) readable(ctx context.Context, tx pgx.Tx, sc spi.Scope, m *model, key string) (bool, error) {
	const op = serrors.Op("extruntime.data.Store.readable")
	b := s.newBuilder(ctx, tx, sc, m)
	where, err := b.where(sc.Subject, nil, rootAlias+"."+quoteIdent(m.KeyCol)+" = "+m.KeyType.bind(key, b.args))
	if err != nil {
		return false, err
	}
	rows, err := tx.Query(ctx, "SELECT 1"+b.from()+where, b.args.Values()...)
	if err != nil {
		return false, s.dbErr(op, m, err)
	}
	found := rows.Next()
	rows.Close()
	return found, rows.Err()
}

func (s *Store) lockRow(ctx context.Context, tx pgx.Tx, m *model, sub spi.Subject, key string) (*projection, *record, error) {
	a := NewArgs(0)
	p := m.project(rootAlias, allFields)
	sql := fmt.Sprintf("SELECT %s FROM %s AS %s WHERE %s.%s = %s AND %s.%s = %s FOR UPDATE",
		p.selectList(), m.qualified(), rootAlias,
		rootAlias, quoteIdent(m.TenantCol), a.Add(sub.TenantID),
		rootAlias, quoteIdent(m.KeyCol), m.KeyType.bind(key, a))
	rows, err := tx.Query(ctx, sql, a.Values()...)
	if err != nil {
		return nil, nil, err
	}
	recs, err := p.collect(rows)
	if err != nil {
		return nil, nil, err
	}
	if len(recs) == 0 {
		return p, nil, nil
	}
	return p, recs[0], nil
}

func (s *Store) check(ctx context.Context, tx pgx.Tx, sc spi.Scope, m *model, opName, key string, existing, proposed json.RawMessage, changed []string) error {
	if s.guard == nil {
		return nil
	}
	if changed == nil {
		changed = []string{}
	}
	return s.guard.CheckWrite(ctx, tx, sc, spi.WriteChange{
		Entity: m.ID, Key: key, Op: opName, Existing: existing, Proposed: proposed, Changed: changed,
	})
}

func (s *Store) dbErr(op serrors.Op, m *model, err error) error {
	code, pg := pgCode(err)
	switch code {
	case "23505":
		if f, ok := m.uniques[pg.ConstraintName]; ok {
			return failField(op, ErrUniqueViolation, f, "field %q already holds this value", f)
		}
		return fail(op, ErrUniqueViolation, "a unique field already holds this value")
	case "23503":
		return fail(op, ErrReferenceViolation, "a referenced record is missing or the record is still referenced")
	case "23502", "23514", "22003", "22001", "22P02", "22007", "22008", "22021":
		return fail(op, ErrInvalidValue, "the database rejected a value (%s)", code)
	}
	return serrors.FromDB(op, err)
}

// Insert creates a record. values may carry an explicit "key" (uuid).
func (s *Store) Insert(ctx context.Context, tx pgx.Tx, sc spi.Scope, entity string, values json.RawMessage) (json.RawMessage, error) {
	const op = serrors.Op("extruntime.data.Store.Insert")
	m, err := s.writable(ctx, tx, sc, op, entity, kindOwn)
	if err != nil {
		return nil, err
	}
	obj, err := parseObject(values)
	if err != nil {
		return nil, fail(op, ErrInvalidValue, "%v", err)
	}
	id := uuid.New()
	if raw, ok := obj["key"]; ok {
		var ks string
		if json.Unmarshal(raw, &ks) != nil {
			return nil, fail(op, ErrInvalidValue, "key must be a uuid string")
		}
		if id, err = uuid.Parse(ks); err != nil {
			return nil, fail(op, ErrInvalidValue, "key must be a uuid string")
		}
		delete(obj, "key")
	}
	writes, err := parseWrites(op, sc.Subject, m, obj)
	if err != nil {
		return nil, err
	}
	written := map[string]bool{}
	for _, w := range writes {
		written[w.f.Column] = true
	}
	for _, f := range m.Fields {
		if !written[f.Column] && f.NotNull && f.Default == nil {
			return nil, fail(op, ErrInvalidValue, "field %q is required", f.Key)
		}
	}
	if err := s.verifyNativeRefs(ctx, tx, op, sc.Subject, writes); err != nil {
		return nil, err
	}

	proposed := &record{key: id.String(), vals: map[string]json.RawMessage{}}
	changed := make([]string, 0, len(writes))
	for _, f := range m.Fields {
		if f.Default != nil {
			proposed.vals[f.Column] = f.Default
		}
	}
	for _, w := range writes {
		changed = append(changed, w.f.Key)
		if v, ok := recordValue(w.f, w.canon); ok {
			proposed.vals[w.f.Column] = v
		} else {
			delete(proposed.vals, w.f.Column)
		}
	}
	all := m.project(rootAlias, allFields)
	if err := s.check(ctx, tx, sc, m, "insert", proposed.key, json.RawMessage("null"), all.marshal(proposed), changed); err != nil {
		return nil, err
	}

	a := NewArgs(0)
	cols := []string{"tenant_id", "id"}
	exprs := []string{a.Add(sc.Subject.TenantID), a.Add(id)}
	for _, w := range writes {
		e, err := w.expr(a)
		if err != nil {
			return nil, fail(op, ErrInvalidValue, "field %q: %v", w.f.Key, err)
		}
		cols = append(cols, quoteIdent(w.f.Column))
		exprs = append(exprs, e)
	}
	out := m.project(rootAlias, visibleTo(m, sc.Subject))
	sql := fmt.Sprintf("INSERT INTO %s AS %s (%s) VALUES (%s) RETURNING %s",
		m.qualified(), rootAlias, strings.Join(cols, ", "), strings.Join(exprs, ", "), out.selectList())
	rows, err := tx.Query(ctx, sql, a.Values()...)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	recs, err := out.collect(rows)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	return out.marshal(recs[0]), nil
}

// Update applies a patch. A "rowVersion" in the patch is an expected version:
// a mismatch fails with ErrVersionConflict. A patch that changes nothing
// writes nothing.
func (s *Store) Update(ctx context.Context, tx pgx.Tx, sc spi.Scope, entity, key string, patch json.RawMessage) (json.RawMessage, error) {
	const op = serrors.Op("extruntime.data.Store.Update")
	m, err := s.writable(ctx, tx, sc, op, entity, kindOwn)
	if err != nil {
		return nil, err
	}
	canonKey, err := m.KeyType.Canonical(key)
	if err != nil {
		return nil, fail(op, ErrInvalidValue, "%v", err)
	}
	obj, err := parseObject(patch)
	if err != nil {
		return nil, fail(op, ErrInvalidValue, "%v", err)
	}
	var expected *int64
	if raw, ok := obj["rowVersion"]; ok {
		var n int64
		if json.Unmarshal(raw, &n) != nil {
			return nil, fail(op, ErrInvalidValue, "rowVersion must be an integer")
		}
		expected = &n
		delete(obj, "rowVersion")
	}
	if raw, ok := obj["key"]; ok {
		if k, err := m.KeyType.Canonical(strings.Trim(string(raw), `"`)); err != nil || k != canonKey {
			return nil, fail(op, ErrInvalidValue, "key is immutable")
		}
		delete(obj, "key")
	}
	writes, err := parseWrites(op, sc.Subject, m, obj)
	if err != nil {
		return nil, err
	}
	all, existing, err := s.lockRow(ctx, tx, m, sc.Subject, canonKey)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	if existing != nil {
		if ok, verr := s.readable(ctx, tx, sc, m, canonKey); verr != nil {
			return nil, verr
		} else if !ok {
			existing = nil
		}
	}
	if existing == nil {
		return nil, fail(op, ErrNotFound, "%s %s", entity, canonKey)
	}
	if expected != nil && *expected != existing.rowVersion {
		return nil, fail(op, ErrVersionConflict, "%s %s is at version %d, expected %d", entity, canonKey, existing.rowVersion, *expected)
	}
	visible := m.project(rootAlias, visibleTo(m, sc.Subject))

	changedWrites := make([]colWrite, 0, len(writes))
	changed := make([]string, 0, len(writes))
	proposed := *existing
	proposed.vals = make(map[string]json.RawMessage, len(existing.vals))
	for k, v := range existing.vals {
		proposed.vals[k] = v
	}
	for _, w := range writes {
		if w.canon != nil && string(w.canon) == existing.value(w.f) {
			continue
		}
		if isNullRaw(w.canon) && existing.value(w.f) == "null" {
			continue
		}
		changedWrites = append(changedWrites, w)
		changed = append(changed, w.f.Key)
		if v, ok := recordValue(w.f, w.canon); ok {
			proposed.vals[w.f.Column] = v
		} else {
			delete(proposed.vals, w.f.Column)
		}
	}
	if len(changedWrites) == 0 {
		return visible.marshal(existing), nil
	}
	if err := s.verifyNativeRefs(ctx, tx, op, sc.Subject, changedWrites); err != nil {
		return nil, err
	}
	if err := s.check(ctx, tx, sc, m, "update", canonKey, all.marshal(existing), all.marshal(&proposed), changed); err != nil {
		return nil, err
	}

	a := NewArgs(0)
	sets := make([]string, 0, len(changedWrites)+2)
	for _, w := range changedWrites {
		e, err := w.expr(a)
		if err != nil {
			return nil, fail(op, ErrInvalidValue, "field %q: %v", w.f.Key, err)
		}
		sets = append(sets, quoteIdent(w.f.Column)+" = "+e)
	}
	sets = append(sets, "updated_at = now()", "row_version = "+rootAlias+".row_version + 1")
	sql := fmt.Sprintf("UPDATE %s AS %s SET %s WHERE %s.tenant_id = %s AND %s.id = %s RETURNING %s",
		m.qualified(), rootAlias, strings.Join(sets, ", "),
		rootAlias, a.Add(sc.Subject.TenantID), rootAlias, m.KeyType.bind(canonKey, a), visible.selectList())
	rows, err := tx.Query(ctx, sql, a.Values()...)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	recs, err := visible.collect(rows)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	return visible.marshal(recs[0]), nil
}

// Delete removes a record. Deleting a missing record is ErrNotFound.
func (s *Store) Delete(ctx context.Context, tx pgx.Tx, sc spi.Scope, entity, key string) error {
	const op = serrors.Op("extruntime.data.Store.Delete")
	m, err := s.writable(ctx, tx, sc, op, entity, kindOwn)
	if err != nil {
		return err
	}
	canonKey, err := m.KeyType.Canonical(key)
	if err != nil {
		return fail(op, ErrInvalidValue, "%v", err)
	}
	all, existing, err := s.lockRow(ctx, tx, m, sc.Subject, canonKey)
	if err != nil {
		return s.dbErr(op, m, err)
	}
	if existing != nil {
		if ok, verr := s.readable(ctx, tx, sc, m, canonKey); verr != nil {
			return verr
		} else if !ok {
			existing = nil
		}
	}
	if existing == nil {
		return fail(op, ErrNotFound, "%s %s", entity, canonKey)
	}
	changed := make([]string, len(m.Fields))
	for i, f := range m.Fields {
		changed[i] = f.Key
	}
	if err := s.check(ctx, tx, sc, m, "delete", canonKey, all.marshal(existing), json.RawMessage("null"), changed); err != nil {
		return err
	}
	a := NewArgs(0)
	sql := fmt.Sprintf("DELETE FROM %s WHERE tenant_id = %s AND id = %s", m.qualified(), a.Add(sc.Subject.TenantID), m.KeyType.bind(canonKey, a))
	if _, err := tx.Exec(ctx, sql, a.Values()...); err != nil {
		return s.dbErr(op, m, err)
	}
	return nil
}

// PutOverlay upserts the overlay row of a base record. The base must exist in
// the tenant and, for native bases, the subject must hold the read permission.
func (s *Store) PutOverlay(ctx context.Context, tx pgx.Tx, sc spi.Scope, overlay, baseKey string, values json.RawMessage) (json.RawMessage, error) {
	const op = serrors.Op("extruntime.data.Store.PutOverlay")
	m, err := s.writable(ctx, tx, sc, op, overlay, kindOverlay)
	if err != nil {
		return nil, err
	}
	canonKey, err := m.KeyType.Canonical(baseKey)
	if err != nil {
		return nil, fail(op, ErrInvalidValue, "%v", err)
	}
	obj, err := parseObject(values)
	if err != nil {
		return nil, fail(op, ErrInvalidValue, "%v", err)
	}
	writes, err := parseWrites(op, sc.Subject, m, obj)
	if err != nil {
		return nil, err
	}
	if err := s.lockBase(ctx, tx, op, sc, m, canonKey); err != nil {
		return nil, err
	}
	all, existing, err := s.lockRow(ctx, tx, m, sc.Subject, canonKey)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	if existing == nil {
		written := map[string]bool{}
		for _, w := range writes {
			written[w.f.Column] = true
		}
		for _, f := range m.Fields {
			if !written[f.Column] && f.NotNull && f.Default == nil {
				return nil, fail(op, ErrInvalidValue, "field %q is required", f.Key)
			}
		}
	}
	if err := s.verifyNativeRefs(ctx, tx, op, sc.Subject, writes); err != nil {
		return nil, err
	}

	proposed := &record{key: canonKey, vals: map[string]json.RawMessage{}}
	opName := "insert"
	if existing != nil {
		opName = "update"
		for k, v := range existing.vals {
			proposed.vals[k] = v
		}
	} else {
		for _, f := range m.Fields {
			if f.Default != nil {
				proposed.vals[f.Column] = f.Default
			}
		}
	}
	changed := make([]string, 0, len(writes))
	changedWrites := make([]colWrite, 0, len(writes))
	for _, w := range writes {
		if existing != nil && (string(w.canon) == existing.value(w.f)) {
			continue
		}
		changedWrites = append(changedWrites, w)
		changed = append(changed, w.f.Key)
		if v, ok := recordValue(w.f, w.canon); ok {
			proposed.vals[w.f.Column] = v
		} else {
			delete(proposed.vals, w.f.Column)
		}
	}
	visible := m.project(rootAlias, visibleTo(m, sc.Subject))
	if existing != nil && len(changedWrites) == 0 {
		return visible.marshal(existing), nil
	}
	existingJSON := json.RawMessage("null")
	if existing != nil {
		existingJSON = all.marshal(existing)
	}
	if err := s.check(ctx, tx, sc, m, opName, canonKey, existingJSON, all.marshal(proposed), changed); err != nil {
		return nil, err
	}

	a := NewArgs(0)
	var sql string
	if existing == nil {
		cols := []string{"tenant_id", "base_key"}
		exprs := []string{a.Add(sc.Subject.TenantID), m.KeyType.bind(canonKey, a)}
		for _, w := range changedWrites {
			e, err := w.expr(a)
			if err != nil {
				return nil, fail(op, ErrInvalidValue, "field %q: %v", w.f.Key, err)
			}
			cols = append(cols, quoteIdent(w.f.Column))
			exprs = append(exprs, e)
		}
		sql = fmt.Sprintf("INSERT INTO %s AS %s (%s) VALUES (%s) RETURNING %s",
			m.qualified(), rootAlias, strings.Join(cols, ", "), strings.Join(exprs, ", "), visible.selectList())
	} else {
		sets := make([]string, 0, len(changedWrites))
		for _, w := range changedWrites {
			e, err := w.expr(a)
			if err != nil {
				return nil, fail(op, ErrInvalidValue, "field %q: %v", w.f.Key, err)
			}
			sets = append(sets, quoteIdent(w.f.Column)+" = "+e)
		}
		sql = fmt.Sprintf("UPDATE %s AS %s SET %s WHERE %s.tenant_id = %s AND %s.base_key = %s RETURNING %s",
			m.qualified(), rootAlias, strings.Join(sets, ", "),
			rootAlias, a.Add(sc.Subject.TenantID), rootAlias, m.KeyType.bind(canonKey, a), visible.selectList())
	}
	rows, err := tx.Query(ctx, sql, a.Values()...)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	recs, err := visible.collect(rows)
	if err != nil {
		return nil, s.dbErr(op, m, err)
	}
	return visible.marshal(recs[0]), nil
}

// lockBase verifies and share-locks the base row of an overlay write. The
// subject must be able to read the base row: the read permission of a native
// base, the read rules of an own base entity (an invisible row is reported as
// not found).
func (s *Store) lockBase(ctx context.Context, tx pgx.Tx, op serrors.Op, sc spi.Scope, m *model, key string) error {
	sub := sc.Subject
	b := m.Base
	tenantCol := "tenant_id"
	if b.Native {
		n, ok := s.natives.Get(b.Entity)
		if !ok {
			return fail(op, ErrUnknownEntity, "native base %s has no descriptor", b.Entity)
		}
		if !sub.Has(n.ReadPermission) {
			return fail(op, ErrDenied, "writing an overlay of %s", b.Entity)
		}
		tenantCol = n.TenantColumn
	} else {
		bm, err := s.load(ctx, tx, sc, b.Entity)
		if err != nil {
			return err
		}
		ok, err := s.readable(ctx, tx, sc, bm, key)
		if err != nil {
			return err
		}
		if !ok {
			return fail(op, ErrNotFound, "%s %s", b.Entity, key)
		}
	}
	a := NewArgs(0)
	sql := fmt.Sprintf("SELECT 1 FROM %s WHERE %s = %s AND %s = %s FOR KEY SHARE",
		qualified(b.Schema, b.Table), quoteIdent(tenantCol), a.Add(sub.TenantID), quoteIdent(b.KeyColumn), b.KeyType.bind(key, a))
	rows, err := tx.Query(ctx, sql, a.Values()...)
	if err != nil {
		return serrors.FromDB(op, err)
	}
	found := rows.Next()
	rows.Close()
	if err := rows.Err(); err != nil {
		return serrors.FromDB(op, err)
	}
	if !found {
		return fail(op, ErrNotFound, "%s %s", b.Entity, key)
	}
	return nil
}
