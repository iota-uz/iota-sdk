package ui

import (
	"embed"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

// NewComponent returns the composition component of the UI slice: the page
// host controller (with the asset route and the browser RPC), the request
// middleware (native list columns, navigation, shell activation) and the
// palette spotlight provider. overlays is the typed data store (it joins
// overlay columns into native lists); nil disables contributed columns.
func NewComponent(cfg Config, overlays listext.OverlayJoiner) (composition.Component, error) {
	rt, err := NewRuntime(cfg)
	if err != nil {
		return nil, err
	}
	return &component{rt: rt, overlays: overlays}, nil
}

type component struct {
	rt       *Runtime
	overlays listext.OverlayJoiner
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "extruntime-ui"}
}

func (c *component) LocaleFS() []*embed.FS { return nil }

// Runtime returns the shared runtime, for wiring tests and management.
func (c *component) Runtime() *Runtime { return c.rt }

func (c *component) Build(builder *composition.Builder) error {
	composition.ContributeMiddleware(builder, func(*composition.Container) ([]mux.MiddlewareFunc, error) {
		return []mux.MiddlewareFunc{c.rt.Middleware(c.overlays)}, nil
	})
	composition.ContributeSpotlightProviders(builder, func(*composition.Container) ([]spotlight.SearchProvider, error) {
		return []spotlight.SearchProvider{c.rt.SpotlightProvider()}, nil
	})
	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.AddControllers(builder, application.Controller(NewController(c.rt)))
	}
	return nil
}
