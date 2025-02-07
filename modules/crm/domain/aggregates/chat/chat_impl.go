package chat

import (
	"errors"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

var (
	ErrEmptyMessage = errors.New("message is empty")
)

func New(
	clientID uint,
) Chat {
	return &chat{
		id:            0,
		clientID:      clientID,
		lastMessageAt: nil,
		createdAt:     time.Now(),
	}
}

func NewWithID(
	id uint,
	clientID uint,
	createdAt time.Time,
	messages []Message,
	lastMessageAt *time.Time,
) Chat {
	return &chat{
		id:            id,
		clientID:      clientID,
		messages:      messages,
		lastMessageAt: lastMessageAt,
		createdAt:     createdAt,
	}
}

type chat struct {
	id            uint
	clientID      uint
	messages      []Message
	lastMessageAt *time.Time
	createdAt     time.Time
}

func (c *chat) ID() uint {
	return c.id
}

func (c *chat) Messages() []Message {
	return c.messages
}

func (c *chat) ClientID() uint {
	return c.clientID
}

// AddMessage adds a new message to the chat
func (c *chat) AddMessage(content string, sender Sender, attachments ...*upload.Upload) (Message, error) {
	if content == "" && len(attachments) == 0 {
		return nil, ErrEmptyMessage
	}

	msg := WithAttachments(
		content,
		sender,
		attachments...,
	)

	c.messages = append(c.messages, msg)
	c.lastMessageAt = mapping.Pointer(time.Now())

	return msg, nil
}

func (c *chat) LastMessage() (Message, error) {
	if len(c.messages) == 0 {
		return nil, errors.New("no messages")
	}

	return c.messages[len(c.messages)-1], nil
}

func (c *chat) LastMessageAt() *time.Time {
	return c.lastMessageAt
}

func (c *chat) CreatedAt() time.Time {
	return c.createdAt
}

// -------
// Message
// -------

func NewMessage(
	msg string,
	sender Sender,
) Message {
	return &message{
		id:          0,
		message:     msg,
		sender:      sender,
		isRead:      false,
		readAt:      nil,
		attachments: []*upload.Upload{},
		createdAt:   time.Now(),
	}
}

func WithAttachments(
	msg string,
	sender Sender,
	attachments ...*upload.Upload,
) Message {
	return &message{
		id:          0,
		message:     msg,
		sender:      sender,
		isRead:      false,
		readAt:      nil,
		attachments: attachments,
		createdAt:   time.Now(),
	}
}

func NewMessageWithID(
	id uint,
	msg string,
	sender Sender,
	isRead bool,
	attachments []*upload.Upload,
	createdAt time.Time,
) Message {
	return &message{
		id:          id,
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

func (m *message) MarkAsRead() {
	m.isRead = true
	m.readAt = mapping.Pointer(time.Now())
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
