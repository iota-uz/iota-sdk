package services

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

// sourceSet holds the documentation sources contributed by other components.
// It is shared by every ContentService derived from the same WithSources call.
type sourceSet struct {
	list []helpdocs.Source
	// warned remembers the mounts whose collision with the file tree was
	// already logged, so a per-request merge does not repeat it.
	warned sync.Map
}

// WithSources returns a copy of the service that serves the documentation of
// sources next to its own files: each source is a category at its mount in the
// sidebar tree and its documents are opened at /help/doc/<mount>/<path>.
//
// Mounts are validated here, at construction, so a collision fails the
// application start instead of hiding documentation at run time. A mount must
// be a clean slash-separated path, must not overlap another source's mount
// (equal or nested), and must not sit on a category or document of the file
// tree of any content locale. A mount may be nested under a file category
// ("modules/extra" next to "modules/crm.md"); the category is then shared.
//
// The file tree can still change after construction when it is read from
// disk. A source whose mount then collides with the file tree is left out of
// the sidebar (the files win) and the collision is logged once.
func (s *ContentService) WithSources(sources []helpdocs.Source) (*ContentService, error) {
	const op = "ContentService.WithSources"
	if len(sources) == 0 {
		return s, nil
	}
	mounts := make([]string, len(sources))
	for i, source := range sources {
		if source == nil {
			return nil, serrors.Wrap(op, errors.New("help source is nil"))
		}
		mount := source.Mount()
		if !validMount(mount) {
			return nil, serrors.Wrap(op, fmt.Errorf("help source mount %q must be a clean slash-separated path without leading or trailing slash", mount))
		}
		for j := range i {
			if mountsOverlap(mounts[j], mount) {
				return nil, serrors.Wrap(op, fmt.Errorf("help source mounts %q and %q overlap", mounts[j], mount))
			}
		}
		mounts[i] = mount
	}
	if err := s.checkFileTreeMounts(mounts); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	derived := *s
	derived.sources = &sourceSet{list: append([]helpdocs.Source(nil), sources...)}
	return &derived, nil
}

// HasSources reports whether documentation sources are contributed.
func (s *ContentService) HasSources() bool {
	return s.sources != nil
}

// IsSourcePath reports whether docPath is served by a contributed source
// rather than by the Help Center's files.
func (s *ContentService) IsSourcePath(docPath string) bool {
	_, _, ok := s.sources.resolve(docPath)
	return ok
}

func validMount(mount string) bool {
	return mount != "." && fs.ValidPath(mount) && !strings.ContainsAny(mount, "\\\x00")
}

func mountsOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

// resolve finds the source whose mount holds docPath and returns the path
// relative to that mount. A nil set resolves nothing.
func (set *sourceSet) resolve(docPath string) (helpdocs.Source, string, bool) {
	if set == nil {
		return nil, "", false
	}
	docPath = strings.TrimPrefix(docPath, "/")
	for _, source := range set.list {
		if mount := source.Mount(); strings.HasPrefix(docPath, mount+"/") {
			return source, strings.TrimPrefix(docPath, mount+"/"), true
		}
	}
	return nil, "", false
}

// sourceLocale is the reader's locale as the Help Center resolved it for the
// request. It is not narrowed to the file content's locales: a source falls
// back to its own default locale, so it can serve languages the Help Center's
// markdown tree does not have.
func (s *ContentService) sourceLocale(ctx context.Context) string {
	return localeString(ctx, s.config.DefaultLocale)
}

func (s *ContentService) getSourceDocument(ctx context.Context, source helpdocs.Source, rel string) (*Document, error) {
	if rel == "" || strings.Contains(rel, "\x00") || path.Clean(rel) != rel || !fs.ValidPath(rel) {
		return nil, ErrInvalidPath
	}
	doc, err := source.Get(ctx, s.sourceLocale(ctx), rel)
	if err != nil {
		if errors.Is(err, helpdocs.ErrNotFound) {
			return nil, ErrDocumentNotFound
		}
		return nil, serrors.Wrap("ContentService.getSourceDocument", err)
	}
	title := strings.TrimSpace(doc.Title)
	if title == "" {
		title = titleFromSegment(path.Base(rel))
	}
	return &Document{
		Title:   title,
		Path:    source.Mount() + "/" + rel,
		Content: doc.Markdown,
		Meta: DocumentMeta{
			Audience:       strings.TrimSpace(doc.Audience),
			Updated:        doc.Updated,
			ReadingMinutes: estimateReadingMinutes(doc.Markdown),
		},
		SourceMount: source.Mount(),
	}, nil
}

