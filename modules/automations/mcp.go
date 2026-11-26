package automations

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/automations/presentation/graphs"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/integrations"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/mcpops"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	mcpkit "github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/changes"
	"github.com/iota-uz/iota-sdk/pkg/mcp/mcpwiring"
)

// pageURLs are the module's pages a person opens from a link: the automations list, the editor and its
// runs, and the integrations page. Absolute under the application's origin, paths without one.
type pageURLs struct {
	origin string
	path   string
}

var _ mcpops.URLs = pageURLs{}

// pages are the module's pages under origin (httpconfig's), which is dropped unless it is an http(s)
// URL with a host.
func (c *component) pages(origin string) pageURLs {
	if u, err := url.Parse(origin); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		origin = ""
	}
	path := c.automationsPath
	if path == "" {
		path = graphs.DefaultPath
	}
	return pageURLs{origin: strings.TrimRight(origin, "/"), path: "/" + strings.Trim(path, "/")}
}

func (p pageURLs) Editor(id uuid.UUID, version int) string {
	link := p.origin + p.path + "/" + id.String()
	if version > 0 {
		link += "?version=" + strconv.Itoa(version)
	}
	return link
}

func (p pageURLs) Run(id, runID uuid.UUID) string {
	return p.origin + p.path + "/" + id.String() + "/runs/" + runID.String()
}

func (p pageURLs) Automations() string { return p.origin + p.path }

func (p pageURLs) Connections() string { return p.origin + integrations.DefaultPath }

// contributeMCP contributes the MCP operations of agents on automations (pkg/automation/mcpops) to
// the application's MCP composition (pkg/mcp/mcpwiring): the read tools to its registry and the kinds
// of entity "automation" to its changes service. Without the engine nothing is contributed; without
// an MCP composition the contributions are not read.
func (c *component) contributeMCP(b *composition.Builder, pages pageURLs) {
	if c.noEngine {
		return
	}
	mcpwiring.ContributeTools(b, func(container *composition.Container) ([]mcpkit.Tool, error) {
		h, reg, err := c.mcpDeps(container)
		if err != nil || h == nil {
			return nil, err
		}
		return mcpops.Tools(h.rt.Service(), h.rt.Authoring(), reg, pages, mcpops.WithBehaviors(h.behaviors))
	})
	mcpwiring.ContributeKinds(b, func(container *composition.Container) ([]changes.Kind, error) {
		h, _, err := c.mcpDeps(container)
		if err != nil || h == nil {
			return nil, err
		}
		return mcpops.Kinds(h.rt.Service(), h.rt.Authoring(), pages, mcpops.WithBehaviors(h.behaviors)), nil
	})
}

// mcpDeps are the engine and the catalog; a nil holder when the composition runs no engine.
func (c *component) mcpDeps(container *composition.Container) (*engineHolder, catalog.Registry, error) {
	if !composition.Has[*engineHolder](container) {
		return nil, nil, nil
	}
	h, err := composition.Resolve[*engineHolder](container)
	if err != nil || h.rt == nil {
		return nil, nil, err
	}
	reg, err := composition.Resolve[catalog.Registry](container)
	if err != nil {
		return nil, nil, err
	}
	return h, reg, nil
}
