package persistence

import (
	"encoding/json"

	"github.com/iota-uz/iota-sdk/modules/bichat/domain/entities/dialogue"
	"github.com/iota-uz/iota-sdk/modules/bichat/domain/entities/llm"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence/models"
)

func toDBChatCompletionMessage(messages []llm.ChatCompletionMessage) (string, error) {
	bytes, err := json.Marshal(messages)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func toDomainChatCompletionMessage(dbMessages string) ([]llm.ChatCompletionMessage, error) {
	var messages []llm.ChatCompletionMessage
	if err := json.Unmarshal([]byte(dbMessages), &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func toDBDialogue(entity dialogue.Dialogue) (*models.Dialogue, error) {
	dbMessages, err := toDBChatCompletionMessage(entity.Messages())
	if err != nil {
		return nil, err
	}
	return &models.Dialogue{
		ID:        entity.ID(),
		TenantID:  entity.TenantID(),
		UserID:    entity.UserID(),
		Label:     entity.Label(),
		Messages:  dbMessages,
		CreatedAt: entity.CreatedAt(),
		UpdatedAt: entity.UpdatedAt(),
	}, nil
}

func toDomainDialogue(dbDialogue *models.Dialogue) (dialogue.Dialogue, error) {
	messages, err := toDomainChatCompletionMessage(dbDialogue.Messages)
	if err != nil {
		return nil, err
	}
	return dialogue.NewWithID(
		dbDialogue.ID,
		dbDialogue.TenantID,
		dbDialogue.UserID,
		dbDialogue.Label,
		messages,
		dbDialogue.CreatedAt,
		dbDialogue.UpdatedAt,
	), nil
}
