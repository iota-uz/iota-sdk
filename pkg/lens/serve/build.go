package serve

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

// BuildDocument constructs and stores a typed snapshot without an HTTP round trip.
// Config owns inline depth and progressive mode. Options override document
// identity, locale, endpoints, controls and presentation; zero values use defaults.
func (h *Handlers) BuildDocument(ctx context.Context, req lensruntime.Request, opts document.BuildOptions) (*document.DashboardDocument, error) {
	scope := lensruntime.DashboardScope()
	if h.progressive {
		scope = lensruntime.ShellScope()
	}
	result, err := h.engine.Execute(ctx, h.spec, req, scope)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("executor returned a nil dashboard result")
	}
	if result.Panels == nil {
		result.Panels = make(map[string]*lensruntime.PanelResult)
	}
	frozen := freezeParams(result, req)
	for _, target := range inlineTargets(h.spec, h.inlineDepth) {
		if h.progressive {
			break
		}
		existing := result.Panel(target.panel.ID)
		if existing != nil && existing.Error == nil && existing.Frames != nil && existing.Frames.Primary() != nil {
			continue
		}
		levelResult, execErr := h.executeLevel(ctx, req, frozen, target, 0)
		if execErr != nil {
			if levelResult != nil && levelResult.Error != nil {
				continue
			}
			return nil, execErr
		}
		result.Panels[target.panel.ID] = levelResult
	}
	for _, target := range sourceDataTargets(h.spec, h.inlineDepth) {
		if h.progressive {
			break
		}
		existing := result.Panel(target.panel.ID)
		if existing != nil && existing.Error == nil && existing.Frames != nil && existing.Frames.Primary() != nil {
			continue
		}
		sourceResult, execErr := h.executeSourcePanel(ctx, req, frozen, target)
		if execErr != nil {
			// A panel-level failure drops the disclosure (document.Build skips
			// declarations without an executed frame) instead of failing the
			// whole document; transport failures still surface.
			if sourceResult != nil && sourceResult.Error != nil {
				continue
			}
			return nil, execErr
		}
		result.Panels[target.panel.ID] = sourceResult
	}
	defaults := document.BuildOptions{
		Locale: result.Locale, InlineDepth: h.inlineDepth,
		I18n: document.RuntimeI18nDefaults(),
		Endpoints: document.Endpoints{
			Query: h.endpoint("/lens/query"), Export: h.endpoint("/export"),
			Release: h.endpoint("/lens/release"),
			Panel: func() string {
				if h.progressive {
					return h.endpoint("/lens/panel")
				}
				return ""
			}(),
			Drawer: func() string {
				if h.drawerResolver != nil {
					return h.endpoint("/lens/drawer")
				}
				return ""
			}(),
		},
		DeferPanels: h.progressive,
		Filters:     wireVariableFilters(result.Filters),
		URLState:    &document.URLStateContract{Version: document.URLStateVersion, Param: document.URLStateParam, MaxBytes: document.URLStateMaxBytes},
	}
	if opts.Endpoints != (document.Endpoints{}) {
		defaults.Endpoints = opts.Endpoints
	}
	if opts.Locale != "" {
		defaults.Locale = opts.Locale
	}
	if opts.I18n != nil {
		defaults.I18n = opts.I18n
	}
	if opts.Filters != nil {
		defaults.Filters = opts.Filters
	}
	defaults.Header, defaults.Drawer = opts.Header, opts.Drawer
	if opts.URLState != nil {
		defaults.URLState = opts.URLState
	}
	defaults.SnapshotID, defaults.GeneratedAt = opts.SnapshotID, opts.GeneratedAt
	defaults.Theme = opts.Theme
	doc, err := document.Build(h.spec, result, defaults)
	if err != nil {
		return nil, err
	}
	if err := h.snapshots.Put(ctx, &document.Snapshot{
		ID: doc.SnapshotID, Params: frozen, Frames: doc.Frames, Levels: doc.Drill.Edges, CreatedAt: doc.Meta.GeneratedAt,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}
