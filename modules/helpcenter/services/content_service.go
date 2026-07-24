// Package services provides this package.
package services

import (
	"bytes"
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
	"gopkg.in/yaml.v3"
)

var (
	ErrContentRootRequired = errors.New("help center content root is required")
	ErrDocumentNotFound    = errors.New("help center document not found")
	ErrMediaNotFound       = errors.New("help center media not found")
	ErrInvalidPath         = errors.New("invalid help center document path")
)

var allowedMediaExtensions = map[string]struct{}{
	".avif": {},
	".gif":  {},
	".ico":  {},
	".jpeg": {},
	".jpg":  {},
	".png":  {},
	".svg":  {},
	".webp": {},
}

type Document struct {
	Title   string
	Path    string
	Content []byte
}

type Media struct {
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
	// CategoryTitles maps locale and relative category path to a localized
	// sidebar label while keeping document paths stable across locales.
	CategoryTitles map[string]map[string]string
	// HideNav omits the Help Center sidebar nav node when the component is
	// registered only to serve inline help-article links (see component.go).
	HideNav bool
	// HiddenSections contains exact Markdown heading titles that remain in the
	// source content but are omitted from documents returned for display.
	HiddenSections []string
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

// Media returns a file from the active locale's content tree. A missing file
// falls back to the default locale in the same way as help documents.
func (s *ContentService) Media(ctx context.Context, mediaPath string) (*Media, error) {
	localeDir, locale, err := s.localeRoot(ctx)
	if err != nil {
		return nil, err
	}
	cleanPath, err := cleanMediaPath(mediaPath)
	if err != nil {
		return nil, err
	}

	content, err := s.readContentFile(localeDir, cleanPath)
	if errors.Is(err, fs.ErrNotExist) && locale != s.config.DefaultLocale {
		content, err = s.readContentFile(s.config.DefaultLocale, cleanPath)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMediaNotFound
	}
	if err != nil {
		return nil, err
	}
	return &Media{Path: cleanPath, Content: content}, nil
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
	content, err := s.readContentFile(localeDir, cleanPath)
	if err != nil {
		return nil, err
	}
	title, content := parseMarkdownDocument(content, cleanPath)
	content = stripMarkdownSections(content, s.config.HiddenSections)
	return &Document{
		Title:   title,
		Path:    cleanPath,
		Content: content,
	}, nil
}

func (s *ContentService) readContentFile(localeDir, cleanPath string) ([]byte, error) {
	full := path.Join(localeDir, cleanPath)
	if !fs.ValidPath(full) {
		return nil, ErrInvalidPath
	}
	info, err := fs.Stat(s.fsys, full)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fs.ErrNotExist
	}
	return fs.ReadFile(s.fsys, full)
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
	var tree []*categoryTreeNode

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
		title, _ := parseMarkdownDocument(content, rel)
		doc := viewmodels.CategoryNode{Title: title, Path: rel}
		s.insertNode(&tree, locale, "", parts[:len(parts)-1], doc)
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

	docs := categoryViewModels(tree)
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

// cleanMediaPath accepts only a relative, slash-separated image path within the
// configured content root.
func cleanMediaPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" || strings.Contains(p, "\x00") || strings.Contains(p, "\\") {
		return "", ErrInvalidPath
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == "." || segment == ".." {
			return "", ErrInvalidPath
		}
	}
	clean := path.Clean(p)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) || !fs.ValidPath(clean) {
		return "", ErrInvalidPath
	}
	if _, ok := allowedMediaExtensions[strings.ToLower(path.Ext(clean))]; !ok {
		return "", ErrInvalidPath
	}
	return clean, nil
}

func isMarkdown(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	return ext == ".md" || ext == ".markdown"
}

func parseMarkdownDocument(content []byte, docPath string) (string, []byte) {
	frontMatterTitle, body := splitFrontMatter(content)
	if frontMatterTitle != "" {
		return frontMatterTitle, body
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title != "" {
				return title, body
			}
		}
		if line != "" {
			break
		}
	}
	name := strings.TrimSuffix(path.Base(docPath), path.Ext(docPath))
	return titleFromSegment(name), body
}

