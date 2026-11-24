package mcpops

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Names of the tools.
const (
	ToolCatalog  = "automation_catalog"
	ToolList     = "automation_list"
	ToolGet      = "automation_get"
	ToolRuns     = "automation_runs"
	ToolRunGet   = "automation_run_get"
	ToolValidate = "automation_validate"
	ToolDryRun   = "automation_dry_run"
)

// ReadPolicy and AuthorPolicy guard the tools; the engine checks the same permissions again.
var (
	ReadPolicy   = mcp.Policy{Scopes: []string{ScopeRead}, Permissions: []string{engine.PermRead}}
	AuthorPolicy = mcp.Policy{Scopes: []string{ScopeAuthor}, Permissions: []string{engine.PermAuthor}}
)

// Tools are the read-only operations of automations for the shared registry: reads, validate and dry
// run. Writes are Kinds of prepare_changes. Test node is not offered: it makes a real external effect.
func Tools(svc engine.Service, auth engine.Authoring, reg catalog.Registry, urls URLs, opts ...Option) ([]mcp.Tool, error) {
	const op serrors.Op = "mcpops.Tools"
	if svc == nil || auth == nil || reg == nil || urls == nil {
		return nil, serrors.New(serrors.Invalid, "service, authoring, catalog and URLs are required").WithOp(op)
	}
	t := &tools{svc: svc, auth: auth, reg: reg, urls: urls, o: buildOptions(opts)}
	var out []mcp.Tool
	var errs []error
	add := func(tool mcp.Tool, err error) {
		if err != nil {
			errs = append(errs, err)
			return
		}
		out = append(out, tool)
	}
	readOnly := func(title string) mcp.Annotations {
		return mcp.Annotations{Title: title, ReadOnly: true, Idempotent: true}
	}
	add(newTool(t.o, mcp.Spec{
		Name: ToolCatalog,
		Description: "The node types and events you may use in automation graphs of your tenant, with their params, outcomes " +
			"and payload schemas, and the JSON Schema of a graph. Events and nodes you may not use, or whose connection " +
			"the tenant lacks, are left out; sensitive payload fields you may not read are removed. Requires " + engine.PermRead + ".",
		Input: objectSchema(nil), Annotations: readOnly("Automation catalog"), Policy: ReadPolicy,
	}, t.catalog))
	add(newTool(t.o, mcp.Spec{
		Name:        ToolList,
		Description: "List automation graphs of your tenant, most recently changed first. Requires " + engine.PermRead + ".",
		Input: objectSchema(map[string]*jsonschema.Schema{
			"query":  {Type: "string", MaxLength: jsonschema.Ptr(200), Description: "A substring of the name."},
			"status": {Type: "string", Enum: []any{"enabled", "disabled", "attention"}, Description: "attention: graphs with failed or blocked runs."},
			"cursor": {Type: "string", Description: "next_cursor of the previous page."},
			"limit":  {Type: "integer", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(float64(MaxPageLimit))},
		}),
		Annotations: readOnly("List automations"), Policy: ReadPolicy,
	}, t.list))
	add(newTool(t.o, mcp.Spec{
		Name: ToolGet,
		Description: "Read a graph: a version (the latest by default), its layout, the versions and the editor URL. revision is " +
			"the expected_revision of automation.update, layout_revision the one of automation.layout. Requires " + engine.PermRead + ".",
		Input: objectSchema(map[string]*jsonschema.Schema{
			"id":      {Type: "string", Format: "uuid"},
			"version": {Type: "integer", Minimum: jsonschema.Ptr(1.0)},
		}, "id"),
		Annotations: readOnly("Get automation"), Policy: ReadPolicy,
	}, t.get))
	add(newTool(t.o, mcp.Spec{
		Name:        ToolRuns,
		Description: "List runs of a graph, newest first. Requires " + engine.PermRead + ".",
		Input: objectSchema(map[string]*jsonschema.Schema{
			"id":     {Type: "string", Format: "uuid"},
			"status": {Type: "array", Items: &jsonschema.Schema{Type: "string", Enum: runStatuses()}},
			"cursor": {Type: "string"},
			"limit":  {Type: "integer", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(float64(MaxPageLimit))},
		}, "id"),
		Annotations: readOnly("List automation runs"), Policy: ReadPolicy,
	}, t.runs))
	add(newTool(t.o, mcp.Spec{
		Name: ToolRunGet,
		Description: "Read a run with its tokens, steps and waits. Steps that derive from data you may not read are hidden. " +
			"Requires " + engine.PermRead + ".",
		Input:       objectSchema(map[string]*jsonschema.Schema{"run_id": {Type: "string", Format: "uuid"}}, "run_id"),
		Annotations: readOnly("Get automation run"), Policy: ReadPolicy,
	}, t.runGet))
	add(newTool(t.o, mcp.Spec{
		Name: ToolValidate,
		Description: "Check a graph against your catalog as saving would, without writing anything. Requires " +
			engine.PermAuthor + ".",
		Input:       objectSchema(map[string]*jsonschema.Schema{"graph": {Type: "object"}}, "graph"),
		Annotations: readOnly("Validate automation"), Policy: AuthorPolicy,
	}, t.validate))
	add(newTool(t.o, mcp.Spec{
		Name: ToolDryRun,
		Description: "Run a graph (given, or a saved version by id) over a sequence of events without persisting a run or " +
			"making effects: effects are captured and calls are answered from context. decisions: scripted (default), cached " +
			"(decision nodes answered from the decision cache, else from context) or live. Real calls happen only with " +
			"allow_live: true, only for decision nodes (live, or cache misses), at most " + strconv.Itoa(MaxLiveCalls) +
			"; more is limit_exceeded. At most 50 events, the entries of a batch counted one by one. Requires " + engine.PermAuthor + ".",
		Input: dryRunSchema(), Annotations: readOnly("Dry-run automation"), Policy: AuthorPolicy,
	}, t.dryRun))
	if len(errs) > 0 {
		return nil, serrors.Wrap(op, errors.Join(errs...))
	}
	return out, nil
}

