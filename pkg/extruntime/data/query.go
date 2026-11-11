package data

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	maxPathDepth  = 4
	maxInList     = 1000
	defaultLimit  = 100
	maxLimit      = 1000
	rootAlias     = "t0"
	opQuery       = serrors.Op("extruntime.data.query")
	userPlacehold = "user"
)

type pathMode int

const (
	// modeUser: filters and orders from callers. Hidden fields are rejected
	// and joined entities are visible only through their own read access.
	modeUser pathMode = iota
	// modeTrusted: access rules authored by the owning package.
	modeTrusted
)

type joinInfo struct {
	alias string
	model *model
}

// builder compiles one statement: filter, order and access predicates over a
// root model with explicit LEFT JOINs through ref fields.
type builder struct {
	s     *Store
	ctx   context.Context
	q     Querier
	sc    spi.Scope
	args  *Args
	root  *model
	joins []string
	index map[string]*joinInfo
	// prefix and seq name the join aliases ("j1", "j2", ...). A nested
	// builder (a subquery of its own) uses a different prefix.
	prefix string
	seq    int
}

func (s *Store) newBuilder(ctx context.Context, q Querier, sc spi.Scope, root *model) *builder {
	return &builder{s: s, ctx: ctx, q: q, sc: sc, args: NewArgs(0), root: root, index: map[string]*joinInfo{}, prefix: "j"}
}

// nextAlias hands out a join alias that no other join of the statement uses,
// including the joins that the access predicate of an earlier join adds.
func (b *builder) nextAlias() string {
	b.seq++
	prefix := b.prefix
	if prefix == "" {
		prefix = "j"
	}
	return prefix + strconv.Itoa(b.seq)
}

// column resolves a (possibly dotted) key against base, whose table is aliased
// baseAlias, adding the joins a ref path needs.
func (b *builder) column(base *model, baseAlias, path string, mode pathMode, depth int) (string, *fieldModel, *model, error) {
	segs := strings.Split(path, ".")
	if len(segs) > maxPathDepth {
		return "", nil, nil, fail(opQuery, ErrInvalidQuery, "path %q is deeper than %d", path, maxPathDepth)
	}
	cur, alias := base, baseAlias
	prefix := baseAlias
	for _, seg := range segs[:len(segs)-1] {
		f, err := b.field(cur, seg, mode)
		if err != nil {
			return "", nil, nil, err
		}
		if f.Type.Kind != manifest.KindRef {
			return "", nil, nil, fail(opQuery, ErrInvalidQuery, "%q is not a ref, cannot continue path %q", seg, path)
		}
		prefix += "." + seg
		ji, ok := b.index[prefix+"|"+strconv.Itoa(int(mode))]
		if !ok {
			target, err := b.s.load(b.ctx, b.q, b.sc, f.Type.Entity)
			if err != nil {
				return "", nil, nil, err
			}
			ji = &joinInfo{alias: b.nextAlias(), model: target}
			on := fmt.Sprintf("%s.%s = %s.%s AND %s.%s = %s",
				ji.alias, quoteIdent(target.TenantCol), alias, quoteIdent(cur.TenantCol),
				ji.alias, quoteIdent(target.KeyCol), alias+"."+quoteIdent(f.Column))
			src := target.qualified() + " AS " + ji.alias
			if mode == modeUser {
				// The read predicate of the joined entity may walk ref paths
				// of its own. Those joins hang off the joined table, so they
				// are grouped with it: the ON clause of this join may use
				// them, and they cannot clash with the outer joins.
				outer := b.joins
				b.joins = nil
				pred, err := b.access(target, ji.alias)
				nested := b.joins
				b.joins = outer
				if err != nil {
					return "", nil, nil, err
				}
				on += " AND " + pred
				if len(nested) > 0 {
					src = "(" + src + " " + strings.Join(nested, " ") + ")"
				}
			}
			b.joins = append(b.joins, fmt.Sprintf("LEFT JOIN %s ON %s", src, on))
			b.index[prefix+"|"+strconv.Itoa(int(mode))] = ji
		}
		cur, alias = ji.model, ji.alias
	}
	f, err := b.field(cur, segs[len(segs)-1], mode)
	if err != nil {
		return "", nil, nil, err
	}
	return alias + "." + quoteIdent(f.Column), f, cur, nil
}

