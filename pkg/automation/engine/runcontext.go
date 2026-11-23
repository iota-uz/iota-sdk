package engine

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// Links give the absolute links of a run's context: subject.url (the record's page) and run.url (the run
// in the editor).
type Links interface {
	// SubjectURL is the page of the record; "" when its module registers no link.
	SubjectURL(ctx context.Context, tenantID uuid.UUID, s SubjectRef) (string, error)
	RunURL(tenantID, automationID, runID uuid.UUID) string
}

// SubjectURLResolver is the link a module gives for a record of its entity.
type SubjectURLResolver func(ctx context.Context, tenantID uuid.UUID, key string) (string, error)

// LinkRegistry is Links over a base URL and a resolver per entity, which modules register.
type LinkRegistry struct {
	base string
	// RunPath is the path of a run in the editor: %s are the automation and the run IDs.
	runPath string

	mu        sync.RWMutex
	resolvers map[string]SubjectURLResolver
}

// NewLinks builds the links of runs under baseURL (https://erp.example.com); runPath has two %s, the
// automation's ID and the run's ID ("" → /automations/%s/runs/%s).
func NewLinks(baseURL, runPath string) *LinkRegistry {
	if runPath == "" {
		runPath = "/automations/%s/runs/%s"
	}
	return &LinkRegistry{base: strings.TrimRight(baseURL, "/"), runPath: runPath, resolvers: map[string]SubjectURLResolver{}}
}

// Register adds the resolver of an entity; an entity twice is an error.
func (l *LinkRegistry) Register(entity string, r SubjectURLResolver) error {
	if entity == "" || r == nil {
		return errors.New("automation links: an entity and a resolver are required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, dup := l.resolvers[entity]; dup {
		return fmt.Errorf("automation links: entity %s is registered twice", entity)
	}
	l.resolvers[entity] = r
	return nil
}

// SubjectURL is the resolver's link, made absolute under the base URL.
func (l *LinkRegistry) SubjectURL(ctx context.Context, tenantID uuid.UUID, s SubjectRef) (string, error) {
	l.mu.RLock()
	r := l.resolvers[s.Entity]
	l.mu.RUnlock()
	if r == nil || s.Key == "" {
		return "", nil
	}
	link, err := r(ctx, tenantID, s.Key)
	if err != nil || link == "" {
		return "", err
	}
	if u, err := url.Parse(link); err == nil && u.IsAbs() {
		return link, nil
	}
	return l.base + "/" + strings.TrimLeft(link, "/"), nil
}

// RunURL is the run in the editor.
func (l *LinkRegistry) RunURL(_ uuid.UUID, automationID, runID uuid.UUID) string {
	return l.base + fmt.Sprintf(l.runPath, automationID, runID)
}

// RunContextInput is what a run's context is built from.
type RunContextInput struct {
	TenantID     uuid.UUID
	AutomationID uuid.UUID
	RunID        uuid.UUID // uuid.Nil: a dry run or a test (run.id and run.url are empty)
	Subject      SubjectRef
	RunAs        spi.Subject
	Event        map[string]any
	// Events are the events a trigger's collect merged, in order; nil: Event alone.
	Events []map[string]any
	Nodes  map[string]graph.NodeState
	// Foreach is foreach.<id> of a token inside a foreach.
	Foreach map[string]graph.ForeachItem
	Now     time.Time
	// Vars are the subject's variables; nil: loaded with the run-as rights (SubjectLoader).
	Vars map[string]any
	// Overrides replace parts of the context: "nodes.<id>.output" (a dry run's given answers) or a
	// top-level variable of the subject. Applied last.
	Overrides map[string]any
}

// BuildRunContext is the one builder of a run's context: a live trigger and step, a dry run, a test
// node and a backtest read the same variables (event, events, nodes, the subject's variables, subject
// {entity, key, url}, run {id, url}, tenant, now).
func (r *Runtime) BuildRunContext(ctx context.Context, in RunContextInput) (graph.Context, error) {
	subject := normSubject(in.Subject)
	vars := in.Vars
	if vars == nil {
		vars = map[string]any{}
		if r.cfg.Subjects != nil && subject.Entity != graph.SubjectNone {
			raw, err := r.cfg.Subjects.Load(ctx, in.RunAs, subject)
			switch {
			case errors.Is(err, ErrNotFound):
			case err != nil:
				return graph.Context{}, err
			default:
				if vars, err = decodeMap(raw); err != nil {
					vars = map[string]any{}
				}
			}
		}
	}
	gctx := graph.Context{
		Event: in.Event, Events: in.Events, Nodes: in.Nodes, Vars: vars, Now: in.Now, Foreach: in.Foreach,
		Tenant:  map[string]any{"id": in.TenantID.String()},
		Subject: map[string]any{"entity": subject.Entity, "key": subject.Key, "url": ""},
		Run:     map[string]any{"id": "", "url": ""},
	}
	if gctx.Event == nil {
		gctx.Event = map[string]any{}
	}
	if gctx.Nodes == nil {
		gctx.Nodes = map[string]graph.NodeState{}
	}
	if links := r.cfg.Links; links != nil {
		if subject.Entity != graph.SubjectNone {
			link, err := links.SubjectURL(ctx, in.TenantID, subject)
			if err != nil {
				r.cfg.Logger.Warn("automation: no link for the subject", "entity", subject.Entity, "error", err)
			}
			gctx.Subject["url"] = link
		}
		if in.RunID != uuid.Nil {
			gctx.Run["url"] = links.RunURL(in.TenantID, in.AutomationID, in.RunID)
		}
	}
	if in.RunID != uuid.Nil {
		gctx.Run["id"] = in.RunID.String()
	}
	applyOverrides(&gctx, in.Overrides)
	return gctx, nil
}

// applyOverrides sets nodes.<id>.output (and .outcome) and top-level variables of a context.
func applyOverrides(gctx *graph.Context, overrides map[string]any) {
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := overrides[k]
		parts := strings.Split(k, ".")
		switch {
		case len(parts) >= 2 && parts[0] == "nodes":
			id := parts[1]
			st := gctx.Nodes[id]
			switch {
			case len(parts) == 2:
				if m, ok := v.(map[string]any); ok {
					if out, ok := m["output"].(map[string]any); ok {
						st.Output = out
					}
					if oc, ok := m["outcome"].(string); ok {
						st.Outcome = oc
					}
				}
			case parts[2] == "output":
				if m, ok := v.(map[string]any); ok {
					st.Output = m
				}
			case parts[2] == "outcome":
				if s, ok := v.(string); ok {
					st.Outcome = s
				}
			}
			gctx.Nodes[id] = st
		case k == "event":
			if m, ok := v.(map[string]any); ok {
				for f, x := range m {
					gctx.Event[f] = x
				}
			}
		case len(parts) == 1:
			if gctx.Vars == nil {
				gctx.Vars = map[string]any{}
			}
			gctx.Vars[k] = v
		}
	}
}

