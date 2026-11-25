package engine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// projector removes the Sensitive fields of event payloads a subject may not read: the fields the
// system view of the catalog has and the subject's author view (catalog.AsAuthor) removed. Views are
// read once per projector and cached by the rights snapshot of the subject (its user and permissions),
// so a subject loaded again with other rights gets its own view.
type projector struct {
	r      *Runtime
	tenant uuid.UUID
	system *catalog.View
	views  map[string]*catalog.View
	hidden map[string][]string
	blind  map[string]bool // event|rights the subject may not read at all
}

func (r *Runtime) projector(tenant uuid.UUID) *projector {
	return &projector{r: r, tenant: tenant, views: map[string]*catalog.View{}, hidden: map[string][]string{}, blind: map[string]bool{}}
}

// rightsKey identifies the rights snapshot of a subject; nil is a subject without permissions.
func rightsKey(as *spi.Subject) string {
	if as == nil {
		return "-"
	}
	perms := append([]string(nil), as.Permissions...)
	sort.Strings(perms)
	sum := sha256.Sum256([]byte(strings.Join(perms, "\x00")))
	return fmt.Sprintf("%d|%x", as.UserID, sum[:8])
}

// hiddenPaths are the dotted payload paths of event that as may not read ("*" stands for every entry of
// a map). Without a subject (nil) every sensitive field is hidden.
func (p *projector) hiddenPaths(ctx context.Context, event string, as *spi.Subject) ([]string, error) {
	rights := rightsKey(as)
	key := event + "|" + rights
	if h, ok := p.hidden[key]; ok {
		return h, nil
	}
	if p.system == nil {
		v, err := p.r.cfg.Catalog.ForTenant(ctx, p.tenant)
		if err != nil {
			return nil, err
		}
		p.system = &v
	}
	view, ok := p.views[rights]
	if !ok {
		author := spi.Subject{TenantID: p.tenant}
		if as != nil {
			author = *as
		}
		v, err := p.r.cfg.Catalog.ForTenant(ctx, p.tenant, catalog.AsAuthor(author))
		if err != nil {
			return nil, err
		}
		view = &v
		p.views[rights] = view
	}
	full, ok := p.system.Catalog.Events[event]
	if !ok {
		p.hidden[key] = nil
		return nil, nil
	}
	seen, ok := view.Catalog.Events[event]
	var hidden []string
	if !ok {
		// The subject may not read the event at all: nothing of its payload is kept (project drops
		// undeclared keys too).
		p.blind[key] = true
		for _, f := range full.Payload {
			hidden = append(hidden, f.Name)
		}
	} else {
		hidden = diffPaths(full.Payload, seen.Payload, "")
	}
	p.hidden[key] = hidden
	return hidden, nil
}

// project removes the hidden fields from a payload in place; for an event the subject may not read at
// all it keeps nothing, undeclared keys included.
func (p *projector) project(ctx context.Context, event string, payload map[string]any, as *spi.Subject) (map[string]any, error) {
	hidden, err := p.hiddenPaths(ctx, event, as)
	if err != nil {
		return payload, err
	}
	if p.blind[event+"|"+rightsKey(as)] {
		return map[string]any{}, nil
	}
	for _, path := range hidden {
		removePath(payload, strings.Split(path, "."))
	}
	return payload, nil
}

// diffPaths are the fields of full that seen lacks, as dotted paths: a list is transparent, every entry
// of a map is "*".
func diffPaths(full, seen expr.Fields, prefix string) []string {
	var out []string
	for _, f := range full {
		s, ok := seen.Get(f.Name)
		if !ok {
			out = append(out, prefix+f.Name)
			continue
		}
		out = append(out, diffField(f.Field, s, prefix+f.Name+".")...)
	}
	return out
}

func diffField(full, seen expr.Field, prefix string) []string {
	f, s := full.Unwrap(), seen.Unwrap()
	switch {
	case f.Kind == expr.List && f.Elem != nil && s.Kind == expr.List && s.Elem != nil:
		return diffField(*f.Elem, *s.Elem, prefix)
	case f.Kind == expr.Map && f.Elem != nil && s.Kind == expr.Map && s.Elem != nil:
		return diffField(*f.Elem, *s.Elem, prefix+"*.")
	case f.Kind == expr.Object && s.Kind == expr.Object:
		return diffPaths(f.Props, s.Props, prefix)
	}
	return nil
}

