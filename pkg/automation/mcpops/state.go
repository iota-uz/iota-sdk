package mcpops

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/automation/state"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/changes"
)

// The memory of a graph between its runs (the keys of set_state and state.get): two read tools and two
// guarded changes. They are offered when the engine's Memory is given (WithMemory).
//
//	automation_state_list   automation.state.list   keys of a graph, with a search      Automations.Read
//	automation_state_get    automation.state.get    one key                              Automations.Read
//	automation.state.set    a change: write a key                                         Automations.Author
//	automation.state.delete a change: delete a key                                        Automations.Author
//
// Tool names allow no dots, so the reads are automation_state_list and automation_state_get; the changes
// are the entity "automation" with the ops state.set and state.delete.
const (
	ToolStateList = "automation_state_list"
	ToolStateGet  = "automation_state_get"

	OpStateSet    = "state.set"
	OpStateDelete = "state.delete"
)

// WithMemory offers the operations on the memory of graphs: the tools automation_state_list and
// automation_state_get, and the changes automation.state.set and automation.state.delete.
func WithMemory(m engine.Memory) Option { return func(o *options) { o.memory = m } }

type stateEntryOut struct {
	Key          string          `json:"key"`
	Value        json.RawMessage `json:"value"`
	ExpiresAt    *string         `json:"expires_at"`
	UpdatedAt    string          `json:"updated_at"`
	UpdatedByRun *string         `json:"updated_by_run"`
}

func stateEntryOf(e state.Entry) stateEntryOut {
	out := stateEntryOut{Key: e.Key, Value: orNull(e.Value), UpdatedAt: timeString(e.UpdatedAt), ExpiresAt: timePtr(e.ExpiresAt)}
	if e.UpdatedByRun != nil {
		s := e.UpdatedByRun.String()
		out.UpdatedByRun = &s
	}
	return out
}

