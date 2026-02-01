package dialogue

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/domain/entities/llm"
)

func New(tenantID uuid.UUID, userID uint, label string) Dialogue {
	return &dialogue{
		tenantID:  tenantID,
		userID:    userID,
		label:     label,
		messages:  Messages{},
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
}

func NewWithID(id uint, tenantID uuid.UUID, userID uint, label string, messages Messages, createdAt, updatedAt time.Time) Dialogue {
	return &dialogue{
		id:        id,
		tenantID:  tenantID,
		userID:    userID,
		label:     label,
		messages:  messages,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

type dialogue struct {
	id        uint
	tenantID  uuid.UUID
	userID    uint
	label     string
	messages  Messages
	createdAt time.Time
	updatedAt time.Time
}

func (d *dialogue) ID() uint {
	return d.id
}

func (d *dialogue) TenantID() uuid.UUID {
	return d.tenantID
}

func (d *dialogue) UserID() uint {
	return d.userID
}

func (d *dialogue) Label() string {
	return d.label
}

func (d *dialogue) Messages() Messages {
	return d.messages
}

func (d *dialogue) LastMessage() (llm.ChatCompletionMessage, error) {
	if len(d.messages) == 0 {
		return llm.ChatCompletionMessage{}, ErrNoMessages
	}
	return d.messages[len(d.messages)-1], nil
}

func (d *dialogue) CreatedAt() time.Time {
	return d.createdAt
}

func (d *dialogue) UpdatedAt() time.Time {
	return d.updatedAt
}

func (d *dialogue) AddMessages(messages ...llm.ChatCompletionMessage) Dialogue {
	return &dialogue{
		id:        d.id,
		tenantID:  d.tenantID,
		userID:    d.userID,
		label:     d.label,
		messages:  append(d.messages, messages...),
		createdAt: d.createdAt,
		updatedAt: time.Now(),
	}
}

func (d *dialogue) SetMessages(messages Messages) Dialogue {
	return &dialogue{
		id:        d.id,
		tenantID:  d.tenantID,
		userID:    d.userID,
		label:     d.label,
		messages:  messages,
		createdAt: d.createdAt,
		updatedAt: time.Now(),
	}
}

func (d *dialogue) SetLastMessage(msg llm.ChatCompletionMessage) Dialogue {
	messages := d.messages
	messages[len(messages)-1] = msg
	return &dialogue{
		id:        d.id,
		tenantID:  d.tenantID,
		userID:    d.userID,
		label:     d.label,
		messages:  messages,
		createdAt: d.createdAt,
		updatedAt: time.Now(),
	}
}