// removePath deletes a dotted path in a decoded value: lists are walked through, "*" visits every entry
// of a map.
func removePath(v any, path []string) {
	if len(path) == 0 {
		return
	}
	switch x := v.(type) {
	case map[string]any:
		if path[0] == "*" {
			for _, e := range x {
				removePath(e, path[1:])
			}
			return
		}
		if len(path) == 1 {
			delete(x, path[0])
			return
		}
		removePath(x[path[0]], path[1:])
	case []any:
		for _, e := range x {
			removePath(e, path)
		}
	}
}

// Hidden replaces what a viewer may not read in run history.
const Hidden = "[hidden]"

// sensitiveRef is a sensitive field of an event that a run read: its data, and everything a step derived
// after it (rendered params, outputs, errors), is shown only to a viewer who may read the field.
type sensitiveRef struct {
	Event string `json:"event"`
	Path  string `json:"path"`
}

// sensitiveIn are the sensitive fields an event's payload carries: those a subject without permissions
// may not read (all of them when reading the event itself needs a permission) that have a value.
func (p *projector) sensitiveIn(ctx context.Context, event string, payload map[string]any) ([]sensitiveRef, error) {
	hidden, err := p.hiddenPaths(ctx, event, nil)
	if err != nil {
		return nil, err
	}
	var out []sensitiveRef
	for _, path := range hidden {
		present := false
		// An empty value (null, "") carries nothing to protect.
		valuesAt(payload, strings.Split(path, "."), func(v any) { present = present || (v != nil && v != "") })
		if present {
			out = append(out, sensitiveRef{Event: event, Path: path})
		}
	}
	return out, nil
}

// known reports whether the tenant's system catalog has the event.
func (p *projector) known(ctx context.Context, event string) (bool, error) {
	if p.system == nil {
		v, err := p.r.cfg.Catalog.ForTenant(ctx, p.tenant)
		if err != nil {
			return false, err
		}
		p.system = &v
	}
	_, ok := p.system.Catalog.Events[event]
	return ok, nil
}

// resolvable reports whether the catalog still classifies a stored reference: its event is known and
// its path is (under) a sensitive path of the event now. A field the event lost, or one no longer
// marked sensitive, has no access rule left to check it by, so it is not treated as readable.
func (p *projector) resolvable(ctx context.Context, ref sensitiveRef) (bool, error) {
	ok, err := p.known(ctx, ref.Event)
	if err != nil || !ok {
		return false, err
	}
	// The path itself must still exist in the event's schema: a protected parent does not vouch for a
	// child the schema no longer has (secret.pin when secret became a scalar), whose own rule is gone.
	if !fieldsHavePath(p.system.Catalog.Events[ref.Event].Payload, strings.Split(ref.Path, ".")) {
		return false, nil
	}
	sensitive, err := p.hiddenPaths(ctx, ref.Event, nil)
	if err != nil {
		return false, err
	}
	for _, h := range sensitive {
		if h == ref.Path || strings.HasPrefix(ref.Path, h+".") {
			return true, nil
		}
	}
	return false, nil
}

// fieldsHavePath reports whether a dotted path (lists transparent, "*" for a map's entries) names a field
// of the schema.
func fieldsHavePath(fs expr.Fields, path []string) bool {
	if len(path) == 0 {
		return true
	}
	f, ok := fs.Get(path[0])
	if !ok {
		return false
	}
	return fieldHasPath(f, path[1:])
}

func fieldHasPath(f expr.Field, path []string) bool {
	if len(path) == 0 {
		return true
	}
	f = f.Unwrap()
	switch {
	case f.Kind == expr.List && f.Elem != nil:
		return fieldHasPath(*f.Elem, path)
	case f.Kind == expr.Map && f.Elem != nil:
		return path[0] == "*" && fieldHasPath(*f.Elem, path[1:])
	case f.Kind == expr.Object:
		return fieldsHavePath(f.Props, path)
	}
	return false
}

// readable reports whether the viewer may read every sensitive field of refs. It fails closed: a
// reference to an event the tenant's catalog no longer has (its connector left) cannot be checked, so
// what derives from it is not shown.
func (p *projector) readable(ctx context.Context, refs []sensitiveRef, viewer *spi.Subject) (bool, error) {
	for _, ref := range refs {
		ok, err := p.resolvable(ctx, ref)
		if err != nil || !ok {
			return false, err
		}
		hidden, err := p.hiddenPaths(ctx, ref.Event, viewer)
		if err != nil {
			return false, err
		}
		for _, h := range hidden {
			// The field itself, a field around it, or one inside it the viewer may not read.
			if h == ref.Path || strings.HasPrefix(ref.Path, h+".") || strings.HasPrefix(h, ref.Path+".") {
				return false, nil
			}
		}
	}
	return true, nil
}

