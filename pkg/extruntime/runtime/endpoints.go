package runtime

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/endpoint"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
)

// EndpointConfig tunes package HTTP endpoints.
type EndpointConfig struct {
	// PublicTenant resolves the tenant of a request without a session (a
	// webhook), e.g. by host name. Without it public endpoints answer 503
	// unless the application middleware put a tenant in the request context.
	PublicTenant func(r *http.Request) (uuid.UUID, bool)
	MaxBodyBytes int64
	Timeout      time.Duration
}

// EndpointHandler serves /_ext/endpoints/<namespace>/<path> over the active
// package versions, as the session user or, for public endpoints, the trigger
// owner.
func (rt *Runtime) EndpointHandler() (*endpoint.Handler, error) {
	var subjects host.SubjectLoader
	if rt.Subjects != nil {
		subjects = rt.Subjects
	}
	return endpoint.NewHandler(endpoint.Config{
		Source:       rt.Manage.EndpointSource(),
		Runner:       rt.Host,
		Owners:       &endpoint.PgOwners{DB: rt.db},
		Subjects:     subjects,
		PublicTenant: rt.Config.Endpoints.PublicTenant,
		MaxBodyBytes: rt.Config.Endpoints.MaxBodyBytes,
		Timeout:      rt.Config.Endpoints.Timeout,
	})
}
