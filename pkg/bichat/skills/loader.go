package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const skillFileName = "SKILL.md"

type skillFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	WhenToUse   []string `yaml:"when_to_use"`
	Tags        []string `yaml:"tags"`
}

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

	catalog := &Catalog{
		Root:     root,
		Skills:   make([]Skill, 0),
		BySlug:   make(map[string]Skill),
		Children: make(map[string][]string),
	}

	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || d.Name() != skillFileName {
			return nil
		}

		skill, err := loadSkill(root, path)
		if err != nil {
			return err
		}

		if _, exists := catalog.BySlug[skill.Slug]; exists {
			return fmt.Errorf("duplicate skill slug %q", skill.Slug)
		}
		catalog.BySlug[skill.Slug] = skill
		catalog.Skills = append(catalog.Skills, skill)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
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

func loadSkill(root, path string) (Skill, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, fmt.Errorf("failed to read %q: %w", path, err)
	}

	frontmatterText, body, err := splitFrontmatter(string(raw))
	if err != nil {
		return Skill{}, fmt.Errorf("%s: %w", path, err)
	}

	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatterText), &fm); err != nil {
		return Skill{}, fmt.Errorf("%s: failed to parse frontmatter: %w", path, err)
	}

	metadata := normalizeMetadata(SkillMetadata{
		Name:        fm.Name,
		Description: fm.Description,
		WhenToUse:   fm.WhenToUse,
		Tags:        fm.Tags,
	})
	if err := validateMetadata(metadata); err != nil {
		return Skill{}, fmt.Errorf("%s: %w", path, err)
	}

	body = strings.TrimSpace(body)
	if body == "" {
		return Skill{}, fmt.Errorf("%s: skill body cannot be empty", path)
	}

	relDir, err := filepath.Rel(root, filepath.Dir(path))
	if err != nil {
		return Skill{}, fmt.Errorf("failed to resolve relative path for %q: %w", path, err)
	}

	slug := buildSlug(relDir)
	parentSlug := parentSlugFromSlug(slug)

	return Skill{
		Slug:       slug,
		Path:       path,
		ParentSlug: parentSlug,
		Metadata:   metadata,
		Body:       body,
	}, nil
}

func splitFrontmatter(raw string) (string, string, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("missing YAML frontmatter block")
	}

	closing := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closing = i
			break
		}
	}
	if closing == -1 {
		return "", "", fmt.Errorf("unterminated YAML frontmatter block")
	}

	frontmatter := strings.Join(lines[1:closing], "\n")
	body := strings.Join(lines[closing+1:], "\n")
	return frontmatter, body, nil
}

func buildSlug(relDir string) string {
	slug := filepath.ToSlash(relDir)
	slug = strings.TrimSpace(slug)
	if slug == "." || slug == "" {
		return "root"
	}
	return strings.ToLower(strings.Trim(slug, "/"))
}

func parentSlugFromSlug(slug string) string {
	if slug == "" || slug == "root" {
		return ""
	}
	parent := filepath.ToSlash(filepath.Dir(slug))
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
