package message

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func New(
	chatID uint,
	msg string,
	sender Sender,
) Message {
	return &message{
		id:          0,
		chatID:      chatID,
		message:     msg,
		sender:      sender,
		isRead:      false,
		readAt:      nil,
		attachments: []*upload.Upload{},
		createdAt:   time.Now(),
	}
}

func WithAttachments(
	chatID uint,
	msg string,
	sender Sender,
	attachments ...*upload.Upload,
) Message {
	return &message{
		id:          0,
		chatID:      chatID,
		message:     msg,
		sender:      sender,
		isRead:      false,
		readAt:      nil,
		attachments: attachments,
		createdAt:   time.Now(),
	}
}

func NewWithID(
	id uint,
	chatID uint,
	msg string,
	sender Sender,
	isRead bool,
	attachments []*upload.Upload,
	createdAt time.Time,
) Message {
	return &message{
		id:          id,
		chatID:      chatID,
		message:     msg,
		sender:      sender,
		isRead:      isRead,
		readAt:      nil,
		attachments: attachments,
		createdAt:   createdAt,
	}
}

type message struct {
	id          uint
	chatID      uint
	message     string
	sender      Sender
	isRead      bool
	readAt      *time.Time
	attachments []*upload.Upload
	createdAt   time.Time
}

func (m *message) ID() uint {
	return m.id
}

func (m *message) ChatID() uint {
	return m.chatID
}

func (m *message) Message() string {
	return m.message
}

func (m *message) Sender() Sender {
	return m.sender
}

func (m *message) IsRead() bool {
	return m.isRead
}

func (m *message) MarkAsRead() Message {
	return &message{
		id:          m.id,
		chatID:      m.chatID,
		message:     m.message,
		sender:      m.sender,
		isRead:      true,
		readAt:      mapping.Pointer(time.Now()),
		attachments: m.attachments,
		createdAt:   m.createdAt,
	}
}

func (m *message) ReadAt() *time.Time {
	return m.readAt
}

func (m *message) Attachments() []*upload.Upload {
	return m.attachments
}

func (m *message) CreatedAt() time.Time {
	return m.createdAt
}

// --------
// Sender
// --------

func NewUserSender(clientID uint, firstName, lastName string) Sender {
	return &sender{
		id:        clientID,
		isClient:  false,
		firstName: firstName,
		lastName:  lastName,
	}
}

func NewClientSender(userID uint, firstName, lastName string) Sender {
	return &sender{
		id:        userID,
		isClient:  true,
		firstName: firstName,
		lastName:  lastName,
	}
}

type sender struct {
	id        uint
	isClient  bool
	firstName string
	lastName  string
}

func (s *sender) ID() uint {
	return s.id
}

func (s *sender) IsClient() bool {
	return s.isClient
}

func (s *sender) IsUser() bool {
	return !s.isClient
}

func (s *sender) FirstName() string {
	return s.firstName
}

func (s *sender) LastName() string {
	return s.lastName
}
