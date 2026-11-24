// Package catalog is where modules and applications register the node types, trigger sources and
// connectors of automation graphs, and where the catalog of a tenant is built from them: only what the
// tenant has a working connection for and what the author may use.
//
// The package depends on pkg/automation/graph, pkg/automation/expr and the native-event interface; it
// does not import SDK modules, the engine or pkg/automation/connectors (connectors imports catalog).
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// ConnectorKind names a connector: "telegram_bot", "whatsapp_cloud", "smtp", "imap", "eskiz", "twilio",
// "openai", "jev", "mcp", "didox", or one an application adds.
type ConnectorKind string

// Connector kinds the SDK's own connectors use.
const (
	KindTelegramBot   ConnectorKind = "telegram_bot"
	KindWhatsAppCloud ConnectorKind = "whatsapp_cloud"
	KindSMTP          ConnectorKind = "smtp"
	KindIMAP          ConnectorKind = "imap"
	KindEskiz         ConnectorKind = "eskiz"
	KindTwilio        ConnectorKind = "twilio"
	KindOpenAI        ConnectorKind = "openai"
	KindJEV           ConnectorKind = "jev"
	KindMCP           ConnectorKind = "mcp"
	KindDidox         ConnectorKind = "didox"
)

// ConnectionParam is the reserved param a node or a trigger that needs a connector selects its
// connection with: a connection ID, or "platform:<kind>" for the platform credential. Left out, the one
// available connection of the needed kinds is used.
const ConnectionParam = "connection"

// PlatformPrefix starts the connection ID of a platform credential: "platform:<kind>".
const PlatformPrefix = "platform:"

// PlatformConnectionID is the connection ID of the platform credential of a kind.
func PlatformConnectionID(kind ConnectorKind) string { return PlatformPrefix + string(kind) }

// Reasons an entry is in View.Unavailable.
const (
	ReasonConnectionUnavailable = "connection_unavailable"
	ReasonPermission            = "permission"
)

// Graph error codes of ConnectionRule.
const (
	CodeConnectionUnavailable = "connection_unavailable"
	// CodeConnectionRequired: several connections fit and the node or trigger does not name one.
	CodeConnectionRequired = "connection_required"
)

// NodeType is a node type a module, an application or a connector contributes.
type NodeType interface {
	Spec() graph.NodeSpec      // behavior, params, outcomes, expressions, output
	Requires() []ConnectorKind // the node works with a connection of any of these kinds; empty: always available
}

// PermissionedNode is an optional interface of a NodeType: the RBAC permission an author needs to use the
// node. Without it the author's right to author automations (checked by the caller) is enough.
type PermissionedNode interface {
	Permission() string
}

// LocalizedNode is an optional interface of a NodeType: the spec in a locale. Without it Spec() is used
// as is.
type LocalizedNode interface {
	Localize(spec graph.NodeSpec, locale string) graph.NodeSpec
}

// PreparingNode is an optional interface of a NodeType whose params are checked against the outside
// world when a graph is saved, validated or checked before a dry run or a test: a connector's live data
// (the tools of an MCP server). Prepare may fill params in (a schema pinned when the node carries none);
// its errors refuse the graph, its warnings do not. It is never called while a run executes.
type PreparingNode interface {
	Prepare(ctx context.Context, in PrepareInput) (PrepareResult, error)
}

// PrepareInput is a node of a graph being checked.
type PrepareInput struct {
	TenantID uuid.UUID
	NodeID   string
	// Params are the node's params as the graph holds them (templates and expressions not evaluated),
	// without the connection param.
	Params map[string]any
	// Connection is the connection the node resolves to; nil when it needs none.
	Connection *ConnectionInfo
	// Author is who checks the graph; the node reads the outside world with the author's rights.
	Author spi.Subject
	// Fill: the graph is being saved or tried, so params may be filled in. Off (a stored version being
	// enabled), a node that would change its params reports it as an error instead.
	Fill bool
	// TypeOf is the CEL type of an expression in the node's scope ("string", "int", "list(dyn)", ...),
	// or its check's first issue: what a node compares with a schema it learns elsewhere.
	TypeOf func(src string) (string, error)
}

