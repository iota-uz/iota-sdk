package skills

import "context"

const (
	defaultSelectionLimit = 3
	defaultMaxChars       = 8000
)

// SkillMetadata stores required frontmatter fields from SKILL.md.
type SkillMetadata struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	WhenToUse   []string `yaml:"when_to_use"`
	Tags        []string `yaml:"tags"`
}

// Skill represents a loaded skill markdown document.
type Skill struct {
	Slug       string
	Path       string
	ParentSlug string
	Metadata   SkillMetadata
	Body       string
}

// Catalog is an indexed, immutable skill tree loaded from filesystem.
type Catalog struct {
	Root     string
	Skills   []Skill
	BySlug   map[string]Skill
	Children map[string][]string
}

// SelectionRequest is input for selecting skills for a chat turn.
type SelectionRequest struct {
	Message string
}

// SelectedSkill is a ranked skill chosen for injection.
type SelectedSkill struct {
	Skill    Skill
	Priority int
	Score    int
	Source   string
}

// SelectionResult is the final selection and rendered reference text.
type SelectionResult struct {
	Selected   []SelectedSkill
	Reference  string
	Mentioned  []string
	SkippedFor []string
}

// Selector decides which skills to inject for a turn.
type Selector interface {
	Select(ctx context.Context, req SelectionRequest) (SelectionResult, error)
}

func (c *Catalog) Get(slug string) (Skill, bool) {
	if c == nil || c.BySlug == nil {
		return Skill{}, false
	}
	skill, ok := c.BySlug[slug]
	return skill, ok
}
