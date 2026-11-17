package roles

import (
	"strings"
	"unicode"

	"github.com/iota-uz/iota-sdk/pkg/types"
)

// ModuleLabel returns the translated permission module name. It looks up the
// exact Modules.* key first, then case-normalized variants ("CORE", "cOrE" and
// "core" all resolve to Modules.Core), and humanizes unknown modules instead of
// showing the raw identifier.
func ModuleLabel(pageCtx types.PageContext, module string) string {
	if module == "" {
		return ""
	}
	if label := translate(pageCtx, "Modules.", module); label != "" {
		return label
	}
	return Humanize(module)
}

// ResourceLabel returns the translated permission resource name, looking up
// the exact Resources.* key first and then case-normalized variants, and
// humanizing unknown resources ("notification_rules" -> "Notification rules").
func ResourceLabel(pageCtx types.PageContext, resource string) string {
	if resource == "" {
		return ""
	}
	if label := translate(pageCtx, "Resources.", resource); label != "" {
		return label
	}
	return Humanize(resource)
}

func translate(pageCtx types.PageContext, prefix, name string) string {
	lower := strings.ToLower(name)
	seen := make(map[string]struct{}, 4)
	for _, candidate := range []string{name, lower, upperFirst(lower), strings.ToUpper(name)} {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if label := pageCtx.TSafe(prefix + candidate); label != "" {
			return label
		}
	}
	return ""
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
