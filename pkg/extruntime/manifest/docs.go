package manifest

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Package documentation. A package ships end-user documentation as markdown
// files under docs/<locale>/<path>.md in its source tree. The build turns each
// file into a Doc of the manifest and a file of the same name in the artifact
// (front matter removed), so the documentation is part of the immutable
// version: the source and artifact hashes cover it and a reviewer approves it
// with the code.
const (
	// DocsDir is the directory of the source tree, and of the artifact, that
	// holds the documentation.
	DocsDir = "docs"
	// DefaultDocLocale is the locale every document must exist in; readers
	// whose locale has no translation get this one.
	DefaultDocLocale = "en"

	// Bounds of the documentation of one version.
	MaxDocs         = 200
	MaxDocBytes     = 256 << 10
	MaxDocsBytes    = 4 << 20
	MaxDocPathBytes = 120
	MaxDocSegments  = 5
	MaxDocTitle     = 120
	MaxDocAudience  = 60
	MaxDocOrder     = 10_000
	MaxDocPermBytes = 120
)

// Doc is one document of a package in one locale. The text is the artifact
// file DocFile(d); Path is the document's address within the package, shared
// by all its translations.
type Doc struct {
	// Path is "getting-started" or "admin/setup": lowercase segments of
	// [a-z0-9_-], no extension, no locale.
	Path string `json:"path"`
	// Locale is the lowercase language tag of the translation ("en", "ru",
	// "pt-br").
	Locale string `json:"locale"`
	// Title is plain text, shown in the Help Center sidebar and heading.
	Title string `json:"title"`
	// Order places the document among its siblings; lower first, then Title.
	Order int `json:"order,omitempty"`
	// Audience is a short plain-text hint of who the document is for.
	Audience string `json:"audience,omitempty"`
	// Permission, when set, restricts the document to holders of the
	// permission: one the package declares or a native one.
	Permission string `json:"permission,omitempty"`
}

var (
	docSegmentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	docLocalePattern  = regexp.MustCompile(`^[a-z]{2,3}(-[a-z0-9]{2,8}){0,2}$`)
	// A declared permission ID ("acme.loyalty.manage") or a native RBAC name
	// ("Client.Read").
	docPermIDPattern     = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-zA-Z0-9_]*)+$`)
	docPermNativePattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*\.[A-Z][A-Za-z0-9]*$`)
)

// ValidDocPath reports whether p is a document path: 1..MaxDocSegments
// lowercase segments of [a-z0-9_-] joined by "/".
func ValidDocPath(p string) bool {
	if p == "" || len(p) > MaxDocPathBytes {
		return false
	}
	segments := strings.Split(p, "/")
	if len(segments) > MaxDocSegments {
		return false
	}
	for _, s := range segments {
		if !docSegmentPattern.MatchString(s) {
			return false
		}
	}
	return true
}

// ValidDocLocale reports whether l is a locale directory name.
func ValidDocLocale(l string) bool { return docLocalePattern.MatchString(l) }

// ValidDocPermission reports whether p is shaped like a permission: a
// declared ID or a native RBAC name. Whether it exists is checked by whoever
// knows the catalog.
func ValidDocPermission(p string) bool {
	return len(p) <= MaxDocPermBytes && (docPermIDPattern.MatchString(p) || docPermNativePattern.MatchString(p))
}

// NativeDocPermission reports whether p is shaped like a native RBAC name
// ("Client.Read") rather than a permission ID of the catalog: the catalog
// cannot vouch for it, so it is accepted as written.
func NativeDocPermission(p string) bool { return docPermNativePattern.MatchString(p) }

// DeclaresPermission reports whether the manifest declares the permission
// resource id.
func (m *Manifest) DeclaresPermission(id string) bool {
	for _, r := range m.Resources {
		if r.Kind == ResPermission && r.ID == id {
			return true
		}
	}
	return false
}

// PlainText reports whether s is a single line of plain text of 1..limit runes,
// without surrounding space, control characters or angle brackets. A title is
// shown literally and never interpreted as markdown or HTML; refusing the
// brackets keeps it unambiguous wherever it is displayed.
func PlainText(s string, limit int) bool {
	if s == "" || strings.TrimSpace(s) != s || !utf8.ValidString(s) || utf8.RuneCountInString(s) > limit {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || r == '<' || r == '>' || r == 0x2028 || r == 0x2029
	})
}

// DocFile is the artifact path of the document's text.
func DocFile(d Doc) string { return DocsDir + "/" + d.Locale + "/" + d.Path + ".md" }

