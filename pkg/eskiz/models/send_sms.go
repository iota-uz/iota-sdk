package models

type SendSMSOption func(s *sendSMS)

type SendSMS interface {
	Message() string
	PhoneNumber() string
	From() string
	CallbackUrl() string
}

func SendSmsWithFrom(from string) SendSMSOption {
	return func(s *sendSMS) {
		s.from = from
	}
}

func SendSmsWithCallbackUrl(callbackUrl string) SendSMSOption {
	return func(s *sendSMS) {
		s.callbackUrl = callbackUrl
	}
}

func NewSendSMS(
	phoneNumber string,
	message string,
	opts ...SendSMSOption,
) SendSMS {
	s := &sendSMS{
		message: message,
		phone:   phoneNumber,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

type sendSMS struct {
	message     string
	phone       string
	from        string
	callbackUrl string
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

func (s *sendSMS) CallbackUrl() string {
	return s.callbackUrl
}