type tools struct {
	svc  engine.Service
	auth engine.Authoring
	reg  catalog.Registry
	urls URLs
	o    options
}

// newTool builds a typed tool with an explicit input schema and the output schema inferred with the
// JSON shapes of raw documents and UUIDs.
func newTool[In, Out any](o options, spec mcp.Spec, h func(context.Context, engine.Actor, In) (Out, error)) (mcp.Tool, error) {
	out, err := schemaOf[Out]()
	if err != nil {
		return nil, serrors.WrapContext(serrors.Op("mcpops.newTool"), err, "output schema of "+spec.Name)
	}
	spec.Output = out
	return mcp.New(spec, func(ctx context.Context, in In) (Out, error) {
		var zero Out
		a, _, err := o.actorOf(ctx)
		if err != nil {
			return zero, err
		}
		res, err := h(ctx, a, in)
		if err != nil {
			return zero, failure(err)
		}
		return res, nil
	})
}

func schemaOf[T any]() (*jsonschema.Schema, error) {
	anything := &jsonschema.Schema{}
	return jsonschema.For[T](&jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[json.RawMessage](): anything,
		reflect.TypeFor[uuid.UUID]():       {Type: "string", Format: "uuid"},
		reflect.TypeFor[time.Time]():       {Type: "string", Format: "date-time"},
	}})
}

func objectSchema(props map[string]*jsonschema.Schema, required ...string) *jsonschema.Schema {
	if props == nil {
		props = map[string]*jsonschema.Schema{}
	}
	return &jsonschema.Schema{Type: "object", Properties: props, Required: required, AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}}}
}

func runStatuses() []any {
	return []any{string(engine.RunQueued), string(engine.RunRunning), string(engine.RunWaiting), string(engine.RunBlocked),
		string(engine.RunDone), string(engine.RunFailed), string(engine.RunCancelled), string(engine.RunFilterFailed)}
}

func pageLimit(n int) int {
	if n <= 0 || n > MaxPageLimit {
		return MaxPageLimit
	}
	return n
}

