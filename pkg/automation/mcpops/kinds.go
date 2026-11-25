package mcpops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/changes"
)

// Ops of the kinds of entity "automation".
const (
	OpCreate = "create"
	OpUpdate = "update"
	OpLayout = "layout"
)

// Kinds are the guarded writes of graphs for changes.Config.Kinds. A saved version never runs until a
// person enables it, so no change needs approval; a change that asks to enable is refused in Plan with
// HUMAN_REQUIRED.
func Kinds(svc engine.Service, auth engine.Authoring, urls URLs, opts ...Option) []changes.Kind {
	b := base{svc: svc, auth: auth, urls: urls, o: buildOptions(opts)}
	return []changes.Kind{createKind{b}, updateKind{b}, layoutKind{b}}
}

type base struct {
	svc  engine.Service
	auth engine.Authoring
	urls URLs
	o    options
}

func (base) Entity() string     { return Entity }
func (base) Policy() mcp.Policy { return AuthorPolicy }

func layoutSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object", Description: "Node and trigger id → canvas position.",
		AdditionalProperties: &jsonschema.Schema{Type: "object", Required: []string{"x", "y"},
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			Properties:           map[string]*jsonschema.Schema{"x": {Type: "number"}, "y": {Type: "number"}}}}
}

// fields is the decoded Change.Fields of every kind.
type fields struct {
	Graph     json.RawMessage `json:"graph"`
	Layout    graph.Layout    `json:"layout"`
	Note      string          `json:"note"`
	Positions graph.Layout    `json:"positions"`
	ID        string          `json:"id"`
	// BaseRevision is the layout revision automation.layout is based on.
	BaseRevision revisionArg `json:"base_revision"`
	// LayoutRevision is the layout revision the layout of automation.update is based on.
	LayoutRevision revisionArg `json:"layout_revision"`
	// human are the human-only fields the change names, in humanFields order.
	human []humanField
}

func decodeFields(raw json.RawMessage) (fields, []mcp.Issue) {
	var f fields
	var present map[string]json.RawMessage
	if err := json.Unmarshal(raw, &present); err != nil {
		return f, []mcp.Issue{{Path: "/fields", Code: string(mcp.CodeInvalidInput), Message: "fields must be an object"}}
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, []mcp.Issue{{Path: "/fields", Code: string(mcp.CodeInvalidInput), Message: "fields do not match the schema"}}
	}
	for _, h := range humanFields {
		if _, ok := present[h.name]; ok {
			f.human = append(f.human, h)
		}
	}
	return f, nil
}

// maxRevision is the largest revision a JSON number carries exactly.
const maxRevision = 1 << 53

// revisionArg is a revision a change names (a new graph's layout is at 0): the JSON integer or the decimal string automation_get
// returns ("3"), normalized to one number.
type revisionArg struct {
	n   int
	set bool
}

func (r *revisionArg) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		return nil
	}
	bad := errors.New("a revision is a non-negative integer or its decimal string")
	if unq, err := strconv.Unquote(s); err == nil {
		n, err := strconv.Atoi(unq)
		if err != nil || n < 0 {
			return bad
		}
		r.n, r.set = n, true
		return nil
	}
	// A JSON number: any form of an integral value (1, 1.0, 1e0), as the schema's integer admits.
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 || f != math.Trunc(f) || f > maxRevision {
		return bad
	}
	r.n, r.set = int(f), true
	return nil
}

// revisionSchema accepts a revision as automation_get returns it (a decimal string) or as a number.
func revisionSchema(description string) *jsonschema.Schema {
	return &jsonschema.Schema{Types: []string{"integer", "string"}, Minimum: jsonschema.Ptr(0.0), Pattern: "^(0|[1-9][0-9]*)$",
		Description: description}
}

// humanField is a field of a change only a person may act on: asking for it is refused with
// HUMAN_REQUIRED and the page where the person does it.
type humanField struct {
	name   string
	action string
	page   func(u URLs, id uuid.UUID) string
}

// humanFields are the actions an agent is not offered: enabling or disabling a graph, deciding a human
// task of its runs and managing connections.
var humanFields = []humanField{
	{"enabled", "enable or disable an automation: save it, then a person enables it in the editor", editorOrList},
	{"enabled_version", "choose the version an automation runs: a person enables a version in the editor", editorOrList},
	{"decision", "decide a human task of an automation run", editorOrList},
	{"decisions", "decide human tasks of an automation run", editorOrList},
	{"task", "act on a human task of an automation run", editorOrList},
	{"connection", "create, change or delete a connection", func(u URLs, _ uuid.UUID) string { return u.Connections() }},
	{"connections", "create, change or delete connections", func(u URLs, _ uuid.UUID) string { return u.Connections() }},
}