// checkFileTreeMounts rejects a mount that sits on a category or document of
// the file tree of any content locale.
func (s *ContentService) checkFileTreeMounts(mounts []string) error {
	if s.fsys == nil {
		return nil
	}
	for _, locale := range s.config.Locales {
		if locale != s.config.DefaultLocale && !s.localeDirExists(locale) {
			continue
		}
		nodes, err := s.buildTreeNodes(locale, locale)
		if err != nil {
			if errors.Is(err, ErrDocumentNotFound) {
				continue
			}
			return err
		}
		for _, mount := range mounts {
			if key, ok := fileTreeClaim(nodes, mount); ok {
				return fmt.Errorf("help source mount %q collides with %q of the %s help content", mount, key, locale)
			}
		}
	}
	return nil
}

// fileTreeClaim finds a node of the file tree at the mount or below it. Keys
// are sidebar category paths and real document paths, so the check covers
// both the sidebar and the /help/doc URL space.
func fileTreeClaim(nodes []*categoryTreeNode, mount string) (string, bool) {
	for _, node := range nodes {
		if node.key == mount || strings.HasPrefix(node.key, mount+"/") {
			return node.key, true
		}
		if key, ok := fileTreeClaim(node.children, mount); ok {
			return key, true
		}
	}
	return "", false
}

// mergeSources adds the category of every source that has documents for the
// reader in ctx to the file tree. A source that fails to list its tree is
// logged and left out: the rest of the Help Center keeps working.
func (s *ContentService) mergeSources(ctx context.Context, locale string, nodes *[]*categoryTreeNode) {
	if s.sources == nil {
		return
	}
	sourceLocale := s.sourceLocale(ctx)
	for _, source := range s.sources.list {
		mount := source.Mount()
		tree, err := source.Tree(ctx, sourceLocale)
		if err != nil {
			logrus.WithError(serrors.Wrap("ContentService.mergeSources", err)).WithField("mount", mount).Warn("help source tree unavailable")
			continue
		}
		children, leaves := sourceNodes(mount, tree)
		if leaves == 0 {
			continue
		}
		if key, collides := fileTreeClaim(*nodes, mount); collides {
			if _, logged := s.sources.warned.LoadOrStore(mount, struct{}{}); !logged {
				logrus.WithFields(logrus.Fields{"mount": mount, "collides_with": key}).Warn("help source mount collides with the help content; the help content wins")
			}
			continue
		}
		title := strings.TrimSpace(source.Title(sourceLocale))
		if title == "" {
			title = titleFromSegment(path.Base(mount))
		}
		category := s.mountCategory(locale, nodes, mount, title, source.Order())
		category.children = children
	}
}

// mountCategory returns the category node at mount, creating it and the
// categories above it. Existing categories (the file tree's or another
// source's) are shared; only the last segment takes the source's title and
// order.
func (s *ContentService) mountCategory(locale string, nodes *[]*categoryTreeNode, mount, title string, order int) *categoryTreeNode {
	segments := strings.Split(mount, "/")
	var node *categoryTreeNode
	for i, segment := range segments {
		key := strings.Join(segments[:i+1], "/")
		var next *categoryTreeNode
		for _, candidate := range *nodes {
			if candidate.key == key {
				next = candidate
				break
			}
		}
		if next == nil {
			next = &categoryTreeNode{key: key, title: s.categoryTitle(locale, key, segment), order: s.categoryOrder(key)}
			if i == len(segments)-1 {
				next.title, next.order = title, order
			}
			*nodes = append(*nodes, next)
		}
		node = next
		nodes = &node.children
	}
	return node
}

// sourceNodes turns a source's tree into category tree nodes with full
// document paths (<mount>/<path>) and counts the documents. Entries the Help
// Center cannot serve (unclean or empty paths) and categories left without
// documents are dropped.
func sourceNodes(mount string, nodes []helpdocs.Node) ([]*categoryTreeNode, int) {
	result := make([]*categoryTreeNode, 0, len(nodes))
	leaves := 0
	for _, node := range nodes {
		if len(node.Children) > 0 {
			children, count := sourceNodes(mount, node.Children)
			if count == 0 {
				continue
			}
			title := strings.TrimSpace(node.Title)
			if title == "" {
				title = titleFromSegment(path.Base(node.Path))
			}
			key := mount + "/" + strings.Trim(node.Path, "/")
			result = append(result, &categoryTreeNode{key: key, title: title, order: node.Order, children: children})
			leaves += count
			continue
		}
		if node.Path == "" || strings.Contains(node.Path, "\x00") || path.Clean(node.Path) != node.Path || !fs.ValidPath(node.Path) {
			continue
		}
		title := strings.TrimSpace(node.Title)
		if title == "" {
			title = titleFromSegment(path.Base(node.Path))
		}
		full := mount + "/" + node.Path
		result = append(result, &categoryTreeNode{key: full, title: title, path: full, order: node.Order})
		leaves++
	}
	return result, leaves
}
