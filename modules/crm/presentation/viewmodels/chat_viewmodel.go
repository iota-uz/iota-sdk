package viewmodels

import "time"

type Chat struct {
	ID        string
	Client    *Client
	Messages  []*Message
	CreatedAt string
}

type Message struct {
	ID        string
	IsUserMsg bool
	Message   string
	CreatedAt time.Time
}

func (m *Message) FormattedDate() string {
	return m.CreatedAt.Format("2006/01/02")
}

func (m *Message) FormattedTime() string {
	return m.CreatedAt.Format("15:04")
}
