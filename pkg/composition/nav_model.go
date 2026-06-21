package composition

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/sirupsen/logrus"
)

type navModel struct {
	items      []types.NavigationItem
	quickLinks []*spotlight.QuickLink
}

type navBuildNode struct {
	component string
	node      application.NavNode
	index     int
	auth      application.AuthPolicy
}

func buildNavModel(routes []controllerRoute, contributions []navNodeContribution, failFast bool) (navModel, error) {
	nodes, err := validateNavNodes(routes, contributions, failFast)
	if err != nil {
		return navModel{}, err
	}
	return projectNavModel(nodes), nil
}

func (c *Container) NavItemsForScope(ctx context.Context, scope application.NavScope) []types.NavigationItem {
	if len(c.navProviders) == 0 {
		return nil
	}
	contributions := make([]navNodeContribution, 0)
	for _, provider := range c.navProviders {
		if provider == nil {
			continue
		}
		nodes, err := provider.ProvideNav(ctx, scope)
		if err != nil {
			logger := c.context.Logger()
			if logger == nil {
				logger = logrus.StandardLogger()
			}
			logger.WithError(err).Warn("runtime nav provider failed")
			continue
		}
		for _, node := range nodes {
			contributions = append(contributions, navNodeContribution{component: "runtime", node: node})
		}
	}
	if len(contributions) == 0 {
		return nil
	}
	merged := append([]navNodeContribution(nil), c.navCatalogNodes...)
	merged = append(merged, contributions...)
	model, err := buildNavModel(c.routeAuthRoutes, merged, false)
	if err != nil {
		logger := c.context.Logger()
		if logger == nil {
			logger = logrus.StandardLogger()
		}
		logger.WithError(err).Warn("runtime nav validation failed")
		return nil
	}
	return model.items
}

func validateNavNodes(routes []controllerRoute, contributions []navNodeContribution, failFast bool) ([]navBuildNode, error) {
	routeIndex := newNavRouteIndex(routes)
	seen := make(map[string]navBuildNode, len(contributions))
	nodes := make([]navBuildNode, 0, len(contributions))

	for index, contribution := range contributions {
		node := normalizeNavNode(contribution.node)
		if node.ID == "" {
			if failFast {
				return nil, fmt.Errorf("composition: nav node contributed by %q has empty ID", contribution.component)
			}
			continue
		}
		if existing, exists := seen[node.ID]; exists {
			if failFast {
				return nil, fmt.Errorf("composition: duplicate nav node ID %q contributed by %q and %q", node.ID, existing.component, contribution.component)
			}
			continue
		}

		buildNode := navBuildNode{
			component: contribution.component,
			node:      node,
			index:     index,
		}
		if node.Path != "" {
			if node.Visibility != nil && failFast {
				return nil, fmt.Errorf("composition: nav leaf %q must inherit visibility from its route Auth", node.ID)
			}
			route, ok := routeIndex.routeForPath(node.Path)
			if !ok {
				if failFast {
					return nil, fmt.Errorf("composition: nav node %q path %q does not resolve to a controller route", node.ID, node.Path)
				}
				continue
			}
			buildNode.auth = route.Auth
		} else if node.Visibility != nil {
			buildNode.auth = *node.Visibility
		}

		seen[node.ID] = buildNode
		nodes = append(nodes, buildNode)
	}

	var err error
	nodes, err = filterNavOrphans(nodes, seen, failFast)
	if err != nil {
		return nil, err
	}
	if err := validateNavCycles(nodes); err != nil {
		if failFast {
			return nil, err
		}
		return nil, err
	}
	return nodes, nil
}

func filterNavOrphans(nodes []navBuildNode, seen map[string]navBuildNode, failFast bool) ([]navBuildNode, error) {
	// A same-module node pointing at a missing parent is a programmer error; a
	// cross-module reference to an absent (optional) module is tolerated.
	if failFast {
		for _, node := range nodes {
			parent := node.node.Parent
			if parent == "" {
				continue
			}
			if _, exists := seen[parent]; !exists && sameNavModule(node.node.ID, parent) {
				return nil, fmt.Errorf("composition: nav node %q parent %q was not contributed", node.node.ID, parent)
			}
		}
	}

	// Prune orphans to a fixpoint so that deep chains (a node whose grandparent
	// is missing) are fully removed, not just the first level.
	valid := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		valid[node.node.ID] = true
	}
	for {
		removed := false
		for id := range valid {
			parent := seen[id].node.Parent
			if parent != "" && !valid[parent] {
				delete(valid, id)
				removed = true
			}
		}
		if !removed {
			break
		}
	}

	out := make([]navBuildNode, 0, len(valid))
	for _, node := range nodes {
		if valid[node.node.ID] {
			out = append(out, node)
		}
	}
	return out, nil
}

