package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed title_generation.prompt
var titleGenerationPromptFS embed.FS

var titleGenerationTemplate = template.Must(template.ParseFS(
	titleGenerationPromptFS,
	"title_generation.prompt",
))

// TitleGenerationInput contains prompt variables for session title generation.
type TitleGenerationInput struct {
	UserMessage      string
	AssistantMessage string
}

// RenderTitleGenerationPrompt renders the title generation prompt template.
func RenderTitleGenerationPrompt(input TitleGenerationInput) (string, error) {
	var buf bytes.Buffer
	if err := titleGenerationTemplate.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("render title_generation.prompt: %w", err)
	}
	return buf.String(), nil
}
