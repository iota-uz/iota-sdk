package positions

import "github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"

// IndexPageProps holds the data for rendering the positions index page.
type IndexPageProps struct {
	Positions []*viewmodels.UserPosition
	Page      int
	PerPage   int
	Search    string
	HasMore   bool
}

// UserOption is a lightweight user choice for the user picker.
type UserOption struct {
	ID   string
	Name string
}

// DepartmentOption is a lightweight department choice for the department picker.
type DepartmentOption struct {
	ID   string
	Name string
}

// PositionFormData holds the raw, unvalidated form values used to re-render the
// create form when validation fails.
type PositionFormData struct {
	Title        map[string]string
	UserID       string
	DepartmentID string
	IsManager    bool
	IsPrimary    bool
	Status       string
}

// TitleForLocale returns the entered title for a locale, tolerating a nil map.
func (p *PositionFormData) TitleForLocale(locale string) string {
	if p.Title == nil {
		return ""
	}
	return p.Title[locale]
}
