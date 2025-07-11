package models

type SendSMSOption func(s *sendSMS)

type SendSMS interface {
	Message() string
	PhoneNumber() string
	From() string
}

func SendSmsWithFrom(from string) SendSMSOption {
	return func(s *sendSMS) {
		s.from = from
	}
}

func NewSendSMS(
	phoneNumber string,
	message string,
	opts ...SendSMSOption,
) (SendSMS, error) {
	s := &sendSMS{
		message: message,
		phone:   phoneNumber,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

type sendSMS struct {
	message string
	phone   string
	from    string
}

func (s *sendSMS) Message() string {
	return s.message
}

func (s *sendSMS) PhoneNumber() string {
	return s.phone
}

func (s *sendSMS) From() string {
	return s.from
}
