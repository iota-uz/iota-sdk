package skills

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const defaultCatalogRenderLimit = 50

// RenderCatalogReference renders a compact list of available skills for system/reference context.
// The list is intentionally metadata-only; full skill bodies must be loaded via tool.
func RenderCatalogReference(catalog *Catalog, limit int, maxChars int) string {
	if catalog == nil || len(catalog.Skills) == 0 {
		return ""
	}
	if limit <= 0 {
		limit = defaultCatalogRenderLimit
	}

	skills := append([]Skill(nil), catalog.Skills...)
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Slug < skills[j].Slug
	})

	header := "SKILLS CATALOG:\n" +
		"Use `load_skill` to load full instructions when a task needs a specialized workflow.\n" +
		"Pass the exact slug from this catalog.\n\n" +
		"Available skills:\n"

	var b strings.Builder
	b.WriteString(header)
	runeCount := utf8.RuneCountInString(header)

	listed := 0
	for _, skill := range skills {
		if listed >= limit {
			break
		}
		line := fmt.Sprintf("- @%s: %s. %s\n",
			skill.Slug,
			singleLine(skill.Metadata.Name),
			singleLine(skill.Metadata.Description),
		)
		lineRunes := utf8.RuneCountInString(line)
		if maxChars > 0 && runeCount+lineRunes > maxChars {
			break
		}
		b.WriteString(line)
		runeCount += lineRunes
		listed++
	}

	if listed < len(skills) {
		remaining := len(skills) - listed
		footer := fmt.Sprintf("\nNote: %d additional skill(s) omitted due to catalog limits.", remaining)
		if maxChars <= 0 || runeCount+utf8.RuneCountInString(footer) <= maxChars {
			b.WriteString(footer)
		}
	}

	return strings.TrimSpace(b.String())
}

// RenderLoadedSkill renders the full body of a loaded skill as tool output.
func RenderLoadedSkill(skill Skill, maxChars int) string {
	body := strings.TrimSpace(skill.Body)
	result := fmt.Sprintf(
		"SKILL LOADED: @%s\nname: %s\ndescription: %s\npath: %s\n\ninstructions:\n%s",
		skill.Slug,
		singleLine(skill.Metadata.Name),
		singleLine(skill.Metadata.Description),
		skill.Path,
		body,
	)

	if maxChars > 0 && utf8.RuneCountInString(result) > maxChars {
		return trimToChars(result, maxChars)
	}
	return strings.TrimSpace(result)
}

func singleLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}
