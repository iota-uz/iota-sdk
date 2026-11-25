package openai

import (
	"context"
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/internal/aichat"
)

var (
	_ connectors.CodeAccessor = (*connector)(nil)
	_ connectors.CodeOperator = (*connector)(nil)
)

// CodeAccess offers the operation complete: a text or JSON answer of a model of the connection. The key is
// the connector's: code never gets it, and there are no HTTP methods.
func (c *connector) CodeAccess() connectors.CodeAccess {
	return connectors.CodeAccess{Operations: []connectors.CodeOperation{aichat.CompleteOperation()}}
}

// Authorize sets nothing: the operation calls the provider itself.
func (c *connector) Authorize(context.Context, connectors.Credentials, http.Header) error { return nil }

// CallCodeOperation serves complete as the ai.llm node does: the connection's endpoint and key, the default
// model, the usage recorded on the connection, a refused key marking the connection needs_reauth.
func (c *connector) CallCodeOperation(ctx context.Context, call connectors.CodeCall) (any, error) {
	if call.Operation != aichat.OpComplete {
		return nil, &connectors.CodeOperationError{Code: "unknown_operation", Message: "the connection has no operation " + call.Operation}
	}
	ep, err := endpointOf(call.Input.Credentials, call.Input.Connection != nil && call.Input.Connection.Platform)
	if err != nil {
		return nil, &connectors.CodeOperationError{Code: "provider_refused", Message: err.Error()}
	}
	return aichat.CodeCompleter{
		Client: &aichat.Client{Transport: c.o.transport}, Log: c.o.log, Now: c.o.now,
		DefaultModel: c.o.defaultModel, Unauthorized: c.markNeedsReauth,
	}.Complete(ctx, ep, call)
}
