package jev

import (
	"context"
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/internal/aichat"
)

// DefaultCodeModelOpenRouter is the model complete asks OpenRouter when the code names none: the typesafe/
// model of the ai.jev node is a classifier, not a chat model.
const DefaultCodeModelOpenRouter = "openai/gpt-6-luna"

var (
	_ connectors.CodeAccessor = (*connector)(nil)
	_ connectors.CodeOperator = (*connector)(nil)
)

// CodeAccess offers the operation complete: a text or JSON answer of a chat model of the connection's
// provider (OpenRouter or OpenAI). The key is never given to code.
func (c *connector) CodeAccess() connectors.CodeAccess {
	return connectors.CodeAccess{Operations: []connectors.CodeOperation{aichat.CompleteOperation()}}
}

// Authorize sets nothing: the operation calls the provider itself.
func (c *connector) Authorize(context.Context, connectors.Credentials, http.Header) error { return nil }

// CallCodeOperation serves complete over the chat completions of the connection's provider.
func (c *connector) CallCodeOperation(ctx context.Context, call connectors.CodeCall) (any, error) {
	if call.Operation != aichat.OpComplete {
		return nil, &connectors.CodeOperationError{Code: "unknown_operation", Message: "the connection has no operation " + call.Operation}
	}
	ep, err := endpointOf(call.Input.Credentials)
	if err != nil {
		return nil, &connectors.CodeOperationError{Code: "provider_refused", Message: err.Error()}
	}
	model := c.o.defaults[ProviderOpenAI]
	if ep.OpenRouter {
		model = DefaultCodeModelOpenRouter
	}
	return aichat.CodeCompleter{
		Client: &aichat.Client{Transport: c.o.transport}, Log: c.o.log, Now: c.o.now,
		DefaultModel: model, Unauthorized: c.markNeedsReauth,
	}.Complete(ctx, ep, call)
}