func (b *builder) field(m *model, key string, mode pathMode) (*fieldModel, error) {
	f, ok := m.lookup(key)
	if !ok {
		return nil, fail(opQuery, ErrUnknownField, "%q on %s", key, m.ID)
	}
	if mode == modeUser && !f.Meta && m.hidden(b.sc.Subject, f) {
		return nil, fail(opQuery, ErrFieldNotAccessible, "%q on %s", key, m.ID)
	}
	return f, nil
}

// access returns the row-visibility predicate of m (aliased alias) for the
// subject: native read permission, or the OR of the applicable read rules.
// Rules are compiled before any LIMIT/OFFSET/COUNT sees the rows.
func (b *builder) access(m *model, alias string) (string, error) {
	sub := b.sc.Subject
	if m.Kind == kindNative {
		if !sub.Has(m.readPerm) {
			return "", fail(opQuery, ErrDenied, "reading %s", m.ID)
		}
		return "TRUE", nil
	}
	rules, err := b.ruleAccess(m, alias)
	if err != nil || m.Kind != kindOverlay {
		return rules, err
	}
	// An overlay row is only visible together with its base row: whoever
	// cannot read the base cannot read what is attached to it.
	base, err := b.baseAccess(m, alias)
	if err != nil {
		return "", err
	}
	switch {
	case rules == "FALSE" || base == "FALSE":
		return "FALSE", nil
	case base == "TRUE":
		return rules, nil
	case rules == "TRUE":
		return base, nil
	}
	return "(" + rules + " AND " + base + ")", nil
}

// baseAccess is the predicate that makes an overlay row (aliased alias) as
// visible as its base: the read permission of a native base, the read rules of
// an own base entity.
func (b *builder) baseAccess(m *model, alias string) (string, error) {
	sub := b.sc.Subject
	if m.Base.Native {
		n, ok := b.s.natives.Get(m.Base.Entity)
		if !ok {
			return "", fail(opQuery, ErrUnknownEntity, "native base %s has no descriptor", m.Base.Entity)
		}
		if !sub.Has(n.ReadPermission) {
			return "FALSE", nil
		}
		return "TRUE", nil
	}
	base, err := b.s.load(b.ctx, b.q, b.sc, m.Base.Entity)
	if err != nil {
		return "", err
	}
	nb := b.s.newBuilder(b.ctx, b.q, b.sc, base)
	nb.args = b.args
	nb.prefix = b.prefix + "b"
	const baseAlias = "bt"
	pred, err := nb.access(base, baseAlias)
	if err != nil {
		return "", err
	}
	if pred == "TRUE" || pred == "FALSE" {
		return pred, nil
	}
	return fmt.Sprintf("EXISTS (SELECT 1 FROM %s AS %s%s WHERE %s.%s = %s.%s AND %s.%s = %s.%s AND %s)",
		base.qualified(), baseAlias, joinSQL(nb.joins),
		baseAlias, quoteIdent(base.TenantCol), alias, quoteIdent(m.TenantCol),
		baseAlias, quoteIdent(base.KeyCol), alias, quoteIdent(m.KeyCol), pred), nil
}

// ruleAccess is the visibility of a package entity or overlay by its own
// read rules.
func (b *builder) ruleAccess(m *model, alias string) (string, error) {
	sub := b.sc.Subject
	if len(m.rules) == 0 {
		return "TRUE", nil
	}
	var preds []string
	for _, r := range m.rules {
		if !sub.Has(r.Permission) {
			continue
		}
		if r.All {
			return "TRUE", nil
		}
		if r.Rows == nil {
			continue
		}
		p, err := b.filter(m.policy, alias, r.Rows, modeTrusted)
		if err != nil {
			return "", err
		}
		preds = append(preds, p)
	}
	if len(preds) == 0 {
		return "FALSE", nil
	}
	return "(" + strings.Join(preds, " OR ") + ")", nil
}

