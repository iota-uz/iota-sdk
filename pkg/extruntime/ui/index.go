package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/shellext"
)

// Page is a routable client page resolved from an activated manifest.
type Page struct {
	ID           string
	Pattern      string
	Package      string
	Version      string
	ArtifactHash string
	Chunk        string
	Export       string
	// Permission is the permission name the subject must hold; empty means
	// every authenticated user.
	Permission string
	Nav        *manifest.Nav
	// Route is the shared route resource the page implements, if any.
	Route string
	// Params types the path parameters when the page implements a route.
	Params *manifest.Type
	// Label is the page's own label for the title (nav label when present).
	Label manifest.Label
	segs  []segment
}

type segment struct {
	literal string
	param   string
}

// AssetPath is the immutable URL path of the page's client chunk.
func (p *Page) AssetPath() string {
	return AssetsPrefix + p.ArtifactHash + "/" + strings.TrimPrefix(p.Chunk, "/")
}

// SlotContribution is a component an activated package mounts in a slot. It
// is additive: it exists while its package version is active, with no binding.
type SlotContribution struct {
	ID           string
	Package      string
	Version      string
	ArtifactHash string
	Chunk        string
	Export       string
	// Permission is the permission name a viewer needs to get the
	// contribution; empty means every viewer the slot admits.
	Permission string
	Order      int
}

// Module is the contribution as the browser host loads it.
func (c SlotContribution) Module() shellext.Module {
	return shellext.Module{
		ID: c.ID, Package: c.Package, Version: c.Version,
		Module: AssetsPrefix + c.ArtifactHash + "/" + strings.TrimPrefix(c.Chunk, "/"), Export: c.Export,
	}
}

// ShellRenderer identifies the package shell renderer activated app-wide.
type ShellRenderer struct {
	ID           string
	Package      string
	Version      string
	ArtifactHash string
	Chunk        string
	Export       string
	BindingID    string
}

// AssetPath is the immutable URL path of the shell's client chunk.
func (s *ShellRenderer) AssetPath() string {
	return AssetsPrefix + s.ArtifactHash + "/" + strings.TrimPrefix(s.Chunk, "/")
}

// Navigation is the union of the navigation resources of activated packages.
type Navigation struct {
	Groups     []NavGroup
	Patches    []manifest.NavPatch
	Workspaces []manifest.Workspace
	Palette    []PaletteItem
}

// NavGroup is a declared group with the package that declares it.
type NavGroup struct {
	manifest.NavGroup
	Package string
}

// PaletteItem is a palette entry with its owner.
type PaletteItem struct {
	manifest.PaletteEntry
	Package string
}

// Index is the compiled, immutable view of a Snapshot.
type Index struct {
	Revision string
	// Pages are ordered most specific first.
	Pages []*Page
	// Screens maps a replaced route ID to the replacing page.
	Screens map[string]*Page
	// ScreenPaths maps a replaced route ID to its path pattern (from native
	// routes or a package page).
	ScreenPaths map[string]string
	Shell       *ShellRenderer
	Navigation  Navigation
	Columns     map[string][]listext.Contribution
	// Slots maps a slot ID to its contributions in mounting order (Order,
	// package, ID). The slot's own declaration (typed props, permission) is
	// looked up in the catalog when a page renders it.
	Slots map[string][]SlotContribution
	// Diagnostics describes ignored declarations (path conflicts, unresolved
	// bindings); it never blocks the rest.
	Diagnostics []string

	pageByID map[string]*Page
	natives  map[string]*nativeRoute
	// routeParams types the path parameters of every known route: the
	// routes of activated packages and the native routes the host declared.
	routeParams map[string]*manifest.Type
	// artifacts are the content hashes of the artifacts of the activated
	// package versions; the screen and shell bindings of a snapshot resolve
	// only into these, so it is the whole set the asset route may serve.
	artifacts map[string]bool
}

// ServesArtifact reports whether hash addresses the artifact of an activated
// package version, and so may have its client subtree served.
func (ix *Index) ServesArtifact(hash string) bool { return ix.artifacts[hash] }