func timeString(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func cursorOut(next string) *string {
	if next == "" {
		return nil
	}
	return &next
}

// ---- catalog ----

type catalogOut struct {
	Revision string          `json:"revision"`
	Events   []eventOut      `json:"events"`
	Nodes    []nodeOut       `json:"nodes"`
	Format   json.RawMessage `json:"format"`
}

type eventOut struct {
	Key           string          `json:"key"`
	Title         string          `json:"title"`
	Description   string          `json:"description,omitempty"`
	Group         string          `json:"group"`
	Subject       string          `json:"subject"`
	Payload       json.RawMessage `json:"payload"`
	TriggerParams json.RawMessage `json:"trigger_params"`
}

type nodeOut struct {
	Type        string            `json:"type"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Behavior    string            `json:"behavior"`
	Params      json.RawMessage   `json:"params"`
	Outcomes    []string          `json:"outcomes"`
	OutcomeRule graph.OutcomeRule `json:"outcome_rule"`
	Output      json.RawMessage   `json:"output"`
	Connector   *string           `json:"connector"`
	DryRun      string            `json:"dry_run"`
}

func orNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}

func (t *tools) catalog(ctx context.Context, a engine.Actor, _ struct{}) (catalogOut, error) {
	view, err := t.reg.ForTenant(ctx, a.TenantID, catalog.AsAuthor(a.Subject), catalog.WithLocale(a.Subject.Locale))
	if err != nil {
		return catalogOut{}, err
	}
	cv := view.JSON(t.o.behaviors)
	out := catalogOut{Revision: cv.Revision, Events: make([]eventOut, 0, len(cv.Events)), Nodes: make([]nodeOut, 0, len(cv.Nodes)), Format: graph.JSONSchema()}
	for _, e := range cv.Events {
		out.Events = append(out.Events, eventOut{Key: e.Key, Title: e.Title, Description: e.Description, Group: e.Group,
			Subject: e.Subject, Payload: orNull(e.Payload), TriggerParams: orNull(e.TriggerParams)})
	}
	for _, n := range cv.Nodes {
		no := nodeOut{Type: n.Type, Title: n.Title, Description: n.Description, Behavior: string(n.Behavior),
			Params: orNull(n.Params), Outcomes: append([]string{}, n.OutcomeRule.Fixed...), OutcomeRule: n.OutcomeRule,
			Output: orNull(n.Output), DryRun: string(n.DryRun)}
		if len(n.Requires) > 0 {
			c := n.Requires[0]
			no.Connector = &c
		}
		out.Nodes = append(out.Nodes, no)
	}
	return out, nil
}

// ---- list and get ----

type listIn struct {
	Query  string `json:"query"`
	Status string `json:"status"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

type listItem struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Enabled        bool   `json:"enabled"`
	EnabledVersion *int   `json:"enabled_version"`
	LatestVersion  int    `json:"latest_version"`
	UpdatedAt      string `json:"updated_at"`
}

type listOut struct {
	Items      []listItem `json:"items"`
	NextCursor *string    `json:"next_cursor"`
}

func (t *tools) list(ctx context.Context, a engine.Actor, in listIn) (listOut, error) {
	f := engine.ListFilter{Query: in.Query, Cursor: in.Cursor, Limit: pageLimit(in.Limit)}
	switch in.Status {
	case "enabled", "disabled":
		enabled := in.Status == "enabled"
		f.Enabled = &enabled
	case "attention":
		f.Attention = true
	}
	page, err := t.svc.List(ctx, a, f)
	if err != nil {
		return listOut{}, err
	}
	out := listOut{Items: make([]listItem, 0, len(page.Items)), NextCursor: cursorOut(page.Next)}
	for _, am := range page.Items {
		out.Items = append(out.Items, listItem{ID: am.ID.String(), Name: am.Name, Enabled: am.EnabledVersion != nil,
			EnabledVersion: am.EnabledVersion, LatestVersion: am.LatestVersion, UpdatedAt: timeString(am.UpdatedAt)})
	}
	return out, nil
}

type getIn struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type versionOut struct {
	Version     int     `json:"version"`
	SavedOrigin string  `json:"saved_origin"`
	Note        *string `json:"note"`
	CreatedAt   string  `json:"created_at"`
}

type getOut struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Enabled        bool            `json:"enabled"`
	EnabledVersion *int            `json:"enabled_version"`
	Version        int             `json:"version"`
	SavedOrigin    string          `json:"saved_origin"`
	Revision       string          `json:"revision"`
	LayoutRevision string          `json:"layout_revision"`
	Graph          json.RawMessage `json:"graph"`
	Layout         graph.Layout    `json:"layout"`
	Versions       []versionOut    `json:"versions"`
	EditorURL      string          `json:"editor_url"`
}

