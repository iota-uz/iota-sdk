package graphs

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// HooksControllerID is the descriptor ID of the connectors' webhook endpoint.
const HooksControllerID = "automations.hooks"

// HooksPath is where connectors' providers deliver webhooks: <HooksPath>/<connection_id> and
// <HooksPath>/platform/<kind>. The route is exempt from the CSRF middleware of the router it is
// registered on (middleware.CSRFExemptRoute), and only there.
const HooksPath = "/automations/hooks"

// HooksController mounts engine.InboundHandler without a session, CSRF or tenant middleware: the
// handler finds the tenant by the connection and authenticates the request with the connector's
// Verify.
type HooksController struct {
	handler http.Handler
}

var _ application.Controller = (*HooksController)(nil)

// NewHooksController serves handler (engine.InboundHandler) at HooksPath.
func NewHooksController(handler http.Handler) *HooksController {
	return &HooksController{handler: handler}
}

// Descriptor implements application.Controller.
func (h *HooksController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor(HooksControllerID, 0, application.Prefix(HooksPath, application.Public()))
}

// Register implements application.Controller.
func (h *HooksController) Register(r *mux.Router) {
	// The webhooks authenticate by the connector's Verify, never by a session or a cookie.
	middleware.CSRFExemptRoute(r.PathPrefix(HooksPath+"/").Methods(http.MethodGet, http.MethodPost).Handler(h.handler))
}