func titleFromSegment(segment string) string {
	segment = strings.TrimSuffix(segment, path.Ext(segment))
	segment = strings.ReplaceAll(segment, "-", " ")
	segment = strings.ReplaceAll(segment, "_", " ")
	return cases.Title(language.English).String(segment)
}

func splitFrontMatter(content []byte) (string, []byte) {
	lines := bytes.Split(content, []byte("\n"))
	if len(lines) < 3 || string(bytes.TrimSpace(lines[0])) != "---" {
		return "", content
	}
	for i := 1; i < len(lines); i++ {
		if string(bytes.TrimSpace(lines[i])) != "---" {
			continue
		}
		var metadata struct {
			Title string `yaml:"title"`
		}
		_ = yaml.Unmarshal(bytes.Join(lines[1:i], []byte("\n")), &metadata)
		body := bytes.TrimLeft(bytes.Join(lines[i+1:], []byte("\n")), "\r\n")
		return strings.TrimSpace(metadata.Title), body
	}
	return "", content
}

func stripMarkdownSections(content []byte, hiddenSections []string) []byte {
	if len(hiddenSections) == 0 {
		return content
	}
	hidden := make(map[string]struct{}, len(hiddenSections))
	for _, section := range hiddenSections {
		if section = strings.TrimSpace(section); section != "" {
			hidden[section] = struct{}{}
		}
	}
	if len(hidden) == 0 {
		return content
	}

	lines := bytes.SplitAfter(content, []byte("\n"))
	filtered := make([][]byte, 0, len(lines))
	hiddenLevel := 0
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(string(line))
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		}
		if !inFence {
			if level, title, ok := markdownHeading(trimmed); ok {
				if hiddenLevel != 0 && level <= hiddenLevel {
					hiddenLevel = 0
				}
				if hiddenLevel == 0 {
					if _, ok := hidden[title]; ok {
						hiddenLevel = level
						continue
					}
				}
			}
		}
		if hiddenLevel == 0 {
			filtered = append(filtered, line)
		}
	}
	return bytes.TrimRight(bytes.Join(filtered, nil), "\r\n")
}

func markdownHeading(line string) (int, string, bool) {
	level := 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 || level == len(line) || line[level] != ' ' {
		return 0, "", false
	}
	title := strings.TrimSpace(strings.TrimRight(line[level+1:], "# "))
	if title == "" {
		return 0, "", false
	}
	return level, title, true
}

type categoryTreeNode struct {
	key      string
	title    string
	path     string
	children []*categoryTreeNode
}

func (s *ContentService) insertNode(
	nodes *[]*categoryTreeNode,
	locale string,
	parentPath string,
	categories []string,
	doc viewmodels.CategoryNode,
) {
	if len(categories) == 0 {
		*nodes = append(*nodes, &categoryTreeNode{key: doc.Path, title: doc.Title, path: doc.Path})
		return
	}
	categoryPath := path.Join(parentPath, categories[0])
	title := s.categoryTitle(locale, categoryPath, categories[0])
	for _, node := range *nodes {
		if node.key == categoryPath {
			s.insertNode(&node.children, locale, categoryPath, categories[1:], doc)
			return
		}
	}
	node := &categoryTreeNode{key: categoryPath, title: title}
	*nodes = append(*nodes, node)
	s.insertNode(&node.children, locale, categoryPath, categories[1:], doc)
}

func categoryViewModels(nodes []*categoryTreeNode) []viewmodels.CategoryNode {
	result := make([]viewmodels.CategoryNode, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, viewmodels.CategoryNode{
			Title:    node.title,
			Path:     node.path,
			Children: categoryViewModels(node.children),
		})
	}
	return result
}

func (s *ContentService) categoryTitle(locale, categoryPath, segment string) string {
	if localized, ok := s.config.CategoryTitles[locale]; ok {
		if title := strings.TrimSpace(localized[categoryPath]); title != "" {
			return title
		}
	}
	return titleFromSegment(segment)
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