type nativeRoute struct {
	id      string
	pattern string
	segs    []segment
	params  *manifest.Type
	// permission is the route's own requirement, which a replacing page
	// inherits (a replacement never widens access to the screen).
	permission string
}

// NativeRouteDecl is a native route as the index uses it: its path pattern
// and, when the host declared the route as a typed contract, the types of its
// path parameters (a package page replacing the route gets them).
type NativeRouteDecl struct {
	Pattern string
	Params  *manifest.Type
	// Permission is the permission name the native screen requires; a page
	// replacing the route is served only to subjects who hold it.
	Permission string
}

func parsePattern(pattern string) []segment {
	var out []segment
	for _, s := range strings.Split(strings.Trim(pattern, "/"), "/") {
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, ":") {
			out = append(out, segment{param: s[1:]})
		} else {
			out = append(out, segment{literal: s})
		}
	}
	return out
}

func match(segs []segment, path string) (map[string]string, bool) {
	parts := []string{}
	for _, s := range strings.Split(strings.Trim(path, "/"), "/") {
		if s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) != len(segs) {
		return nil, false
	}
	var params map[string]string
	for i, s := range segs {
		if s.param != "" {
			if params == nil {
				params = map[string]string{}
			}
			params[s.param] = parts[i]
		} else if s.literal != parts[i] {
			return nil, false
		}
	}
	return params, true
}

// moreSpecific orders patterns: at each position a literal beats a parameter.
func moreSpecific(a, b []segment) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if (a[i].param == "") != (b[i].param == "") {
			return a[i].param == ""
		}
	}
	return len(a) > len(b)
}

// BuildIndex compiles a snapshot. It never fails: declarations that cannot be
// served are recorded in Diagnostics.
func BuildIndex(s *Snapshot, natives NativeRoutes) *Index {
	decls := make(map[string]NativeRouteDecl, len(natives))
	for id, pattern := range natives {
		decls[id] = NativeRouteDecl{Pattern: pattern}
	}
	return buildIndex(s, decls)
}