func editorOrList(u URLs, id uuid.UUID) string {
	if id == uuid.Nil {
		return u.Automations()
	}
	return u.Editor(id, 0)
}

// withHumanFields adds the human-only fields to a kind's fields schema: any value is accepted, so the
// request gets HUMAN_REQUIRED from Plan instead of a schema error.
func withHumanFields(s *jsonschema.Schema) *jsonschema.Schema {
	for _, f := range humanFields {
		s.Properties[f.name] = &jsonschema.Schema{Description: "Not settable by an agent: only a person may " + f.action + "; any value is refused with HUMAN_REQUIRED."}
	}
	return s
}

// parseUUID is an id of fields; ok is false when it is not a UUID (a plan issue, not an error).
func parseUUID(s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	return id, err == nil
}

// check is what every kind checks before the graph: the human-only fields and the sizes and layouts.
func (b base) check(ctx context.Context, op string, f fields, id uuid.UUID) []mcp.Issue {
	issues := make([]mcp.Issue, 0, len(f.human)+3)
	for _, h := range f.human {
		issues = append(issues, mcp.Issue{Path: "/fields/" + h.name, Code: IssueHumanRequired, Message: humanMessage(h.action, h.page(b.urls, id))})
	}
	if len(f.human) > 0 {
		auditHumanRequired(ctx, op)
	}
	if len(f.Graph) > MaxGraphBytes {
		issues = append(issues, mcp.Issue{Path: "/fields/graph", Code: string(mcp.CodeLimitExceeded), Message: "a graph is at most 1 MiB"})
	}
	if errs := graph.ValidateLayout(f.Layout); len(errs) > 0 {
		issues = append(issues, graphIssues("/fields/layout", errs)...)
	}
	if errs := graph.ValidateLayout(f.Positions); len(errs) > 0 {
		issues = append(issues, graphIssues("/fields/positions", errs)...)
	}
	return issues
}

// validateGraph checks the graph as the principal; its errors are issues under /fields/graph.
func (b base) validateGraph(ctx context.Context, a engine.Actor, raw json.RawMessage) ([]mcp.Issue, []mcp.Issue, error) {
	warnings, err := b.auth.Validate(ctx, a, raw)
	var ve *engine.ValidationError
	switch {
	case err == nil:
		return nil, graphIssues("/fields/graph", warnings), nil
	case errors.As(err, &ve):
		return graphIssues("/fields/graph", ve.Errors), graphIssues("/fields/graph", ve.Warnings), nil
	}
	return nil, nil, failure(err)
}

func present(raw json.RawMessage) fielddiff.FieldValue {
	if !nonNullRaw(raw) {
		return fielddiff.Absent()
	}
	return fielddiff.Present(raw)
}

func marshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