// CheckRunContext lists the fields of the context schema a context leaves out: the required payload
// fields of its event (and of every merged event), the required variables of its subject, subject
// {entity, key, url} and run {id, url} (run.id and run.url are required of a live run, live = true).
func CheckRunContext(c graph.Catalog, event string, gctx graph.Context, live bool) []string {
	var missing []string
	spec, ok := c.Events[event]
	if !ok {
		return []string{"event: " + event + " is not an event of the catalog"}
	}
	if k, _ := gctx.Event["kind"].(string); k != event {
		missing = append(missing, "event.kind")
	}
	missing = append(missing, requiredIn("event", spec.Payload, gctx.Event)...)
	for i, e := range gctx.Events {
		missing = append(missing, requiredIn(fmt.Sprintf("events[%d]", i), spec.Payload, e)...)
	}
	if s, ok := c.Subjects[spec.Subject]; ok {
		missing = append(missing, requiredIn("", s.Context, gctx.Vars)...)
	}
	for _, f := range []string{"entity", "key", "url"} {
		if _, ok := gctx.Subject[f].(string); !ok {
			missing = append(missing, "subject."+f)
		}
	}
	if e, _ := gctx.Subject["entity"].(string); e == "" {
		missing = append(missing, "subject.entity")
	}
	for _, f := range []string{"id", "url"} {
		v, ok := gctx.Run[f].(string)
		if !ok || (live && v == "") {
			missing = append(missing, "run."+f)
		}
	}
	return missing
}

// requiredIn lists the required fields of fs that v leaves out (or sets to null), depth first.
func requiredIn(prefix string, fs expr.Fields, v map[string]any) []string {
	var out []string
	for _, p := range fs {
		path := p.Name
		if prefix != "" {
			path = prefix + "." + p.Name
		}
		x, ok := v[p.Name]
		if !p.Field.Required() {
			if m, isMap := x.(map[string]any); ok && isMap && p.Field.Unwrap().Kind == expr.Object {
				out = append(out, requiredIn(path, p.Field.Unwrap().Props, m)...)
			}
			continue
		}
		if !ok || x == nil {
			out = append(out, path)
			continue
		}
		if p.Field.Kind == expr.Object {
			if m, isMap := x.(map[string]any); isMap {
				out = append(out, requiredIn(path, p.Field.Props, m)...)
			}
		}
	}
	return out
}
