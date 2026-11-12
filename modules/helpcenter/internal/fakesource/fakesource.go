// Package fakesource is a helpdocs.Source for the Help Center's tests. It
// behaves like a real contributor: it answers per reader and per locale,
// reads the reader from ctx the way the application sets it, and can change
// between requests.
package fakesource

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
)

// Doc is one document of the fake source.
type Doc struct {
	Title    string
	Markdown string
	Audience string
	Updated  time.Time
	// Restricted documents are listed and served only to a reader who holds
	// permissions.UserRead.
	Restricted bool
}

// Source implements helpdocs.Source.
type Source struct {
	MountPath string
	// Names maps a locale to the mount category's title; "" is the fallback.
	Names    map[string]string
	Position int
	// Layout is the full tree, restricted documents included; Tree filters it
	// for the reader.
	Layout []helpdocs.Node
	// Docs maps a path relative to the mount to its document.
	Docs map[string]Doc
	// Err, when set, is returned by Tree.
	Err error

	mu      sync.Mutex
	locales []string
}

var _ helpdocs.Source = (*Source)(nil)

func (s *Source) Mount() string { return s.MountPath }

func (s *Source) Order() int { return s.Position }

func (s *Source) Title(locale string) string {
	if name, ok := s.Names[locale]; ok {
		return name
	}
	return s.Names[""]
}

// Locales lists the locale of every Tree and Get call, in call order.
func (s *Source) Locales() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.locales)
}

func (s *Source) record(locale string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.locales = append(s.locales, locale)
}

func (s *Source) Tree(ctx context.Context, locale string) ([]helpdocs.Node, error) {
	s.record(locale)
	if s.Err != nil {
		return nil, s.Err
	}
	return s.visible(ctx, s.Layout), nil
}

func (s *Source) visible(ctx context.Context, nodes []helpdocs.Node) []helpdocs.Node {
	var out []helpdocs.Node
	for _, node := range nodes {
		if len(node.Children) > 0 {
			if children := s.visible(ctx, node.Children); len(children) > 0 {
				node.Children = children
				out = append(out, node)
			}
			continue
		}
		if doc, ok := s.Docs[node.Path]; ok && s.allowed(ctx, doc) {
			out = append(out, node)
		}
	}
	return out
}

func (s *Source) Get(ctx context.Context, locale, path string) (helpdocs.Document, error) {
	s.record(locale)
	doc, ok := s.Docs[path]
	if !ok || !s.allowed(ctx, doc) {
		return helpdocs.Document{}, helpdocs.ErrNotFound
	}
	return helpdocs.Document{
		Title:    doc.Title,
		Path:     path,
		Markdown: []byte(doc.Markdown),
		Updated:  doc.Updated,
		Audience: doc.Audience,
	}, nil
}

func (s *Source) allowed(ctx context.Context, doc Doc) bool {
	if !doc.Restricted {
		return true
	}
	reader, err := composables.UseUser(ctx)
	return err == nil && reader.Can(permissions.UserRead)
}
