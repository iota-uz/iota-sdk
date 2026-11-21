package departments

import (
	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
)

// IndexPageProps holds the data for rendering the departments index page.
type IndexPageProps struct {
	Departments []*viewmodels.Department
	Page        int
	PerPage     int
	Search      string
	HasMore     bool
	// Filters is the filter chip bar (parent, status); a nil Registry hides it.
	Filters filterbuilder.Props
}

// DepartmentFormData holds the raw, unvalidated form values used to re-render
// the create form when validation fails.
type DepartmentFormData struct {
	Name     map[string]string
	Code     string
	ParentID string
	Order    string
	Status   string
}

// NameForLocale returns the entered name for a locale, tolerating a nil map.
func (d *DepartmentFormData) NameForLocale(locale string) string {
	if d.Name == nil {
		return ""
	}
	return d.Name[locale]
}
