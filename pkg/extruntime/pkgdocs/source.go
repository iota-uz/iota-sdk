// Package pkgdocs serves the end-user documentation that extension packages
// ship (manifest.Docs, the docs/ files of the version artifact) as a Help
// Center source.
//
// The source is mounted at "apps", the end-user area of the Help Center (the
// developer reference lives under "extensions"): one category per active
// package, its documents in order. What the reader sees follows the platform's
// own visibility rule for the package (reference.PackageVisible) and, per
// document, the permission its front matter names. A document the reader may
// not read does not exist for them (helpdocs.ErrNotFound). The content is
// always that of the version that is active now, so it changes with
// activation and deactivation.
package pkgdocs

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Mount is where package documentation sits in the Help Center tree. It must
// not collide with the Help Center's own content tree (getting-started,
// modules, workflows) nor with reference.Mount.
const Mount = "apps"

// Packages lists the active package versions a reader may see
// (*reference.Builder).
type Packages interface {
	VisiblePackages(ctx context.Context, sub spi.Subject) ([]catalog.ActiveVersion, error)
}

// Files reads a file of a build artifact (*artifact.Store).
type Files interface {
	ReadFile(ctx context.Context, artifactHash, path string) ([]byte, error)
}

// Config wires the source.
type Config struct {
	// Packages decides which packages the reader sees. Required.
	Packages Packages
	// Files reads the documents of the active artifacts. Required.
	Files Files
	// Subject resolves the reader of a request. Required.
	Subject reference.SubjectFunc
}

// Entry is one document a reader may open.
type Entry struct {
	// Path is the document's address within the package ("getting-started").
	Path     string `json:"path"`
	Title    string `json:"title"`
	Audience string `json:"audience,omitempty"`
	Order    int    `json:"order,omitempty"`
}

// Listing is the documentation of one package the reader may read.
type Listing struct {
	Namespace string  `json:"namespace"`
	Version   string  `json:"version"`
	Docs      []Entry `json:"docs"`
}

// Page is one document with its text, links resolved.
type Page struct {
	Entry
	Namespace string
	Markdown  []byte
}

// Library answers what documentation a reader may see and read. The Help
// Center source and the MCP tool are both views of it, so they cannot
// disagree.
type Library struct {
	packages Packages
	files    Files
	cache    *textCache
}

// NewLibrary builds the library over the catalog (packages) and the artifact
// store (files).
func NewLibrary(packages Packages, files Files) (*Library, error) {
	if packages == nil || files == nil {
		return nil, serrors.New(serrors.Invalid, "pkgdocs: Packages and Files are required").WithOp("extruntime.pkgdocs.NewLibrary")
	}
	return &Library{packages: packages, files: files, cache: newTextCache(cacheBytes)}, nil
}

// Language is the lowercase primary language of a locale tag ("pt_BR" -> "pt").
func Language(locale string) string {
	lang, _, _ := strings.Cut(normalize(locale), "-")
	return lang
}

func normalize(locale string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(locale), "_", "-"))
}