// graphDiff compares two graphs by their top-level fields (name, triggers, nodes, ...).
func graphDiff(before, after json.RawMessage) []fielddiff.Diff {
	var bm, am map[string]json.RawMessage
	_ = json.Unmarshal(before, &bm)
	if err := json.Unmarshal(after, &am); err != nil {
		return []fielddiff.Diff{{Field: "graph", Before: present(before), After: present(after)}}
	}
	keys := make([]string, 0, len(bm)+len(am))
	for k := range bm {
		keys = append(keys, k)
	}
	for k := range am {
		if _, ok := bm[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	out := []fielddiff.Diff{}
	for _, k := range keys {
		if canonical(bm[k]) == canonical(am[k]) {
			continue
		}
		out = append(out, fielddiff.Diff{Field: "graph." + k, Before: present(bm[k]), After: present(am[k])})
	}
	return out
}

func canonical(raw json.RawMessage) string {
	if !nonNullRaw(raw) {
		return ""
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return string(raw)
	}
	return string(marshal(v))
}

// joined is the context whose engine writes join the transaction of the apply (changes.Store.InTx): the
// graph commits with the operation's receipt, so repeating apply_changes with the same key after a lost
// response never saves it twice.
func joined(ctx context.Context) context.Context {
	if tx, err := composables.UseTx(ctx); err == nil {
		if ptx, ok := tx.(pgx.Tx); ok {
			return engine.WithTx(ctx, ptx)
		}
	}
	return ctx
}

func existingID(c changes.Change) (uuid.UUID, []mcp.Issue) {
	id, err := uuid.Parse(c.ID)
	if err != nil {
		return uuid.Nil, []mcp.Issue{{Path: "/id", Code: "uuid", Message: "id must be a UUID"}}
	}
	return id, nil
}

func notFound() []mcp.Issue {
	return []mcp.Issue{{Path: "/id", Code: string(mcp.CodeNotFound), Message: "the automation is not found"}}
}

// savedResult is what a save reports: the new version, the version that runs (false: none) and the
// editor where a person reviews and enables it.
type savedResult struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	Enabled   any    `json:"enabled"`
	EditorURL string `json:"editor_url"`
}

func enabledOf(v *int) any {
	if v == nil {
		return false
	}
	return *v
}

// ---- automation.create ----

type createKind struct{ base }

func (createKind) Op() string     { return OpCreate }
func (createKind) Existing() bool { return false }
func (createKind) Description() string {
	return "Create a graph from a graph document (name, description and run_as are fields of graph) and its optional " +
		"layout. The saved version is disabled until a person enables it in the editor."
}

func (createKind) Fields() *jsonschema.Schema {
	return withHumanFields(objectSchema(map[string]*jsonschema.Schema{
		"graph":  {Type: "object", Description: "The graph document; automation_catalog returns its JSON Schema as format."},
		"layout": layoutSchema(),
	}, "graph"))
}

func (k createKind) Plan(ctx context.Context, c changes.Change) (changes.Planned, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return changes.Planned{}, err
	}
	f, issues := decodeFields(c.Fields)
	if len(issues) > 0 {
		return changes.Planned{Errors: issues}, nil
	}
	p := changes.Planned{Errors: k.check(ctx, OpCreate, f, uuid.Nil)}
	if len(f.Graph) <= MaxGraphBytes {
		errs, warns, err := k.validateGraph(ctx, a, f.Graph)
		if err != nil {
			return changes.Planned{}, err
		}
		p.Errors, p.Warnings = append(p.Errors, errs...), warns
	}
	if len(f.Graph) <= MaxGraphBytes {
		p.Diff = graphDiff(nil, f.Graph)
	}
	if len(f.Layout) > 0 {
		p.Diff = append(p.Diff, fielddiff.Diff{Field: "layout", Before: fielddiff.Absent(), After: fielddiff.Present(marshal(f.Layout))})
	}
	return p, nil
}

func (createKind) Revision(context.Context, string) (string, error) { return "", nil }

func (k createKind) Apply(ctx context.Context, c changes.Change) (changes.Applied, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return changes.Applied{}, err
	}
	f, issues := decodeFields(c.Fields)
	if len(issues) > 0 || len(f.human) > 0 {
		return changes.Applied{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryReadThenPrepare, "the change is not applicable")
	}
	am, v, err := k.svc.Create(joined(ctx), a, engine.CreateInput{Graph: f.Graph, Layout: f.Layout})
	if err != nil {
		return changes.Applied{}, failure(err)
	}
	return changes.Applied{ID: am.ID.String(), Revision: strconv.Itoa(v.Version),
		Result: marshal(savedResult{ID: am.ID.String(), Version: v.Version, Enabled: enabledOf(am.EnabledVersion), EditorURL: k.urls.Editor(am.ID, v.Version)})}, nil
}

// ---- automation.update ----

type updateKind struct{ base }

func (updateKind) Op() string     { return OpUpdate }
func (updateKind) Existing() bool { return true }
func (updateKind) Description() string {
	return "Save a new version of a graph. expected_revision is the latest version as automation_get returns it (\"7\"); " +
		"a stale one is a conflict: read the graph again. The version does not run until a person enables it; an enabled " +
		"graph keeps running the version it runs."
}

func (updateKind) Fields() *jsonschema.Schema {
	return withHumanFields(objectSchema(map[string]*jsonschema.Schema{
		"graph":           {Type: "object", Description: "The whole graph document of the new version."},
		"layout":          layoutSchema(),
		"note":            {Type: "string", MaxLength: jsonschema.Ptr(2000), Description: "What the version changes, shown in the version list."},
		"layout_revision": revisionSchema("Required with layout: layout_revision as automation_get returns it; a stale one is VERSION_CONFLICT."),
	}, "graph"))
}

