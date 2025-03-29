package dialogue

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/bichat/domain/entities/llm"
)

func New(tenantID uint, userID uint, label string) Dialogue {
	return &dialogue{
		tenantID:  tenantID,
		userID:    userID,
		label:     label,
		messages:  Messages{},
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
}

func NewWithID(id uint, tenantID uint, userID uint, label string, messages Messages, createdAt, updatedAt time.Time) Dialogue {
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
	tenantID  uint
	userID    uint
	label     string
	messages  Messages
	createdAt time.Time
	updatedAt time.Time
}

func (d *dialogue) ID() uint {
	return d.id
}

func (d *dialogue) TenantID() uint {
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

// TODO: handle empty messages
func (d *dialogue) LastMessage() llm.ChatCompletionMessage {
	if len(d.messages) == 0 {
		return llm.ChatCompletionMessage{}
	}
	return d.messages[len(d.messages)-1]
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
