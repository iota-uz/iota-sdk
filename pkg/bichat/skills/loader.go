package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/definition"
)

const skillFileName = "SKILL.md"

// LoadCatalog recursively scans the root directory and loads all SKILL.md files.
func LoadCatalog(root string) (*Catalog, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("skills root directory is required")
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("failed to stat skills root %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skills root must be a directory")
	}

	catalog, err := LoadCatalogFS(os.DirFS(root), ".")
	if err != nil {
		return nil, err
	}
	catalog.Root = root

	bySlug := make(map[string]Skill, len(catalog.Skills))
	for i := range catalog.Skills {
		skill := catalog.Skills[i]
		skill.Path = filepath.Join(root, filepath.FromSlash(skill.Path))
		catalog.Skills[i] = skill
		bySlug[skill.Slug] = skill
	}
	catalog.BySlug = bySlug
	return catalog, nil
}

// LoadCatalogFS recursively scans root within the provided filesystem and loads
// all SKILL.md files.
func LoadCatalogFS(source fs.FS, root string) (*Catalog, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}

	catalog := &Catalog{
		Root:     root,
		Skills:   make([]Skill, 0),
		BySlug:   make(map[string]Skill),
		Children: make(map[string][]string),
	}

	files, err := definition.LoadFiles(source, definition.LoadFilesOptions{
		Root:      root,
		Recursive: true,
		Match: func(_ string, entry fs.DirEntry) bool {
			return entry.Name() == skillFileName
		},
	})
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		skill, err := loadSkill(root, file)
		if err != nil {
			return nil, err
		}

		if _, exists := catalog.BySlug[skill.Slug]; exists {
			return nil, fmt.Errorf("duplicate skill slug %q", skill.Slug)
		}
		catalog.BySlug[skill.Slug] = skill
		catalog.Skills = append(catalog.Skills, skill)
	}

	sort.Slice(catalog.Skills, func(i, j int) bool {
		return catalog.Skills[i].Slug < catalog.Skills[j].Slug
	})

	for _, skill := range catalog.Skills {
		parent := skill.ParentSlug
		catalog.Children[parent] = append(catalog.Children[parent], skill.Slug)
	}
	for parent := range catalog.Children {
		sort.Strings(catalog.Children[parent])
	}

	return catalog, nil
}

func loadSkill(root string, source definition.SourceFile) (Skill, error) {
	parsed, err := definition.ParseDocument[SkillMetadata](
		source.Content,
		source.Path,
		definition.ParseDocumentOptions{
			KnownFields: true,
			RequireBody: true,
		},
	)
	if err != nil {
		return Skill{}, err
	}

	metadata := normalizeMetadata(parsed.FrontMatter)
	if err := validateMetadata(metadata); err != nil {
		return Skill{}, fmt.Errorf("%s: %w", parsed.Path, err)
	}

	dirPath := path.Dir(source.Path)
	relDir := dirPath
	if root != "." {
		relNative, relErr := filepath.Rel(filepath.FromSlash(root), filepath.FromSlash(dirPath))
		if relErr != nil {
			return Skill{}, fmt.Errorf("failed to resolve relative path for %q: %w", source.Path, relErr)
		}
		relDir = filepath.ToSlash(relNative)
	}

	slug := buildSlug(relDir)
	parentSlug := parentSlugFromSlug(slug)

	return Skill{
		Slug:       slug,
		Path:       source.Path,
		ParentSlug: parentSlug,
		Metadata:   metadata,
		Body:       parsed.Body,
	}, nil
}

func buildSlug(relDir string) string {
	slug := strings.TrimSpace(relDir)
	if slug == "." || slug == "" {
		return "root"
	}
	slug = path.Clean(slug)
	return strings.ToLower(strings.Trim(slug, "/"))
}

func parentSlugFromSlug(slug string) string {
	if slug == "" || slug == "root" {
		return ""
	}
	parent := path.Dir(slug)
	if parent == "." || parent == "" {
		return ""
	}
	return strings.ToLower(strings.Trim(parent, "/"))
}

func normalizeMetadata(meta SkillMetadata) SkillMetadata {
	meta.Name = strings.TrimSpace(meta.Name)
	meta.Description = strings.TrimSpace(meta.Description)

	cleanWhenToUse := make([]string, 0, len(meta.WhenToUse))
	for _, item := range meta.WhenToUse {
		item = strings.TrimSpace(item)
		if item != "" {
			cleanWhenToUse = append(cleanWhenToUse, item)
		}
	}
	meta.WhenToUse = cleanWhenToUse

	cleanTags := make([]string, 0, len(meta.Tags))
	for _, tag := range meta.Tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			cleanTags = append(cleanTags, tag)
		}
	}
	meta.Tags = cleanTags

	return meta
}

func validateMetadata(meta SkillMetadata) error {
	if meta.Name == "" {
		return fmt.Errorf("frontmatter field \"name\" is required")
	}
	if meta.Description == "" {
		return fmt.Errorf("frontmatter field \"description\" is required")
	}
	if len(meta.WhenToUse) == 0 {
		return fmt.Errorf("frontmatter field \"when_to_use\" is required")
	}
	if len(meta.Tags) == 0 {
		return fmt.Errorf("frontmatter field \"tags\" is required")
	}
	return nil
}
