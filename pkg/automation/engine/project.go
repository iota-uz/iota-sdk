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

// Hidden replaces a sensitive value a viewer may not read in run history.
const Hidden = "[hidden]"

// redactor masks the values of sensitive fields wherever they ended up in a run's history: rendered
// into an effect's params, copied into a later node's output or an error. The values are taken from the
// events the run read (its trigger and the events its waits woke on) at the paths the viewer may not read.
type redactor struct {
	strs []string
	nums map[string]bool
}

func newRedactor() *redactor { return &redactor{nums: map[string]bool{}} }

// minSubstring is the shortest sensitive string masked inside longer strings; shorter ones are masked
// only where a value equals them, so a one-letter secret does not mask every word.
const minSubstring = 3

// collect takes the sensitive values of an event's payload for the viewer.
func (rd *redactor) collect(ctx context.Context, p *projector, event string, payload map[string]any, viewer *spi.Subject) error {
	hidden, err := p.hiddenPaths(ctx, event, viewer)
	if err != nil {
		return err
	}
	for _, path := range hidden {
		valuesAt(payload, strings.Split(path, "."), rd.add)
	}
	return nil
}

func valuesAt(v any, path []string, add func(any)) {
	if len(path) == 0 {
		leaves(v, add)
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

func leaves(v any, add func(any)) {
	switch x := v.(type) {
	case map[string]any:
		for _, e := range x {
			leaves(e, add)
		}
	case []any:
		for _, e := range x {
			leaves(e, add)
		}
	default:
		add(x)
	}
}

func (rd *redactor) add(v any) {
	switch x := v.(type) {
	case string:
		if x != "" {
			rd.strs = append(rd.strs, x)
		}
	case nil, bool:
	default:
		rd.nums[fmt.Sprint(x)] = true
	}
}

func (rd *redactor) empty() bool { return len(rd.strs) == 0 && len(rd.nums) == 0 }

// value masks a decoded JSON value.
func (rd *redactor) value(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = rd.value(e)
		}
		return x
	case []any:
		for i, e := range x {
			x[i] = rd.value(e)
		}
		return x
	case string:
		return rd.text(x)
	case nil, bool:
		return x
	default:
		if rd.nums[fmt.Sprint(x)] {
			return Hidden
		}
		return x
	}
}

// text masks a string: one that equals a sensitive value, or contains a long enough one or a number.
func (rd *redactor) text(s string) string {
	for _, sec := range rd.strs {
		if s == sec {
			return Hidden
		}
		if len(sec) >= minSubstring {
			s = strings.ReplaceAll(s, sec, Hidden)
		}
	}
	for n := range rd.nums {
		if s == n {
			return Hidden
		}
		if len(n) >= minSubstring {
			s = strings.ReplaceAll(s, n, Hidden)
		}
	}
	return s
}

// raw masks JSON; a value that does not decode is returned as it is.
func (rd *redactor) raw(b json.RawMessage) json.RawMessage {
	if rd.empty() || len(b) == 0 {
		return b
	}
	var v any
	if err := expr.UnmarshalJSON(b, &v); err != nil {
		return b
	}
	out, err := json.Marshal(rd.value(v))
	if err != nil {
		return b
	}
	return out
}

func (rd *redactor) stepError(e *StepError) *StepError {
	if e == nil || rd.empty() {
		return e
	}
	c := *e
	c.Message = rd.text(c.Message)
	return &c
}
