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

// SendBatchOptions captures envelope-level fields. Eskiz applies these to
// every row in the batch (not per-message).
type SendBatchOptions struct {
	From string
}

type SendBatchOption func(*SendBatchOptions)

// SendBatchWithFrom sets the sender id (alpha-name / nickname). Must be
// pre-approved on the Eskiz account.
func SendBatchWithFrom(from string) SendBatchOption {
	return func(o *SendBatchOptions) { o.From = from }
}

type BatchResult interface {
	DispatchID() string
	Message() string
	Status() []string
}

func NewBatchResult(resp *eskizapi.SendSmsBatchResponse) BatchResult {
	if resp == nil {
		return nil
	}
	r := &batchResult{}
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
	dispatchID string
	message    string
	status     []string
}

func (r *batchResult) DispatchID() string { return r.dispatchID }
func (r *batchResult) Message() string    { return r.message }

// Status returns a defensive copy.
func (r *batchResult) Status() []string {
	if len(r.status) == 0 {
		return nil
	}
	out := make([]string, len(r.status))
	copy(out, r.status)
	return out
}