func (b *builder) filter(base *model, alias string, f *manifest.Filter, mode pathMode) (string, error) {
	if f == nil {
		return "TRUE", nil
	}
	var parts []string
	keys := make([]string, 0, len(f.Fields))
	for k := range f.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		expr, fm, _, err := b.column(base, alias, k, mode, 0)
		if err != nil {
			return "", err
		}
		sql, err := b.condition(expr, fm, f.Fields[k], k)
		if err != nil {
			return "", err
		}
		parts = append(parts, sql)
	}
	if len(f.Or) > 0 {
		alts := make([]string, 0, len(f.Or))
		for i := range f.Or {
			sub, err := b.filter(base, alias, &f.Or[i], mode)
			if err != nil {
				return "", err
			}
			alts = append(alts, sub)
		}
		parts = append(parts, "("+strings.Join(alts, " OR ")+")")
	}
	if len(parts) == 0 {
		return "TRUE", nil
	}
	return "(" + strings.Join(parts, " AND ") + ")", nil
}

func orderable(k manifest.TypeKind) bool {
	switch k {
	case manifest.KindString, manifest.KindInt, manifest.KindInt64, manifest.KindDecimal,
		manifest.KindDate, manifest.KindDateTime, manifest.KindUUID, manifest.KindEnum:
		return true
	}
	return false
}

