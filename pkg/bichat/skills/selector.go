package skills

import (
	"context"
	"regexp"
	"sort"
	"strings"
)

var (
	mentionRegex = regexp.MustCompile(`@([a-z0-9][a-z0-9_/-]*)`)
	tokenRegex   = regexp.MustCompile(`[a-z0-9]+`)
)

type selector struct {
	catalog        *Catalog
	selectionLimit int
	maxChars       int
	tokensBySlug   map[string]map[string]struct{}
}

type SelectorOption func(*selector)

func WithSelectionLimit(limit int) SelectorOption {
	return func(s *selector) {
		s.selectionLimit = limit
	}
}

func WithMaxChars(maxChars int) SelectorOption {
	return func(s *selector) {
		s.maxChars = maxChars
	}
}

func NewSelector(catalog *Catalog, opts ...SelectorOption) Selector {
	s := &selector{
		catalog:        catalog,
		selectionLimit: defaultSelectionLimit,
		maxChars:       defaultMaxChars,
		tokensBySlug:   buildSkillTokenIndex(catalog),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.selectionLimit <= 0 {
		s.selectionLimit = defaultSelectionLimit
	}
	if s.maxChars <= 0 {
		s.maxChars = defaultMaxChars
	}
	return s
}

func (s *selector) Select(ctx context.Context, req SelectionRequest) (SelectionResult, error) {
	_ = ctx
	if s.catalog == nil || len(s.catalog.Skills) == 0 {
		return SelectionResult{}, nil
	}

	message := strings.ToLower(strings.TrimSpace(req.Message))
	mentioned := parseMentionedSlugs(message)

	selected := make([]SelectedSkill, 0, s.selectionLimit)
	explicitSet := make(map[string]struct{})
	resolvedMentions := make([]string, 0, len(mentioned))

	for _, slug := range mentioned {
		skill, ok := s.catalog.Get(slug)
		if !ok {
			continue
		}
		if _, exists := explicitSet[skill.Slug]; exists {
			continue
		}
		explicitSet[skill.Slug] = struct{}{}
		resolvedMentions = append(resolvedMentions, skill.Slug)
		selected = append(selected, SelectedSkill{
			Skill:    skill,
			Priority: 1,
			Score:    1000,
			Source:   "mention",
		})
		if len(selected) >= s.selectionLimit {
			reference := RenderReference(selected, s.maxChars)
			return SelectionResult{Selected: selected, Reference: reference, Mentioned: resolvedMentions}, nil
		}
	}

	queryTokens := tokenize(message)
	autoCandidates := make([]SelectedSkill, 0)
	for _, skill := range s.catalog.Skills {
		if _, explicit := explicitSet[skill.Slug]; explicit {
			continue
		}
		skillTokens := s.tokensBySlug[skill.Slug]
		score := lexicalOverlapScore(queryTokens, skillTokens)
		if score == 0 {
			continue
		}
		autoCandidates = append(autoCandidates, SelectedSkill{
			Skill:    skill,
			Priority: 2,
			Score:    score,
			Source:   "auto",
		})
	}

	sort.Slice(autoCandidates, func(i, j int) bool {
		if autoCandidates[i].Score != autoCandidates[j].Score {
			return autoCandidates[i].Score > autoCandidates[j].Score
		}
		return autoCandidates[i].Skill.Slug < autoCandidates[j].Skill.Slug
	})

	for _, candidate := range autoCandidates {
		if len(selected) >= s.selectionLimit {
			break
		}
		selected = append(selected, candidate)
	}

	reference := RenderReference(selected, s.maxChars)

	skipped := make([]string, 0)
	if reference == "" && len(selected) > 0 {
		for _, item := range selected {
			skipped = append(skipped, item.Skill.Slug)
		}
	}

	return SelectionResult{
		Selected:   selected,
		Reference:  reference,
		Mentioned:  resolvedMentions,
		SkippedFor: skipped,
	}, nil
}

func parseMentionedSlugs(message string) []string {
	matches := mentionRegex.FindAllStringSubmatch(message, -1)
	if len(matches) == 0 {
		return nil
	}

	result := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		slug := strings.Trim(strings.ToLower(match[1]), "/")
		if slug == "" {
			continue
		}
		result = append(result, slug)
	}
	return result
}

func buildSkillTokenIndex(catalog *Catalog) map[string]map[string]struct{} {
	index := make(map[string]map[string]struct{})
	if catalog == nil {
		return index
	}
	for _, skill := range catalog.Skills {
		tokens := make(map[string]struct{})
		for _, token := range tokenize(skill.Slug) {
			tokens[token] = struct{}{}
		}
		for _, token := range tokenize(skill.Metadata.Name) {
			tokens[token] = struct{}{}
		}
		for _, token := range tokenize(skill.Metadata.Description) {
			tokens[token] = struct{}{}
		}
		for _, hint := range skill.Metadata.WhenToUse {
			for _, token := range tokenize(hint) {
				tokens[token] = struct{}{}
			}
		}
		for _, tag := range skill.Metadata.Tags {
			for _, token := range tokenize(tag) {
				tokens[token] = struct{}{}
			}
		}
		index[skill.Slug] = tokens
	}
	return index
}

func lexicalOverlapScore(queryTokens []string, skillTokens map[string]struct{}) int {
	if len(queryTokens) == 0 || len(skillTokens) == 0 {
		return 0
	}
	seen := make(map[string]struct{})
	score := 0
	for _, token := range queryTokens {
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		if _, ok := skillTokens[token]; ok {
			score++
		}
	}
	return score
}

func tokenize(text string) []string {
	if text == "" {
		return nil
	}
	matches := tokenRegex.FindAllString(strings.ToLower(text), -1)
	if len(matches) == 0 {
		return nil
	}
	return matches
}
