// Package settings resolves the settings hub registry for the current user.
//
// Components contribute sections at build time through
// composition.ContributeSettings. Resolve turns the merged registry into a
// permission-filtered, translated view for the user in the request context,
// so the hub page and any other consumer (for example a dashboard catalog)
// share one source of truth.
package settings

import (
	"context"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// Source provides the merged, sorted settings registry with untranslated
// keys. *composition.Container implements it.
type Source interface {
	SettingsSections() []types.SettingsSection
}

// SourceFunc adapts a function to Source.
type SourceFunc func() []types.SettingsSection

func (f SourceFunc) SettingsSections() []types.SettingsSection {
	if f == nil {
		return nil
	}
	return f()
}

// ResolvedSection is a settings section visible to the current user, with
// its name translated.
type ResolvedSection struct {
	Key      string
	Name     string
	Priority int
	Pages    []ResolvedPage
}

// ResolvedPage is a settings page visible to the current user, with its name
// and description translated.
type ResolvedPage struct {
	Key         string
	Name        string
	Description string
	Href        string
	Icon        templ.Component
	Order       int
}

// Resolve returns the sections from source that the user in ctx may see.
// Pages the user lacks permission for are dropped, sections left without
// pages are omitted, and names and descriptions are translated with the
// localizer in ctx, falling back to the raw value. Without a user in ctx the
// result is empty. Source ordering is preserved.
func Resolve(ctx context.Context, source Source) []ResolvedSection {
	if source == nil {
		return nil
	}
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return nil
	}
	sections := source.SettingsSections()
	out := make([]ResolvedSection, 0, len(sections))
	for _, section := range sections {
		pages := make([]ResolvedPage, 0, len(section.Pages))
		for _, page := range section.Pages {
			if !page.HasPermission(u) {
				continue
			}
			pages = append(pages, ResolvedPage{
				Key:         page.Key,
				Name:        translate(ctx, page.Name),
				Description: translate(ctx, page.Description),
				Href:        page.Href,
				Icon:        page.Icon,
				Order:       page.Order,
			})
		}
		if len(pages) == 0 {
			continue
		}
		out = append(out, ResolvedSection{
			Key:      section.Key,
			Name:     translate(ctx, section.Name),
			Priority: section.Priority,
			Pages:    pages,
		})
	}
	return out
}

// ResolveFromContext resolves the registry of the composition container
// stored in ctx by the server middleware.
func ResolveFromContext(ctx context.Context) ([]ResolvedSection, error) {
	const op serrors.Op = "settings.ResolveFromContext"
	container, err := composition.UseContainer(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return Resolve(ctx, container), nil
}

func translate(ctx context.Context, key string) string {
	if key == "" {
		return ""
	}
	return intl.TWithDefault(ctx, key, key)
}

// Well-known section keys shared by SDK modules. Applications may contribute
// pages to these sections or declare their own.
const (
	SectionGeneral      = "general"
	SectionAccess       = "access"
	SectionOrganization = "organization"
)

// GeneralSection returns the general section holding pages.
func GeneralSection(pages ...types.SettingsPage) types.SettingsSection {
	return types.SettingsSection{Key: SectionGeneral, Name: "Settings.Hub.Sections.General", Priority: 10, Pages: pages}
}

// AccessSection returns the users and access section holding pages.
func AccessSection(pages ...types.SettingsPage) types.SettingsSection {
	return types.SettingsSection{Key: SectionAccess, Name: "Settings.Hub.Sections.Access", Priority: 20, Pages: pages}
}

// OrganizationSection returns the organization structure section holding pages.
func OrganizationSection(pages ...types.SettingsPage) types.SettingsSection {
	return types.SettingsSection{Key: SectionOrganization, Name: "Settings.Hub.Sections.Organization", Priority: 30, Pages: pages}
}
