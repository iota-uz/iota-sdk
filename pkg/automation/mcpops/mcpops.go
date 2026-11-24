// Package mcpops is what an agent connected to the application's MCP does with the tenant's automations,
// within the rights of its user: it reads the catalog, graphs, versions and runs, validates a graph and
// dry-runs it (read-only tools of the shared registry, pkg/mcp), and creates and edits graphs through the
// guarded changes of pkg/mcp/changes (entity "automation").
//
// Every call acts as engine.Actor{Origin: agent}, set from the MCP principal, never from the model's
// arguments. A version an agent saves never runs until a person enables it. Enabling or disabling a
// graph, deciding a human task and managing connections are not offered: they answer HUMAN_REQUIRED (the
// shared failure contract approval_required/open_approval with issue code HUMAN_REQUIRED) and the URL of
// the page where a person acts. The engine marks automation.run and automation.call human-only
// (workflow.NativeContract.HumanOnly), so the extension runtime's workflow tools (pkg/extruntime/mcptools)
// refuse the same actions on their instances and tasks wherever they are registered.
//
// Composition:
//
//	tools, err := mcpops.Tools(rt.Service(), rt.Authoring(), catalogRegistry, urls)
//	registry.Register(tools...)
//	changes.NewService(changes.Config{Kinds: append(kinds, mcpops.Kinds(rt.Service(), rt.Authoring(), urls)...), ...})
//
// HUMAN_REQUIRED refusals of a plan and every applied change are entries of the MCP engine's audit
// (mcp.Audit), with the principal and the client. engine.Config.Links.RunURL is the run page the
// workflow tools point to.
package mcpops

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// OAuth scopes of the operations. They narrow RBAC and never widen it.
const (
	ScopeRead   = "automations:read"
	ScopeAuthor = "automations:author"
)

// Entity is the changes entity of graphs.
const Entity = "automation"

// IssueHumanRequired is the issue code of an action only a person may take.
const IssueHumanRequired = "HUMAN_REQUIRED"

// Limits of the operations.
const (
	// MaxGraphBytes bounds a graph in fields and in automation_validate / automation_dry_run.
	MaxGraphBytes = 1 << 20
	// MaxPageLimit bounds limit of the list tools.
	MaxPageLimit = 50
	// MaxLiveCalls bounds the real calls of one automation_dry_run (live decisions and cache misses).
	MaxLiveCalls = 20
)

// URLs are the trusted pages of #1339 a person acts on.
type URLs interface {
	// Editor is the editor of a graph's version (0: the latest).
	Editor(id uuid.UUID, version int) string
	// Run is the page of a run of a graph (the editor's Runs tab).
	Run(id, runID uuid.UUID) string
	// Automations is the list of graphs, where a person creates and enables them.
	Automations() string
	// Connections is the page where a person manages the tenant's connections.
	Connections() string
}

// SubjectResolver builds the current rights of the principal's user from the context the MCP
// authorizer returned. The default reads the user the authorizer put in the context
// (pkg/mcp/rbacauth).
type SubjectResolver func(ctx context.Context, p mcp.Principal) (spi.Subject, error)

// Option configures Tools and Kinds.
type Option func(*options)

type options struct {
	subjects  SubjectResolver
	behaviors *graph.Behaviors
}

// WithSubjects replaces how the acting user's rights are read.
func WithSubjects(r SubjectResolver) Option { return func(o *options) { o.subjects = r } }

// WithBehaviors are the behaviors graphs run with (the engine's Config.Behaviors); default
// graph.DefaultBehaviors.
func WithBehaviors(b *graph.Behaviors) Option { return func(o *options) { o.behaviors = b } }

func buildOptions(opts []Option) options {
	o := options{subjects: contextSubject}
	for _, f := range opts {
		f(&o)
	}
	return o
}

// contextSubject reads the user the authorizer put in the context, with its effective permissions.
func contextSubject(ctx context.Context, p mcp.Principal) (spi.Subject, error) {
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil || u.ID() != p.UserID || u.TenantID() != p.TenantID {
		return spi.Subject{}, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryReauthenticate, "the account is not available")
	}
	return spi.Subject{
		TenantID: p.TenantID, UserID: int64(p.UserID), Kind: spi.SubjectInitiator,
		Permissions: composables.EffectivePermissionNames(u), Locale: string(u.UILanguage()),
	}, nil
}

// actorOf is the agent acting for the principal's user: the origin is agent whatever the input says.
func (o options) actorOf(ctx context.Context) (engine.Actor, mcp.Principal, error) {
	p, ok := mcp.PrincipalFrom(ctx)
	if !ok || p.UserID == 0 {
		return engine.Actor{}, mcp.Principal{}, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryReauthenticate, "automations need a signed-in user")
	}
	sub, err := o.subjects(ctx, p)
	if err != nil {
		var f *mcp.Failure
		if errors.As(err, &f) {
			return engine.Actor{}, p, f
		}
		return engine.Actor{}, p, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryReauthenticate, "the account is not available")
	}
	client := p.ClientID
	if client == "" {
		client = "personal"
	}
	sub.TenantID, sub.UserID, sub.Credential = p.TenantID, int64(p.UserID), "mcp:"+client
	return engine.Actor{TenantID: p.TenantID, UserID: int64(p.UserID), Origin: engine.OriginAgent, Subject: sub}, p, nil
}