func buildIndex(s *Snapshot, natives map[string]NativeRouteDecl) *Index {
	ix := &Index{
		Screens:     map[string]*Page{},
		ScreenPaths: map[string]string{},
		Columns:     map[string][]listext.Contribution{},
		Slots:       map[string][]SlotContribution{},
		pageByID:    map[string]*Page{},
		natives:     map[string]*nativeRoute{},
		routeParams: map[string]*manifest.Type{},
		artifacts:   map[string]bool{},
	}
	if s == nil {
		return ix
	}
	ix.Revision = s.Revision
	for _, p := range s.Packages {
		ix.artifacts[p.ArtifactHash] = true
	}
	for id, n := range natives {
		ix.natives[id] = &nativeRoute{id: id, pattern: n.Pattern, segs: parsePattern(n.Pattern), params: n.Params, permission: n.Permission}
		if n.Params != nil {
			ix.routeParams[id] = n.Params
		}
	}
	pkgs := append([]ActivePackage(nil), s.Packages...)
	sort.SliceStable(pkgs, func(i, j int) bool { return pkgs[i].Package < pkgs[j].Package })

	routes := map[string]manifest.Resource{}
	for _, p := range pkgs {
		for _, r := range p.Manifest.Resources {
			if r.Kind == manifest.ResRoute {
				routes[r.ID] = r
				if r.Params != nil {
					ix.routeParams[r.ID] = r.Params
				}
			}
		}
	}
	columnOwner := map[string]string{}
	var cells []cellDecl
	byPattern := map[string]*Page{}
	for _, p := range pkgs {
		for _, r := range p.Manifest.Resources {
			switch r.Kind {
			case manifest.ResPage:
				// A page for a route of the platform or of another package is a screen
				// replacement candidate: it is served only through a screen binding.
				if r.Route != "" && !strings.HasPrefix(r.Route, p.Manifest.Namespace+".") {
					continue
				}
				page := &Page{
					ID: r.ID, Pattern: normalizePattern(r.Path), Package: p.Package, Version: p.Version,
					ArtifactHash: p.ArtifactHash, Chunk: r.Chunk, Export: r.Export,
					Permission: r.Permission, Nav: r.Nav, Route: r.Route, Label: r.Label,
				}
				if r.Nav != nil && len(page.Label) == 0 {
					page.Label = r.Nav.Label
				}
				if r.Route != "" {
					if route, ok := routes[r.Route]; ok {
						page.Params = route.Params
						if page.Pattern == "" {
							page.Pattern = normalizePattern(route.Path)
						}
					} else if n, ok := ix.natives[r.Route]; ok {
						page.Params = n.params
						if page.Pattern == "" {
							page.Pattern = normalizePattern(n.pattern)
						}
					}
				}
				if page.Pattern == "" || !servableAsset(p.ArtifactHash, r.Chunk) {
					ix.Diagnostics = append(ix.Diagnostics, fmt.Sprintf("page %s: missing path, or chunk/artifact cannot be served", r.ID))
					continue
				}
				if prev, dup := byPattern[page.Pattern]; dup {
					ix.Diagnostics = append(ix.Diagnostics, fmt.Sprintf("page %s: path %s is already served by %s", r.ID, page.Pattern, prev.ID))
					continue
				}
				page.segs = parsePattern(page.Pattern)
				byPattern[page.Pattern] = page
				ix.Pages = append(ix.Pages, page)
				ix.pageByID[page.ID] = page
			case manifest.ResNavigation:
				for _, g := range r.Groups {
					ix.Navigation.Groups = append(ix.Navigation.Groups, NavGroup{NavGroup: g, Package: p.Package})
				}
				ix.Navigation.Patches = append(ix.Navigation.Patches, r.Patches...)
				ix.Navigation.Workspaces = append(ix.Navigation.Workspaces, r.Workspaces...)
				for _, e := range r.Palette {
					ix.Navigation.Palette = append(ix.Navigation.Palette, PaletteItem{PaletteEntry: e, Package: p.Package})
				}
			case manifest.ResColumn:
				if r.List == "" || r.Overlay == "" || r.Field == "" {
					ix.Diagnostics = append(ix.Diagnostics, fmt.Sprintf("column %s: missing list, overlay or field", r.ID))
					continue
				}
				columnOwner[r.ID] = p.Package
				ix.Columns[r.List] = append(ix.Columns[r.List], listext.Contribution{
					ID: r.ID, Package: p.Package, Version: p.Version, Label: r.Label,
					Overlay: r.Overlay, Field: r.Field,
					Filterable: r.Filterable, Sortable: r.Sortable, Exportable: r.Exportable,
				})
			case manifest.ResSlot:
				// A declaration (no target) is a contract; only a contribution is mounted.
				if r.Target == "" {
					continue
				}
				if r.Export == "" || !servableAsset(p.ArtifactHash, r.Chunk) {
					ix.Diagnostics = append(ix.Diagnostics, fmt.Sprintf("slot contribution %s: missing export, or chunk/artifact cannot be served", r.ID))
					continue
				}
				ix.Slots[r.Target] = append(ix.Slots[r.Target], SlotContribution{
					ID: r.ID, Package: p.Package, Version: p.Version, ArtifactHash: p.ArtifactHash, Chunk: r.Chunk, Export: r.Export,
					Permission: r.Permission, Order: r.Order,
				})
			case manifest.ResCell:
				if r.Target == "" || r.Export == "" || !servableAsset(p.ArtifactHash, r.Chunk) {
					ix.Diagnostics = append(ix.Diagnostics, fmt.Sprintf("cell %s: missing column or export, or chunk/artifact cannot be served", r.ID))
					continue
				}
				cells = append(cells, cellDecl{SlotContribution{ID: r.ID, Package: p.Package, Version: p.Version, ArtifactHash: p.ArtifactHash, Chunk: r.Chunk, Export: r.Export}, r.Target})
			case manifest.ResEntity, manifest.ResOverlay, manifest.ResQuery, manifest.ResCommand, manifest.ResOperation,
				manifest.ResEvent, manifest.ResWorkflow, manifest.ResNotice, manifest.ResRoute, manifest.ResPermission,
				manifest.ResIssue, manifest.ResDomainError, manifest.ResShell,
				manifest.ResContactResolver, manifest.ResDeliveryPolicy:
				// Not page, navigation or column contributions.
			}
		}
	}
	sort.SliceStable(ix.Pages, func(i, j int) bool { return moreSpecific(ix.Pages[i].segs, ix.Pages[j].segs) })
	for _, list := range ix.Slots {
		sort.SliceStable(list, func(i, j int) bool {
			a, b := list[i], list[j]
			if a.Order != b.Order {
				return a.Order < b.Order
			}
			if a.Package != b.Package {
				return a.Package < b.Package
			}
			return a.ID < b.ID
		})
	}
	ix.attachCells(cells, columnOwner)

	// Bindings. The newest open row wins when several exist for the shell.
	bindings := append([]Binding(nil), s.Bindings...)
	sort.SliceStable(bindings, func(i, j int) bool { return bindings[i].ActiveFrom.After(bindings[j].ActiveFrom) })
	for _, b := range bindings {
		switch b.SlotKind {
		case SlotScreen:
			if _, taken := ix.Screens[b.TargetID]; taken {
				continue
			}
			page := ix.findPage(pkgs, b)
			if page == nil {
				ix.Diagnostics = append(ix.Diagnostics, fmt.Sprintf("screen binding %s: %s@%s has no page %q", b.TargetID, b.Package, b.Version, b.Export))
				continue
			}
			pattern, ok := ix.resolveRoutePattern(b.TargetID)
			if !ok {
				ix.Diagnostics = append(ix.Diagnostics, fmt.Sprintf("screen binding %s: the target route is unknown", b.TargetID))
				continue
			}
			ix.Screens[b.TargetID] = page
			ix.ScreenPaths[b.TargetID] = pattern
		case SlotShell:
			if ix.Shell != nil {
				continue
			}
			if sh := ix.findShell(pkgs, b); sh != nil {
				ix.Shell = sh
			} else {
				ix.Diagnostics = append(ix.Diagnostics, fmt.Sprintf("shell binding: %s@%s has no shell renderer %q", b.Package, b.Version, b.Export))
			}
		}
	}
	return ix
}

