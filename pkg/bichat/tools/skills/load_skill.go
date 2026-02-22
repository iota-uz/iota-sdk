package skills

import (
	"context"
	"fmt"
	"sort"
	"strings"

	bichatagents "github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/logging"
	bichatskills "github.com/iota-uz/iota-sdk/pkg/bichat/skills"
)

const (
	defaultLoadSkillMaxChars = 16000
	loadSkillToolName        = "load_skill"
)

type LoadSkillTool struct {
	catalog  *bichatskills.Catalog
	maxChars int
	logger   logging.Logger
}

// LoadSkillToolOption customizes LoadSkillTool behavior.
type LoadSkillToolOption func(*LoadSkillTool)

// WithLoadSkillMaxChars sets the maximum number of characters returned by load_skill.
func WithLoadSkillMaxChars(maxChars int) LoadSkillToolOption {
	return func(t *LoadSkillTool) {
		t.maxChars = maxChars
	}
}

// WithLoadSkillLogger sets the logger for the tool.
func WithLoadSkillLogger(logger logging.Logger) LoadSkillToolOption {
	return func(t *LoadSkillTool) {
		if logger != nil {
			t.logger = logger
		}
	}
}

// NewLoadSkillTool creates a tool that returns full markdown instructions for one skill.
func NewLoadSkillTool(catalog *bichatskills.Catalog, opts ...LoadSkillToolOption) *LoadSkillTool {
	tool := &LoadSkillTool{
		catalog:  catalog,
		maxChars: defaultLoadSkillMaxChars,
		logger:   logging.NewNoOpLogger(),
	}
	for _, opt := range opts {
		opt(tool)
	}
	if tool.maxChars <= 0 {
		tool.maxChars = defaultLoadSkillMaxChars
	}
	return tool
}

func (t *LoadSkillTool) Name() string {
	return loadSkillToolName
}

func (t *LoadSkillTool) Description() string {
	return "Load full instructions for one skill from the skills catalog. " +
		"Always pass a skill slug from SKILLS CATALOG (e.g. \"finance/month-end\")."
}

func (t *LoadSkillTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"skill": map[string]any{
				"type":        "string",
				"description": "Skill slug from SKILLS CATALOG, e.g. \"insurance/reserves\".",
			},
		},
		"required":             []string{"skill"},
		"additionalProperties": false,
	}
}

type loadSkillInput struct {
	Skill string `json:"skill"`
}

func (t *LoadSkillTool) Call(_ context.Context, input string) (string, error) {
	if t.catalog == nil || len(t.catalog.Skills) == 0 {
		return "No skills catalog is configured for this agent.", nil
	}

	args, err := bichatagents.ParseToolInput[loadSkillInput](input)
	if err != nil {
		return "", fmt.Errorf("invalid load_skill arguments: %w", err)
	}

	identifier := strings.TrimSpace(args.Skill)
	if identifier == "" {
		return "Missing required field `skill`. Pass a skill slug from SKILLS CATALOG.", nil
	}

	skill, suggestions, ok := resolveSkill(t.catalog, sanitizeSkillIdentifier(identifier))
	if !ok {
		msg := fmt.Sprintf("Skill not found: %q.", identifier)
		if len(suggestions) > 0 {
			msg += "\nClosest available slugs:\n- " + strings.Join(suggestions, "\n- ")
		}
		return msg, nil
	}

	return bichatskills.RenderLoadedSkill(skill, t.maxChars), nil
}

func resolveSkill(catalog *bichatskills.Catalog, identifier string) (bichatskills.Skill, []string, bool) {
	slug := normalizeSkillSlug(identifier)
	if skill, ok := catalog.Get(slug); ok {
		return skill, nil, true
	}

	// Fallback: exact name match (case-insensitive) when unique.
	matches := make([]bichatskills.Skill, 0)
	for _, skill := range catalog.Skills {
		if strings.EqualFold(strings.TrimSpace(skill.Metadata.Name), strings.TrimSpace(identifier)) {
			matches = append(matches, skill)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil, true
	}

	return bichatskills.Skill{}, closestSlugs(catalog, slug, 8), false
}

func sanitizeSkillIdentifier(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}
	normalized = strings.TrimPrefix(normalized, "@")
	normalized = strings.TrimSuffix(normalized, "/SKILL.md")
	normalized = strings.TrimSuffix(normalized, "/skill.md")
	return strings.TrimSpace(normalized)
}

func normalizeSkillSlug(value string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(value)), "/")
}

func closestSlugs(catalog *bichatskills.Catalog, fragment string, limit int) []string {
	if catalog == nil || len(catalog.Skills) == 0 || limit <= 0 {
		return nil
	}

	prefix := make([]string, 0, limit)
	contains := make([]string, 0, limit)
	other := make([]string, 0, limit)

	for _, skill := range catalog.Skills {
		slug := skill.Slug
		switch {
		case strings.HasPrefix(slug, fragment):
			prefix = append(prefix, slug)
		case strings.Contains(slug, fragment):
			contains = append(contains, slug)
		default:
			other = append(other, slug)
		}
	}

	sort.Strings(prefix)
	sort.Strings(contains)
	sort.Strings(other)

	combined := append(prefix, contains...)
	combined = append(combined, other...)
	if len(combined) > limit {
		combined = combined[:limit]
	}
	return combined
}
