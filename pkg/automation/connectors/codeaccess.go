package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// CodeAccessor is an optional interface of a Connector: its connections are reachable from Code nodes as
// env.connections.<name> and from HTTP nodes (auth: {connection}). What a connection offers there is its
// CodeAccess:
//
//   - HTTP methods (env.connections.<name>.<method>(path, body)) and, with Fetch, a fetch
//     (env.connections.<name>.fetch(path, init)) on the API's origin. The SDK adds the authorization, so a
//     token never reaches the code, and every request stays on the API's origin.
//   - Operations implemented in Go (env.connections.<name>.<operation>(args)) when the connector is also a
//     CodeOperator: send a message, send mail, call a model.
//
// Every call follows the connection's tenant, status and the run-as subject's permissions: a disabled or
// broken connection is a typed error (ErrCodeConnectionDisabled, ErrCodeConnectionBroken), never a call.
type CodeAccessor interface {
	CodeAccess() CodeAccess
	// Authorize sets the headers that authorize a request of the API with the connection's credentials
	// (Authorization: Bearer <token>, an API key header). It returns no credential in an error. A connector
	// that offers only operations sets nothing.
	Authorize(ctx context.Context, creds Credentials, header http.Header) error
}

// CodeBaseURLResolver is an optional interface of a CodeAccessor whose API's base URL is a connection's
// setting (a generic HTTP/API connection): CodeBaseURL is the https base URL of those credentials, which
// replaces CodeAccess().BaseURL for that connection. It returns no credential in an error.
type CodeBaseURLResolver interface {
	CodeBaseURL(creds Credentials) (string, error)
}

// CodeOperator is an optional interface of a CodeAccessor: it serves the Operations of its CodeAccess.
type CodeOperator interface {
	// CallCodeOperation performs one operation. Its result is a JSON value for the code. A refusal the code
	// may handle is a *CodeOperationError; any other error reaches the code as a masked message. Neither
	// the result nor the error carries a credential: the SDK masks the connection's secrets in both anyway.
	CallCodeOperation(ctx context.Context, call CodeCall) (any, error)
}

// CodeCall is one operation called from code.
type CodeCall struct {
	// Operation is the name of one of CodeAccess().Operations.
	Operation string
	// Args is the JSON argument the code passed (null when none).
	Args json.RawMessage
	// Input is the call of the Code node with the connection: tenant, run, run-as, the connection's info and
	// decrypted credentials, the deadline and the usage recorder (an AI call records its tokens with it, as
	// the AI nodes do). Params and Context are the Code node's, not the operation's.
	Input catalog.CallInput
}

// CodeOperationError is an operation the connector refused (invalid arguments, a provider's refusal): the
// code sees a ConnectionError with this code and message.
type CodeOperationError struct {
	Code    string // snake_case: invalid_args, provider_refused, rate_limited, ...
	Message string
	// Retryable reports that the same call may succeed later (a rate limit, a provider outage).
	Retryable bool
}

func (e *CodeOperationError) Error() string { return e.Code + ": " + e.Message }

// Typed refusals of a connection opened for code (access.Open, the HTTP node's {connection}): the code and
// the node's output.error.code are these values' Code.
var (
	// ErrCodeConnectionDisabled: the connection is disabled by an administrator.
	ErrCodeConnectionDisabled = errors.New("connection_disabled")
	// ErrCodeConnectionBroken: the connection needs re-authorization or its last check failed.
	ErrCodeConnectionBroken = errors.New("connection_broken")
)

// CodeAccess is what Code and HTTP nodes may do with a connector's connections.
type CodeAccess struct {
	// BaseURL is the https URL the paths of env.connections are relative to; only its origin is reachable.
	// It may be empty for a connector that offers only operations, or that resolves it per connection
	// (CodeBaseURLResolver).
	BaseURL string
	// Methods are the HTTP methods of env.connections.<name>: get(path) and list(path) for a read-only API.
	Methods []CodeMethod
	// Fetch offers env.connections.<name>.fetch(pathOrURL, init): any HTTP method and headers on the API's
	// origin, with the authorization added by the SDK (a header the code sets of the same name is replaced).
	Fetch bool
	// Operations are the Go operations of env.connections.<name> (the connector is a CodeOperator).
	Operations []CodeOperation
}

// CodeMethod is one method of env.connections.<name>: its name in the code and the HTTP method it sends.
type CodeMethod struct {
	Name       string // ^[a-z][A-Za-z0-9_]{0,39}$
	HTTPMethod string // GET, HEAD, POST, PUT, PATCH, DELETE
}

