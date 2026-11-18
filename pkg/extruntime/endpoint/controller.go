package endpoint

import (
	"embed"
	"math"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// ControllerID is the descriptor ID of the endpoint host.
const ControllerID = "extruntime.endpoint"

// Controller mounts the Handler into the application. The route is declared
// public: a public endpoint is reachable without a session, and every endpoint
// decides itself whether it needs one. When a session exists the usual
// authentication middleware still resolves the user, so an authenticated
// endpoint runs as that user.
type Controller struct {
	*Handler
}

var _ application.Controller = (*Controller)(nil)

func NewController(h *Handler) *Controller { return &Controller{Handler: h} }

func (c *Controller) Descriptor() application.ControllerDescriptor {
	return application.Descriptor(ControllerID, math.MinInt32, application.Prefix(PathPrefix, application.Public()))
}

func (c *Controller) Register(r *mux.Router) {
	sub := r.PathPrefix(PathPrefix).Subrouter()
	sub.Use(middleware.Authorize(), middleware.ProvideUser())
	sub.PathPrefix("/").Handler(c.Handler)
}

// NewComponent returns the composition component of the endpoint slice.
func NewComponent(cfg Config) (composition.Component, error) {
	h, err := NewHandler(cfg)
	if err != nil {
		return nil, err
	}
	return &component{h: h}, nil
}

type component struct {
	h *Handler
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "extruntime-endpoint"}
}

func (c *component) LocaleFS() []*embed.FS { return nil }

func (c *component) Build(builder *composition.Builder) error {
	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.AddControllers(builder, application.Controller(NewController(c.h)))
	}
	return nil
}
