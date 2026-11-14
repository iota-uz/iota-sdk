package data

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var aliasRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,30}$`)

// OverlayFieldRef names one overlay field, as a package's column contract
// does.
type OverlayFieldRef struct {
	Overlay string
	Field   string
}

type overlayCol struct {
	expr  string
	field *fieldModel
}

// OverlayJoin lets a native list (or export) read overlay fields as extra
// columns: a SQL join fragment plus select, filter and order expressions. An
// overlay field reads as its declared default, or NULL, for a base record
// without an overlay row, and for rows the subject's read rules hide. Fields
// the subject may not read are left out (see Hidden) and cannot be used.
type OverlayJoin struct {
	// SQL holds one LEFT JOIN per overlay; append it to the list's FROM
	// clause, after the base table aliased as passed to NativeOverlayJoin.
	SQL string
	// Hidden lists the requested fields removed by field access.
	Hidden []OverlayFieldRef
	cols   map[OverlayFieldRef]overlayCol
}

// NativeOverlayJoin builds the join for the requested overlay fields over the
// native entity nativeID, whose table is aliased baseAlias in the caller's
// query. Placeholders are numbered through args, shared with the caller's
// statement. Active package versions define the field keys.
func (s *Store) NativeOverlayJoin(ctx context.Context, q Querier, sub spi.Subject, nativeID, baseAlias string, refs []OverlayFieldRef, args *Args) (*OverlayJoin, error) {
	const op = serrors.Op("extruntime.data.Store.NativeOverlayJoin")
	if !aliasRe.MatchString(baseAlias) {
		return nil, fail(op, ErrInvalidQuery, "invalid base alias %q", baseAlias)
	}
	n, ok := s.natives.Get(nativeID)
	if !ok {
		return nil, fail(op, ErrUnknownEntity, "%s is not a native entity", nativeID)
	}
	sc := spi.Scope{Subject: sub}
	out := &OverlayJoin{cols: map[OverlayFieldRef]overlayCol{}}
	models := map[string]*model{}
	aliases := map[string]string{}
	var order []string
	var joins []string
	for _, ref := range refs {
		m := models[ref.Overlay]
		if m == nil {
			var err error
			if m, err = s.load(ctx, q, sc, ref.Overlay); err != nil {
				return nil, err
			}
			if m.Kind != kindOverlay || m.Base.Entity != nativeID {
				return nil, fail(op, ErrInvalidQuery, "%s is not an overlay of %s", ref.Overlay, nativeID)
			}
			models[ref.Overlay] = m
			alias := "ov" + strconv.Itoa(len(order)+1)
			aliases[ref.Overlay] = alias
			order = append(order, ref.Overlay)

			b := s.newBuilder(ctx, q, sc, m)
			b.args = args
			pred, err := b.access(m, rootAlias)
			if err != nil {
				return nil, err
			}
			src := m.qualified()
			if pred != "TRUE" {
				src = fmt.Sprintf("(SELECT %s.* FROM %s AS %s%s WHERE %s)",
					rootAlias, m.qualified(), rootAlias, joinSQL(b.joins), pred)
			}
			joins = append(joins, fmt.Sprintf("LEFT JOIN %s AS %s ON %s.tenant_id = %s.%s AND %s.base_key = %s.%s",
				src, alias, alias, baseAlias, quoteIdent(n.TenantColumn), alias, baseAlias, quoteIdent(n.KeyColumn)))
		}
		f, ok := m.byKey[ref.Field]
		if !ok {
			return nil, fail(op, ErrUnknownField, "%q on %s", ref.Field, ref.Overlay)
		}
		if m.hidden(sub, f) {
			out.Hidden = append(out.Hidden, ref)
			continue
		}
		alias := aliases[ref.Overlay]
		col := alias + "." + quoteIdent(f.Column)
		expr := col
		if f.Default != nil {
			lit, err := defaultLiteral(f.Type, f.Default)
			if err != nil {
				return nil, serrors.Wrap(op, err)
			}
			expr = fmt.Sprintf("CASE WHEN %s.base_key IS NULL THEN %s ELSE %s END", alias, lit, col)
		}
		out.cols[ref] = overlayCol{expr: expr, field: f}
	}
	out.SQL = strings.Join(joins, " ")
	return out, nil
}

func joinSQL(joins []string) string {
	if len(joins) == 0 {
		return ""
	}
	return " " + strings.Join(joins, " ")
}

// Visible reports whether ref can be selected, filtered and ordered.
func (j *OverlayJoin) Visible(ref OverlayFieldRef) bool {
	_, ok := j.cols[ref]
	return ok
}

// Type is the declared wire type of an overlay field.
func (j *OverlayJoin) Type(ref OverlayFieldRef) (manifest.Type, bool) {
	c, ok := j.cols[ref]
	if !ok {
		return manifest.Type{}, false
	}
	return c.field.Type, true
}

func (j *OverlayJoin) col(ref OverlayFieldRef) (overlayCol, error) {
	c, ok := j.cols[ref]
	if !ok {
		return overlayCol{}, fail("extruntime.data.OverlayJoin", ErrFieldNotAccessible, "%s.%s", ref.Overlay, ref.Field)
	}
	return c, nil
}

// Select is the SQL expression that reads the field, default-read applied.
func (j *OverlayJoin) Select(ref OverlayFieldRef) (string, error) {
	c, err := j.col(ref)
	return c.expr, err
}

// Filter compiles cond over the field. The {"$":"user"} placeholder resolves
// to sub.
func (j *OverlayJoin) Filter(ref OverlayFieldRef, cond manifest.Condition, sub spi.Subject, args *Args) (string, error) {
	c, err := j.col(ref)
	if err != nil {
		return "", err
	}
	b := &builder{args: args, sc: spi.Scope{Subject: sub}}
	return b.condition(c.expr, c.field, cond, ref.Field)
}

// OrderBy renders "<expr> ASC|DESC" for an ORDER BY clause.
func (j *OverlayJoin) OrderBy(ref OverlayFieldRef, desc bool) (string, error) {
	c, err := j.col(ref)
	if err != nil {
		return "", err
	}
	k := c.field.Type.Kind
	if !orderable(k) && k != manifest.KindBool {
		return "", fail("extruntime.data.OverlayJoin", ErrInvalidQuery, "%s.%s (%s) cannot be ordered", ref.Overlay, ref.Field, k)
	}
	if desc {
		return c.expr + " DESC", nil
	}
	return c.expr + " ASC", nil
}