// CodeOperation is one Go operation of env.connections.<name>.
type CodeOperation struct {
	Name string // ^[a-z][A-Za-z0-9_]{0,39}$, not a method's name
	// Description is one line for the editor's declarations.
	Description string
	// Params and Result are TypeScript types of the argument and the result for the editor's declarations
	// ("{ chat_id: string | number; text: string }"); empty: no argument, unknown.
	Params, Result string
	// Permission is the RBAC permission name the run-as subject must hold to call it ("": none).
	Permission string
}

var codeMethodName = regexp.MustCompile(`^[a-z][A-Za-z0-9_]{0,39}$`)

// reservedCodeNames are the properties env.connections.<name> has besides its methods and operations.
var reservedCodeNames = map[string]bool{"then": true, "fetch": true, "baseUrl": true, "constructor": true}

// Validate checks the access a connector declares: an https base URL without credentials, query or
// fragment when it has methods or a fetch (unless resolved is set: the base URL is a connection's), and
// methods and operations with code names (each once) and HTTP methods.
func (a CodeAccess) Validate() error { return a.validate(false) }

// ValidateResolved is Validate for a connector that resolves its base URL per connection
// (CodeBaseURLResolver): an empty BaseURL is allowed.
func (a CodeAccess) ValidateResolved() error { return a.validate(true) }

func (a CodeAccess) validate(resolved bool) error {
	needsBase := len(a.Methods) > 0 || a.Fetch
	if a.BaseURL != "" || (needsBase && !resolved) {
		if err := CheckCodeBaseURL(a.BaseURL); err != nil {
			return fmt.Errorf("%w: code access: %v", ErrInvalid, err)
		}
	}
	seen := map[string]bool{}
	for _, m := range a.Methods {
		switch {
		case !codeMethodName.MatchString(m.Name) || reservedCodeNames[m.Name]:
			return fmt.Errorf("%w: code access: method name %q", ErrInvalid, m.Name)
		case seen[m.Name]:
			return fmt.Errorf("%w: code access: method %s is declared twice", ErrInvalid, m.Name)
		}
		switch strings.ToUpper(m.HTTPMethod) {
		case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			return fmt.Errorf("%w: code access: method %s sends %q", ErrInvalid, m.Name, m.HTTPMethod)
		}
		seen[m.Name] = true
	}
	for _, op := range a.Operations {
		switch {
		case !codeMethodName.MatchString(op.Name) || reservedCodeNames[op.Name]:
			return fmt.Errorf("%w: code access: operation name %q", ErrInvalid, op.Name)
		case seen[op.Name]:
			return fmt.Errorf("%w: code access: %s is declared twice", ErrInvalid, op.Name)
		}
		seen[op.Name] = true
	}
	return nil
}

// CheckCodeBaseURL checks a base URL of code access: https, a host, no credentials, query or fragment.
// Its error names the URL's problem, never the URL (a connection's URL may carry a token).
func CheckCodeBaseURL(s string) error {
	u, err := url.Parse(s)
	switch {
	case err != nil:
		return errors.New("the base URL is not a URL")
	case u.Scheme != "https" || u.Host == "":
		return errors.New("the base URL is not an https URL")
	case u.User != nil:
		return errors.New("the base URL carries credentials")
	case u.RawQuery != "" || u.Fragment != "":
		return errors.New("the base URL has a query or a fragment")
	}
	return nil
}

// Method is the declared method of that name.
func (a CodeAccess) Method(name string) (CodeMethod, bool) {
	for _, m := range a.Methods {
		if m.Name == name {
			return m, true
		}
	}
	return CodeMethod{}, false
}

// Operation is the declared operation of that name.
func (a CodeAccess) Operation(name string) (CodeOperation, bool) {
	for _, op := range a.Operations {
		if op.Name == name {
			return op, true
		}
	}
	return CodeOperation{}, false
}

// MethodNames are the names of the methods, in order.
func (a CodeAccess) MethodNames() []string {
	out := make([]string, len(a.Methods))
	for i, m := range a.Methods {
		out[i] = m.Name
	}
	return out
}

// OperationNames are the names of the operations, in order.
func (a CodeAccess) OperationNames() []string {
	out := make([]string, len(a.Operations))
	for i, op := range a.Operations {
		out[i] = op.Name
	}
	return out
}
