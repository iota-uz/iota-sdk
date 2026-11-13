package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/a-h/templ"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/shellext"
)

// RuntimeContextElementID is the id of the JSON script element the browser
// runtime reads on every authenticated page that has a shell binding.
const RuntimeContextElementID = "iota-runtime-context"

// RuntimeContextVersion is the major.minor of the RuntimeContext wire
// contract.
const RuntimeContextVersion = "1.0.0"

// RuntimeContext is published on authenticated pages when a package shell
// renderer is bound. It carries what the renderer needs beyond
// pkg/shellcontext: workspaces, palette entries and the identity of the
// renderer module. Navigation items themselves come from shellcontext, so
// Templ pages and Solid pages see one navigation.
type RuntimeContext struct {
	Version           string            `json:"version"`
	ProtocolVersion   string            `json:"protocolVersion"`
	SDKReleaseVersion string            `json:"sdkReleaseVersion"`
	SDKCommit         string            `json:"sdkCommit"`
	RPC               string            `json:"rpc"`
	Locale            string            `json:"locale"`
	Permissions       []string          `json:"permissions"`
	User              *RuntimeUser      `json:"user,omitempty"`
	Shell             *ShellRef         `json:"shell,omitempty"`
	Navigation        RuntimeNavigation `json:"navigation"`
	// Slots are the slot contributions the user gets, by slot ID, for the
	// `<Slot>` components of the shell renderer.
	Slots map[string][]shellext.Module `json:"slots"`
}

// ShellRef identifies the shell renderer module to load.
type ShellRef struct {
	ID      string `json:"id"`
	Package string `json:"package"`
	Version string `json:"version"`
	Module  string `json:"module"`
	Export  string `json:"export,omitempty"`
	Binding string `json:"binding"`
	// Styles are the immutable URLs of the stylesheets of the renderer's version.
	Styles []string `json:"styles,omitempty"`
}

// RuntimeNavigation holds the navigation extras of the shell model.
type RuntimeNavigation struct {
	Workspaces []WorkspaceRef `json:"workspaces"`
	Palette    []PaletteRef   `json:"palette"`
}

// WorkspaceRef is a declared workspace.
type WorkspaceRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Icon  string `json:"icon,omitempty"`
	Order int    `json:"order"`
}

// PaletteRef is a command palette entry that links to a page the user may
// open.
type PaletteRef struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Href     string   `json:"href"`
	Keywords []string `json:"keywords"`
}

// Palette resolves the palette entries a subject may use: entries whose
// target is a page (or a route with a fixed path) the subject has permission
// for.
func Palette(ix *Index, sub spi.Subject, lang string) []PaletteRef {
	out := []PaletteRef{}
	for _, e := range ix.Navigation.Palette {
		pattern, ok := ix.resolveRoutePattern(e.Target)
		if !ok || containsParam(pattern) {
			continue
		}
		if p, isPage := ix.PageByID(e.Target); isPage && !sub.Has(p.Permission) {
			continue
		}
		kw := append([]string{}, e.Keywords...)
		out = append(out, PaletteRef{ID: e.ID, Label: listext.LabelFor(e.Label, lang, e.ID), Href: pattern, Keywords: kw})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func containsParam(pattern string) bool {
	for _, s := range parsePattern(pattern) {
		if s.param != "" {
			return true
		}
	}
	return false
}

// signedInSubject is the invoking subject of the request, if a user is signed in.
func signedInSubject(ctx context.Context) (spi.Subject, bool) {
	sub, err := listext.SubjectFromContext(ctx)
	if err != nil {
		return spi.Subject{}, false
	}
	return sub, true
}

// Context builds the runtime context for a request, or nil when no shell
// renderer is bound (a page then needs no runtime).
func (r *Runtime) Context(ctx context.Context) (*RuntimeContext, error) {
	ix, err := r.Index(ctx)
	if err != nil || ix.Shell == nil {
		return nil, err
	}
	sub, signedIn := signedInSubject(ctx)
	if !signedIn {
		// No signed-in user: there is nothing to bootstrap.
		return nil, nil //nolint:nilnil // A nil context is the documented "no runtime needed" answer, as for an unbound shell.
	}
	lang := listext.LangFromContext(ctx)
	rc := &RuntimeContext{
		Version:           RuntimeContextVersion,
		ProtocolVersion:   clienthost.ProtocolVersion,
		SDKReleaseVersion: r.cfg.Host.SDKReleaseVersion,
		SDKCommit:         r.cfg.Host.SDKCommit,
		RPC:               RPCPath,
		Locale:            lang,
		Permissions:       append([]string{}, sub.Permissions...),
		User:              runtimeUser(ctx),
		Shell: &ShellRef{
			ID: ix.Shell.ID, Package: ix.Shell.Package, Version: ix.Shell.Version,
			Module: ix.Shell.AssetPath(), Export: ix.Shell.Export, Binding: ix.Shell.BindingID, Styles: ix.Shell.Styles,
		},
		Navigation: RuntimeNavigation{Workspaces: []WorkspaceRef{}, Palette: Palette(ix, sub, lang)},
		Slots:      r.visibleSlots(ctx, ix, sub),
	}
	for _, w := range ix.Navigation.Workspaces {
		rc.Navigation.Workspaces = append(rc.Navigation.Workspaces, WorkspaceRef{ID: w.ID, Label: listext.LabelFor(w.Label, lang, w.ID), Icon: w.Icon, Order: w.Order})
	}
	sort.SliceStable(rc.Navigation.Workspaces, func(i, j int) bool { return rc.Navigation.Workspaces[i].Order < rc.Navigation.Workspaces[j].Order })
	return rc, nil
}

// Markup is the layout contributor that activates the bound shell on every
// authenticated page, legacy Templ and Solid alike: the runtime context, the
// host stylesheet and the runtime host entry module. Closing the shell
// binding makes it render nothing, which returns the standard shell.
func (r *Runtime) Markup(ctx context.Context) templ.Component {
	rc, err := r.Context(ctx)
	if err != nil {
		r.cfg.Logger.WithError(err).Error("extruntime ui: shell context")
		return nil
	}
	if rc == nil {
		return nil
	}
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		b, err := json.Marshal(rc)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, `<script id="%s" type="application/json">%s</script>`, RuntimeContextElementID, b); err != nil {
			return err
		}
		// The host is written once per request, whichever of the shell and
		// the first island comes first.
		if isl, ok := shellext.IslandsFrom(ctx); ok {
			if ri, ok := isl.(*requestIslands); ok {
				return ri.writeHost(w)
			}
		}
		return r.writeHost(w)
	})
}
