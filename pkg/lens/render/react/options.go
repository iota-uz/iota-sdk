package react

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
)

type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

type Option func(*dashboardOptions)

type dashboardOptions struct {
	Locale        string
	Theme         Theme
	CSRF          string
	AssetBasePath string
	IncludeAssets bool
	EntryURL      string
	Stylesheets   []string
	Skeleton      *lens.DashboardSpec
}

// WithSkeleton renders the prepared dashboard's layout-shaped placeholder
// inside the mount point. The runtime keeps it on screen until the first
// document arrives, so the page never shows a bare spinner and the grid does
// not jump when the data lands.
func WithSkeleton(spec lens.DashboardSpec) Option {
	return func(options *dashboardOptions) {
		options.Skeleton = &spec
	}
}

func WithLocale(locale string) Option {
	return func(options *dashboardOptions) {
		if value := strings.TrimSpace(locale); value != "" {
			options.Locale = value
		}
	}
}

func WithTheme(theme Theme) Option {
	return func(options *dashboardOptions) {
		if theme == ThemeDark {
			options.Theme = ThemeDark
		} else {
			options.Theme = ThemeLight
		}
	}
}

func WithCSRF(token string) Option {
	return func(options *dashboardOptions) {
		options.CSRF = token
	}
}

func WithAssetBasePath(basePath string) Option {
	return func(options *dashboardOptions) {
		options.AssetBasePath = normalizeAssetBasePath(basePath)
	}
}

func WithoutAssets() Option {
	return func(options *dashboardOptions) {
		options.IncludeAssets = false
	}
}

func resolveDashboardOptions(options ...Option) dashboardOptions {
	resolved := dashboardOptions{
		Locale:        "en",
		Theme:         ThemeLight,
		AssetBasePath: DefaultAssetBasePath,
		IncludeAssets: true,
	}
	for _, option := range options {
		if option != nil {
			option(&resolved)
		}
	}

	assets := Assets()
	resolved.EntryURL = joinAssetURL(resolved.AssetBasePath, assets.Entry)
	resolved.Stylesheets = make([]string, 0, len(assets.Stylesheets))
	for _, stylesheet := range assets.Stylesheets {
		resolved.Stylesheets = append(resolved.Stylesheets, joinAssetURL(resolved.AssetBasePath, stylesheet))
	}
	return resolved
}

func joinAssetURL(basePath, assetPath string) string {
	return strings.TrimRight(basePath, "/") + "/" + strings.TrimLeft(assetPath, "/")
}
