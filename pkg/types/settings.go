package types

import (
	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

// SettingsSection groups related settings pages under a category heading.
type SettingsSection struct {
	Name     string // i18n key for category header
	Priority int    // Sort order (lower = first)
	Pages    []SettingsPage
}

// SettingsPage represents a single page in the settings compartment.
type SettingsPage struct {
	Name        string // i18n key for display name
	Description string // i18n key for hub page card description
	Href        string // URL path
	Icon        templ.Component
	Permissions []permission.Permission
	Children    []NavigationItem // Optional sub-navigation for sidebar
}