// DocsIn returns the documents of locale (exactly) in manifest order.
func (m *Manifest) DocsIn(locale string) []Doc {
	var out []Doc
	for _, d := range m.Docs {
		if d.Locale == locale {
			out = append(out, d)
		}
	}
	return out
}

// validateDocs checks the structure of the documentation list: the same rules
// the build applies to the source tree, so a hand-made artifact cannot forge
// what the build would refuse.
func (m *Manifest) validateDocs() error {
	if len(m.Docs) > MaxDocs {
		return fmt.Errorf("manifest: %d documents, the limit is %d", len(m.Docs), MaxDocs)
	}
	type key struct{ path, locale string }
	seen := map[key]bool{}
	perm := map[string]string{}
	defaults := map[string]bool{}
	for _, d := range m.Docs {
		where := "document " + d.Locale + "/" + d.Path
		if !ValidDocPath(d.Path) {
			return fmt.Errorf("manifest: document path %q is invalid (1 to %d segments of lowercase letters, digits, - and _)", d.Path, MaxDocSegments)
		}
		if !ValidDocLocale(d.Locale) {
			return fmt.Errorf("manifest: %s has an invalid locale %q", where, d.Locale)
		}
		if seen[key{d.Path, d.Locale}] {
			return fmt.Errorf("manifest: %s is declared twice", where)
		}
		seen[key{d.Path, d.Locale}] = true
		if !PlainText(d.Title, MaxDocTitle) {
			return fmt.Errorf("manifest: %s has an invalid title (one line of plain text, 1 to %d characters, no < or >)", where, MaxDocTitle)
		}
		if d.Audience != "" && !PlainText(d.Audience, MaxDocAudience) {
			return fmt.Errorf("manifest: %s has an invalid audience (one line of plain text, up to %d characters, no < or >)", where, MaxDocAudience)
		}
		if d.Order < -MaxDocOrder || d.Order > MaxDocOrder {
			return fmt.Errorf("manifest: %s has order %d, outside ±%d", where, d.Order, MaxDocOrder)
		}
		if d.Permission != "" {
			if !ValidDocPermission(d.Permission) {
				return fmt.Errorf("manifest: %s has an invalid permission %q", where, d.Permission)
			}
			// A permission in the package's own namespace must be declared by it.
			if strings.HasPrefix(d.Permission, m.Namespace+".") && !m.DeclaresPermission(d.Permission) {
				return fmt.Errorf("manifest: %s requires permission %q, which the package does not declare", where, d.Permission)
			}
		}
		if prior, ok := perm[d.Path]; ok && prior != d.Permission {
			return fmt.Errorf("manifest: document %s has different permissions in its translations (%q and %q)", d.Path, prior, d.Permission)
		}
		perm[d.Path] = d.Permission
		if d.Locale == DefaultDocLocale {
			defaults[d.Path] = true
		}
	}
	for _, d := range m.Docs {
		if !defaults[d.Path] {
			return fmt.Errorf("manifest: document %s/%s has no %s version (every document exists in %s)", d.Locale, d.Path, DefaultDocLocale, DefaultDocLocale)
		}
	}
	return nil
}

// ValidateDocFiles checks that the artifact holds exactly the documents the
// manifest lists: every Doc has its file, within the size bounds, valid
// UTF-8 without NUL; and nothing else sits under docs/. files maps an
// artifact path to its content.
func ValidateDocFiles(m Manifest, files map[string][]byte) error {
	var total int
	listed := map[string]bool{}
	for _, d := range m.Docs {
		p := DocFile(d)
		listed[p] = true
		body, ok := files[p]
		if !ok {
			return fmt.Errorf("manifest: document %s is listed but the artifact has no %s", d.Locale+"/"+d.Path, p)
		}
		if len(body) > MaxDocBytes {
			return fmt.Errorf("%s is %d bytes, the limit is %d", p, len(body), MaxDocBytes)
		}
		if !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
			return fmt.Errorf("%s is not valid UTF-8 text", p)
		}
		total += len(body)
	}
	if total > MaxDocsBytes {
		return fmt.Errorf("the documentation is %d bytes, the limit is %d", total, MaxDocsBytes)
	}
	for p := range files {
		if strings.HasPrefix(p, DocsDir+"/") && !listed[p] {
			return fmt.Errorf("the artifact has %s, which the manifest does not list", p)
		}
	}
	return nil
}
