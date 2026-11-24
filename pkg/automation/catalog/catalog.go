// Package catalog is where modules and applications register the node types, trigger sources and
// connectors of automation graphs, and where the catalog of a tenant is built from them: only what the
// tenant has a working connection for and what the author may use.
//
// The package depends on pkg/automation/graph, pkg/automation/expr and the native-event interface; it
// does not import SDK modules, the engine or pkg/automation/connectors (connectors imports catalog).
package catalog

import (
	"context"
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

// CallNode is a node with an external effect (behavior call). The engine calls Execute outside of a
// locked step.
type CallNode interface {
	NodeType
	Execute(ctx context.Context, in CallInput) (CallResult, error)
	// Idempotent reports whether the engine may repeat a call automatically with the same IdempotencyKey.
	Idempotent() bool
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
	Subject        graph.Subject
	RunAs          spi.Subject     // a node acting in the SDK checks rights itself
	Connection     *ConnectionInfo // nil: the node needs no connector
	Credentials    Credentials     // decrypted by the engine; the node neither keeps nor logs them
	Deadline       time.Time
	Usage          UsageRecorder
}

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
