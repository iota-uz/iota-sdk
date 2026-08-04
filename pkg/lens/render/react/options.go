package react

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
)

type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

type Option func(*dashboardOptions)

type dashboardOptions struct {
	Locale          string
	Theme           Theme
	CSRF            string
	AssetBasePath   string
	IncludeAssets   bool
	EntryURL        string
	Stylesheets     []string
	Skeleton        *lens.DashboardSpec
	InitialDocument *document.DashboardDocument
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

// WithInitialDocument embeds a lightweight, already-materialized document in
// the custom element. Progressive dashboards use it to paint their real
// header, filters, panel titles and stable grid on the first React frame while
// deferred panel bodies hydrate independently.
func WithInitialDocument(doc *document.DashboardDocument) Option {
	return func(options *dashboardOptions) {
		options.InitialDocument = doc
	}
}

func initialDocumentAttribute(doc *document.DashboardDocument) string {
	if doc == nil {
		return ""
	}
	payload, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(payload)
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

	if resolved.IncludeAssets {
		assets := Assets()
		resolved.EntryURL = versionedAssetURL(resolved.AssetBasePath, assets.Revision, assets.Entry)
		resolved.Stylesheets = make([]string, 0, len(assets.Stylesheets))
		for _, stylesheet := range assets.Stylesheets {
			resolved.Stylesheets = append(resolved.Stylesheets, versionedAssetURL(resolved.AssetBasePath, assets.Revision, stylesheet))
		}
	}
	return resolved
}

func versionedAssetURL(basePath, revision, assetPath string) string {
	return joinAssetURL(joinAssetURL(basePath, revision), assetPath)
}

func joinAssetURL(basePath, assetPath string) string {
	return strings.TrimRight(basePath, "/") + "/" + strings.TrimLeft(assetPath, "/")
}