func (k updateKind) Plan(ctx context.Context, c changes.Change) (changes.Planned, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return changes.Planned{}, err
	}
	id, issues := existingID(c)
	if len(issues) > 0 {
		return changes.Planned{Errors: issues}, nil
	}
	f, issues := decodeFields(c.Fields)
	if len(issues) > 0 {
		return changes.Planned{Errors: issues}, nil
	}
	am, err := k.svc.Get(ctx, a, id)
	if errors.Is(err, engine.ErrNotFound) {
		return changes.Planned{Errors: notFound(), Revision: c.ExpectedRevision}, nil
	}
	if err != nil {
		return changes.Planned{}, failure(err)
	}
	p := changes.Planned{Revision: strconv.Itoa(am.LatestVersion), Errors: k.check(ctx, OpUpdate, f, id)}
	if len(f.Graph) <= MaxGraphBytes {
		errs, warns, err := k.validateGraph(ctx, a, f.Graph)
		if err != nil {
			return changes.Planned{}, err
		}
		p.Errors, p.Warnings = append(p.Errors, errs...), warns
	}
	if len(f.Layout) > 0 {
		switch {
		case !f.LayoutRevision.set:
			p.Errors = append(p.Errors, mcp.Issue{Path: "/fields/layout_revision", Code: "required",
				Message: "positions are based on a layout: pass layout_revision as automation_get returns it"})
		case f.LayoutRevision.n != am.LayoutRevision:
			p.Errors = append(p.Errors, mcp.Issue{Path: "/fields/layout_revision", Code: "VERSION_CONFLICT",
				Message: "the layout revision is " + strconv.Itoa(am.LayoutRevision) + "; read the graph again and prepare again"})
		}
	}
	if len(f.Graph) <= MaxGraphBytes {
		latest, err := k.svc.GetVersion(ctx, a, id, am.LatestVersion)
		if err != nil {
			return changes.Planned{}, failure(err)
		}
		p.Diff = graphDiff(latest.Graph, f.Graph)
	}
	if len(f.Layout) > 0 {
		// What Apply saves: the given positions over the stored ones.
		merged := make(graph.Layout, len(am.Layout)+len(f.Layout))
		for k, xy := range am.Layout {
			merged[k] = xy
		}
		for k, xy := range f.Layout {
			merged[k] = xy
		}
		// As SaveVersion: only the steps of the new graph keep a position.
		if g, errs := graph.Decode(f.Graph); len(errs) == 0 {
			merged = graph.LayoutOf(g, merged)
		}
		p.Diff = append(p.Diff, fielddiff.Diff{Field: "layout", Before: fielddiff.Present(marshal(am.Layout)), After: fielddiff.Present(marshal(merged))})
	}
	if f.Note != "" {
		p.Diff = append(p.Diff, fielddiff.Diff{Field: "note", Before: fielddiff.Absent(), After: fielddiff.Present(marshal(f.Note))})
	}
	return p, nil
}

func (k updateKind) Revision(ctx context.Context, rawID string) (string, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return "", err
	}
	id, err := parseID("id", rawID)
	if err != nil {
		return "", err
	}
	am, err := k.svc.Get(joined(ctx), a, id)
	if err != nil {
		return "", failure(err)
	}
	return strconv.Itoa(am.LatestVersion), nil
}

func (k updateKind) Apply(ctx context.Context, c changes.Change) (changes.Applied, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return changes.Applied{}, err
	}
	id, err := parseID("id", c.ID)
	if err != nil {
		return changes.Applied{}, err
	}
	f, issues := decodeFields(c.Fields)
	baseVersion, perr := strconv.Atoi(c.ExpectedRevision)
	if len(issues) > 0 || len(f.human) > 0 || perr != nil {
		return changes.Applied{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryReadThenPrepare, "the change is not applicable")
	}
	in := engine.SaveInput{BaseVersion: baseVersion, Graph: f.Graph, Layout: f.Layout, Note: f.Note}
	if len(f.Layout) > 0 {
		if !f.LayoutRevision.set {
			return changes.Applied{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryReadThenPrepare, "the change is not applicable")
		}
		// SaveVersion compares it with the layout revision under the graph's lock and, strict, saves nothing
		// when another write moved the layout since the plan: the plan applies whole or not at all.
		in.LayoutBase, in.StrictLayout = &f.LayoutRevision.n, true
	}
	v, err := k.svc.SaveVersion(joined(ctx), a, id, in)
	var lc *engine.LayoutConflictError
	if errors.As(err, &lc) {
		return changes.Applied{}, mcp.Fail(mcp.CodeConflict, mcp.RecoveryReadThenPrepare,
			"the layout changed: its revision is "+strconv.Itoa(lc.Revision)+"; read the graph again and prepare again",
			mcp.Issue{Path: "/fields/layout_revision", Code: "VERSION_CONFLICT", Message: "current revision is " + strconv.Itoa(lc.Revision)})
	}
	if err != nil {
		return changes.Applied{}, failure(err)
	}
	// Read in the apply's transaction, which holds the graph's row lock since the save: the version that
	// runs is the one the save saw.
	am, err := k.svc.Get(joined(ctx), a, id)
	if err != nil {
		return changes.Applied{}, failure(err)
	}
	// An agent's version never becomes the enabled one: the graph runs what it ran before.
	return changes.Applied{ID: id.String(), Revision: strconv.Itoa(v.Version),
		Result: marshal(savedResult{ID: id.String(), Version: v.Version, Enabled: enabledOf(am.EnabledVersion), EditorURL: k.urls.Editor(id, v.Version)})}, nil
}

