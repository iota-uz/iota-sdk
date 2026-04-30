package models

import (
	"time"

	eskizapi "github.com/iota-uz/eskiz"
)

// SMSStatus describes the delivery state of a single outgoing SMS as reported
// by Eskiz's /message/sms/status-by-id endpoint.
//
// This type reflects the snapshot at fetch time. For real-time updates,
// consumers should register a callback URL in the Service config — Eskiz will
// POST the same status fields back via webhook.
type SMSStatus interface {
	// ID is the Eskiz-assigned message id (matches SendSMSResult.ID()).
	ID() string
	// Status is the coarse lifecycle: "waiting" | "transmitting" |
	// "delivered" | "notdelivered" | "failed" | "expired" | ...
	// Exact enumeration comes from Eskiz — consumers map it to their own
	// delivery state machine.
	Status() string
	// To is the recipient phone number as Eskiz saw it.
	To() string
	// CreatedAt is when Eskiz accepted the request.
	CreatedAt() time.Time
	// DeliveredAt is set only when Status transitions to a terminal
	// delivered state. Zero time otherwise.
	DeliveredAt() time.Time
	// Parts is the number of SMS segments billed.
	Parts() int
	// TotalPrice is the final billing amount in account units.
	TotalPrice() float64
}

// NewSMSStatus wraps an Eskiz SmsStatusResponse. Returns nil if the response
// lacks a Data payload (never observed in practice, but Eskiz occasionally
// returns {status: "error"} with no body on invalid ids).
func NewSMSStatus(resp *eskizapi.SmsStatusResponse) SMSStatus {
	if resp == nil || resp.Data == nil {
		return nil
	}
	d := resp.Data
	s := &smsStatus{}
	if d.Id != nil {
		s.id = intToString(*d.Id)
	}
	if resp.Status != nil {
		s.status = *resp.Status
	}
	if d.To != nil {
		s.to = *d.To
	}
	if d.CreatedAt != nil {
		s.createdAt = *d.CreatedAt
	}
	// Eskiz doesn't expose an explicit DeliveredAt field. When Status is a
	// terminal delivered state, UpdatedAt is the delivery transition time.
	if d.UpdatedAt != nil && s.isTerminal() {
		s.deliveredAt = *d.UpdatedAt
	}
	if d.PartsCount != nil {
		s.parts = *d.PartsCount
	}
	if d.TotalPrice != nil {
		s.totalPrice = *d.TotalPrice
	}
	return s
}

type smsStatus struct {
	id          string
	status      string
	to          string
	createdAt   time.Time
	deliveredAt time.Time
	parts       int
	totalPrice  float64
}

func (s *smsStatus) ID() string           { return s.id }
func (s *smsStatus) Status() string       { return s.status }
func (s *smsStatus) To() string           { return s.to }
func (s *smsStatus) CreatedAt() time.Time { return s.createdAt }
func (s *smsStatus) DeliveredAt() time.Time {
	return s.deliveredAt
}
func (s *smsStatus) Parts() int          { return s.parts }
func (s *smsStatus) TotalPrice() float64 { return s.totalPrice }

// isTerminal reports whether Status is a final delivery state.
// Conservative — non-terminal statuses keep DeliveredAt as zero time.
func (s *smsStatus) isTerminal() bool {
	switch s.status {
	case "delivered", "notdelivered", "failed", "expired":
		return true
	}
	return false
}

// intToString converts an int to decimal string without importing strconv at
// top level (localized to avoid conflicts).
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