func (t *tools) get(ctx context.Context, a engine.Actor, in getIn) (getOut, error) {
	id, err := parseID("id", in.ID)
	if err != nil {
		return getOut{}, err
	}
	am, err := t.svc.Get(ctx, a, id)
	if err != nil {
		return getOut{}, err
	}
	version := in.Version
	if version == 0 {
		version = am.LatestVersion
	}
	v, err := t.svc.GetVersion(ctx, a, id, version)
	if err != nil {
		return getOut{}, err
	}
	infos, err := t.svc.ListVersions(ctx, a, id)
	if err != nil {
		return getOut{}, err
	}
	out := getOut{ID: am.ID.String(), Name: am.Name, Enabled: am.EnabledVersion != nil, EnabledVersion: am.EnabledVersion,
		Version: v.Version, SavedOrigin: string(v.SavedOrigin), Revision: strconv.Itoa(am.LatestVersion),
		LayoutRevision: strconv.Itoa(am.LayoutRevision), Graph: v.Graph, Layout: am.Layout,
		Versions: make([]versionOut, 0, len(infos)), EditorURL: t.urls.Editor(am.ID, v.Version)}
	if out.Layout == nil {
		out.Layout = graph.Layout{}
	}
	for _, vi := range infos {
		vo := versionOut{Version: vi.Version, SavedOrigin: string(vi.SavedOrigin), CreatedAt: timeString(vi.CreatedAt)}
		if vi.Note != "" {
			note := vi.Note
			vo.Note = &note
		}
		out.Versions = append(out.Versions, vo)
	}
	return out, nil
}

// ---- runs ----

type runsIn struct {
	ID     string   `json:"id"`
	Status []string `json:"status"`
	Cursor string   `json:"cursor"`
	Limit  int      `json:"limit"`
}

