package composition

import (
	"fmt"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/types"
)

// ContributeSettings attaches settings hub sections. Sections sharing a Key
// are merged across components; page keys must be unique across the whole
// registry. Section and page Name/Description are i18n message IDs that are
// translated at render time.
func ContributeSettings(builder *Builder, factory func(*Container) ([]types.SettingsSection, error)) {
	appendFactory(builder, "settings", factory, &builder.settingsFactories)
}

// AddSettings attaches static settings hub sections without a closure.
func AddSettings(builder *Builder, sections ...types.SettingsSection) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if len(sections) == 0 {
		return
	}
	captured := cloneSettingsSections(sections)
	ContributeSettings(builder, func(*Container) ([]types.SettingsSection, error) {
		return captured, nil
	})
}

// RemoveSettingsPagesByKey removes contributed settings pages with matching
// keys after every component has contributed. A section left without pages
// is omitted. To move a page, remove its key and contribute a page under a
// new key in the target section.
func RemoveSettingsPagesByKey(builder *Builder, keys ...string) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		builder.settingsPageRemovals = append(builder.settingsPageRemovals, key)
	}
}

func mergeSettingsSections(contributions []types.SettingsSection, removals []string) ([]types.SettingsSection, error) {
	if len(contributions) == 0 {
		return nil, nil
	}
	removed := stringSet(removals)
	byKey := make(map[string]int, len(contributions))
	pageSections := make(map[string]string)
	merged := make([]types.SettingsSection, 0, len(contributions))
	for _, contribution := range contributions {
		sectionKey := strings.TrimSpace(contribution.Key)
		if sectionKey == "" {
			return nil, fmt.Errorf("composition: settings section %q has no key", contribution.Name)
		}
		idx, ok := byKey[sectionKey]
		if !ok {
			idx = len(merged)
			byKey[sectionKey] = idx
			merged = append(merged, types.SettingsSection{
				Key:      sectionKey,
				Name:     contribution.Name,
				Priority: contribution.Priority,
			})
		} else if merged[idx].Name == "" {
			merged[idx].Name = contribution.Name
		}
		for _, page := range contribution.Pages {
			pageKey := strings.TrimSpace(page.Key)
			if pageKey == "" {
				return nil, fmt.Errorf("composition: settings page %q in section %q has no key", page.Name, sectionKey)
			}
			if strings.TrimSpace(page.Href) == "" {
				return nil, fmt.Errorf("composition: settings page %q in section %q has no href", pageKey, sectionKey)
			}
			if owner, dup := pageSections[pageKey]; dup {
				return nil, fmt.Errorf("composition: duplicate settings page %q (sections %q and %q); remove it with composition.RemoveSettingsPagesByKey and contribute the replacement under a new key", pageKey, owner, sectionKey)
			}
			pageSections[pageKey] = sectionKey
			if _, drop := removed[pageKey]; drop {
				continue
			}
			page.Key = pageKey
			merged[idx].Pages = append(merged[idx].Pages, clonePage(page))
		}
	}

	out := merged[:0]
	for _, section := range merged {
		if len(section.Pages) == 0 {
			continue
		}
		sort.SliceStable(section.Pages, func(i, j int) bool {
			a, b := section.Pages[i], section.Pages[j]
			if a.Order != b.Order {
				return a.Order < b.Order
			}
			return a.Key < b.Key
		})
		out = append(out, section)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].Key < out[j].Key
	})
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func cloneSettingsSections(sections []types.SettingsSection) []types.SettingsSection {
	if sections == nil {
		return nil
	}
	out := make([]types.SettingsSection, len(sections))
	for i, section := range sections {
		out[i] = section
		if section.Pages != nil {
			out[i].Pages = make([]types.SettingsPage, len(section.Pages))
			for j, page := range section.Pages {
				out[i].Pages[j] = clonePage(page)
			}
		}
	}
	return out
}

func clonePage(page types.SettingsPage) types.SettingsPage {
	if page.Permissions != nil {
		page.Permissions = append(page.Permissions[:0:0], page.Permissions...)
	}
	if page.Children != nil {
		page.Children = append(page.Children[:0:0], page.Children...)
	}
	return page
}