func validateNavCycles(nodes []navBuildNode) error {
	parentByID := make(map[string]string, len(nodes))
	for _, node := range nodes {
		parentByID[node.node.ID] = node.node.Parent
	}
	visiting := make(map[string]bool, len(nodes))
	visited := make(map[string]bool, len(nodes))
	var visit func(string) error
	visit = func(id string) error {
		if visited[id] {
			return nil
		}
		if visiting[id] {
			return fmt.Errorf("composition: nav cycle detected at %q", id)
		}
		visiting[id] = true
		if parent := parentByID[id]; parent != "" {
			if _, exists := parentByID[parent]; exists {
				if err := visit(parent); err != nil {
					return err
				}
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for id := range parentByID {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func projectNavModel(nodes []navBuildNode) navModel {
	children := make(map[string][]navBuildNode)
	for _, node := range nodes {
		children[node.node.Parent] = append(children[node.node.Parent], node)
	}
	for parent := range children {
		sortNavSiblings(children[parent])
	}

	var model navModel
	var buildItems func(parent string) []types.NavigationItem
	buildItems = func(parent string) []types.NavigationItem {
		out := make([]types.NavigationItem, 0, len(children[parent]))
		for _, node := range children[parent] {
			item := navNodeToItem(node)
			item.Children = buildItems(node.node.ID)
			if item.Href == "" && len(item.Children) == 0 {
				continue
			}
			if navSurfaceVisible(node.node, application.SurfaceSidebar) {
				out = append(out, item)
			}
			if navSurfaceVisible(node.node, application.SurfaceSpotlight) {
				model.quickLinks = append(model.quickLinks, navNodeQuickLinks(node)...)
			}
		}
		return out
	}
	model.items = buildItems("")
	return model
}

func sortNavSiblings(nodes []navBuildNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		left := nodes[i].node
		right := nodes[j].node
		if left.Before == right.ID || right.After == left.ID {
			return true
		}
		if right.Before == left.ID || left.After == right.ID {
			return false
		}
		if left.Order != right.Order {
			return left.Order < right.Order
		}
		return nodes[i].index < nodes[j].index
	})
}

func navNodeToItem(node navBuildNode) types.NavigationItem {
	nav := node.node
	options := nav.Surfaces[application.SurfaceSidebar]
	titleKey := firstNonEmpty(options.TitleKey, nav.TitleKey)
	path := firstNonEmpty(options.Path, nav.Path)
	icon := nav.Icon
	if options.Icon != nil {
		icon = options.Icon
	}
	keywords := append([]string(nil), nav.Keywords...)
	keywords = append(keywords, options.Keywords...)
	return types.NavigationItem{
		Key:         nav.ID,
		Workspace:   nav.Workspace,
		Name:        titleKey,
		Href:        path,
		Keywords:    keywords,
		Icon:        icon,
		Permissions: authPermissions(node.auth),
		Logic:       navItemPermissionLogic(node.auth),
		IsBeta:      nav.IsBeta,
	}
}

// navItemPermissionLogic maps a route's AuthPolicy logic onto the sidebar's
// permission-combination semantics so that a RequireAny route stays visible to
// a user holding any one of its permissions (matching route enforcement),
// instead of being hidden by the historical AND-only filter.
func navItemPermissionLogic(auth application.AuthPolicy) types.PermissionLogic {
	if auth.Logic == application.PermissionLogicAny {
		return types.PermissionLogicAny
	}
	return types.PermissionLogicAll
}

func navNodeQuickLinks(node navBuildNode) []*spotlight.QuickLink {
	nav := node.node
	links := make([]*spotlight.QuickLink, 0, 1+len(nav.Actions))
	if nav.Path != "" {
		if link := navQuickLink(nav.TitleKey, nav.Path, nav.Keywords, node.auth, nav.Surfaces[application.SurfaceSpotlight]); link != nil {
			links = append(links, link)
		}
	}
	for _, action := range nav.Actions {
		if action.Path == "" || action.TitleKey == "" {
			continue
		}
		options := action.Surfaces[application.SurfaceSpotlight]
		if options.Hidden {
			continue
		}
		auth := node.auth
		if action.Auth != nil {
			auth = *action.Auth
		}
		titleKey := firstNonEmpty(options.TitleKey, action.TitleKey)
		path := firstNonEmpty(options.Path, action.Path)
		if link := navQuickLink(titleKey, path, options.Keywords, auth, options); link != nil {
			links = append(links, link)
		}
	}
	return links
}

func navQuickLink(titleKey, path string, keywords []string, auth application.AuthPolicy, options application.SurfaceOptions) *spotlight.QuickLink {
	if options.Hidden || titleKey == "" || path == "" {
		return nil
	}
	builder := spotlight.NewQuickLinkBuilder(firstNonEmpty(options.TitleKey, titleKey), firstNonEmpty(options.Path, path)).
		WithKeywords(append(keywords, options.Keywords...)...)
	if auth.Public || len(auth.Permissions) == 0 {
		builder.Public()
	} else {
		builder.WithPermissions(authPermissionNames(auth.Permissions)...)
		if auth.Logic == application.PermissionLogicAll {
			builder.WithPermissionLogic(spotlight.PermissionLogicAll)
		} else {
			builder.WithPermissionLogic(spotlight.PermissionLogicAny)
		}
	}
	return builder.Build()
}

func navSurfaceVisible(node application.NavNode, surface application.Surface) bool {
	if len(node.Surfaces) == 0 {
		return surface == application.SurfaceSidebar || surface == application.SurfaceSpotlight
	}
	options, exists := node.Surfaces[surface]
	return exists && !options.Hidden
}

func authPermissions(auth application.AuthPolicy) []permission.Permission {
	if auth.Public || len(auth.Permissions) == 0 {
		return nil
	}
	return append([]permission.Permission(nil), auth.Permissions...)
}

func authPermissionNames(perms []permission.Permission) []string {
	names := make([]string, 0, len(perms))
	for _, perm := range perms {
		if perm == nil || perm.Name() == "" {
			continue
		}
		names = append(names, perm.Name())
	}
	return names
}

type navRouteIndex struct {
	routes []application.RouteSpec
}

func (c *Container) AuthPolicyForRoute(method, host, rawPath string) (application.AuthPolicy, bool) {
	method = strings.ToUpper(strings.TrimSpace(method))
	host = normalizeRouteHost(host)
	routePath := normalizeNavPath(rawPath)
	for _, candidate := range c.routeAuthRoutes {
		route := normalizeRoute(candidate.route)
		if route.Prefix {
			continue
		}
		if routeMatchesRequest(route, method, host, routePath) {
			return route.Auth, true
		}
	}
	for _, candidate := range c.routeAuthRoutes {
		route := normalizeRoute(candidate.route)
		if !route.Prefix {
			continue
		}
		if routeMatchesRequest(route, method, host, routePath) {
			return route.Auth, true
		}
	}
	return application.AuthPolicy{}, false
}

func routeMatchesRequest(route application.RouteSpec, method, host, routePath string) bool {
	if route.Method != "" && route.Method != method {
		return false
	}
	if route.Host != "" && route.Host != host {
		return false
	}
	if route.Prefix {
		return strings.HasPrefix(routePath, route.Path)
	}
	return routePatternMatches(route.Path, routePath)
}

func routePatternMatches(pattern, routePath string) bool {
	if pattern == routePath {
		return true
	}
	patternParts := splitRoutePath(pattern)
	pathParts := splitRoutePath(routePath)
	if len(patternParts) != len(pathParts) {
		return false
	}
	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			continue
		}
		if part != pathParts[i] {
			return false
		}
	}
	return true
}

func splitRoutePath(routePath string) []string {
	routePath = strings.Trim(application.NormalizeRoutePath(routePath), "/")
	if routePath == "" {
		return nil
	}
	return strings.Split(routePath, "/")
}

func normalizeRouteHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if idx := strings.IndexByte(host, ':'); idx > 0 {
		return host[:idx]
	}
	return host
}

