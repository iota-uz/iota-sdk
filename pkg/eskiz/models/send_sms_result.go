// Package models provides this package.
package models

import eskizapi "github.com/iota-uz/eskiz"

type SendSMSResult interface {
	ID() string
	Message() string
	Status() string
}

func NewSendSMSResult(resp *eskizapi.SendSmsResponse) SendSMSResult {
	return &sendSMSResult{
		id:      resp.GetId(),
		message: resp.GetMessage(),
		status:  resp.GetStatus(),
	}
}

type sendSMSResult struct {
	id      string
	message string
	status  string
}

func (s *sendSMSResult) ID() string {
	return s.id
}

func (s *sendSMSResult) Message() string {
	return s.message
}

func (s *sendSMSResult) Status() string {
	return s.status
}
