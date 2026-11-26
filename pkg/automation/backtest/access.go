package backtest

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

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
	s    *Svc
	a    Actor
	view catalog.View
	// system is the tenant's catalog with nothing withheld: what the reader's view removes beyond it is
	// what the reader may not read (an undeclared key of a payload is not).
	system catalog.View
	conns  map[string]bool
	mu     sync.Mutex // the caches below: a backtest's examples read through one reader in parallel
	// subjects caches whether the reader may read, now, every field a stored snapshot holds.
	subjects map[engine.SubjectRef]map[string]bool
}

func (s *Svc) readerOf(ctx context.Context, a Actor) (*reader, error) {
	view, err := s.cfg.Catalog.ForTenant(ctx, a.TenantID, catalog.AsAuthor(a.Subject))
	if err != nil {
		return nil, err
	}
	system, err := s.cfg.Catalog.ForTenant(ctx, a.TenantID)
	if err != nil {
		return nil, err
	}
	return &reader{s: s, a: a, view: view, system: system, conns: map[string]bool{}, subjects: map[engine.SubjectRef]map[string]bool{}}, nil
}

// mayRead is whether the reader may read the history of an origin.
func (r *reader) mayRead(ctx context.Context, origin string) bool {
	switch {
	case strings.HasPrefix(origin, "connection:"):
		r.mu.Lock()
		ok, seen := r.conns[origin]
		r.mu.Unlock()
		if seen {
			return ok
		}
		ok = false
		if id, err := uuid.Parse(strings.TrimPrefix(origin, "connection:")); err == nil && r.s.cfg.Connections != nil {
			if conn, _, err := r.s.cfg.Connections.Get(ctx, r.a.TenantID, id); err == nil {
				ok = r.s.cfg.ConnectionAccess(r.a, conn)
			}
		}
		r.mu.Lock()
		r.conns[origin] = ok
		r.mu.Unlock()
		return ok
	case strings.HasPrefix(origin, "module:"):
		hs, ok := r.s.source(strings.TrimPrefix(origin, "module:"))
		return ok && (hs.Permission == "" || r.a.Subject.Has(hs.Permission))
	}
	return true
}

// sight is what a reader sees of an example.
type sight int

const (
	seeAll     sight = iota // every field of the stored event
	seePartial              // the event without fields the reader may not read now
	seeNone                 // the kind only
)

// example applies the reader's current rights to an example, its importer included (rights change
// after an import): the event is projected on the reader's catalog view, the subject's snapshot is
// never returned (it was read with the importer's rights then), and an example the reader may not read
// keeps its kind only, without title or subject.
func (r *reader) example(ctx context.Context, x *Example) sight {
	snapshot := x.Snapshot
	x.Snapshot = nil
	if !r.mayRead(ctx, x.Origin) {
		r.hide(x)
		return seeNone
	}
	var payload map[string]any
	if expr.UnmarshalJSON(x.Event, &payload) != nil || payload == nil {
		r.hide(x)
		return seeNone
	}
	full, known := r.system.Catalog.Events[x.EventKind]
	spec, ok := r.view.Catalog.Events[x.EventKind]
	switch {
	case !known, !ok:
		// The catalog no longer knows the event (what was restricted in it cannot be told), or the reader
		// may not read the event at all.
		r.hide(x)
		return seeNone
	}
	bySelf, hasBySelf := payload[catalog.BySelfField]
	project := func(fs expr.Fields) (json.RawMessage, error) {
		kept := engine.KeepDeclared(payload, fs)
		kept["kind"] = x.EventKind
		if hasBySelf {
			kept[catalog.BySelfField] = bySelf
		}
		return json.Marshal(kept)
	}
	// Partial: the reader's view removes fields the full catalog keeps (sensitive ones). A key no
	// schema declares is dropped from both, so it makes nothing partial.
	whole, err := project(full.Payload)
	if err != nil {
		r.hide(x)
		return seeNone
	}
	seen, err := project(spec.Payload)
	if err != nil {
		r.hide(x)
		return seeNone
	}
	x.Event = seen
	// The effects and the trace also derive from the subject's snapshot: they are the reader's only when
	// the reader may read every field of it now.
	if string(whole) != string(seen) || !r.subjectOK(ctx, x.Subject, snapshot) {
		return seePartial
	}
	return seeAll
}

// subjectOK is whether the reader may read, now, every field of a stored snapshot of the subject: the
// record as the subject loader gives it to the reader holds them all. No snapshot needs nothing.
func (r *reader) subjectOK(ctx context.Context, ref engine.SubjectRef, snapshot json.RawMessage) bool {
	if len(snapshot) == 0 || string(snapshot) == "null" {
		return true
	}
	var stored map[string]json.RawMessage
	if json.Unmarshal(snapshot, &stored) != nil {
		return false
	}
	if len(stored) == 0 {
		return true
	}
	r.mu.Lock()
	fields, seen := r.subjects[ref]
	r.mu.Unlock()
	if !seen {
		fields = map[string]bool{}
		if r.s.cfg.Subjects != nil && ref.Key != "" {
			as := r.a.Subject
			as.TenantID = r.a.TenantID
			if raw, err := r.s.cfg.Subjects.Load(ctx, as, ref); err == nil {
				var now map[string]json.RawMessage
				if json.Unmarshal(raw, &now) == nil {
					for k := range now {
						fields[k] = true
					}
				}
			}
		}
		r.mu.Lock()
		r.subjects[ref] = fields
		r.mu.Unlock()
	}
	for k := range stored {
		if !fields[k] {
			return false
		}
	}
	return true
}

func (r *reader) hide(x *Example) {
	x.Event = kindOnly(x.EventKind)
	x.Title, x.Snapshot, x.HasSnapshot, x.Hidden = "", nil, false, true
	x.Subject = engine.SubjectRef{}
}

// result applies the reader's rights to a result: the effects' params and the trace are rendered from
// the whole event, so a reader who may not read all of it does not see them.
func (r *reader) result(ctx context.Context, x *Result) {
	if r.example(ctx, &x.Example) == seeAll {
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
