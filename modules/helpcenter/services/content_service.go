// Package services provides this package.
package services

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	ErrContentRootRequired = errors.New("help center content root is required")
	ErrDocumentNotFound    = errors.New("help center document not found")
	ErrInvalidPath         = errors.New("invalid help center document path")
)

type Document struct {
	Title   string
	Path    string
	Content []byte
}

type ContentConfig struct {
	// Root is the content directory. When FS is nil it is an on-disk path
	// (read via os.DirFS); when FS is set it is the sub-directory within FS
	// that holds the per-locale content tree.
	Root string
	// FS, when set, serves markdown from an in-memory/embedded filesystem
	// (e.g. //go:embed) instead of disk — deploy-safe under GOWORK=off where
	// the on-disk content dir is absent. Rooted at Root.
	FS            fs.FS
	Locales       []string
	DefaultLocale string
	// HideNav omits the Help Center sidebar nav node when the component is
	// registered only to serve inline help-article links (see component.go).
	HideNav bool
}

func (c ContentConfig) Normalized() ContentConfig {
	if c.DefaultLocale == "" {
		c.DefaultLocale = "en"
	}
	if len(c.Locales) == 0 {
		c.Locales = []string{c.DefaultLocale}
	}
	return c
}

type ContentService struct {
	config ContentConfig
	// fsys is the resolved filesystem the content tree is read from: os.DirFS
	// rooted at Root for disk mode, or a sub-FS of config.FS for embedded mode.
	// nil when no content source is configured.
	fsys fs.FS
}

func NewContentService(config ContentConfig) *ContentService {
	cfg := config.Normalized()
	svc := &ContentService{config: cfg}
	switch {
	case cfg.FS != nil:
		if cfg.Root != "" && cfg.Root != "." {
			if sub, err := fs.Sub(cfg.FS, cfg.Root); err == nil {
				svc.fsys = sub
			}
		} else {
			svc.fsys = cfg.FS
		}
	case cfg.Root != "":
		svc.fsys = os.DirFS(cfg.Root)
	}
	return svc
}

func (s *ContentService) Tree(ctx context.Context) ([]viewmodels.CategoryNode, error) {
	localeDir, locale, err := s.localeRoot(ctx)
	if err != nil {
		return nil, err
	}
	return s.buildTree(localeDir, locale)
}

func (s *ContentService) Get(ctx context.Context, docPath string) (*Document, error) {
	localeDir, locale, err := s.localeRoot(ctx)
	if err != nil {
		return nil, err
	}
	cleanPath, err := cleanDocPath(docPath)
	if err != nil {
		return nil, err
	}

	doc, err := s.readDoc(localeDir, cleanPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && locale != s.config.DefaultLocale {
			return s.getFromLocale(s.config.DefaultLocale, cleanPath)
		}
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}
	return doc, nil
}

func (s *ContentService) DefaultDocument(ctx context.Context) (*Document, error) {
	nodes, err := s.Tree(ctx)
	if err != nil {
		return nil, err
	}
	docPath := firstDocPath(nodes)
	if docPath == "" {
		return nil, ErrDocumentNotFound
	}
	return s.Get(ctx, docPath)
}

func (s *ContentService) Locale(ctx context.Context) string {
	_, locale, err := s.localeRoot(ctx)
	if err != nil {
		return s.config.DefaultLocale
	}
	return locale
}

func (s *ContentService) getFromLocale(locale, docPath string) (*Document, error) {
	doc, err := s.readDoc(locale, docPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}
	return doc, nil
}

// readDoc reads one markdown document at <localeDir>/<cleanPath> from fsys.
// cleanPath is already validated by cleanDocPath; the joined path is checked
// against fs.ValidPath as defense in depth.
func (s *ContentService) readDoc(localeDir, cleanPath string) (*Document, error) {
	full := path.Join(localeDir, cleanPath)
	if !fs.ValidPath(full) {
		return nil, ErrInvalidPath
	}
	content, err := fs.ReadFile(s.fsys, full)
	if err != nil {
		return nil, err
	}
	return &Document{
		Title:   titleFromMarkdown(content, cleanPath),
		Path:    cleanPath,
		Content: content,
	}, nil
}

