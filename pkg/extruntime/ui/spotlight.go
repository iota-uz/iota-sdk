package ui

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

// SpotlightProvider returns the provider that exposes palette entries of
// activated navigation resources to the spotlight index: one navigation
// document per entry whose target page has a fixed path, restricted to the
// permission of the target page.
func (r *Runtime) SpotlightProvider() spotlight.SearchProvider { return paletteProvider{r} }

type paletteProvider struct{ r *Runtime }

func (paletteProvider) ProviderID() string { return "extruntime.palette" }

func (paletteProvider) Capabilities() spotlight.ProviderCapabilities {
	return spotlight.ProviderCapabilities{EntityTypes: []string{"extension_palette"}}
}

func (p paletteProvider) StreamDocuments(ctx context.Context, scope spotlight.ProviderScope, emit spotlight.DocumentBatchEmitter) error {
	ix, err := p.r.Index(ctx)
	if err != nil {
		return err
	}
	docs := make([]spotlight.SearchDocument, 0, len(ix.Navigation.Palette))
	for _, e := range ix.Navigation.Palette {
		pattern, ok := ix.resolveRoutePattern(e.Target)
		if !ok || containsParam(pattern) {
			continue
		}
		access := spotlight.AccessPolicy{Visibility: spotlight.VisibilityPublic}
		if page, isPage := ix.PageByID(e.Target); isPage && page.Permission != "" {
			access = spotlight.AccessPolicy{Visibility: spotlight.VisibilityRestricted, AllowedPermissions: []string{page.Permission}}
		}
		title := listext.LabelFor(e.Label, scope.Language, e.ID)
		docs = append(docs, spotlight.SearchDocument{
			ID:         fmt.Sprintf("extruntime:palette:%s", e.ID),
			TenantID:   scope.TenantID,
			Provider:   "extruntime.palette",
			EntityType: "extension_palette",
			Domain:     spotlight.ResultDomainNavigate,
			Title:      title,
			SearchText: title,
			ExactTerms: append([]string{title}, e.Keywords...),
			URL:        pattern,
			Language:   scope.Language,
			Metadata:   map[string]string{"source": "extruntime.palette", "package": e.Package},
			Access:     access,
		})
	}
	if len(docs) == 0 {
		return nil
	}
	return emit(docs)
}