// PrepareResult is what Prepare found. Paths of issues are dotted paths inside the node's params
// ("tool", "arguments.city"); an empty path is the node itself.
type PrepareResult struct {
	// Params, when non-nil, replace the node's params (the connection param is kept by the caller).
	Params           map[string]any
	Errors, Warnings []PrepareIssue
}

// PrepareIssue is one finding of Prepare.
type PrepareIssue struct {
	Path    string
	Code    string
	Message string // no secrets
	// FixRemove is a dotted param path whose removal resolves the issue (the editor offers it, labelled
	// FixLabel); empty: no such action.
	FixRemove string
	FixLabel  string
}

// CallNode is a node with an external effect (behavior call). The engine calls Execute outside of a
// locked step.
type CallNode interface {
	NodeType
	Execute(ctx context.Context, in CallInput) (CallResult, error)
	// Idempotent reports whether the engine may repeat a call automatically with the same IdempotencyKey.
	Idempotent() bool
}

// IdempotentCall is an optional interface of a CallNode whose repeat safety depends on its params (an
// HTTP GET may be repeated, a POST without an idempotency header may not): the engine asks it instead of
// Idempotent. Params are the rendered params of the call.
type IdempotentCall interface {
	IdempotentWith(params map[string]any) bool
}

// IsIdempotent reports whether the engine may repeat a call of the node with these params.
func IsIdempotent(n CallNode, params map[string]any) bool {
	if ic, ok := n.(IdempotentCall); ok {
		return ic.IdempotentWith(params)
	}
	return n.Idempotent()
}

// TerminalUnknown is an optional interface of a CallNode whose call must never be repeated by anyone, an
// operator included (an SMS): its unknown outcome always ends as the failed outcome with code
// outcome_unknown, never blocks the call for an operator to retry it.
type TerminalUnknown interface {
	UnknownIsTerminal() bool
}

// CallInput is what a call node is executed with.
type CallInput struct {
	TenantID       uuid.UUID
	AutomationID   uuid.UUID
	RunID          uuid.UUID
	NodeID         string
	Attempt        int            // 1: the first attempt
	IdempotencyKey string         // stable between attempts: run:node:token:seq
	Params         map[string]any // templates and values are already evaluated
	// Context is the step's expression context (event, nodes, the subject's variables, subject, run,
	// tenant, now) for a node type that declares graph.NodeSpec.StepContext; nil otherwise.
	Context     map[string]any
	Subject     graph.Subject
	RunAs       spi.Subject     // a node acting in the SDK checks rights itself
	Connection  *ConnectionInfo // nil: the node needs no connector
	Credentials Credentials     // decrypted by the engine; the node neither keeps nor logs them
	Deadline    time.Time
	Usage       UsageRecorder
}

// CommandCall is an optional interface of a CallNode whose call is a command: the engine calls it with
// the workflow's call primitive (wf.call, NativeRun.CallAs) under the call's invocation ID (InvocationIDOf
// of its idempotency key), so the workflow journal and the host's invocation record keep its outcome and
// a call that may have run is never repeated blindly. Execute is not used for such a call.
type CommandCall interface {
	// Command is the command and its input for the call; a *CallError refuses the call before it is made.
	Command(in CallInput) (command string, input json.RawMessage, err error)
	// CommandResult is the call's result from the command's output; a *CallError fails it.
	CommandResult(in CallInput, output json.RawMessage) (CallResult, error)
}

// InvocationIDOf is the invocation ID (a UUID) a call node passes to SDK commands: derived from the
// call's idempotency key, so every attempt of one call is the same invocation.
func InvocationIDOf(idempotencyKey string) string {
	return uuid.NewSHA1(invocationNamespace, []byte(idempotencyKey)).String()
}

var invocationNamespace = uuid.MustParse("6f1d1f0e-8a8b-4c39-9b0e-3c1b2a7a5d10")

// Credentials are the decrypted fields of a connection. They never appear in a Connection, an error or
// a log.
type Credentials map[string]string

