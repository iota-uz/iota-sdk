package templ

import (
	"strings"

	templpkg "github.com/a-h/templ"
	icons "github.com/iota-uz/icons/phosphor"
)

func dashboardBodyClass(className string) string {
	trimmed := strings.TrimSpace(className)
	if trimmed == "" {
		return "space-y-6"
	}
	return trimmed
}

func pageShellIcon(icon templpkg.Component) templpkg.Component {
	if icon != nil {
		return icon
	}
	return icons.ChartBar(icons.Props{Size: "24", Class: "text-white"})
}
