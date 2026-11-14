package roles

import (
	"strings"
	"unicode"

	"github.com/iota-uz/iota-sdk/pkg/types"
)

// ModuleLabel returns the translated permission module name. It matches the
// Modules.* keys regardless of the case the module was registered with
// ("core" resolves to Modules.Core) and humanizes unknown modules instead of
// showing the raw identifier.
func ModuleLabel(pageCtx types.PageContext, module string) string {
	if module == "" {
		return ""
	}
	for _, candidate := range []string{module, upperFirst(module), strings.ToUpper(module)} {
		if label := pageCtx.TSafe("Modules." + candidate); label != "" {
			return label
		}
	}
	return Humanize(module)
}

// ResourceLabel returns the translated permission resource name, humanizing
// unknown resources ("notification_rules" -> "Notification rules").
func ResourceLabel(pageCtx types.PageContext, resource string) string {
	if resource == "" {
		return ""
	}
	if label := pageCtx.TSafe("Resources." + resource); label != "" {
		return label
	}
	return Humanize(resource)
}

// Humanize turns a machine identifier into a readable label.
func Humanize(identifier string) string {
	words := strings.Fields(strings.NewReplacer("_", " ", "-", " ", ".", " ").Replace(identifier))
	if len(words) == 0 {
		return identifier
	}
	return upperFirst(strings.Join(words, " "))
}

func upperFirst(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
