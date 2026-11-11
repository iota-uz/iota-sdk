package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"sort"

	"github.com/a-h/templ"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/listext"
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
}

// ShellRef identifies the shell renderer module to load.
type ShellRef struct {
	ID      string `json:"id"`
	Package string `json:"package"`
	Version string `json:"version"`
	Module  string `json:"module"`
	Export  string `json:"export,omitempty"`
	Binding string `json:"binding"`
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

// Context builds the runtime context for a request, or nil when no shell
// renderer is bound (a page then needs no runtime).
func (r *Runtime) Context(ctx context.Context) (*RuntimeContext, error) {
	ix, err := r.Index(ctx)
	if err != nil || ix.Shell == nil {
		return nil, err
	}
	sub, err := listext.SubjectFromContext(ctx)
	if err != nil {
		return nil, nil
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
			Module: ix.Shell.AssetPath(), Export: ix.Shell.Export, Binding: ix.Shell.BindingID,
		},
		Navigation: RuntimeNavigation{Workspaces: []WorkspaceRef{}, Palette: Palette(ix, sub, lang)},
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
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		b, err := json.Marshal(rc)
		if err != nil {
			return err
		}
		m := r.cfg.Host
		for _, style := range m.Styles {
			if _, err := fmt.Fprintf(w, `<link rel="stylesheet" href="%s">`, template.HTMLEscapeString(style)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, `<script id="%s" type="application/json">%s</script>`, RuntimeContextElementID, b); err != nil {
			return err
		}
		integrity := ""
		if v := m.Integrity[m.Entry]; v != "" {
			integrity = ` integrity="` + template.HTMLEscapeString(v) + `" crossorigin="anonymous"`
		}
		_, err = fmt.Fprintf(w, `<script type="module" src="%s" data-sdk-commit="%s" data-package="%s"%s></script>`,
			template.HTMLEscapeString(m.Entry), template.HTMLEscapeString(m.SDKCommit), template.HTMLEscapeString(m.Package), integrity)
		return err
	})
}
