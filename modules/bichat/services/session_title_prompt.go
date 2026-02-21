package services

import (
	"strings"

	moduleprompts "github.com/iota-uz/iota-sdk/modules/bichat/prompts"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func renderSessionTitlePrompt(userMessage, assistantMessage string) (string, error) {
	const op serrors.Op = "services.renderSessionTitlePrompt"

	prompt, err := moduleprompts.RenderTitleGenerationPrompt(moduleprompts.TitleGenerationInput{
		UserMessage:      strings.TrimSpace(userMessage),
		AssistantMessage: strings.TrimSpace(assistantMessage),
	})
	if err != nil {
		return "", serrors.E(op, err)
	}
	return prompt, nil
}
