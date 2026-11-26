package backtest

import (
	"context"
	"encoding/json"
	"reflect"
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
//   - the subject's snapshot (read with the importer's rights) is the reader's when they read it with
//     those rights (see subjectOK).
type reader struct {
	s    *Svc
	a    Actor
	view catalog.View
	// system is the tenant's catalog with nothing withheld: what the reader's view removes beyond it is
	// what the reader may not read (an undeclared key of a payload is not).
	system catalog.View
	conns  map[string]bool
	mu     sync.Mutex // the caches below: a backtest's examples read through one reader in parallel
	// subjects caches the record of each subject as the reader may read it now (nil: none).
	subjects map[engine.SubjectRef]any
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
	return &reader{s: s, a: a, view: view, system: system, conns: map[string]bool{}, subjects: map[engine.SubjectRef]any{}}, nil
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
		// Lookups of one origin may run at once: a denial recorded by one is never replaced by another's
		// allow (rights may have been revoked between them).
		r.mu.Lock()
		if prev, seen := r.conns[origin]; seen {
			ok = ok && prev
		}
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
	snapshot, readWith := x.Snapshot, x.SnapshotReadWith
	x.Snapshot, x.SnapshotReadWith = nil, nil
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
	if string(whole) != string(seen) || !r.subjectOK(ctx, x.Subject, snapshot, x.CreatedBy, readWith) {
		return seePartial
	}
	return seeAll
}

// subjectOK is whether the reader may read a stored snapshot of the subject now. The snapshot is the
// record as the loader gave it to its author (by) with the permissions readWith, at the event:
//   - its author, who still holds every one of those permissions, reads it as they read it then, whatever
//     the record became since (changed values and list items, a record deleted since): the loader's
//     rights are the author's and their permissions;
//   - anyone else (or the author who lost one of them) reads it only when the record as the loader gives
//     it to them now holds each of its fields, at every depth (see covers); a subject that cannot be
//     loaded allows no snapshot then.
//
// No snapshot needs nothing.
func (r *reader) subjectOK(ctx context.Context, ref engine.SubjectRef, snapshot json.RawMessage, by int64, readWith []string) bool {
	if len(snapshot) == 0 || string(snapshot) == "null" {
		return true
	}
	if readWith != nil && by == r.a.UserID && holdsAll(r.a.Subject.Permissions, readWith) {
		return true
	}
	var stored any
	if expr.UnmarshalJSON(snapshot, &stored) != nil {
		return false
	}
	if m, ok := stored.(map[string]any); ok && len(m) == 0 {
		return true
	}
	r.mu.Lock()
	now, seen := r.subjects[ref]
	r.mu.Unlock()
	if !seen {
		now = nil
		if r.s.cfg.Subjects != nil && ref.Key != "" {
			as := r.a.Subject
			as.TenantID = r.a.TenantID
			if raw, err := r.s.cfg.Subjects.Load(ctx, as, ref); err == nil {
				var v any
				if expr.UnmarshalJSON(raw, &v) == nil {
					now = v
				}
			}
		}
		r.mu.Lock()
		r.subjects[ref] = now
		r.mu.Unlock()
	}
	return now != nil && covers(now, stored)
}

// covers is whether the record the reader may read now has every field the stored one has, at every
// depth: each key of a stored object is in the current object and covered there; a scalar is covered by
// any scalar, its value may have changed since. A loader may filter a list's items by the reader's
// rights, and an item's position or values say nothing about which item it is: each item of a stored
// list must be in the current list as it is (unchanged), a distinct one for each (equal items count),
// or the list is not covered. A field the
// reader's rights withhold is missing from the current record, so a stored snapshot that holds it is
// not covered.
func covers(now, stored any) bool {
	switch st := stored.(type) {
	case map[string]any:
		n, ok := now.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range st {
			nv, ok := n[k]
			if !ok || !covers(nv, v) {
				return false
			}
		}
		return true
	case []any:
		n, ok := now.([]any)
		if !ok {
			return false
		}
		// Each current item matches one stored item at most: two equal stored items need two.
		used := make([]bool, len(n))
		for _, item := range st {
			found := false
			for i, x := range n {
				if !used[i] && reflect.DeepEqual(x, item) {
					used[i], found = true, true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	case nil:
		return true
	default:
		switch now.(type) {
		case map[string]any, []any:
			return false
		}
		return true
	}
}

// holdsAll is whether have includes every permission of need.
func holdsAll(have, need []string) bool {
	set := make(map[string]bool, len(have))
	for _, p := range have {
		set[p] = true
	}
	for _, p := range need {
		if !set[p] {
			return false
		}
	}
	return true
}

func (r *reader) hide(x *Example) {
	x.Event = kindOnly(x.EventKind)
	x.Title, x.Snapshot, x.SnapshotReadWith, x.HasSnapshot, x.Hidden = "", nil, nil, false, true
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