// String hides the values, so a Credentials printed by mistake shows only its field names.
func (c Credentials) String() string {
	return fmt.Sprintf("Credentials(%d fields)", len(c))
}

// GoString hides the values for %#v as well.
func (c Credentials) GoString() string { return c.String() }

// ConnectionInfo is the connection a call runs with.
type ConnectionInfo struct {
	ID       string // a UUID, or "platform:<kind>"
	Kind     ConnectorKind
	Name     string
	Platform bool
}

// UsageRecorder records what a call spent.
type UsageRecorder interface {
	Record(ctx context.Context, u UsageEntry) error
}

// UsageEntry is one record of spending.
type UsageEntry struct {
	TenantID     uuid.UUID
	Kind         ConnectorKind
	ConnectionID string // "platform:<kind>" for the platform credential
	Unit         string // "message", "token_in", "token_out", "request", "sms"
	Quantity     int64
	RunID        *uuid.UUID
	At           time.Time
}

// CallResult is the result of a call.
type CallResult struct {
	Outcome    string         // one of the node's outcomes
	Output     map[string]any // ≤ 1 MiB as JSON; checked against Output/OutputFrom
	RetryAfter time.Duration  // > 0: asks for a repeat (429) within the deadline
}

// Codes of a CallError.
const (
	CallInvalidRequest = "invalid_request"
	CallUnauthorized   = "unauthorized"
	CallRateLimited    = "rate_limited"
	CallUnavailable    = "unavailable"
	CallRejected       = "rejected"
	CallOutcomeUnknown = "outcome_unknown"
)

// CallError is a refusal of Execute with a code; any other error is an infrastructure error the engine
// retries by its rules.
type CallError struct {
	Code      string
	Message   string // no secrets
	Retryable bool
}

func (e *CallError) Error() string { return e.Code + ": " + e.Message }

// TriggerSource contributes events a graph may be triggered by (or wait for).
type TriggerSource interface {
	Events() []graph.EventSpec
	Requires() []ConnectorKind
}

// TenantConnectionNode is an optional interface of a NodeType: the node works only with a tenant's own
// connection, never with the platform credential (a wait for a message that only a tenant's own bot
// receives). The catalog neither offers nor accepts the platform credential for it.
type TenantConnectionNode interface {
	TenantConnectionOnly() bool
}

// TenantConnectionTriggers is an optional interface of a TriggerSource: events that only a tenant's own
// connection can deliver (a platform credential never receives them: a message to the platform bot belongs
// to no tenant). A graph that triggers on such an event is offered, and valid with, the tenant's own
// connections only.
type TenantConnectionTriggers interface {
	TenantConnectionOnly(eventKey string) bool
}

// EventAccess is who may see an event in the catalog: an author needs ReadPermission (empty: any
// author); a payload field listed in Sensitive (a dotted path) is removed from the schema of an author
// who lacks any of its permissions.
type EventAccess struct {
	ReadPermission string
	Sensitive      map[string][]string
}

// PermissionedTriggers is an optional interface of a TriggerSource: the access rule of each event.
type PermissionedTriggers interface {
	Access(eventKey string) (EventAccess, bool)
}

// LocalizedTriggers is an optional interface of a TriggerSource: an event's spec in a locale.
type LocalizedTriggers interface {
	Localize(spec graph.EventSpec, locale string) graph.EventSpec
}

// ConnectorContribution is what a connector adds to the catalog (connectors.Connector extends it). Its
// nodes and triggers require the connector's kind besides what they declare.
type ConnectorContribution interface {
	Kind() ConnectorKind
	Nodes() []NodeType
	Triggers() []TriggerSource
}

// Availability is which kinds a tenant has connected now (connectors.ConnectionStore implements it).
// Refs of a kind are in order of preference: tenant connections before the platform credential.
type Availability interface {
	Connected(ctx context.Context, tenantID uuid.UUID) (map[ConnectorKind][]ConnectionRef, error)
}

// ConnectionRef is a working connection: a UUID, or "platform:<kind>" for the platform credential.
type ConnectionRef struct {
	ID   string
	Name string
}
