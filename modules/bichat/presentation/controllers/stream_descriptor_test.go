package controllers

import (
	"embed"
	"net/http"
	"testing"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/stretchr/testify/require"
)

type descriptorTestController struct {
	descriptor application.ControllerDescriptor
}

func (c descriptorTestController) Descriptor() application.ControllerDescriptor { return c.descriptor }
func (descriptorTestController) Register(*mux.Router)                           {}

type descriptorTestComponent struct {
	descriptor  composition.Descriptor
	controllers []application.Controller
}

func (c descriptorTestComponent) Descriptor() composition.Descriptor { return c.descriptor }
func (descriptorTestComponent) LocaleFS() []*embed.FS                { return nil }
func (c descriptorTestComponent) Build(builder *composition.Builder) error {
	composition.ContributeControllers(builder, func(*composition.Container) ([]application.Controller, error) {
		return c.controllers, nil
	})
	return nil
}

func TestStreamControllerDescriptorAllowsHostPageAtBasePath(t *testing.T) {
	t.Parallel()

	const basePath = "/admin/ali/chat"
	stream := NewStreamController(nil, nil, nil, WithBasePath(basePath))
	routes := stream.Descriptor().Routes
	require.Equal(t, []application.RouteSpec{
		application.Post(basePath + "/stream"),
		application.Post(basePath + "/stream/stop"),
		application.Get(basePath + "/stream/status"),
		application.Post(basePath + "/stream/resume"),
		application.Get(basePath + "/stream/events"),
		application.Get(basePath + "/stream/active-runs"),
	}, routes)
	require.Equal(t, basePath, stream.Descriptor().Nav[0].Path)

	host := descriptorTestController{descriptor: application.Descriptor(
		"host.ali.chat", 0, application.Route(http.MethodGet, basePath),
	)}
	engine := composition.NewEngine()
	require.NoError(t, engine.Register(
		descriptorTestComponent{
			descriptor: composition.Descriptor{Name: "bichat"}, controllers: []application.Controller{stream},
		},
		descriptorTestComponent{
			descriptor:  composition.Descriptor{Name: "host", Requires: []string{"bichat"}},
			controllers: []application.Controller{host},
		},
	))
	_, err := engine.Compile(composition.BuildContext{})
	require.NoError(t, err)
}
