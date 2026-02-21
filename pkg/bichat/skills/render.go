package skills

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

func RenderReference(selected []SelectedSkill, maxChars int) string {
	if len(selected) == 0 {
		return ""
	}

	sorted := append([]SelectedSkill(nil), selected...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority < sorted[j].Priority
		}
		return sorted[i].Skill.Slug < sorted[j].Skill.Slug
	})

	var b strings.Builder
	header := "SKILLS CONTEXT:\nUse these project skills when relevant.\n\n"
	b.WriteString(header)
	runeCount := utf8.RuneCountInString(header)

	included := 0
	for _, item := range sorted {
		section := renderSkillSection(item)
		sectionRunes := utf8.RuneCountInString(section)
		if maxChars > 0 && runeCount+sectionRunes > maxChars {
			continue
		}
		b.WriteString(section)
		runeCount += sectionRunes
		included++
	}

	if included == 0 {
		fallback := strings.TrimSpace(header + renderSkillSection(sorted[0]))
		return trimToChars(fallback, maxChars)
	}

	return strings.TrimSpace(b.String())
}

func renderSkillSection(item SelectedSkill) string {
	meta := item.Skill.Metadata

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## @%s\n", item.Skill.Slug))
	b.WriteString(fmt.Sprintf("name: %s\n", meta.Name))
	b.WriteString(fmt.Sprintf("description: %s\n", meta.Description))
	b.WriteString("when_to_use:\n")
	for _, hint := range meta.WhenToUse {
		b.WriteString("- ")
		b.WriteString(hint)
		b.WriteString("\n")
	}
	b.WriteString("tags: ")
	b.WriteString(strings.Join(meta.Tags, ", "))
	b.WriteString("\n")
	b.WriteString("instructions:\n")
	b.WriteString(item.Skill.Body)
	b.WriteString("\n\n")
	return b.String()
}

func trimToChars(text string, maxChars int) string {
	if maxChars <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}
	return strings.TrimSpace(string(runes[:maxChars]))
}
