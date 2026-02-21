package services

import (
	"fmt"
	"strings"

	moduleprompts "github.com/iota-uz/iota-sdk/modules/bichat/prompts"
)

func renderSessionTitlePrompt(userMessage, assistantMessage string) (string, error) {
	prompt, err := moduleprompts.RenderTitleGenerationPrompt(moduleprompts.TitleGenerationInput{
		UserMessage:      strings.TrimSpace(userMessage),
		AssistantMessage: strings.TrimSpace(assistantMessage),
	})
	if err != nil {
		return "", fmt.Errorf("render session title prompt: %w", err)
	}
	return prompt, nil
}