// auditHumanRequired puts a HUMAN_REQUIRED refusal of a plan in the MCP engine's audit.
func auditHumanRequired(ctx context.Context, op string) {
	mcp.Audit(ctx, "changes:"+Entity+"."+op, mcp.CodeApprovalRequired, "")
}

// HumanRequired is the refusal of an action only a person may take at url.
func HumanRequired(path, action, url string) *mcp.Failure {
	return mcp.Fail(mcp.CodeApprovalRequired, mcp.RecoveryOpenApproval, humanMessage(action, url),
		mcp.Issue{Path: path, Code: IssueHumanRequired, Message: humanMessage(action, url)})
}

func humanMessage(action, url string) string {
	msg := "only a person may " + action
	if url != "" {
		msg += "; open " + url
	}
	return msg
}

var graphIndex = regexp.MustCompile(`\[(\d+)\]`)

// pointer turns a graph error path ("nodes[2].params.expression", "graph") into a JSON pointer under
// base ("/fields/graph/nodes/2/params/expression").
func pointer(base, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "graph" {
		return base
	}
	path = graphIndex.ReplaceAllString(path, ".$1")
	parts := strings.Split(strings.Trim(path, "."), ".")
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~", "~0"), "/", "~1")
	}
	return base + "/" + strings.Join(parts, "/")
}

func graphIssues(base string, errs []graph.GraphError) []mcp.Issue {
	out := make([]mcp.Issue, 0, len(errs))
	for _, e := range errs {
		out = append(out, mcp.Issue{Path: pointer(base, e.Path), Code: e.Code, Message: e.Message})
	}
	return out
}

// failure maps an engine error onto the public contract; internal errors keep their messages inside.
func failure(err error) error {
	if err == nil {
		return nil
	}
	var f *mcp.Failure
	if errors.As(err, &f) {
		return f
	}
	var ve *engine.ValidationError
	var lc *engine.LayoutConflictError
	var vc *engine.ConflictError
	switch {
	case errors.As(err, &ve):
		return mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "the graph does not validate", graphIssues("/graph", ve.Errors)...)
	case errors.As(err, &lc):
		return mcp.Fail(mcp.CodeConflict, mcp.RecoveryReadThenPrepare,
			"the layout changed: its revision is "+strconv.Itoa(lc.Revision)+"; read the graph again",
			mcp.Issue{Path: "/fields/base_revision", Code: "VERSION_CONFLICT", Message: "current revision is " + strconv.Itoa(lc.Revision)})
	case errors.As(err, &vc):
		return mcp.Fail(mcp.CodeConflict, mcp.RecoveryReadThenPrepare,
			"another version was saved: the latest is "+strconv.Itoa(vc.Latest)+"; read the graph again",
			mcp.Issue{Path: "/expected_revision", Code: "VERSION_CONFLICT", Message: "current revision is " + strconv.Itoa(vc.Latest)})
	case errors.Is(err, engine.ErrNotFound):
		return mcp.Fail(mcp.CodeNotFound, mcp.RecoveryFixInput, "the automation or run is not found")
	case errors.Is(err, engine.ErrForbidden):
		return mcp.Fail(mcp.CodeForbidden, mcp.RecoveryRequestAccess, "access denied")
	case errors.Is(err, engine.ErrHumanRequired):
		return HumanRequired("", "do this", "")
	case errors.Is(err, engine.ErrLimitExceeded):
		return mcp.Fail(mcp.CodeLimitExceeded, mcp.RecoveryFixInput, publicMessage(err, "a limit of automations was exceeded"))
	case errors.Is(err, engine.ErrInvalidInput), errors.Is(err, engine.ErrConnectionUnavailable):
		return mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, publicMessage(err, "invalid input"))
	case errors.Is(err, engine.ErrNotEnabled), errors.Is(err, engine.ErrRunFinished), errors.Is(err, engine.ErrReviewRequired):
		return mcp.Fail(mcp.CodeConflict, mcp.RecoveryReadThenPrepare, publicMessage(err, "the automation is not in a state that allows this"))
	}
	if serrors.CodeOf(err) != serrors.Internal {
		return mcp.FailureFrom(err)
	}
	return mcp.Fail(mcp.CodeDependencyUnavailable, mcp.RecoveryRetryLater, "the operation could not be completed")
}

// publicMessage is the text of an engine sentinel error ("automation: invalid_input: cursor") without its
// package prefix; the engine writes these for authors.
func publicMessage(err error, fallback string) string {
	msg := strings.TrimPrefix(err.Error(), "automation: ")
	if msg == "" {
		return fallback
	}
	return msg
}

func parseID(path, v string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(v))
	if err != nil {
		return uuid.Nil, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, path+" must be a UUID",
			mcp.Issue{Path: "/" + path, Code: "uuid", Message: path + " must be a UUID"})
	}
	return id, nil
}

func tooLarge(path string) *mcp.Failure {
	return mcp.Fail(mcp.CodeLimitExceeded, mcp.RecoveryFixInput, "the graph exceeds 1 MiB",
		mcp.Issue{Path: path, Code: string(mcp.CodeLimitExceeded), Message: "a graph is at most 1 MiB"})
}

// nonNullRaw reports whether a JSON value is present and not null.
func nonNullRaw(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "null"
}
