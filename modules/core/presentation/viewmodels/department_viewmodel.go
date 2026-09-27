// Package viewmodels provides this package.
package viewmodels

import "strings"

// Department is the presentation model for a department aggregate. Name holds
// the display value resolved in the request locale, while NameI18n holds the
// per-locale values used to pre-fill the edit form.
type Department struct {
	ID         string
	Code       string
	Name       string
	NameI18n   map[string]string
	ParentID   string
	ParentName string
	Order      string
	Status     string
	CreatedAt  string
	UpdatedAt  string
	CanUpdate  bool
	CanDelete  bool
}

// NameForLocale returns the stored name for a specific locale, falling back to
// an empty string when the locale is missing. Used by the edit form to render
// one input per locale.
//
// NameI18n keys are normalized to lowercase by models.MultiLang.GetAll (e.g.
// "uz-cyrl"), while callers pass UI locale codes from intl.GetSupportedLanguages
// such as "uz-Cyrl". The lookup key is lowercased so cased locales round-trip
// and the edit form pre-fills every translation.
func (d *Department) NameForLocale(locale string) string {
	if d.NameI18n == nil {
		return ""
	}
	return d.NameI18n[strings.ToLower(locale)]
}

// GetInitials returns the first letters of each word in the department name.
func (d *Department) GetInitials() string {
	if d.Name == "" {
		return ""
	}

	words := strings.Fields(d.Name)
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
