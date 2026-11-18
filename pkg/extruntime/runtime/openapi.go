package runtime

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// OpenAPIPath is where the OpenAPI 3.1 document is served.
const OpenAPIPath = reference.OpenAPIPath

// OpenAPI serves the OpenAPI 3.1 document of the extension runtime's HTTP
// interface for the signed-in reader: the browser RPC and package endpoints
// of everything the reader may see and, for holders of Extensions.Read, the
// management API.
type OpenAPI struct {
	ref     *reference.Builder
	subject SubjectFunc
	log     func(msg string, err error)
}

// NewOpenAPI builds the controller over the reference.
func NewOpenAPI(ref *reference.Builder, subject SubjectFunc, log func(msg string, err error)) *OpenAPI {
	if log == nil {
		log = func(string, error) {}
	}
	return &OpenAPI{ref: ref, subject: subject, log: log}
}

var _ application.Controller = (*OpenAPI)(nil)

// Descriptor implements application.Controller.
func (o *OpenAPI) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("extruntime.openapi", 0, application.Get(OpenAPIPath, application.Authenticated()))
}

// Register implements application.Controller.
func (o *OpenAPI) Register(r *mux.Router) {
	sub := r.Path(OpenAPIPath).Methods(http.MethodGet).Subrouter()
	sub.Use(middleware.Authorize(), middleware.ProvideUser())
	sub.Path("").Handler(o)
}

// ServeHTTP serves the document.
func (o *OpenAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := o.subject(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return
	}
	m, err := o.ref.Model(r.Context(), actor, "")
	if err != nil {
		o.log("extruntime openapi: build reference", err)
		writeAPIError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	doc, err := BuildOpenAPI(m, sessionCookieName(r.Context()))
	if err != nil {
		o.log("extruntime openapi: render", err)
		writeAPIError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(doc)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + message + `"}}` + "\n"))
}

// sessionCookieName is the application's session cookie, as the
// authentication middleware reads it ("sid" by default).
func sessionCookieName(ctx context.Context) string {
	container, err := composition.UseContainer(ctx)
	if err != nil {
		return "sid"
	}
	cfg, err := composition.Resolve[*cookies.Config](container)
	if err != nil || cfg == nil || cfg.SID == "" {
		return "sid"
	}
	return cfg.SID
}
