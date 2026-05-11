package models

import (
	"strconv"
	"time"

	eskizapi "github.com/iota-uz/eskiz"
)

type MessageStatus interface {
	MessageID() string
	UserSmsID() string
	DispatchID() int64
	To() string
	Message() string
	Status() string
	SentAt() time.Time
	DeliveredAt() time.Time
	CreatedAt() time.Time
}

type messageStatus struct {
	messageID   string
	userSmsID   string
	dispatchID  int64
	to          string
	message     string
	status      string
	sentAt      time.Time
	deliveredAt time.Time
	createdAt   time.Time
}

func (m *messageStatus) MessageID() string     { return m.messageID }
func (m *messageStatus) UserSmsID() string     { return m.userSmsID }
func (m *messageStatus) DispatchID() int64     { return m.dispatchID }
func (m *messageStatus) To() string            { return m.to }
func (m *messageStatus) Message() string       { return m.message }
func (m *messageStatus) Status() string        { return m.status }
func (m *messageStatus) SentAt() time.Time     { return m.sentAt }
func (m *messageStatus) DeliveredAt() time.Time { return m.deliveredAt }
func (m *messageStatus) CreatedAt() time.Time  { return m.createdAt }

func NewMessageStatus(messageID, userSmsID string, dispatchID int64, to, message, status string, sentAt, deliveredAt, createdAt time.Time) MessageStatus {
	return &messageStatus{
		messageID:   messageID,
		userSmsID:   userSmsID,
		dispatchID:  dispatchID,
		to:          to,
		message:     message,
		status:      status,
		sentAt:      sentAt,
		deliveredAt: deliveredAt,
		createdAt:   createdAt,
	}
}

func MessageStatusesFromResponse(resp *eskizapi.UserMessagesResponse) []MessageStatus {
	if resp == nil || resp.Data == nil {
		return nil
	}
	rows := resp.Data.Result
	out := make([]MessageStatus, 0, len(rows))
	for _, r := range rows {
		s := &messageStatus{}
		if r.Id != nil {
			s.messageID = strconv.Itoa(*r.Id)
		}
		if r.UserSmsId != nil {
			s.userSmsID = *r.UserSmsId
		}
		if r.DispatchId.IsSet() && r.DispatchId.Get() != nil {
			if v, perr := strconv.ParseInt(*r.DispatchId.Get(), 10, 64); perr == nil {
				s.dispatchID = v
			}
		}
		if r.To != nil {
			s.to = *r.To
		}
		if r.Message != nil {
			s.message = *r.Message
		}
		if r.Status != nil {
			s.status = *r.Status
		}
		if r.SentAt != nil {
			s.sentAt = *r.SentAt
		}
		if r.DeliverySmAt != nil {
			s.deliveredAt = *r.DeliverySmAt
		}
		if r.CreatedAt != nil {
			s.createdAt = *r.CreatedAt
		}
		out = append(out, s)
	}
	return out
}