// candidates is the order locales are tried in for a reader: the exact tag,
// its language, then the default locale.
func candidates(locale string) []string {
	loc := normalize(locale)
	out := make([]string, 0, 3)
	for _, c := range []string{loc, Language(loc), manifest.DefaultDocLocale} {
		if c != "" && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}

// pick chooses the translation of the document at path for locale.
func pick(m manifest.Manifest, path, locale string) (manifest.Doc, bool) {
	for _, loc := range candidates(locale) {
		for _, d := range m.Docs {
			if d.Path == path && d.Locale == loc {
				return d, true
			}
		}
	}
	return manifest.Doc{}, false
}

// readable reports whether the reader may open d. Extensions.Read holders see
// every document, as they see every resource of the reference.
func readable(sub spi.Subject, d manifest.Doc) bool {
	return reference.CanReadAll(sub) || d.Permission == "" || sub.Has(d.Permission)
}

// paths lists the distinct document paths of the manifest in first-seen order.
func paths(m manifest.Manifest) []string {
	var out []string
	for _, d := range m.Docs {
		if !slices.Contains(out, d.Path) {
			out = append(out, d.Path)
		}
	}
	return out
}

// plain makes a title safe to show as text anywhere: one line, no angle
// brackets, bounded. Titles are validated at publish; this keeps the tree
// plain text even for a manifest that bypassed validation.
func plain(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '<' || r == '>':
			return -1
		case r < 0x20 || r == 0x7f || r == 0x2028 || r == 0x2029:
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > manifest.MaxDocTitle {
		s = string(r[:manifest.MaxDocTitle])
	}
	return s
}

// List returns, per active package the reader sees, the documents they may
// read in locale (their translation, else the default locale's), ordered.
func (l *Library) List(ctx context.Context, sub spi.Subject, locale string) ([]Listing, error) {
	pkgs, err := l.packages.VisiblePackages(ctx, sub)
	if err != nil {
		return nil, err
	}
	out := make([]Listing, 0, len(pkgs))
	for _, av := range pkgs {
		var docs []Entry
		for _, p := range paths(av.Manifest) {
			d, ok := pick(av.Manifest, p, locale)
			if !ok || !readable(sub, d) {
				continue
			}
			docs = append(docs, Entry{Path: d.Path, Title: plain(d.Title), Audience: plain(d.Audience), Order: d.Order})
		}
		if len(docs) == 0 {
			continue
		}
		slices.SortStableFunc(docs, func(a, b Entry) int {
			return cmp.Or(cmp.Compare(a.Order, b.Order), strings.Compare(a.Title, b.Title))
		})
		out = append(out, Listing{Namespace: av.Namespace, Version: av.Version, Docs: docs})
	}
	return out, nil
}

// Read returns one document, or helpdocs.ErrNotFound when it does not exist
// or the reader may not read it.
func (l *Library) Read(ctx context.Context, sub spi.Subject, locale, namespace, path string) (Page, error) {
	if !manifest.ValidDocPath(path) {
		return Page{}, helpdocs.ErrNotFound
	}
	pkgs, err := l.packages.VisiblePackages(ctx, sub)
	if err != nil {
		return Page{}, err
	}
	i := slices.IndexFunc(pkgs, func(av catalog.ActiveVersion) bool { return av.Namespace == namespace })
	if i < 0 {
		return Page{}, helpdocs.ErrNotFound
	}
	av := pkgs[i]
	d, ok := pick(av.Manifest, path, locale)
	if !ok || !readable(sub, d) {
		return Page{}, helpdocs.ErrNotFound
	}
	file := manifest.DocFile(d)
	body, err := l.cache.get(av.ArtifactHash, file, func() ([]byte, error) {
		return l.files.ReadFile(ctx, av.ArtifactHash, file)
	})
	if err != nil {
		if serrors.CodeOf(err) == serrors.NotFound {
			return Page{}, helpdocs.ErrNotFound
		}
		return Page{}, serrors.Wrap("extruntime.pkgdocs.Library.Read", err)
	}
	return Page{
		Entry:     Entry{Path: d.Path, Title: plain(d.Title), Audience: plain(d.Audience), Order: d.Order},
		Namespace: namespace,
		Markdown:  RewriteLinks(body, av.Manifest),
	}, nil
}

type source struct {
	lib     *Library
	subject reference.SubjectFunc
}

// NewSource returns the helpdocs.Source mounted at "apps".
func NewSource(cfg Config) (helpdocs.Source, error) {
	lib, err := NewLibrary(cfg.Packages, cfg.Files)
	if err != nil {
		return nil, err
	}
	return NewSourceOf(lib, cfg.Subject)
}

// NewSourceOf returns the helpdocs.Source over an existing library, so that
// the Help Center and other views share its cache.
func NewSourceOf(lib *Library, subject reference.SubjectFunc) (helpdocs.Source, error) {
	if lib == nil || subject == nil {
		return nil, serrors.New(serrors.Invalid, "pkgdocs: Library and Subject are required").WithOp("extruntime.pkgdocs.NewSourceOf")
	}
	return &source{lib: lib, subject: subject}, nil
}

var mountTitles = map[string]string{"en": "Applications", "ru": "Приложения", "uz": "Ilovalar"}

func (s *source) Mount() string { return Mount }
func (s *source) Order() int    { return 150 }
func (s *source) Title(locale string) string {
	if t := mountTitles[Language(locale)]; t != "" {
		return t
	}
	return mountTitles[manifest.DefaultDocLocale]
}

// reader returns the reader of the request, or ok=false when there is none
// (nothing is shown to an unidentified request).
func (s *source) reader(ctx context.Context) (spi.Subject, bool) {
	sub, err := s.subject(ctx)
	return sub, err == nil
}

// Tree implements helpdocs.Source: one category per active package the
// reader sees, holding the documents they may read in locale. The category is
// labelled with the namespace (a manifest carries no display name).
func (s *source) Tree(ctx context.Context, locale string) ([]helpdocs.Node, error) {
	sub, ok := s.reader(ctx)
	if !ok {
		return nil, nil
	}
	listings, err := s.lib.List(ctx, sub, locale)
	if err != nil {
		return nil, err
	}
	tree := make([]helpdocs.Node, 0, len(listings))
	for i, l := range listings {
		node := helpdocs.Node{Title: l.Namespace, Path: l.Namespace, Order: i}
		for _, d := range l.Docs {
			node.Children = append(node.Children, helpdocs.Node{Title: d.Title, Path: l.Namespace + "/" + d.Path, Order: d.Order})
		}
		tree = append(tree, node)
	}
	return tree, nil
}

// Get implements helpdocs.Source. docPath is "<namespace>/<document path>".
func (s *source) Get(ctx context.Context, locale, docPath string) (helpdocs.Document, error) {
	ns, rel, ok := strings.Cut(strings.Trim(docPath, "/"), "/")
	if !ok {
		return helpdocs.Document{}, helpdocs.ErrNotFound
	}
	sub, ok := s.reader(ctx)
	if !ok {
		return helpdocs.Document{}, helpdocs.ErrNotFound
	}
	page, err := s.lib.Read(ctx, sub, locale, ns, rel)
	if err != nil {
		return helpdocs.Document{}, err
	}
	return helpdocs.Document{Title: page.Title, Path: ns + "/" + rel, Markdown: page.Markdown, Audience: page.Audience}, nil
}

var _ helpdocs.Source = (*source)(nil)
