package types

import (
	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

// SettingsSection groups related settings pages under a category heading.
//
// Sections are contributed at build time through composition.ContributeSettings.
// Contributions sharing a Key are merged into one section, so several modules
// can add pages to the same heading. Name is an i18n message ID translated at
// render time; a value without a translation is shown as is.
type SettingsSection struct {
	Key      string // stable identifier; contributions with the same Key merge
	Name     string // i18n message ID for the category header
	Priority int    // sort order (lower = first), ties broken by Key
	Pages    []SettingsPage
}

// SettingsPage represents a single page in the settings compartment.
//
// Name and Description are i18n message IDs translated at render time; a value
// without a translation is shown as is. A page without Permissions is visible
// to every authenticated user. Logic selects whether the user needs all
// Permissions (the zero value) or any of them.
type SettingsPage struct {
	Key         string // stable identifier, unique across all sections
	Name        string // i18n message ID for the display name
	Description string // i18n message ID for the hub card description
	Href        string // URL path
	Icon        templ.Component
	Order       int // sort order within the section (lower = first), ties broken by Key
	Permissions []permission.Permission
	Logic       PermissionLogic
	Children    []NavigationItem // Optional sub-navigation for sidebar
}

// HasPermission reports whether u may see the page.
func (p SettingsPage) HasPermission(u user.User) bool {
	if u == nil {
		return false
	}
	return NavigationItem{Permissions: p.Permissions, Logic: p.Logic}.HasPermission(u)
}