type runItem struct {
	RunID     string  `json:"run_id"`
	Version   int     `json:"version"`
	Status    string  `json:"status"`
	Reason    *string `json:"reason"`
	TriggerID string  `json:"trigger_id"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type runsOut struct {
	Items      []runItem `json:"items"`
	NextCursor *string   `json:"next_cursor"`
}

func (t *tools) runs(ctx context.Context, a engine.Actor, in runsIn) (runsOut, error) {
	id, err := parseID("id", in.ID)
	if err != nil {
		return runsOut{}, err
	}
	if _, err := t.svc.Get(ctx, a, id); err != nil {
		return runsOut{}, err
	}
	f := engine.RunFilter{AutomationID: &id, Cursor: in.Cursor, Limit: pageLimit(in.Limit)}
	for _, s := range in.Status {
		f.Status = append(f.Status, engine.RunStatus(s))
	}
	page, err := t.svc.ListRuns(ctx, a, f)
	if err != nil {
		return runsOut{}, err
	}
	out := runsOut{Items: make([]runItem, 0, len(page.Items)), NextCursor: cursorOut(page.Next)}
	for _, r := range page.Items {
		out.Items = append(out.Items, runItemOf(r))
	}
	return out, nil
}

func runItemOf(r engine.RunSummary) runItem {
	it := runItem{RunID: r.RunID.String(), Version: r.Version, Status: string(r.Status), TriggerID: r.TriggerID,
		CreatedAt: timeString(r.CreatedAt), UpdatedAt: timeString(r.UpdatedAt)}
	if r.Reason != "" {
		reason := r.Reason
		it.Reason = &reason
	}
	return it
}

type runGetIn struct {
	RunID string `json:"run_id"`
}

type tokenOut struct {
	ID           string  `json:"id"`
	Parent       *string `json:"parent"`
	NodeID       string  `json:"node_id"`
	Status       string  `json:"status"`
	Branch       string  `json:"branch,omitempty"`
	ForeachIndex *int    `json:"foreach_index,omitempty"`
}

type stepOut struct {
	TokenID   string            `json:"token_id"`
	Seq       int               `json:"seq"`
	NodeID    string            `json:"node_id"`
	Attempt   int               `json:"attempt"`
	Input     json.RawMessage   `json:"input,omitempty"`
	Output    json.RawMessage   `json:"output,omitempty"`
	Outcome   string            `json:"outcome,omitempty"`
	Error     *engine.StepError `json:"error,omitempty"`
	StartedAt string            `json:"started_at"`
	EndedAt   *string           `json:"ended_at"`
	Warnings  []string          `json:"warnings,omitempty"`
	Hidden    bool              `json:"hidden,omitempty"`
}

type waitOut struct {
	TokenID   string  `json:"token_id"`
	WaitID    string  `json:"wait_id"`
	Kind      string  `json:"kind"`
	EventKind string  `json:"event_kind,omitempty"`
	TimeoutAt *string `json:"timeout_at"`
	Status    string  `json:"status"`
}

type runGetOut struct {
	runItem
	AutomationID string            `json:"automation_id"`
	TriggerKind  string            `json:"trigger_kind"`
	Subject      graph.Subject     `json:"subject"`
	CurrentNodes []string          `json:"current_nodes"`
	Error        *engine.StepError `json:"error"`
	Tokens       []tokenOut        `json:"tokens"`
	Steps        []stepOut         `json:"steps"`
	Waits        []waitOut         `json:"waits"`
	InstanceID   *string           `json:"instance_id"`
	EditorURL    string            `json:"editor_url"`
}

func timePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := timeString(*t)
	return &s
}

func (t *tools) runGet(ctx context.Context, a engine.Actor, in runGetIn) (runGetOut, error) {
	id, err := parseID("run_id", in.RunID)
	if err != nil {
		return runGetOut{}, err
	}
	d, err := t.svc.GetRun(ctx, a, id)
	if err != nil {
		return runGetOut{}, err
	}
	out := runGetOut{runItem: runItemOf(d.RunSummary), AutomationID: d.AutomationID.String(), TriggerKind: d.TriggerKind,
		Subject: d.Subject, CurrentNodes: append([]string{}, d.CurrentNodes...), Error: d.Error,
		Tokens: make([]tokenOut, 0, len(d.Tokens)), Steps: make([]stepOut, 0, len(d.Steps)), Waits: make([]waitOut, 0, len(d.Waits)),
		EditorURL: t.urls.Run(d.AutomationID, d.RunID)}
	if d.InstanceID != nil {
		inst := d.InstanceID.String()
		out.InstanceID = &inst
	}
	for _, tk := range d.Tokens {
		to := tokenOut{ID: tk.ID.String(), NodeID: tk.NodeID, Status: tk.Status, Branch: tk.Branch, ForeachIndex: tk.ForeachIndex}
		if tk.Parent != nil {
			p := tk.Parent.String()
			to.Parent = &p
		}
		out.Tokens = append(out.Tokens, to)
	}
	for _, s := range d.Steps {
		out.Steps = append(out.Steps, stepOut{TokenID: s.TokenID.String(), Seq: s.Seq, NodeID: s.NodeID, Attempt: s.Attempt,
			Input: s.Input, Output: s.Output, Outcome: s.Outcome, Error: s.Error, StartedAt: timeString(s.StartedAt),
			EndedAt: timePtr(s.EndedAt), Warnings: s.Warnings, Hidden: s.Hidden})
	}
	for _, w := range d.Waits {
		out.Waits = append(out.Waits, waitOut{TokenID: w.TokenID.String(), WaitID: w.WaitID, Kind: w.Kind, EventKind: w.EventKind,
			TimeoutAt: timePtr(w.TimeoutAt), Status: w.Status})
	}
	return out, nil
}

// ---- validate and dry run ----

type validateIn struct {
	Graph json.RawMessage `json:"graph"`
}

type validateOut struct {
	OK       bool               `json:"ok"`
	Errors   []graph.GraphError `json:"errors"`
	Warnings []graph.GraphError `json:"warnings"`
}

func (t *tools) validate(ctx context.Context, a engine.Actor, in validateIn) (validateOut, error) {
	if len(in.Graph) > MaxGraphBytes {
		return validateOut{}, tooLarge("/graph")
	}
	out := validateOut{OK: true, Errors: []graph.GraphError{}, Warnings: []graph.GraphError{}}
	err := t.auth.Validate(ctx, a, in.Graph)
	var ve *engine.ValidationError
	switch {
	case err == nil:
	case errors.As(err, &ve):
		out.OK = false
		out.Errors = append(out.Errors, ve.Errors...)
		out.Warnings = append(out.Warnings, ve.Warnings...)
	default:
		return validateOut{}, err
	}
	return out, nil
}

func dryRunSchema() *jsonschema.Schema {
	s := objectSchema(map[string]*jsonschema.Schema{
		"graph":   {Type: "object", Description: "A graph to run; exactly one of graph and id."},
		"id":      {Type: "string", Format: "uuid", Description: "A saved graph; version defaults to the latest."},
		"version": {Type: "integer", Minimum: jsonschema.Ptr(1.0)},
		"subject": objectSchema(map[string]*jsonschema.Schema{
			"entity": {Type: "string", MinLength: jsonschema.Ptr(1)},
			"key":    {Type: "string"},
		}, "entity", "key"),
		"context": {Type: "object", Description: "Overrides of nodes.<id>.output and subject fields; scripted answers of execute nodes."},
		"events": {Type: "array", Items: &jsonschema.Schema{Type: "object"},
			Description: `At most 50 events fed to triggers and waits: {kind, ...payload}, {"timeout": true} or {"events": [...]}; more is limit_exceeded.`},
		"decisions":  {Type: "string", Enum: []any{engine.DecisionsScripted, engine.DecisionsCached, engine.DecisionsLive}},
		"allow_live": {Type: "boolean", Description: "Lets decision nodes make real calls (required for decisions live); nothing else ever does."},
	}, "subject", "events")
	s.OneOf = []*jsonschema.Schema{{Required: []string{"graph"}}, {Required: []string{"id"}}}
	return s
}

type dryRunIn struct {
	Graph     json.RawMessage            `json:"graph"`
	ID        string                     `json:"id"`
	Version   int                        `json:"version"`
	Subject   graph.Subject              `json:"subject"`
	Context   map[string]json.RawMessage `json:"context"`
	Events    []json.RawMessage          `json:"events"`
	Decisions string                     `json:"decisions"`
	AllowLive bool                       `json:"allow_live"`
}

type traceOut struct {
	Node    string            `json:"node"`
	Token   string            `json:"token,omitempty"`
	Kind    string            `json:"kind"`
	Outcome string            `json:"outcome,omitempty"`
	Path    string            `json:"path,omitempty"`
	Message string            `json:"message,omitempty"`
	Status  string            `json:"status,omitempty"`
	Effects []json.RawMessage `json:"effects,omitempty"`
	Output  json.RawMessage   `json:"output,omitempty"`
}

type dryRunOut struct {
	Trace []traceOut `json:"trace"`
}

func (t *tools) dryRun(ctx context.Context, a engine.Actor, in dryRunIn) (dryRunOut, error) {
	if engine.DryRunEventCount(in.Events) > engine.MaxDryRunEvents {
		return dryRunOut{}, mcp.Fail(mcp.CodeLimitExceeded, mcp.RecoveryFixInput, "a dry run takes at most 50 events, the entries of a batch counted one by one")
	}
	if len(in.Graph) > MaxGraphBytes {
		return dryRunOut{}, tooLarge("/graph")
	}
	if in.Decisions == engine.DecisionsLive && !in.AllowLive {
		return dryRunOut{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "decisions live makes real calls: pass allow_live true",
			mcp.Issue{Path: "/allow_live", Code: "required", Message: "live decisions need allow_live: true"})
	}
	raw := in.Graph
	if in.ID != "" {
		id, err := parseID("id", in.ID)
		if err != nil {
			return dryRunOut{}, err
		}
		version := in.Version
		if version == 0 {
			am, err := t.svc.Get(ctx, a, id)
			if err != nil {
				return dryRunOut{}, err
			}
			version = am.LatestVersion
		}
		v, err := t.svc.GetVersion(ctx, a, id, version)
		if err != nil {
			return dryRunOut{}, err
		}
		raw = v.Graph
	}
	// The same limit for a saved graph as for one given inline.
	if len(raw) > MaxGraphBytes {
		return dryRunOut{}, tooLarge("/id")
	}
	dr := engine.DryRunInput{Graph: raw, Subject: in.Subject, Context: in.Context, Events: in.Events, Decisions: in.Decisions,
		RealCalls: engine.RealCallsNone}
	if in.AllowLive {
		// Only decision nodes call for real, at most MaxLiveCalls times; every other execute node is
		// answered from context, as a captured one.
		dr.RealCalls, dr.MaxCalls = engine.RealCallsDecisions, MaxLiveCalls
	}
	res, err := t.auth.DryRun(ctx, a, dr)
	if err != nil {
		return dryRunOut{}, err
	}
	out := dryRunOut{Trace: make([]traceOut, 0, len(res.Trace))}
	for _, s := range res.Trace {
		out.Trace = append(out.Trace, traceOut{Node: s.Node, Token: s.Token, Kind: s.Kind, Outcome: s.Outcome, Path: s.Path,
			Message: s.Message, Status: s.Status, Effects: s.Effects, Output: s.Output})
	}
	return out, nil
}
