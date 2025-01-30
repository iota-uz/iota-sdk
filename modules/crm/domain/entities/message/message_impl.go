package message

import "time"

func NewMessage(
	chatID uint,
	msg string,
	sender Sender,
) Message {
	return &message{
		id:        0,
		chatID:    chatID,
		message:   msg,
		sender:    sender,
		isActive:  true,
		createdAt: time.Now(),
	}
}

func NewMessageWithID(
	id uint,
	chatID uint,
	msg string,
	sender Sender,
	isActive bool,
	createdAt time.Time,
) Message {
	return &message{
		id:        id,
		chatID:    chatID,
		message:   msg,
		sender:    sender,
		isActive:  isActive,
		createdAt: createdAt,
	}
}

type message struct {
	id        uint
	chatID    uint
	message   string
	sender    Sender
	isActive  bool
	createdAt time.Time
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

func (m *message) IsActive() bool {
	return m.isActive
}

func (m *message) CreatedAt() time.Time {
	return m.createdAt
}

func NewUserSender(id uint) Sender {
	return &sender{
		id:       id,
		isClient: false,
	}
}

func NewClientSender(id uint) Sender {
	return &sender{
		id:       id,
		isClient: true,
	}
}

type sender struct {
	id       uint
	isClient bool
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
