package reference

import (
	"context"
	"io/fs"
	"strings"

	"github.com/iota-uz/iota-sdk/docs/extensions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Mount is where the extension documentation sits in the Help Center tree.
const Mount = "extensions"

// Audience of every document of the source.
const Audience = "Developers"

// SubjectFunc resolves the reader of a request from its context (the session
// user, as /_ext/rpc and the management API resolve it).
type SubjectFunc func(ctx context.Context) (spi.Subject, error)

// SourceConfig wires the Help Center source.
type SourceConfig struct {
	// Builder produces the reference. Required.
	Builder *Builder
	// Subject resolves the reader. Required.
	Subject SubjectFunc
	// ManageRoutes lists the management API for readers who hold
	// Extensions.Read (the API document shows it to them only).
	ManageRoutes []APIRoute
	// Guides overrides the embedded SDK guides (tests).
	Guides fs.FS
	// Ref is the git ref repository links point at. Default: SourceRef().
	Ref string
}

type source struct {
	cfg    SourceConfig
	ref    string
	guides []guide
}

type guide struct {
	path     string
	title    string
	markdown []byte
}

// NewSource returns the helpdocs.Source mounted at "extensions": the SDK
// guides, the live reference (one document per namespace the reader may see)
// and the HTTP API document.
func NewSource(cfg SourceConfig) (helpdocs.Source, error) {
	const op = serrors.Op("extruntime.reference.NewSource")
	if cfg.Builder == nil || cfg.Subject == nil {
		return nil, serrors.New(serrors.Invalid, "reference: Builder and Subject are required").WithOp(op)
	}
	if cfg.Guides == nil {
		cfg.Guides = extensions.FS
	}
	s := &source{cfg: cfg, ref: cfg.Ref}
	if s.ref == "" {
		s.ref = SourceRef()
	}
	for _, g := range guideDocs {
		raw, err := fs.ReadFile(cfg.Guides, g.File)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		s.guides = append(s.guides, guide{
			path: g.Path, title: guideTitle(raw, g.File), markdown: RewriteGuideLinks(raw, s.ref),
		})
	}
	return s, nil
}

func (s *source) Mount() string       { return Mount }
func (s *source) Title(string) string { return "Extensions" }
func (s *source) Order() int          { return 200 }

// reader returns the model the request's reader sees, or ok=false when there
// is no reader (nothing is shown to an unidentified request).
func (s *source) reader(ctx context.Context, locale string) (*Model, bool, error) {
	sub, err := s.cfg.Subject(ctx)
	if err != nil {
		return nil, false, nil //nolint:nilerr // no reader: the mount is simply empty
	}
	m, err := s.cfg.Builder.Model(ctx, sub, locale)
	if err != nil {
		return nil, false, err
	}
	return m, true, nil
}

// Tree implements helpdocs.Source.
func (s *source) Tree(ctx context.Context, locale string) ([]helpdocs.Node, error) {
	m, ok, err := s.reader(ctx, locale)
	if err != nil || !ok {
		return nil, err
	}
	guides := helpdocs.Node{Title: "Guides", Path: GuidesPath, Order: 10}
	for i, g := range s.guides {
		guides.Children = append(guides.Children, helpdocs.Node{Title: g.title, Path: g.path, Order: i})
	}
	ref := helpdocs.Node{Title: "Reference", Path: ReferencePath, Order: 20, Children: []helpdocs.Node{
		{Title: "Overview", Path: IndexPath, Order: 0},
	}}
	for i, ns := range m.Namespaces {
		ref.Children = append(ref.Children, helpdocs.Node{Title: ns.Name, Path: NamespacePath(ns.Name), Order: i + 1})
	}
	return []helpdocs.Node{guides, ref, {Title: "HTTP API", Path: APIPath, Order: 30}}, nil
}

// Get implements helpdocs.Source.
func (s *source) Get(ctx context.Context, locale, docPath string) (helpdocs.Document, error) {
	docPath = strings.Trim(docPath, "/")
	m, ok, err := s.reader(ctx, locale)
	if err != nil {
		return helpdocs.Document{}, err
	}
	if !ok {
		return helpdocs.Document{}, helpdocs.ErrNotFound
	}
	doc := helpdocs.Document{Path: docPath, Audience: Audience}
	switch {
	case docPath == IndexPath:
		doc.Title, doc.Markdown = "Extension reference", RenderIndex(m, true)
	case docPath == APIPath:
		doc.Title = "HTTP API"
		var routes []APIRoute
		if m.Full {
			routes = s.cfg.ManageRoutes
		}
		doc.Markdown = RenderAPI(m, routes)
	case strings.HasPrefix(docPath, ReferencePath+"/"):
		name := strings.TrimPrefix(docPath, ReferencePath+"/")
		body, found := RenderNamespace(m, name)
		if !found {
			return helpdocs.Document{}, helpdocs.ErrNotFound
		}
		doc.Title, doc.Markdown = name, body
	default:
		for _, g := range s.guides {
			if g.path == docPath {
				doc.Title, doc.Markdown = g.title, g.markdown
				return doc, nil
			}
		}
		return helpdocs.Document{}, helpdocs.ErrNotFound
	}
	return doc, nil
}

var _ helpdocs.Source = (*source)(nil)
