package help

import (
	"strings"

	"github.com/a-h/templ"
)

const (
	defaultBasePath = "/help"
	defaultLabel    = "Open help article"
)

// LinkProps configures a contextual Help Center link.
type LinkProps struct {
	Path     string
	BasePath string
	Label    string
	Tooltip  string
	Class    any
	NewTab   bool
	Attrs    templ.Attributes
}

// DocURL resolves a Help Center article path to the route served by the help module.
func DocURL(basePath, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return resolveBasePath(basePath)
	}
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	base := strings.TrimRight(resolveBasePath(basePath), "/")
	path = strings.TrimLeft(path, "/")
	if strings.HasPrefix(path, "doc/") {
		return base + "/" + path
	}
	return base + "/doc/" + path
}

func resolveBasePath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return defaultBasePath
	}
	return basePath
}

func resolvedLabel(props LinkProps) string {
	if strings.TrimSpace(props.Label) != "" {
		return props.Label
	}
	return defaultLabel
}

func resolvedTooltip(props LinkProps) string {
	if strings.TrimSpace(props.Tooltip) != "" {
		return props.Tooltip
	}
	return resolvedLabel(props)
}

func linkAttrs(props LinkProps) templ.Attributes {
	attrs := templ.Attributes{
		"x-tooltip.raw": resolvedTooltip(props),
	}
	if props.NewTab {
		attrs["target"] = "_blank"
		attrs["rel"] = "noopener noreferrer"
	}
	for key, value := range props.Attrs {
		attrs[key] = value
	}
	return attrs
}
