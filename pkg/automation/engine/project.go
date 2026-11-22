package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// projector removes the Sensitive fields of event payloads a subject may not read: the fields the
// system view of the catalog has and the subject's author view (catalog.AsAuthor) removed. Views are
// read once per projector.
type projector struct {
	r      *Runtime
	tenant uuid.UUID
	system *catalog.View
	views  map[int64]*catalog.View
	hidden map[string][]string
}

func (r *Runtime) projector(tenant uuid.UUID) *projector {
	return &projector{r: r, tenant: tenant, views: map[int64]*catalog.View{}, hidden: map[string][]string{}}
}

// hiddenPaths are the dotted payload paths of event that as may not read. Without a subject (nil) every
// sensitive field is hidden.
func (p *projector) hiddenPaths(ctx context.Context, event string, as *spi.Subject) ([]string, error) {
	var user int64 = -1
	if as != nil {
		user = as.UserID
	}
	key := fmt.Sprintf("%s|%d", event, user)
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
	view, ok := p.views[user]
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
		p.views[user] = view
	}
	full, ok := p.system.Catalog.Events[event]
	if !ok {
		p.hidden[key] = nil
		return nil, nil
	}
	seen, ok := view.Catalog.Events[event]
	var hidden []string
	if !ok {
		// The subject may not read the event at all: nothing of its payload is kept.
		for _, f := range full.Payload {
			hidden = append(hidden, f.Name)
		}
	} else {
		hidden = diffPaths(full.Payload, seen.Payload, "")
	}
	p.hidden[key] = hidden
	return hidden, nil
}

// project removes the hidden fields from a payload in place.
func (p *projector) project(ctx context.Context, event string, payload map[string]any, as *spi.Subject) (map[string]any, error) {
	hidden, err := p.hiddenPaths(ctx, event, as)
	if err != nil || len(hidden) == 0 {
		return payload, err
	}
	for _, path := range hidden {
		removePath(payload, strings.Split(path, "."))
	}
	return payload, nil
}

// diffPaths are the fields of full that seen lacks, as dotted paths.
func diffPaths(full, seen expr.Fields, prefix string) []string {
	var out []string
	for _, f := range full {
		s, ok := seen.Get(f.Name)
		if !ok {
			out = append(out, prefix+f.Name)
			continue
		}
		fp, sp := elemProps(f.Field), elemProps(s)
		if len(fp) > 0 {
			out = append(out, diffPaths(fp, sp, prefix+f.Name+".")...)
		}
	}
	return out
}

// elemProps are the props of an object, or of the objects of a list.
func elemProps(f expr.Field) expr.Fields {
	f = f.Unwrap()
	for (f.Kind == expr.List || f.Kind == expr.Map) && f.Elem != nil {
		f = f.Elem.Unwrap()
	}
	if f.Kind == expr.Object {
		return f.Props
	}
	return nil
}

func removePath(v any, path []string) {
	if len(path) == 0 {
		return
	}
	switch x := v.(type) {
	case map[string]any:
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
			if h == ref.Path || strings.HasPrefix(ref.Path, h+".") {
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
		if next, ok := x[path[0]]; ok {
			valuesAt(next, path[1:], add)
		}
	case []any:
		for _, e := range x {
			valuesAt(e, path, add)
		}
	}
}
