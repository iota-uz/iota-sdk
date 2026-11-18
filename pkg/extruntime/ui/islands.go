package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"sort"
	"sync/atomic"

	"github.com/a-h/templ"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/shellext"
)

// Attribute names of the island mount elements; the browser host
// (web/runtime/src/client-host/islands.ts) reads the same ones.
const (
	SlotAttr       = "data-iota-slot"
	SlotConfigAttr = "data-iota-slot-config"
	CellAttr       = "data-iota-cell"
	CellConfigAttr = "data-iota-cell-config"
)

// IslandEnv is what an island needs of the runtime. Each mount element
// carries it, so an HTMX fragment is self-contained and the page needs no
// global runtime context.
type IslandEnv struct {
	RPC               string `json:"rpc"`
	Locale            string `json:"locale"`
	ProtocolVersion   string `json:"protocolVersion"`
	SDKReleaseVersion string `json:"sdkReleaseVersion"`
	SDKCommit         string `json:"sdkCommit"`
}

// slotConfig is the JSON of a slot mount element.
type slotConfig struct {
	Props         json.RawMessage   `json:"props"`
	Contributions []shellext.Module `json:"contributions"`
	Env           IslandEnv         `json:"env"`
}

// cellConfig is the JSON of a cell mount element.
type cellConfig struct {
	Contribution shellext.Module `json:"contribution"`
	Value        any             `json:"value"`
	Row          map[string]any  `json:"row"`
	Env          IslandEnv       `json:"env"`
}

// requestIslands is the per-request islands host: it emits the runtime host
// (styles and entry module) once, with the first island of the request.
type requestIslands struct {
	rt          *Runtime
	hostEmitted atomic.Bool
}

var _ shellext.Islands = (*requestIslands)(nil)

func (r *Runtime) newRequestIslands() *requestIslands { return &requestIslands{rt: r} }

func (r *Runtime) env(ctx context.Context) IslandEnv {
	return IslandEnv{
		RPC: RPCPath, Locale: listext.LangFromContext(ctx),
		ProtocolVersion: clienthost.ProtocolVersion, SDKReleaseVersion: r.cfg.Host.SDKReleaseVersion, SDKCommit: r.cfg.Host.SDKCommit,
	}
}

// slotDeclaration resolves the declaration of a slot. A slot is declared by
// native Go (a native resource) or by an activated package (defineSlot); a
// page rendering a slot nobody declared is a programming error, reported and
// rendered empty (ok false). Without a catalog nothing can be checked: the
// declaration is nil and the contributions are mounted as they are.
func (r *Runtime) slotDeclaration(ctx context.Context, id string) (*manifest.Resource, bool) {
	if r.cfg.Catalog == nil {
		return nil, true
	}
	resolved, found, err := r.cfg.Catalog.Resolve(ctx, id)
	if err != nil {
		r.cfg.Logger.WithError(err).Error("extruntime ui: resolve slot " + id)
		return nil, false
	}
	if !found || resolved.Resource.Kind != manifest.ResSlot || resolved.Resource.Target != "" {
		r.cfg.Logger.Warn("extruntime ui: slot " + id + " is not declared; its contributions are not mounted")
		return nil, false
	}
	return &resolved.Resource, true
}

// slotModules is what the subject gets in a slot: nothing unless the
// declaration admits the subject, otherwise every contribution whose own
// permission the subject holds, in mounting order. It returns the
// declaration (nil when it cannot be checked) for the props check.
func (r *Runtime) slotModules(ctx context.Context, ix *Index, sub spi.Subject, id string) ([]shellext.Module, *manifest.Resource) {
	list := ix.Slots[id]
	if len(list) == 0 {
		return nil, nil
	}
	decl, ok := r.slotDeclaration(ctx, id)
	if !ok || (decl != nil && !sub.Has(decl.Permission)) {
		return nil, decl
	}
	mods := make([]shellext.Module, 0, len(list))
	for _, c := range list {
		if sub.Has(c.Permission) {
			mods = append(mods, c.Module())
		}
	}
	return mods, decl
}