// localeRoot resolves the request locale to a content sub-directory (relative
// to fsys) that exists, falling back to the default locale. It returns the
// locale directory and the resolved locale.
func (s *ContentService) localeRoot(ctx context.Context) (string, string, error) {
	if s.fsys == nil {
		return "", "", ErrContentRootRequired
	}
	locale := localeString(ctx, s.config.DefaultLocale)
	if !contains(s.config.Locales, locale) {
		locale = s.config.DefaultLocale
	}
	if locale != s.config.DefaultLocale && !s.localeDirExists(locale) {
		locale = s.config.DefaultLocale
	}
	return locale, locale, nil
}

func (s *ContentService) localeDirExists(locale string) bool {
	if !fs.ValidPath(locale) {
		return false
	}
	info, err := fs.Stat(s.fsys, locale)
	return err == nil && info.IsDir()
}

func (s *ContentService) buildTree(localeDir, locale string) ([]viewmodels.CategoryNode, error) {
	var docs []viewmodels.CategoryNode

	err := fs.WalkDir(s.fsys, localeDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isMarkdown(p) {
			return nil
		}
		rel := strings.TrimPrefix(p, localeDir+"/")
		content, err := fs.ReadFile(s.fsys, p)
		if err != nil {
			return err
		}
		parts := strings.Split(rel, "/")
		doc := viewmodels.CategoryNode{Title: titleFromMarkdown(content, rel), Path: rel}
		insertNode(&docs, parts[:len(parts)-1], doc)
		return nil
	})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && locale != s.config.DefaultLocale {
			return s.buildTree(s.config.DefaultLocale, s.config.DefaultLocale)
		}
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}

	sortNodes(docs)
	return docs, nil
}

func localeString(ctx context.Context, fallback string) string {
	tag, ok := intl.UseLocale(ctx)
	if !ok || tag == language.Und {
		return fallback
	}
	base, _ := tag.Base()
	if base.String() == "und" {
		return fallback
	}
	return base.String()
}

// cleanDocPath validates and normalizes a request doc path to a safe,
// forward-slash relative markdown path (no traversal, no absolute paths).
func cleanDocPath(p string) (string, error) {
	p = strings.TrimSpace(strings.TrimPrefix(p, "/"))
	if p == "" || strings.Contains(p, "\x00") {
		return "", ErrInvalidPath
	}
	clean := path.Clean(filepath.ToSlash(p))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) || !isMarkdown(clean) {
		return "", ErrInvalidPath
	}
	return clean, nil
}

func isMarkdown(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	return ext == ".md" || ext == ".markdown"
}

func titleFromMarkdown(content []byte, docPath string) string {
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title != "" {
				return title
			}
		}
		if line != "" {
			break
		}
	}
	name := strings.TrimSuffix(path.Base(docPath), path.Ext(docPath))
	return titleFromSegment(name)
}

func titleFromSegment(segment string) string {
	segment = strings.TrimSuffix(segment, path.Ext(segment))
	segment = strings.ReplaceAll(segment, "-", " ")
	segment = strings.ReplaceAll(segment, "_", " ")
	return cases.Title(language.English).String(segment)
}

func insertNode(nodes *[]viewmodels.CategoryNode, categories []string, doc viewmodels.CategoryNode) {
	if len(categories) == 0 {
		*nodes = append(*nodes, doc)
		return
	}
	title := titleFromSegment(categories[0])
	for i := range *nodes {
		if (*nodes)[i].Path == "" && (*nodes)[i].Title == title {
			insertNode(&(*nodes)[i].Children, categories[1:], doc)
			return
		}
	}
	*nodes = append(*nodes, viewmodels.CategoryNode{Title: title})
	insertNode(&(*nodes)[len(*nodes)-1].Children, categories[1:], doc)
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func firstDocPath(nodes []viewmodels.CategoryNode) string {
	for _, node := range nodes {
		if node.Path != "" {
			return node.Path
		}
		if child := firstDocPath(node.Children); child != "" {
			return child
		}
	}
	return ""
}

func sortNodes(nodes []viewmodels.CategoryNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].Title < nodes[j].Title
	})
	for i := range nodes {
		sortNodes(nodes[i].Children)
	}
}