// ---- automation.layout ----

// layoutKind moves nodes on the canvas: {id, base_revision, positions}. The layout is outside the
// versions, so its base is the layout revision of automation_get, carried in fields rather than as
// expected_revision; a stale base is VERSION_CONFLICT with the current revision.
type layoutKind struct{ base }

func (layoutKind) Op() string     { return OpLayout }
func (layoutKind) Existing() bool { return false }
func (layoutKind) Description() string {
	return "Move nodes on the canvas: positions are merged into the layout, no version is created. base_revision is " +
		"layout_revision as automation_get returns it; a stale one is VERSION_CONFLICT: read the graph again."
}

func (layoutKind) Fields() *jsonschema.Schema {
	return withHumanFields(objectSchema(map[string]*jsonschema.Schema{
		"id":            {Type: "string", Format: "uuid"},
		"base_revision": revisionSchema("layout_revision as automation_get returns it."),
		"positions":     layoutSchema(),
	}, "id", "base_revision", "positions"))
}

type layoutResult struct {
	ID             string `json:"id"`
	LayoutRevision int    `json:"layout_revision"`
	EditorURL      string `json:"editor_url"`
}

func (k layoutKind) Plan(ctx context.Context, c changes.Change) (changes.Planned, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return changes.Planned{}, err
	}
	f, issues := decodeFields(c.Fields)
	if len(issues) > 0 {
		return changes.Planned{Errors: issues}, nil
	}
	id, ok := parseUUID(f.ID)
	if !ok {
		return changes.Planned{Errors: []mcp.Issue{{Path: "/fields/id", Code: "uuid", Message: "id must be a UUID"}}}, nil
	}
	am, err := k.svc.Get(ctx, a, id)
	if errors.Is(err, engine.ErrNotFound) {
		return changes.Planned{Errors: []mcp.Issue{{Path: "/fields/id", Code: string(mcp.CodeNotFound), Message: "the automation is not found"}}}, nil
	}
	if err != nil {
		return changes.Planned{}, failure(err)
	}
	p := changes.Planned{Errors: k.check(ctx, OpLayout, f, id)}
	if f.BaseRevision.set && f.BaseRevision.n != am.LayoutRevision {
		p.Errors = append(p.Errors, mcp.Issue{Path: "/fields/base_revision", Code: "VERSION_CONFLICT",
			Message: "the layout revision is " + strconv.Itoa(am.LayoutRevision) + "; read the graph again and prepare again"})
	}
	keys := make([]string, 0, len(f.Positions))
	for key := range f.Positions {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		before := fielddiff.Absent()
		if xy, ok := am.Layout[key]; ok {
			before = fielddiff.Present(marshal(xy))
		}
		p.Diff = append(p.Diff, fielddiff.Diff{Field: "layout." + key, Before: before, After: fielddiff.Present(marshal(f.Positions[key]))})
	}
	return p, nil
}

func (layoutKind) Revision(context.Context, string) (string, error) { return "", nil }

func (k layoutKind) Apply(ctx context.Context, c changes.Change) (changes.Applied, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return changes.Applied{}, err
	}
	f, issues := decodeFields(c.Fields)
	id, perr := uuid.Parse(f.ID)
	if len(issues) > 0 || len(f.human) > 0 || perr != nil || !f.BaseRevision.set {
		return changes.Applied{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryReadThenPrepare, "the change is not applicable")
	}
	// SaveLayout compares the base with the revision under the graph's lock: a save that won the race
	// since the plan is VERSION_CONFLICT.
	rev, err := k.svc.SaveLayoutAt(joined(ctx), a, id, f.BaseRevision.n, f.Positions)
	if err != nil {
		return changes.Applied{}, failure(err)
	}
	return changes.Applied{ID: id.String(), Revision: strconv.Itoa(rev),
		Result: marshal(layoutResult{ID: id.String(), LayoutRevision: rev, EditorURL: k.urls.Editor(id, 0)})}, nil
}