type cellDecl struct {
	SlotContribution
	column string
}

// attachCells gives each contributed column its package renderer. Several
// packages may supply a cell for one column; the package that declares the
// column wins, otherwise the first by package name and ID (cells arrive in
// package order), and the others are reported.
func (ix *Index) attachCells(cells []cellDecl, owner map[string]string) {
	chosen := map[string]cellDecl{}
	for _, c := range cells {
		prev, taken := chosen[c.column]
		switch {
		case !taken:
			chosen[c.column] = c
		case owner[c.column] == c.Package && owner[c.column] != prev.Package:
			ix.Diagnostics = append(ix.Diagnostics, fmt.Sprintf("cell %s: column %s is rendered by its owner's %s", prev.ID, c.column, c.ID))
			chosen[c.column] = c
		default:
			ix.Diagnostics = append(ix.Diagnostics, fmt.Sprintf("cell %s: column %s already has the renderer %s", c.ID, c.column, prev.ID))
		}
	}
	for list, cols := range ix.Columns {
		for i := range cols {
			if c, ok := chosen[cols[i].ID]; ok {
				m := c.Module()
				ix.Columns[list][i].Cell = &m
			}
		}
	}
	for column := range chosen {
		if !ix.hasColumn(column) {
			ix.Diagnostics = append(ix.Diagnostics, fmt.Sprintf("cell %s: column %s is not contributed by an activated package", chosen[column].ID, column))
		}
	}
}

func (ix *Index) hasColumn(id string) bool {
	for _, cols := range ix.Columns {
		for _, c := range cols {
			if c.ID == id {
				return true
			}
		}
	}
	return false
}

