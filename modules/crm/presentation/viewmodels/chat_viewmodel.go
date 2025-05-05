package viewmodels

import (
	"strconv"
	"time"
)

type Chat struct {
	ID             string
	Client         *Client
	CreatedAt      string
	Messages       []*Message
	UnreadMessages int
}

func (c *Chat) ReversedMessages() []*Message {
	reversed := make([]*Message, len(c.Messages))
	for i, msg := range c.Messages {
		reversed[len(c.Messages)-1-i] = msg
	}
	return reversed
}

func (c *Chat) LastMessage() *Message {
	if len(c.Messages) == 0 {
		return nil
	}
	return c.Messages[len(c.Messages)-1]
}

func (c *Chat) HasUnreadMessages() bool {
	return c.UnreadMessages > 0
}

func (c *Chat) UnreadMessagesFormatted() string {
	if c.UnreadMessages > 99 {
		return "99+"
	}
	return strconv.Itoa(c.UnreadMessages)
}

type Member struct {
	ID        string
	Transport string
	Sender    MessageSender
	CreatedAt string
}

type MessageSender interface {
	ID() string
	IsUser() bool
	IsClient() bool
	Initials() string
}

func NewUserMessageSender(id, initials string) MessageSender {
	return &messageSender{
		id:       id,
		initials: initials,
		isClient: false,
	}
}

func NewClientMessageSender(id, initials string) MessageSender {
	return &messageSender{
		id:       id,
		initials: initials,
		isClient: true,
	}
}

type messageSender struct {
	id       string
	initials string
	isClient bool
}

func (ms *messageSender) IsUser() bool {
	return !ms.isClient
}

func (ms *messageSender) ID() string {
	return ms.id
}

func (ms *messageSender) IsClient() bool {
	return ms.isClient
}

func (ms *messageSender) Initials() string {
	return ms.initials
}

type Message struct {
	ID        string
	Message   string
	Sender    MessageSender
	CreatedAt time.Time
}

func (m *Message) Date() string {
	return m.CreatedAt.Format("2006/01/02")
}

func (m *Message) Time() string {
	return m.CreatedAt.Format("15:04")
}