// mergeRefs adds refs not yet in base; changed reports whether any was added.
func mergeRefs(base, more []sensitiveRef) ([]sensitiveRef, bool) {
	changed := false
	for _, m := range more {
		found := false
		for _, b := range base {
			if b == m {
				found = true
				break
			}
		}
		if !found {
			base = append(base, m)
			changed = true
		}
	}
	return base, changed
}

func decodeRefs(raw []byte) []sensitiveRef {
	var refs []sensitiveRef
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &refs)
	}
	return refs
}

func valuesAt(v any, path []string, add func(any)) {
	if len(path) == 0 {
		add(v)
		return
	}
	switch x := v.(type) {
	case map[string]any:
		if path[0] == "*" {
			for _, e := range x {
				valuesAt(e, path[1:], add)
			}
			return
		}
		if next, ok := x[path[0]]; ok {
			valuesAt(next, path[1:], add)
		}
	case []any:
		for _, e := range x {
			valuesAt(e, path, add)
		}
	}
}

// historicEvent is a run's stored event ({kind, ...payload}) as a viewer may see it, failing closed:
//   - an event the tenant's catalog no longer has, or one the viewer may not read at all, gives nothing;
//   - only the fields the viewer's view of the event declares are kept (a key the schema lost, or never
//     had, is dropped: no rule is left to check it by);
//   - a sensitive field the run recorded is dropped unless its rule still exists and the viewer passes it.
func (p *projector) historicEvent(ctx context.Context, input json.RawMessage, refs []sensitiveRef, viewer *spi.Subject) (json.RawMessage, error) {
	ev, err := decodeMap(input)
	if err != nil || ev == nil {
		return nil, nil //nolint:nilerr // an input that is not an event object has no event to show
	}
	kind, _ := ev["kind"].(string)
	if kind == "" {
		return nil, nil
	}
	known, err := p.known(ctx, kind)
	if err != nil || !known {
		return nil, err
	}
	if _, err := p.hiddenPaths(ctx, kind, viewer); err != nil {
		return nil, err
	}
	if p.blind[kind+"|"+rightsKey(viewer)] {
		return nil, nil
	}
	seen, ok := p.views[rightsKey(viewer)].Catalog.Events[kind]
	if !ok {
		return nil, nil
	}
	payload := map[string]any{}
	for k, v := range ev {
		if k != "kind" {
			payload[k] = v
		}
	}
	payload = keepDeclared(payload, seen.Payload)
	for _, ref := range refs {
		if ref.Event != kind {
			continue
		}
		ok, err := p.readable(ctx, []sensitiveRef{ref}, viewer)
		if err != nil {
			return nil, err
		}
		if !ok {
			removePath(payload, strings.Split(ref.Path, "."))
		}
	}
	payload["kind"] = kind
	return json.Marshal(payload)
}

// keepDeclared is the part of a decoded object the schema declares, recursively; an object keeps no key
// it does not declare (even an open one: what it would keep is unchecked).
func keepDeclared(v map[string]any, fs expr.Fields) map[string]any {
	out := make(map[string]any, len(v))
	for k, x := range v {
		f, ok := fs.Get(k)
		if !ok {
			continue
		}
		out[k] = keepField(x, f)
	}
	return out
}

// keepField is a value as its field declares it. A value stored under another type (the field's type
// changed since) is dropped when it would carry data the declaration does not check: an object or a
// list where the field is a scalar, a scalar where it is a container. Only dyn keeps any value.
func keepField(x any, f expr.Field) any {
	f = f.Unwrap()
	switch f.Kind {
	case expr.Dyn:
		return x
	case expr.Object:
		if m, ok := x.(map[string]any); ok {
			return keepDeclared(m, f.Props)
		}
		return nil
	case expr.List:
		l, ok := x.([]any)
		if !ok || f.Elem == nil {
			return nil
		}
		out := make([]any, len(l))
		for i, e := range l {
			out[i] = keepField(e, *f.Elem)
		}
		return out
	case expr.Map:
		m, ok := x.(map[string]any)
		if !ok || f.Elem == nil {
			return nil
		}
		out := make(map[string]any, len(m))
		for k, e := range m {
			out[k] = keepField(e, *f.Elem)
		}
		return out
	case expr.String, expr.Int, expr.Double, expr.Bool, expr.Enum, expr.Optional, expr.Timestamp:
	}
	switch x.(type) {
	case map[string]any, []any:
		return nil
	}
	return x
}