func newNavRouteIndex(routes []controllerRoute) navRouteIndex {
	index := navRouteIndex{routes: make([]application.RouteSpec, 0, len(routes))}
	for _, route := range routes {
		index.routes = append(index.routes, normalizeRoute(route.route))
	}
	return index
}

func (i navRouteIndex) routeForPath(rawPath string) (application.RouteSpec, bool) {
	navPath := normalizeNavPath(rawPath)
	for _, route := range i.routes {
		if !route.Prefix && route.Path == navPath {
			return route, true
		}
	}
	for _, route := range i.routes {
		if route.Prefix && strings.HasPrefix(navPath, route.Path) {
			return route, true
		}
	}
	return application.RouteSpec{}, false
}

func normalizeNavNode(node application.NavNode) application.NavNode {
	node.ID = strings.TrimSpace(node.ID)
	node.Parent = strings.TrimSpace(node.Parent)
	node.Workspace = strings.TrimSpace(node.Workspace)
	node.TitleKey = strings.TrimSpace(node.TitleKey)
	node.Path = normalizeNavHref(node.Path)
	node.Before = strings.TrimSpace(node.Before)
	node.After = strings.TrimSpace(node.After)
	node.Keywords = normalizeStringSlice(node.Keywords)
	return node
}

func normalizeNavHref(rawPath string) string {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return ""
	}
	parsed, err := url.Parse(rawPath)
	if err != nil || parsed.Path == "" {
		return application.NormalizeRoutePath(rawPath)
	}
	parsed.Path = application.NormalizeRoutePath(parsed.Path)
	return parsed.String()
}

func normalizeNavPath(rawPath string) string {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return ""
	}
	parsed, err := url.Parse(rawPath)
	if err == nil && parsed.Path != "" {
		return application.NormalizeRoutePath(parsed.Path)
	}
	return application.NormalizeRoutePath(rawPath)
}

func normalizeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(out, value) {
			continue
		}
		out = append(out, value)
	}
	return out
}

func sameNavModule(id, parent string) bool {
	return navModule(id) == navModule(parent)
}

func navModule(id string) string {
	if idx := strings.IndexByte(id, '.'); idx > 0 {
		return id[:idx]
	}
	return id
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
