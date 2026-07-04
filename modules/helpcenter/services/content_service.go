package services

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/intl"
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
	Root          string
	Locales       []string
	DefaultLocale string
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
}

func NewContentService(config ContentConfig) *ContentService {
	return &ContentService{config: config}
}

func (s *ContentService) Tree(ctx context.Context) ([]viewmodels.CategoryNode, error) {
	root, locale, err := s.localeRoot(ctx)
	if err != nil {
		return nil, err
	}
	return s.buildTree(root, locale)
}

func (s *ContentService) Get(ctx context.Context, docPath string) (*Document, error) {
	root, locale, err := s.localeRoot(ctx)
	if err != nil {
		return nil, err
	}
	cleanPath, err := cleanDocPath(docPath)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(root, filepath.FromSlash(cleanPath))
	if !isWithin(root, fullPath) {
		return nil, ErrInvalidPath
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && locale != s.config.DefaultLocale {
			return s.getFromLocale(s.config.DefaultLocale, cleanPath)
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}

	return &Document{
		Title:   titleFromMarkdown(content, cleanPath),
		Path:    cleanPath,
		Content: content,
	}, nil
}

func (s *ContentService) DefaultDocument(ctx context.Context) (*Document, error) {
	nodes, err := s.Tree(ctx)
	if err != nil {
		return nil, err
	}
	path := firstDocPath(nodes)
	if path == "" {
		return nil, ErrDocumentNotFound
	}
	return s.Get(ctx, path)
}

func (s *ContentService) Locale(ctx context.Context) string {
	_, locale, err := s.localeRoot(ctx)
	if err != nil {
		return s.config.DefaultLocale
	}
	return locale
}

func (s *ContentService) getFromLocale(locale, docPath string) (*Document, error) {
	root := filepath.Join(s.config.Root, locale)
	fullPath := filepath.Join(root, filepath.FromSlash(docPath))
	if !isWithin(root, fullPath) {
		return nil, ErrInvalidPath
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}
	return &Document{Title: titleFromMarkdown(content, docPath), Path: docPath, Content: content}, nil
}

func (s *ContentService) localeRoot(ctx context.Context) (string, string, error) {
	if s.config.Root == "" {
		return "", "", ErrContentRootRequired
	}
	locale := localeString(ctx, s.config.DefaultLocale)
	if !contains(s.config.Locales, locale) {
		locale = s.config.DefaultLocale
	}
	root := filepath.Join(s.config.Root, locale)
	if _, err := os.Stat(root); err != nil && locale != s.config.DefaultLocale {
		locale = s.config.DefaultLocale
		root = filepath.Join(s.config.Root, locale)
	}
	return root, locale, nil
}

func (s *ContentService) buildTree(root, locale string) ([]viewmodels.CategoryNode, error) {
	var docs []viewmodels.CategoryNode

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isMarkdown(path) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		slashPath := filepath.ToSlash(rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		parts := strings.Split(slashPath, "/")
		doc := viewmodels.CategoryNode{Title: titleFromMarkdown(content, slashPath), Path: slashPath}
		insertNode(&docs, parts[:len(parts)-1], doc)
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && locale != s.config.DefaultLocale {
			return s.buildTree(filepath.Join(s.config.Root, s.config.DefaultLocale), s.config.DefaultLocale)
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

func cleanDocPath(path string) (string, error) {
	path = strings.TrimSpace(strings.TrimPrefix(path, "/"))
	if path == "" || strings.Contains(path, "\x00") {
		return "", ErrInvalidPath
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." || filepath.IsAbs(clean) || !isMarkdown(clean) {
		return "", ErrInvalidPath
	}
	return clean, nil
}

func isWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

func isMarkdown(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}

func titleFromMarkdown(content []byte, path string) string {
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
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return titleFromSegment(name)
}

func titleFromSegment(segment string) string {
	segment = strings.TrimSuffix(segment, filepath.Ext(segment))
	segment = strings.ReplaceAll(segment, "-", " ")
	segment = strings.ReplaceAll(segment, "_", " ")
	return strings.Title(segment)
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
