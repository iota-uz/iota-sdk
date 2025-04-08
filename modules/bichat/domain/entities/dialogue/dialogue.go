package dialogue

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/domain/entities/llm"
)

type Messages []llm.ChatCompletionMessage

type Dialogue interface {
	ID() uint
	TenantID() uuid.UUID
	UserID() uint
	Label() string
	Messages() Messages
	LastMessage() llm.ChatCompletionMessage
	CreatedAt() time.Time
	UpdatedAt() time.Time

	AddMessages(messages ...llm.ChatCompletionMessage) Dialogue
	SetMessages(messages Messages) Dialogue
	SetLastMessage(msg llm.ChatCompletionMessage) Dialogue
}