// servableAsset reports whether the asset route can serve hash and chunk; a
// declaration that points elsewhere would only produce a 404 in the browser.
func servableAsset(hash, chunk string) bool {
	if !hashPattern.MatchString(hash) {
		return false
	}
	_, ok := cleanAssetPath(strings.TrimPrefix(chunk, "/"))
	return ok
}

func normalizePattern(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}

func (ix *Index) resolveRoutePattern(routeID string) (string, bool) {
	if n, ok := ix.natives[routeID]; ok {
		return normalizePattern(n.pattern), true
	}
	if p, ok := ix.pageByID[routeID]; ok {
		return p.Pattern, true
	}
	for _, p := range ix.Pages {
		if p.Route == routeID {
			return p.Pattern, true
		}
	}
	return "", false
}

func (ix *Index) findPage(pkgs []ActivePackage, b Binding) *Page {
	for _, p := range pkgs {
		if p.Package != b.Package || p.Version != b.Version {
			continue
		}
		for _, r := range p.Manifest.Resources {
			if r.Kind != manifest.ResPage || (r.Export != b.Export && r.ID != b.Export) {
				continue
			}
			if !servableAsset(p.ArtifactHash, r.Chunk) {
				return nil
			}
			page := &Page{
				ID: r.ID, Pattern: normalizePattern(r.Path), Package: p.Package, Version: p.Version,
				ArtifactHash: p.ArtifactHash, Chunk: r.Chunk, Export: r.Export,
				Permission: r.Permission, Route: r.Route, Label: r.Label, Params: ix.routeParams[r.Route],
			}
			if r.Nav != nil && len(page.Label) == 0 {
				page.Label = r.Nav.Label
			}
			return page
		}
	}
	return nil
}

func (ix *Index) findShell(pkgs []ActivePackage, b Binding) *ShellRenderer {
	for _, p := range pkgs {
		if p.Package != b.Package || p.Version != b.Version {
			continue
		}
		for _, r := range p.Manifest.Resources {
			if r.Kind != manifest.ResShell || (r.Export != b.Export && r.ID != b.Export) {
				continue
			}
			if !servableAsset(p.ArtifactHash, r.Chunk) {
				return nil
			}
			return &ShellRenderer{ID: r.ID, Package: p.Package, Version: p.Version, ArtifactHash: p.ArtifactHash, Chunk: r.Chunk, Export: r.Export, BindingID: b.ID}
		}
	}
	return nil
}

// Route is the page chosen for a request path with its path parameters.
type Route struct {
	Page *Page
	// ReplacedRoute is the route ID whose screen Page replaces, "" otherwise.
	ReplacedRoute string
	// ReplacedPermission is the permission the replaced native route
	// requires ("" when none): the replacing page inherits it.
	ReplacedPermission string
	Params             map[string]string
}

// Resolve picks what serves path. A screen binding wins over the native
// route it replaces; otherwise the most specific package page serves.
func (ix *Index) Resolve(path string) (Route, bool) {
	if len(ix.Screens) > 0 {
		ids := make([]string, 0, len(ix.Screens))
		for id := range ix.Screens {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		var best *Route
		var bestSegs []segment
		for _, id := range ids {
			segs := parsePattern(ix.ScreenPaths[id])
			if params, ok := match(segs, path); ok {
				if best == nil || moreSpecific(segs, bestSegs) {
					best = &Route{Page: ix.Screens[id], ReplacedRoute: id, Params: params}
					if n, ok := ix.natives[id]; ok {
						best.ReplacedPermission = n.permission
					}
					bestSegs = segs
				}
			}
		}
		if best != nil {
			return *best, true
		}
	}
	for _, p := range ix.Pages {
		if params, ok := match(p.segs, path); ok {
			return Route{Page: p, Params: params}, true
		}
	}
	return Route{}, false
}

// PageByID returns a page by resource ID.
func (ix *Index) PageByID(id string) (*Page, bool) {
	p, ok := ix.pageByID[id]
	return p, ok
}
