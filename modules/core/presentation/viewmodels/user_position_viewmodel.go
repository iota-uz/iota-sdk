// Package viewmodels provides this package.
package viewmodels

import "strings"

// UserPosition is the presentation model for a user position aggregate. Title
// holds the display value resolved in the request locale, while TitleI18n holds
// the per-locale values used to pre-fill the edit form. UserName and
// DepartmentName are resolved display labels for the linked references.
type UserPosition struct {
	ID             string
	UserID         string
	UserName       string
	DepartmentID   string
	DepartmentName string
	Title          string
	TitleI18n      map[string]string
	IsManager      bool
	IsPrimary      bool
	Status         string
	CreatedAt      string
	UpdatedAt      string
	CanUpdate      bool
	CanDelete      bool
}

// TitleForLocale returns the stored title for a specific locale, falling back
// to an empty string when the locale is missing. Used by the edit form to
// render one input per locale.
//
// TitleI18n keys are normalized to lowercase by models.MultiLang.GetAll (e.g.
// "uz-cyrl"), while callers pass UI locale codes from intl.GetSupportedLanguages
// such as "uz-Cyrl". The lookup key is lowercased so cased locales round-trip
// and the edit form pre-fills every translation.
func (p *UserPosition) TitleForLocale(locale string) string {
	if p.TitleI18n == nil {
		return ""
	}
	return p.TitleI18n[strings.ToLower(locale)]
}

// GetInitials returns the first letters of each word in the position title.
func (p *UserPosition) GetInitials() string {
	if p.Title == "" {
		return ""
	}

	words := strings.Fields(p.Title)
	if len(words) == 0 {
		return ""
	}

	if len(words) == 1 {
		firstWord := []rune(words[0])
		if len(firstWord) > 0 {
			return strings.ToUpper(string(firstWord[0]))
		}
		return ""
	}

	firstWord := []rune(words[0])
	lastWord := []rune(words[len(words)-1])

	initials := ""
	if len(firstWord) > 0 {
		initials += strings.ToUpper(string(firstWord[0]))
	}
	if len(lastWord) > 0 {
		initials += strings.ToUpper(string(lastWord[0]))
	}

	return initials
}