// visibleSlots lists the contributions of every slot with at least one the
// subject gets, for the contexts a Solid page or the shell renderer reads.
func (r *Runtime) visibleSlots(ctx context.Context, ix *Index, sub spi.Subject) map[string][]shellext.Module {
	ids := make([]string, 0, len(ix.Slots))
	for id := range ix.Slots {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := map[string][]shellext.Module{}
	for _, id := range ids {
		if mods, _ := r.slotModules(ctx, ix, sub, id); len(mods) > 0 {
			out[id] = mods
		}
	}
	return out
}

// Slot implements shellext.Islands.
func (i *requestIslands) Slot(ctx context.Context, id string, props json.RawMessage) templ.Component {
	rt := i.rt
	sub, signedIn := signedInSubject(ctx)
	if !signedIn {
		return nil
	}
	ix, err := rt.Index(ctx)
	if err != nil {
		rt.cfg.Logger.WithError(err).Error("extruntime ui: read snapshot")
		return nil
	}
	mods, decl := rt.slotModules(ctx, ix, sub, id)
	if len(mods) == 0 {
		return nil
	}
	// The host page's props are validated against the declared type before
	// any contribution sees them.
	if decl != nil && decl.Props != nil {
		if err := manifest.ValidateValue(*decl.Props, props); err != nil {
			rt.cfg.Logger.WithError(err).Error("extruntime ui: props of slot " + id + " do not match its declaration")
			return nil
		}
	}
	cfg, err := json.Marshal(slotConfig{Props: props, Contributions: mods, Env: rt.env(ctx)})
	if err != nil {
		rt.cfg.Logger.WithError(err).Error("extruntime ui: slot config")
		return nil
	}
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		if err := i.writeHost(w); err != nil {
			return err
		}
		_, err := fmt.Fprintf(w, `<div %s="%s" %s="%s" hx-disable></div>`,
			SlotAttr, template.HTMLEscapeString(id), SlotConfigAttr, template.HTMLEscapeString(string(cfg)))
		return err
	})
}

// Cell implements shellext.Islands.
func (i *requestIslands) Cell(ctx context.Context, cell shellext.Cell, fallback templ.Component) templ.Component {
	rt := i.rt
	if _, signedIn := signedInSubject(ctx); !signedIn {
		return nil
	}
	cfg, err := json.Marshal(cellConfig{Contribution: cell.Renderer, Value: cell.Value, Row: cell.Row, Env: rt.env(ctx)})
	if err != nil {
		rt.cfg.Logger.WithError(err).Error("extruntime ui: cell config")
		return nil
	}
	return templ.ComponentFunc(func(rctx context.Context, w io.Writer) error {
		if err := i.writeHost(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, `<span %s="%s" %s="%s" hx-disable>`,
			CellAttr, template.HTMLEscapeString(cell.Renderer.ID), CellConfigAttr, template.HTMLEscapeString(string(cfg))); err != nil {
			return err
		}
		if err := fallback.Render(rctx, w); err != nil {
			return err
		}
		_, err := io.WriteString(w, `</span>`)
		return err
	})
}

// writeHost emits the host stylesheet and entry module with the first island
// (or the shell markup) of the request; later ones find it done. A module
// script runs once per document however often it is written, so a fragment
// that repeats it is harmless.
func (i *requestIslands) writeHost(w io.Writer) error {
	if i.hostEmitted.Swap(true) {
		return nil
	}
	return i.rt.writeHost(w)
}

// writeHost writes the stylesheets and the entry module of the runtime host.
func (r *Runtime) writeHost(w io.Writer) error {
	m := r.cfg.Host
	for _, style := range m.Styles {
		if _, err := fmt.Fprintf(w, `<link rel="stylesheet" href="%s">`, template.HTMLEscapeString(style)); err != nil {
			return err
		}
	}
	integrity := ""
	if v := m.Integrity[m.Entry]; v != "" {
		integrity = ` integrity="` + template.HTMLEscapeString(v) + `" crossorigin="anonymous"`
	}
	_, err := fmt.Fprintf(w, `<script type="module" src="%s" data-sdk-commit="%s" data-package="%s"%s></script>`,
		template.HTMLEscapeString(m.Entry), template.HTMLEscapeString(m.SDKCommit), template.HTMLEscapeString(m.Package), integrity)
	return err
}