// resolveValue substitutes the {"$":"user"} placeholder.
func (b *builder) resolveValue(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || raw[0] != '{' {
		return raw, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || len(m) != 1 {
		return raw, nil
	}
	name, ok := m["$"]
	if !ok {
		return raw, nil
	}
	var n string
	if err := json.Unmarshal(name, &n); err != nil || n != userPlacehold {
		return nil, fail(opQuery, ErrInvalidQuery, "unknown placeholder %s", string(name))
	}
	return encodeJSON(manifest.RefValue{Entity: userRefEntity, Key: strconv.FormatInt(b.sc.Subject.UserID, 10)}), nil
}

func (b *builder) value(f *fieldModel, raw json.RawMessage, field string) (string, error) {
	v, err := b.resolveValue(raw)
	if err != nil {
		return "", err
	}
	expr, err := bindValue(f.Type, f.RefKeyType, v, b.args)
	if err != nil {
		return "", fail(opQuery, ErrInvalidQuery, "value for %q: %v", field, err)
	}
	return expr, nil
}

func (b *builder) condition(col string, f *fieldModel, c manifest.Condition, field string) (string, error) {
	var parts []string
	set := func(raw json.RawMessage) bool { return len(raw) > 0 }
	cmp := func(op string, raw json.RawMessage) error {
		if !orderable(f.Type.Kind) {
			return fail(opQuery, ErrInvalidQuery, "%q (%s) cannot be compared with %s", field, f.Type.Kind, op)
		}
		e, err := b.value(f, raw, field)
		if err != nil {
			return err
		}
		parts = append(parts, fmt.Sprintf("%s %s %s", col, op, e))
		return nil
	}
	if set(c.Eq) {
		if isNullRaw(c.Eq) {
			parts = append(parts, col+" IS NULL")
		} else {
			e, err := b.value(f, c.Eq, field)
			if err != nil {
				return "", err
			}
			parts = append(parts, fmt.Sprintf("%s = %s", col, e))
		}
	}
	if set(c.Ne) {
		if isNullRaw(c.Ne) {
			parts = append(parts, col+" IS NOT NULL")
		} else {
			e, err := b.value(f, c.Ne, field)
			if err != nil {
				return "", err
			}
			parts = append(parts, fmt.Sprintf("%s IS DISTINCT FROM %s", col, e))
		}
	}
	if c.In != nil {
		if len(c.In) > maxInList {
			return "", fail(opQuery, ErrInvalidQuery, "%q: in-list longer than %d", field, maxInList)
		}
		if len(c.In) == 0 {
			parts = append(parts, "FALSE")
		} else {
			es := make([]string, len(c.In))
			for i, raw := range c.In {
				e, err := b.value(f, raw, field)
				if err != nil {
					return "", err
				}
				es[i] = e
			}
			parts = append(parts, fmt.Sprintf("%s IN (%s)", col, strings.Join(es, ", ")))
		}
	}
	for _, o := range []struct {
		op  string
		raw json.RawMessage
	}{{"<", c.Lt}, {"<=", c.Lte}, {">", c.Gt}, {">=", c.Gte}} {
		if set(o.raw) {
			if err := cmp(o.op, o.raw); err != nil {
				return "", err
			}
		}
	}
	if c.IsNull != nil {
		if *c.IsNull {
			parts = append(parts, col+" IS NULL")
		} else {
			parts = append(parts, col+" IS NOT NULL")
		}
	}
	if c.Like != nil {
		if f.Type.Kind != manifest.KindString && f.Type.Kind != manifest.KindEnum {
			return "", fail(opQuery, ErrInvalidQuery, "%q (%s) does not support like", field, f.Type.Kind)
		}
		parts = append(parts, fmt.Sprintf("%s LIKE %s::text", col, b.args.Add(*c.Like)))
	}
	if len(parts) == 0 {
		return "", fail(opQuery, ErrInvalidQuery, "empty condition for %q", field)
	}
	return "(" + strings.Join(parts, " AND ") + ")", nil
}

func (b *builder) orderBy(orders []spi.Order) (string, error) {
	var parts []string
	for _, o := range orders {
		expr, f, _, err := b.column(b.root, rootAlias, o.Field, modeUser, 0)
		if err != nil {
			return "", err
		}
		if !orderable(f.Type.Kind) && f.Type.Kind != manifest.KindRef && f.Type.Kind != manifest.KindBool {
			return "", fail(opQuery, ErrInvalidQuery, "%q (%s) cannot be ordered", o.Field, f.Type.Kind)
		}
		dir := " ASC"
		if o.Desc {
			dir = " DESC"
		}
		parts = append(parts, expr+dir)
	}
	parts = append(parts, rootAlias+"."+quoteIdent(b.root.KeyCol)+" ASC")
	return " ORDER BY " + strings.Join(parts, ", "), nil
}

// where builds tenant scope, access predicate and the caller's filter. The
// access predicate is always part of the WHERE, so paging and counts only
// ever see visible rows.
func (b *builder) where(sub spi.Subject, f *manifest.Filter, extra ...string) (string, error) {
	tenant := b.args.Add(sub.TenantID)
	parts := []string{fmt.Sprintf("%s.%s = %s", rootAlias, quoteIdent(b.root.TenantCol), tenant)}
	acc, err := b.access(b.root, rootAlias)
	if err != nil {
		return "", err
	}
	if acc != "TRUE" {
		parts = append(parts, acc)
	}
	user, err := b.filter(b.root, rootAlias, f, modeUser)
	if err != nil {
		return "", err
	}
	if user != "TRUE" {
		parts = append(parts, user)
	}
	parts = append(parts, extra...)
	return " WHERE " + strings.Join(parts, " AND "), nil
}

func (b *builder) from() string {
	s := " FROM " + b.root.qualified() + " AS " + rootAlias
	for _, j := range b.joins {
		s += " " + j
	}
	return s
}

func clampPaging(limit, offset int) (int, int, error) {
	if limit < 0 || offset < 0 {
		return 0, 0, fail(opQuery, ErrInvalidQuery, "limit and offset must not be negative")
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return limit, offset, nil
}
