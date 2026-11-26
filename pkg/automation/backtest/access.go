package backtest

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

// reader is what an actor may read of a dataset's examples. An example keeps what its importer could
// read; another author reads it through their own rights:
//   - the history of a connection the reader has no right to (ConnectionAccess), or of a module's source
//     whose permission the reader lacks, is hidden: the event keeps its kind only, its title, effects
//     and trace are not shown;
//   - any other event is projected on the reader's catalog view: the sensitive fields the reader may not
//     read are removed;
//   - the subject's snapshot (read with the importer's rights) is the importer's only.
type reader struct {
	s     *Svc
	a     Actor
	view  catalog.View
	conns map[string]bool
}

func (s *Svc) readerOf(ctx context.Context, a Actor) (*reader, error) {
	view, err := s.cfg.Catalog.ForTenant(ctx, a.TenantID, catalog.AsAuthor(a.Subject))
	if err != nil {
		return nil, err
	}
	return &reader{s: s, a: a, view: view, conns: map[string]bool{}}, nil
}

// mayRead is whether the reader may read the history of an origin.
func (r *reader) mayRead(ctx context.Context, origin string) bool {
	switch {
	case strings.HasPrefix(origin, "connection:"):
		if ok, seen := r.conns[origin]; seen {
			return ok
		}
		ok := false
		if id, err := uuid.Parse(strings.TrimPrefix(origin, "connection:")); err == nil && r.s.cfg.Connections != nil {
			if conn, _, err := r.s.cfg.Connections.Get(ctx, r.a.TenantID, id); err == nil {
				ok = r.s.cfg.ConnectionAccess(r.a, conn)
			}
		}
		r.conns[origin] = ok
		return ok
	case strings.HasPrefix(origin, "module:"):
		hs, ok := r.s.source(strings.TrimPrefix(origin, "module:"))
		return ok && (hs.Permission == "" || r.a.Subject.Has(hs.Permission))
	}
	return true
}

// example applies the reader's rights to an example and reports whether it is hidden (the reader may
// not read it).
func (r *reader) example(ctx context.Context, x *Example) bool {
	if x.CreatedBy != r.a.UserID {
		x.Snapshot = nil
	}
	if !r.mayRead(ctx, x.Origin) {
		x.Event = kindOnly(x.EventKind)
		x.Title, x.Snapshot, x.Hidden = "", nil, true
		return true
	}
	if x.CreatedBy == r.a.UserID {
		return false
	}
	var payload map[string]any
	if expr.UnmarshalJSON(x.Event, &payload) != nil || payload == nil {
		return false
	}
	spec, ok := r.view.Catalog.Events[x.EventKind]
	if !ok {
		x.Event = kindOnly(x.EventKind)
		x.Title, x.Hidden = "", true
		return true
	}
	bySelf, hasBySelf := payload[catalog.BySelfField]
	kept := engine.KeepDeclared(payload, spec.Payload)
	kept["kind"] = x.EventKind
	if hasBySelf {
		kept[catalog.BySelfField] = bySelf
	}
	if b, err := json.Marshal(kept); err == nil {
		x.Event = b
	}
	return false
}

// result applies the reader's rights to a result: a hidden example's effects and trace, rendered from
// its event, are not shown.
func (r *reader) result(ctx context.Context, x *Result) {
	if !r.example(ctx, &x.Example) {
		return
	}
	for i := range x.Effects {
		x.Effects[i].Params = json.RawMessage("null")
	}
	x.Trace = nil
}

// kindOnly is an event that keeps its kind only.
func kindOnly(kind string) json.RawMessage {
	b, err := json.Marshal(map[string]string{"kind": kind})
	if err != nil { // a string always encodes
		return json.RawMessage(`{}`)
	}
	return b
}