type stateListIn struct {
	ID     string `json:"id"`
	Search string `json:"search"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

type stateListOut struct {
	Items      []stateEntryOut `json:"items"`
	NextCursor *string         `json:"next_cursor"`
	// Total is how many keys the graph holds now, the search ignored.
	Total int `json:"total"`
}

type stateGetIn struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

func (t *tools) addState(add func(mcp.Tool, error), readOnly func(string) mcp.Annotations) {
	if t.o.memory == nil {
		return
	}
	add(newTool(t.o, mcp.Spec{
		Name: ToolStateList,
		Description: "List the keys of a graph's memory between runs (what its set_state nodes keep and state.get reads), in key " +
			"order, with their values and expiry. Requires " + engine.PermRead + ".",
		Input: objectSchema(map[string]*jsonschema.Schema{
			"id":     {Type: "string", Format: "uuid"},
			"search": {Type: "string", MaxLength: jsonschema.Ptr(200), Description: "A substring of the key."},
			"cursor": {Type: "string", Description: "next_cursor of the previous page."},
			"limit":  {Type: "integer", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(float64(MaxPageLimit))},
		}, "id"),
		Annotations: readOnly("List automation state"), Policy: ReadPolicy,
	}, t.stateList))
	add(newTool(t.o, mcp.Spec{
		Name:        ToolStateGet,
		Description: "Read one key of a graph's memory; not_found when it is absent or has expired. Requires " + engine.PermRead + ".",
		Input: objectSchema(map[string]*jsonschema.Schema{
			"id":  {Type: "string", Format: "uuid"},
			"key": {Type: "string", MinLength: jsonschema.Ptr(1), MaxLength: jsonschema.Ptr(state.MaxKeyChars)},
		}, "id", "key"),
		Annotations: readOnly("Get automation state"), Policy: ReadPolicy,
	}, t.stateGet))
}

func (t *tools) stateList(ctx context.Context, a engine.Actor, in stateListIn) (stateListOut, error) {
	id, err := parseID("id", in.ID)
	if err != nil {
		return stateListOut{}, err
	}
	page, err := t.o.memory.List(ctx, a, id, state.Filter{Search: in.Search, After: in.Cursor, Limit: pageLimit(in.Limit)})
	if err != nil {
		return stateListOut{}, err
	}
	out := stateListOut{Items: make([]stateEntryOut, 0, len(page.Items)), NextCursor: cursorOut(page.Next), Total: page.Total}
	for _, e := range page.Items {
		out.Items = append(out.Items, stateEntryOf(e))
	}
	return out, nil
}

func (t *tools) stateGet(ctx context.Context, a engine.Actor, in stateGetIn) (stateEntryOut, error) {
	id, err := parseID("id", in.ID)
	if err != nil {
		return stateEntryOut{}, err
	}
	e, err := t.o.memory.Get(ctx, a, id, in.Key)
	if err != nil {
		return stateEntryOut{}, err
	}
	return stateEntryOf(e), nil
}

// ---- automation.state.set and automation.state.delete ----

// stateKinds are the changes of the memory; empty without WithMemory.
func stateKinds(b base) []changes.Kind {
	if b.o.memory == nil {
		return nil
	}
	return []changes.Kind{stateSetKind{b}, stateDeleteKind{b}}
}

type stateFields struct {
	ID    string          `json:"id"`
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
	TTL   string          `json:"ttl"`
}

func decodeStateFields(raw json.RawMessage) (stateFields, []mcp.Issue) {
	var f stateFields
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, []mcp.Issue{{Path: "/fields", Code: string(mcp.CodeInvalidInput), Message: "fields do not match the schema"}}
	}
	return f, nil
}

func (f stateFields) ttl() (time.Duration, bool) {
	if f.TTL == "" {
		return 0, true
	}
	ms, ok := graph.DurationMs(f.TTL)
	return time.Duration(ms) * time.Millisecond, ok
}

// stateIssues are what is wrong with the id, the key, the value and the ttl of a change, without reading.
func stateIssues(f stateFields, withValue bool) (uuid.UUID, []mcp.Issue) {
	var issues []mcp.Issue
	id, ok := parseUUID(f.ID)
	if !ok {
		issues = append(issues, mcp.Issue{Path: "/fields/id", Code: "uuid", Message: "id must be a UUID"})
	}
	if err := state.ValidateKey(f.Key); err != nil {
		issues = append(issues, stateIssue("/fields/key", err))
	}
	if withValue {
		if len(f.Value) == 0 {
			issues = append(issues, mcp.Issue{Path: "/fields/value", Code: "required", Message: "a value is required (JSON, null included)"})
		} else if _, err := state.Encode(f.Value); err != nil {
			issues = append(issues, stateIssue("/fields/value", err))
		}
		ttl, ok := f.ttl()
		switch {
		case !ok:
			issues = append(issues, mcp.Issue{Path: "/fields/ttl", Code: state.CodeTTLInvalid, Message: "ttl is a duration like 20m, 2h or 7d"})
		default:
			if err := state.ValidateTTL(ttl); err != nil {
				issues = append(issues, stateIssue("/fields/ttl", err))
			}
		}
	}
	return id, issues
}

func stateIssue(path string, err error) mcp.Issue {
	var se *state.Error
	if errors.As(err, &se) {
		return mcp.Issue{Path: path, Code: se.Code, Message: se.Message}
	}
	return mcp.Issue{Path: path, Code: string(mcp.CodeInvalidInput), Message: err.Error()}
}

type stateResult struct {
	ID        string  `json:"id"`
	Key       string  `json:"key"`
	Deleted   bool    `json:"deleted"`
	ExpiresAt *string `json:"expires_at"`
	EditorURL string  `json:"editor_url"`
}

// current is the live value of a key for a plan's diff: Absent when it is not there.
func (b base) current(ctx context.Context, a engine.Actor, id uuid.UUID, key string) (fielddiff.FieldValue, bool, error) {
	e, err := b.o.memory.Get(ctx, a, id, key)
	switch {
	case errors.Is(err, engine.ErrNotFound):
		return fielddiff.Absent(), false, nil
	case err != nil:
		return fielddiff.Absent(), false, failure(err)
	}
	return fielddiff.Present(e.Value), true, nil
}

type stateSetKind struct{ base }

func (stateSetKind) Op() string     { return OpStateSet }
func (stateSetKind) Existing() bool { return false }
func (stateSetKind) Description() string {
	return "Write a key of a graph's memory between runs: {id, key, value, ttl}. value is any JSON (at most 16 KiB), ttl is " +
		"optional (20m, 2h, 7d: from 1 minute to 365 days; without it the key stays). A key written replaces the old value."
}

func (stateSetKind) Fields() *jsonschema.Schema {
	return objectSchema(map[string]*jsonschema.Schema{
		"id":    {Type: "string", Format: "uuid"},
		"key":   {Type: "string", MinLength: jsonschema.Ptr(1), MaxLength: jsonschema.Ptr(state.MaxKeyChars)},
		"value": {Description: "Any JSON value, at most 16 KiB."},
		"ttl":   {Type: "string", Pattern: "^[1-9][0-9]{0,6}(m|h|d)$", Description: "20m, 2h or 7d; from 1 minute to 365 days."},
	}, "id", "key", "value")
}

func (k stateSetKind) Plan(ctx context.Context, c changes.Change) (changes.Planned, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return changes.Planned{}, err
	}
	f, issues := decodeStateFields(c.Fields)
	if len(issues) > 0 {
		return changes.Planned{Errors: issues}, nil
	}
	id, issues := stateIssues(f, true)
	if len(issues) > 0 {
		return changes.Planned{Errors: issues}, nil
	}
	if _, err := k.svc.Get(ctx, a, id); errors.Is(err, engine.ErrNotFound) {
		return changes.Planned{Errors: notFound()}, nil
	} else if err != nil {
		return changes.Planned{}, failure(err)
	}
	before, _, err := k.current(ctx, a, id, f.Key)
	if err != nil {
		return changes.Planned{}, err
	}
	p := changes.Planned{Diff: []fielddiff.Diff{{Field: "state." + f.Key, Before: before, After: fielddiff.Present(f.Value)}}}
	if f.TTL != "" {
		p.Diff = append(p.Diff, fielddiff.Diff{Field: "state." + f.Key + ".ttl", Before: fielddiff.Absent(), After: fielddiff.Present(marshal(f.TTL))})
	}
	return p, nil
}

func (stateSetKind) Revision(context.Context, string) (string, error) { return "", nil }

func (k stateSetKind) Apply(ctx context.Context, c changes.Change) (changes.Applied, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return changes.Applied{}, err
	}
	f, issues := decodeStateFields(c.Fields)
	id, more := stateIssues(f, true)
	if len(issues) > 0 || len(more) > 0 {
		return changes.Applied{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryReadThenPrepare, "the change is not applicable")
	}
	ttl, _ := f.ttl()
	e, err := k.o.memory.Set(joined(ctx), a, id, f.Key, f.Value, ttl)
	if err != nil {
		return changes.Applied{}, failure(err)
	}
	return changes.Applied{ID: id.String(), Revision: timeString(e.UpdatedAt),
		Result: marshal(stateResult{ID: id.String(), Key: e.Key, ExpiresAt: timePtr(e.ExpiresAt), EditorURL: k.urls.Editor(id, 0)})}, nil
}

type stateDeleteKind struct{ base }

func (stateDeleteKind) Op() string     { return OpStateDelete }
func (stateDeleteKind) Existing() bool { return false }
func (stateDeleteKind) Description() string {
	return "Delete a key of a graph's memory between runs: {id, key}. A key that is not there (or has expired) is an error."
}

func (stateDeleteKind) Fields() *jsonschema.Schema {
	return objectSchema(map[string]*jsonschema.Schema{
		"id":  {Type: "string", Format: "uuid"},
		"key": {Type: "string", MinLength: jsonschema.Ptr(1), MaxLength: jsonschema.Ptr(state.MaxKeyChars)},
	}, "id", "key")
}

func (k stateDeleteKind) Plan(ctx context.Context, c changes.Change) (changes.Planned, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return changes.Planned{}, err
	}
	f, issues := decodeStateFields(c.Fields)
	if len(issues) > 0 {
		return changes.Planned{Errors: issues}, nil
	}
	id, issues := stateIssues(f, false)
	if len(issues) > 0 {
		return changes.Planned{Errors: issues}, nil
	}
	if _, err := k.svc.Get(ctx, a, id); errors.Is(err, engine.ErrNotFound) {
		return changes.Planned{Errors: notFound()}, nil
	} else if err != nil {
		return changes.Planned{}, failure(err)
	}
	before, found, err := k.current(ctx, a, id, f.Key)
	if err != nil {
		return changes.Planned{}, err
	}
	if !found {
		return changes.Planned{Errors: []mcp.Issue{{Path: "/fields/key", Code: string(mcp.CodeNotFound), Message: "the key is not there"}}}, nil
	}
	return changes.Planned{Diff: []fielddiff.Diff{{Field: "state." + f.Key, Before: before, After: fielddiff.Absent()}}}, nil
}

func (stateDeleteKind) Revision(context.Context, string) (string, error) { return "", nil }

func (k stateDeleteKind) Apply(ctx context.Context, c changes.Change) (changes.Applied, error) {
	a, _, err := k.o.actorOf(ctx)
	if err != nil {
		return changes.Applied{}, err
	}
	f, issues := decodeStateFields(c.Fields)
	id, more := stateIssues(f, false)
	if len(issues) > 0 || len(more) > 0 {
		return changes.Applied{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryReadThenPrepare, "the change is not applicable")
	}
	if err := k.o.memory.Delete(joined(ctx), a, id, f.Key); err != nil {
		return changes.Applied{}, failure(err)
	}
	return changes.Applied{ID: id.String(), Revision: strings.TrimSpace(timeString(time.Now())),
		Result: marshal(stateResult{ID: id.String(), Key: f.Key, Deleted: true, EditorURL: k.urls.Editor(id, 0)})}, nil
}
