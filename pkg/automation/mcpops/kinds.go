package mcpops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"

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

// enabledSchema accepts any value: the field exists only so that asking to enable gets HUMAN_REQUIRED
// instead of a schema error.
func enabledSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Description: "Not settable by an agent: any value is refused with HUMAN_REQUIRED; a person enables a graph in the editor."}
}

// fields is the decoded Change.Fields of every kind.
type fields struct {
	Graph     json.RawMessage `json:"graph"`
	Layout    graph.Layout    `json:"layout"`
	Note      string          `json:"note"`
	Positions graph.Layout    `json:"positions"`
	// enabled is set when the change names "enabled" at all.
	enabled bool
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
	_, f.enabled = present["enabled"]
	return f, nil
}

// check is what every kind checks before the graph: the human-only field and the sizes and layouts.
func (b base) check(ctx context.Context, op string, f fields, editorURL string) []mcp.Issue {
	var issues []mcp.Issue
	if f.enabled {
		if p, ok := mcp.PrincipalFrom(ctx); ok {
			b.o.recordHumanRequired(ctx, p, op)
		}
		action := "enable or disable an automation: save it, then a person enables it in the editor"
		issues = append(issues, mcp.Issue{Path: "/fields/enabled", Code: IssueHumanRequired, Message: humanMessage(action, editorURL)})
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
	err := b.auth.Validate(ctx, a, raw)
	var ve *engine.ValidationError
	switch {
	case err == nil:
		return nil, nil, nil
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
	return objectSchema(map[string]*jsonschema.Schema{
		"graph":   {Type: "object", Description: "The graph document; automation_catalog returns its JSON Schema as format."},
		"layout":  layoutSchema(),
		"enabled": enabledSchema(),
	}, "graph")
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
	p := changes.Planned{Errors: k.check(ctx, OpCreate, f, "")}
	if len(f.Graph) <= MaxGraphBytes {
		errs, warns, err := k.validateGraph(ctx, a, f.Graph)
		if err != nil {
			return changes.Planned{}, err
		}
		p.Errors, p.Warnings = append(p.Errors, errs...), warns
	}
	p.Diff = graphDiff(nil, f.Graph)
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
	if len(issues) > 0 || f.enabled {
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
	return objectSchema(map[string]*jsonschema.Schema{
		"graph":   {Type: "object", Description: "The whole graph document of the new version."},
		"layout":  layoutSchema(),
		"note":    {Type: "string", MaxLength: jsonschema.Ptr(2000), Description: "What the version changes, shown in the version list."},
		"enabled": enabledSchema(),
	}, "graph")
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
	p := changes.Planned{Revision: strconv.Itoa(am.LatestVersion), Errors: k.check(ctx, OpUpdate, f, k.urls.Editor(id, 0))}
	if len(f.Graph) <= MaxGraphBytes {
		errs, warns, err := k.validateGraph(ctx, a, f.Graph)
		if err != nil {
			return changes.Planned{}, err
		}
		p.Errors, p.Warnings = append(p.Errors, errs...), warns
	}
	latest, err := k.svc.GetVersion(ctx, a, id, am.LatestVersion)
	if err != nil {
		return changes.Planned{}, failure(err)
	}
	p.Diff = graphDiff(latest.Graph, f.Graph)
	if len(f.Layout) > 0 {
		p.Diff = append(p.Diff, fielddiff.Diff{Field: "layout", Before: fielddiff.Present(marshal(am.Layout)), After: fielddiff.Present(marshal(f.Layout))})
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
	am, err := k.svc.Get(ctx, a, id)
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
	if len(issues) > 0 || f.enabled || perr != nil {
		return changes.Applied{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryReadThenPrepare, "the change is not applicable")
	}
	am, err := k.svc.Get(ctx, a, id)
	if err != nil {
		return changes.Applied{}, failure(err)
	}
	v, err := k.svc.SaveVersion(joined(ctx), a, id, engine.SaveInput{BaseVersion: baseVersion, Graph: f.Graph, Layout: f.Layout, Note: f.Note})
	if err != nil {
		return changes.Applied{}, failure(err)
	}
	// An agent's version never becomes the enabled one: the graph runs what it ran before.
	return changes.Applied{ID: id.String(), Revision: strconv.Itoa(v.Version),
		Result: marshal(savedResult{ID: id.String(), Version: v.Version, Enabled: enabledOf(am.EnabledVersion), EditorURL: k.urls.Editor(id, v.Version)})}, nil
}

// ---- automation.layout ----

type layoutKind struct{ base }

func (layoutKind) Op() string     { return OpLayout }
func (layoutKind) Existing() bool { return true }
func (layoutKind) Description() string {
	return "Move nodes on the canvas: positions are merged into the layout, no version is created. expected_revision is " +
		"layout_revision as automation_get returns it; a stale one is a conflict."
}

func (layoutKind) Fields() *jsonschema.Schema {
	return objectSchema(map[string]*jsonschema.Schema{"positions": layoutSchema(), "enabled": enabledSchema()}, "positions")
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
	p := changes.Planned{Revision: strconv.Itoa(am.LayoutRevision), Errors: k.check(ctx, OpLayout, f, k.urls.Editor(id, 0))}
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

func (k layoutKind) Revision(ctx context.Context, rawID string) (string, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return "", err
	}
	id, err := parseID("id", rawID)
	if err != nil {
		return "", err
	}
	am, err := k.svc.Get(ctx, a, id)
	if err != nil {
		return "", failure(err)
	}
	return strconv.Itoa(am.LayoutRevision), nil
}

func (k layoutKind) Apply(ctx context.Context, c changes.Change) (changes.Applied, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return changes.Applied{}, err
	}
	id, err := parseID("id", c.ID)
	if err != nil {
		return changes.Applied{}, err
	}
	f, issues := decodeFields(c.Fields)
	baseRevision, perr := strconv.Atoi(c.ExpectedRevision)
	if len(issues) > 0 || f.enabled || perr != nil {
		return changes.Applied{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryReadThenPrepare, "the change is not applicable")
	}
	rev, err := k.svc.SaveLayout(joined(ctx), a, id, engine.LayoutInput{BaseRevision: baseRevision, Positions: f.Positions})
	if err != nil {
		return changes.Applied{}, failure(err)
	}
	return changes.Applied{ID: id.String(), Revision: strconv.Itoa(rev),
		Result: marshal(layoutResult{ID: id.String(), LayoutRevision: rev, EditorURL: k.urls.Editor(id, 0)})}, nil
}
