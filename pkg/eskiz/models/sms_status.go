package models

import (
	"strconv"
	"time"

	eskizapi "github.com/iota-uz/eskiz"
)

// SMSStatus is the delivery snapshot returned by /message/sms/status-by-id.
// For real-time updates, register a callback URL in the Service config and
// Eskiz will POST the same fields via webhook.
type SMSStatus interface {
	ID() string
	// Status is the coarse lifecycle: "waiting" | "transmitting" |
	// "delivered" | "notdelivered" | "failed" | "expired" | …
	Status() string
	To() string
	CreatedAt() time.Time
	// DeliveredAt is the terminal-state transition time; zero otherwise.
	DeliveredAt() time.Time
	Parts() int
	TotalPrice() float64
}

// NewSMSStatus returns nil if the response has no Data payload — Eskiz
// occasionally answers {status:"error"} with no body for invalid ids.
func NewSMSStatus(resp *eskizapi.SmsStatusResponse) SMSStatus {
	if resp == nil || resp.Data == nil {
		return nil
	}
	d := resp.Data
	s := &smsStatus{}
	if d.Id != nil {
		s.id = strconv.Itoa(*d.Id)
	}
	// Per-message delivery state ("waiting", "transmitting", "delivered",
	// "notdelivered", "failed", "expired") lives on Data.Status. The envelope
	// resp.Status is just the API call result ("success" | "error") — using
	// it here would keep isTerminal() false forever and DeliveredAt zero.
	if d.Status != nil {
		s.status = *d.Status
	}
	if d.To != nil {
		s.to = *d.To
	}
	if d.CreatedAt != nil {
		s.createdAt = *d.CreatedAt
	}
	// Eskiz has no explicit DeliveredAt — UpdatedAt holds the
	// transition time once Status enters a terminal state.
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

func (s *smsStatus) isTerminal() bool {
	switch s.status {
	case "delivered", "notdelivered", "failed", "expired":
		return true
	}
	return false
}
