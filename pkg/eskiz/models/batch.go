package models

import (
	"errors"
	"strconv"
	"strings"

	eskizapi "github.com/iota-uz/eskiz"
)

// ErrInvalidBatchPhone is returned when a batch phone isn't a strictly
// digits-only number (Eskiz's batch endpoint encodes the phone as numeric).
var ErrInvalidBatchPhone = errors.New("batch phone must be digits only (no leading +, no spaces)")

type BatchMessage interface {
	// UserSmsID is the caller-supplied correlation id Eskiz echoes back
	// on the per-row delivery webhook. Required.
	UserSmsID() string
	PhoneNumber() string
	Message() string
}

func NewBatchMessage(userSmsID, phone, message string) BatchMessage {
	return &batchMessage{userSmsID: userSmsID, phone: phone, message: message}
}

type batchMessage struct {
	userSmsID string
	phone     string
	message   string
}

func (m *batchMessage) UserSmsID() string   { return m.userSmsID }
func (m *batchMessage) PhoneNumber() string { return m.phone }
func (m *batchMessage) Message() string     { return m.message }

// ToEskizInner rejects phone strings that aren't pure decimals; strconv.ParseFloat
// would otherwise accept "1e6" / "12.5" / signed forms and silently corrupt the
// recipient.
func ToEskizInner(m BatchMessage) (eskizapi.SendSmsBatchRequestMessagesInner, error) {
	userSMSID := m.UserSmsID()
	phoneDigits := strings.TrimPrefix(m.PhoneNumber(), "+")
	if phoneDigits == "" {
		return eskizapi.SendSmsBatchRequestMessagesInner{}, ErrInvalidBatchPhone
	}
	for _, r := range phoneDigits {
		if r < '0' || r > '9' {
			return eskizapi.SendSmsBatchRequestMessagesInner{}, ErrInvalidBatchPhone
		}
	}
	parsed, err := strconv.ParseUint(phoneDigits, 10, 64)
	if err != nil {
		return eskizapi.SendSmsBatchRequestMessagesInner{}, ErrInvalidBatchPhone
	}
	phone := float64(parsed)
	msg := m.Message()
	return eskizapi.SendSmsBatchRequestMessagesInner{
		UserSmsId: &userSMSID,
		To:        &phone,
		Text:      &msg,
	}, nil
}

type SendBatchOptions struct {
	From       string
	DispatchID int64
}

type SendBatchOption func(*SendBatchOptions)

func SendBatchWithFrom(from string) SendBatchOption {
	return func(o *SendBatchOptions) { o.From = from }
}

func SendBatchWithDispatchID(id int64) SendBatchOption {
	return func(o *SendBatchOptions) { o.DispatchID = id }
}

type BatchResult interface {
	DispatchID() string
	SentDispatchID() int64
	Message() string
	Status() []string
}

func NewBatchResult(resp *eskizapi.SendSmsBatchResponse, sentDispatchID int64) BatchResult {
	r := &batchResult{sentDispatchID: sentDispatchID}
	if resp == nil {
		return r
	}
	if resp.Id != nil {
		r.dispatchID = *resp.Id
	}
	if resp.Message != nil {
		r.message = *resp.Message
	}
	r.status = resp.Status
	return r
}

type batchResult struct {
	dispatchID     string
	sentDispatchID int64
	message        string
	status         []string
}

func (r *batchResult) DispatchID() string    { return r.dispatchID }
func (r *batchResult) SentDispatchID() int64 { return r.sentDispatchID }
func (r *batchResult) Message() string       { return r.message }

func (r *batchResult) Status() []string {
	if len(r.status) == 0 {
		return nil
	}
	out := make([]string, len(r.status))
	copy(out, r.status)
	return out
}
