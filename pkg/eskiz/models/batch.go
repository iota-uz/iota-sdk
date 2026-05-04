package models

import (
	"errors"
	"strconv"
	"strings"

	eskizapi "github.com/iota-uz/eskiz"
)

// ErrInvalidBatchPhone is returned by ToEskizInner when a batch message
// phone cannot be parsed as a digits-only number. Eskiz's batch API uses a
// numeric phone field (no leading "+"), so callers must pass normalised input.
var ErrInvalidBatchPhone = errors.New("batch phone must be digits only (no leading +, no spaces)")

// BatchMessage is a single entry in a SendBatch request.
type BatchMessage interface {
	// UserSmsID is the caller-supplied id that Eskiz echoes back per row;
	// callers use it to correlate results with their own item ids. Required.
	UserSmsID() string
	PhoneNumber() string
	Message() string
}

// NewBatchMessage constructs a BatchMessage.
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

// ToEskizInner converts a domain BatchMessage into the generated client's row
// type. The Eskiz batch API encodes phone as numeric — a leading "+" is
// stripped and the rest must be digits. Returns ErrInvalidBatchPhone if the
// remainder isn't strictly decimal (rejecting scientific/decimal/sign forms
// that strconv.ParseFloat would otherwise accept).
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

// SendBatchOptions is the per-call envelope mutated by SendBatchOption.
// Fields map 1:1 to top-level keys on Eskiz's send-batch request body.
type SendBatchOptions struct {
	From string
}

// SendBatchOption configures a SendBatch invocation. Eskiz applies these at
// the batch envelope level (not per-row), so they affect every message in
// the call.
type SendBatchOption func(*SendBatchOptions)

// SendBatchWithFrom sets the sender id (alpha-name / nickname) used for
// every row in the batch. Must be one of the nicknames approved on the
// Eskiz account; unknown sender ids cause Eskiz to reject the dispatch.
func SendBatchWithFrom(from string) SendBatchOption {
	return func(o *SendBatchOptions) { o.From = from }
}

// BatchResult is the Service-level outcome of a SendBatch call. Eskiz returns
// a dispatch id that groups the whole batch — per-row delivery status comes
// later via webhook or GetSMSStatus.
type BatchResult interface {
	// DispatchID groups the batch on Eskiz side. Use with GetSmsLogs or
	// GetDispatchStatus to observe per-row results.
	DispatchID() string
	// Message is Eskiz's batch-level acknowledgement (usually "Waiting for
	// SMS provider" or "Success").
	Message() string
	// Status is the batch-level status slice — Eskiz returns coarse hints
	// like ["waiting"]. Terminal states come via per-row webhook.
	Status() []string
}

// NewBatchResult wraps an Eskiz SendSmsBatchResponse. Returns nil for nil resp.
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

// Status returns a defensive copy so callers can't mutate the cached
// internal slice and surprise other consumers reading the same instance.
func (r *batchResult) Status() []string {
	if len(r.status) == 0 {
		return nil
	}
	out := make([]string, len(r.status))
	copy(out, r.status)
	return out
}
