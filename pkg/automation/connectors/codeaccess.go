package connectors

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// CodeAccessor is an optional interface of a Connector: its API is reachable from Code nodes
// (env.connections.<name>.<method>(path, body)) and HTTP nodes (auth: {connection}) through a connection.
// The SDK adds the authorization, so a token never reaches the code, and every request stays on the
// API's origin.
type CodeAccessor interface {
	CodeAccess() CodeAccess
	// Authorize sets the headers that authorize a request of the API with the connection's credentials
	// (Authorization: Bearer <token>, an API key header). It returns no credential in an error.
	Authorize(ctx context.Context, creds Credentials, header http.Header) error
}

// CodeAccess is what Code and HTTP nodes may do with a connector's API.
type CodeAccess struct {
	// BaseURL is the https URL the paths of env.connections are relative to; only its origin is reachable.
	BaseURL string
	// Methods are the methods of env.connections.<name>: get(path) and list(path) for a read-only API.
	Methods []CodeMethod
}

// CodeMethod is one method of env.connections.<name>: its name in the code and the HTTP method it sends.
type CodeMethod struct {
	Name       string // ^[a-z][A-Za-z0-9_]{0,39}$
	HTTPMethod string // GET, HEAD, POST, PUT, PATCH, DELETE
}

var codeMethodName = regexp.MustCompile(`^[a-z][A-Za-z0-9_]{0,39}$`)

// Validate checks the access a connector declares: an https base URL without credentials, query or
// fragment, and methods with code names (each once) and HTTP methods.
func (a CodeAccess) Validate() error {
	u, err := url.Parse(a.BaseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("%w: code access: base URL %q is not an https URL without credentials, query or fragment", ErrInvalid, a.BaseURL)
	}
	seen := map[string]bool{}
	for _, m := range a.Methods {
		switch {
		case !codeMethodName.MatchString(m.Name) || m.Name == "then":
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

// MethodNames are the names of the methods, in order.
func (a CodeAccess) MethodNames() []string {
	out := make([]string, len(a.Methods))
	for i, m := range a.Methods {
		out[i] = m.Name
	}
	return out
}
