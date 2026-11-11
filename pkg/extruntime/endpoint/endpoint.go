// Package endpoint serves the HTTP endpoints of activated extension packages
// under /_ext/endpoints/<namespace>/<path> (docs/extension-runtime.md §9).
//
// A request is matched by method and path against the active endpoints (the
// manifest `endpoints` of the activated package versions, supplied by the
// management slice through Source) and run as one synchronous `endpoint`
// invocation of the host: the handler's {status, headers, body} is written as
// the HTTP response, there is no durable instance. An authenticated request
// runs as the session user with the permissions held now; a `public` endpoint
// without a session runs as the trigger owner-user assigned for the tenant
// (extruntime.trigger_owners), judged by that user's current rights.
package endpoint

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/listext"
)

// PathPrefix is where endpoints are served: PathPrefix + namespace + path.
const PathPrefix = "/_ext/endpoints/"

// Defaults of Config.
const (
	DefaultMaxBodyBytes = 1 << 20
	DefaultTimeout      = 30 * time.Second
)

// Codes of errors raised by the endpoint transport itself (the `error.code`
// of its JSON error bodies).
const (
	CodeNotFound        = "endpoint.notFound"
	CodeMethodNotAllow  = "endpoint.methodNotAllowed"
	CodeUnauthenticated = "endpoint.unauthenticated"
	CodeBodyTooLarge    = "endpoint.bodyTooLarge"
	CodeBadRequest      = "endpoint.badRequest"
	CodeUnavailable     = "endpoint.unavailable"
	CodeNoTenant        = "endpoint.noTenant"
	CodeOwnerMissing    = "endpoint.ownerMissing"
	CodeOwnerBlocked    = "endpoint.ownerBlocked"
	CodeBadResponse     = "endpoint.badResponse"
	CodeTimeout         = "endpoint.timeout"
	CodeInternal        = "endpoint.internal"
)

// Endpoint is one active endpoint: a manifest endpoint of an activated package
// version together with the artifact that runs it.
type Endpoint struct {
	// ID is the endpoint resource ID; the trigger_owners ID of a public one.
	ID string
	// Package is the package namespace (the first path segment).
	Package string
	Version string
	Export  string
	Method  string
	// Path is the manifest path, starting with "/".
	Path   string
	Public bool
	// Artifact is the immutable artifact of Package@Version.
	Artifact pool.Artifact
	// Secrets are the secret names the package declared.
	Secrets []string
}

// Source lists the active endpoints. The management slice implements it over
// activations and stored manifests. It is called per request and must answer
// from memory or a cheap read.
type Source interface {
	ActiveEndpoints(ctx context.Context) ([]Endpoint, error)
}

// Runner runs one export; *host.Host implements it.
type Runner interface {
	Run(ctx context.Context, req host.RunRequest) (host.Result, error)
}

// Owners resolves the trigger owner-user of a public endpoint in a tenant.
type Owners interface {
	// Owner returns the owner-user of the trigger, found=false when the
	// tenant has none assigned.
	Owner(ctx context.Context, triggerID string, tenant uuid.UUID) (userID int64, found bool, err error)
}

// Config wires the endpoint slice.
type Config struct {
	// Source lists the active endpoints. Required.
	Source Source
	// Runner runs the endpoint invocations. Required.
	Runner Runner
	// Owners and Subjects resolve the subject of a public request without a
	// session: the trigger owner and that user's current rights. Both are
	// required; without them public endpoints answer 503.
	Owners   Owners
	Subjects host.SubjectLoader
	// SessionSubject returns the signed-in user of the request. Default: the
	// request user installed by the authentication middleware (the same
	// subject /_ext/rpc runs as).
	SessionSubject func(ctx context.Context) (spi.Subject, bool)
	// PublicTenant resolves the tenant of a request that has no session (a
	// webhook of a tenant), for example by host name. Default: the tenant the
	// application middleware put in the request context; without one a public
	// request answers 503.
	PublicTenant func(r *http.Request) (uuid.UUID, bool)
	// MaxBodyBytes bounds the request body. Default DefaultMaxBodyBytes.
	MaxBodyBytes int64
	// Timeout bounds one endpoint invocation. Default DefaultTimeout.
	Timeout time.Duration
	Logger  logrus.FieldLogger
}

func (c *Config) validate() error {
	switch {
	case c.Source == nil:
		return fmt.Errorf("extruntime endpoint: Source is required")
	case c.Runner == nil:
		return fmt.Errorf("extruntime endpoint: Runner is required")
	}
	if c.MaxBodyBytes <= 0 {
		c.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.Logger == nil {
		c.Logger = logrus.StandardLogger()
	}
	if c.SessionSubject == nil {
		c.SessionSubject = func(ctx context.Context) (spi.Subject, bool) {
			sub, err := listext.SubjectFromContext(ctx)
			return sub, err == nil
		}
	}
	if c.PublicTenant == nil {
		c.PublicTenant = func(r *http.Request) (uuid.UUID, bool) {
			id, err := composables.UseTenantID(r.Context())
			return id, err == nil
		}
	}
	return nil
}

// Handler serves PathPrefix. It is an http.Handler so tests and other routers
// can mount it; Controller mounts it into the application.
type Handler struct {
	cfg Config
}

// NewHandler validates cfg.
func NewHandler(cfg Config) (*Handler, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &Handler{cfg: cfg}, nil
}

// match splits "<namespace>/<path>" and finds the endpoints with that path.
func match(all []Endpoint, rest string) []Endpoint {
	ns, p, ok := strings.Cut(rest, "/")
	if !ok || ns == "" {
		return nil
	}
	p = "/" + p
	var matched []Endpoint
	for _, ep := range all {
		if ep.Package == ns && ep.Path == p {
			matched = append(matched, ep)
		}
	}
	return matched
}
