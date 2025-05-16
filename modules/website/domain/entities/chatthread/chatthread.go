package chatthread

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
)

var (
	ErrChatThreadNotFound = errors.New("chat thread not found")
)

type ChatThread interface {
	ID() uuid.UUID
	Timestamp() time.Time
	ChatID() uint
	Messages() []chat.Message
}

type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (ChatThread, error)
	Save(ctx context.Context, thread ChatThread) (ChatThread, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]ChatThread, error)
}

type chatThread struct {
	id        uuid.UUID
	timestamp time.Time
	chatID    uint
	messages  []chat.Message
}

func New(chatID uint, messages []chat.Message, opts ...Option) ChatThread {
	thread := &chatThread{
		id:        uuid.New(),
		timestamp: time.Now(),
		chatID:    chatID,
		messages:  messages,
	}

	for _, opt := range opts {
		opt(thread)
	}

	return thread
}

type Option func(*chatThread)

func WithID(id uuid.UUID) Option {
	return func(t *chatThread) {
		if id != uuid.Nil {
			t.id = id
		}
	}
}

func WithTimestamp(timestamp time.Time) Option {
	return func(t *chatThread) {
		if !timestamp.IsZero() {
			t.timestamp = timestamp
		}
	}
}

func (t *chatThread) ID() uuid.UUID {
	return t.id
}

func (t *chatThread) Timestamp() time.Time {
	return t.timestamp
}

func (t *chatThread) ChatID() uint {
	return t.chatID
}

func (t *chatThread) Messages() []chat.Message {
	filteredMessages := make([]chat.Message, 0, len(t.messages))

	for _, msg := range t.messages {
		if !msg.CreatedAt().Before(t.timestamp) {
			filteredMessages = append(filteredMessages, msg)
		}
	}

	return filteredMessages
}
