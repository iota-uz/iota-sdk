package dialogue

import (
	"time"
)

func New(userID uint, label string) Dialogue {
	return &dialogue{
		userID:    userID,
		label:     label,
		messages:  Messages{},
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
}

func NewWithID(id uint, userID uint, label string, messages Messages, createdAt, updatedAt time.Time) Dialogue {
	return &dialogue{
		id:        id,
		userID:    userID,
		label:     label,
		messages:  messages,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

type dialogue struct {
	id        uint
	userID    uint
	label     string
	messages  Messages
	createdAt time.Time
	updatedAt time.Time
}

func (d *dialogue) ID() uint {
	return d.id
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

func (d *dialogue) CreatedAt() time.Time {
	return d.createdAt
}

func (d *dialogue) UpdatedAt() time.Time {
	return d.updatedAt
}

func (d *dialogue) AddMessage(msg ChatCompletionMessage) Dialogue {
	return &dialogue{
		id:        d.id,
		userID:    d.userID,
		label:     d.label,
		messages:  append(d.messages, msg),
		createdAt: d.createdAt,
		updatedAt: time.Now(),
	}
}
