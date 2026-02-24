package twofactor

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	pkgtf "github.com/iota-uz/iota-sdk/pkg/twofactor"
	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

// EmailOTPSender sends OTP codes via email using SMTP with STARTTLS.
type EmailOTPSender struct {
	host     string
	port     int
	username string
	password string
	from     string
}

// NewEmailOTPSender creates an EmailOTPSender with the given SMTP settings.
func NewEmailOTPSender(host string, port int, username, password, from string) *EmailOTPSender {
	return &EmailOTPSender{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// Send delivers an OTP code via email using SMTP with STARTTLS.
func (e *EmailOTPSender) Send(ctx context.Context, req pkgtf.SendRequest) error {
	const op serrors.Op = "EmailOTPSender.Send"

	if req.Channel != pkgtf.ChannelEmail {
		return serrors.E(op, serrors.Invalid, fmt.Errorf("EmailOTPSender only supports email channel, got %s", req.Channel))
	}
	if req.Recipient == "" {
		return serrors.E(op, serrors.Invalid, errors.New("email recipient cannot be empty"))
	}
	if req.Code == "" {
		return serrors.E(op, serrors.Invalid, errors.New("OTP code cannot be empty"))
	}

	subject := e.getSubject(req.LanguageCode)
	body := e.buildMessage(req.Code, req.LanguageCode)
	auth := smtp.PlainAuth("", e.username, e.password, e.host)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		e.from, req.Recipient, subject, body)
	addr := fmt.Sprintf("%s:%d", e.host, e.port)
	err := e.sendWithTLS(ctx, addr, auth, e.from, []string{req.Recipient}, []byte(msg))
	if err != nil {
		return serrors.E(op, err)
	}

	logger := composables.UseLogger(ctx)
	logger.Info("OTP sent via email")

	return nil
}

// sendWithTLS sends email using SMTP with STARTTLS (must run before Auth so credentials are encrypted).
func (e *EmailOTPSender) sendWithTLS(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	const op serrors.Op = "EmailOTPSender.sendWithTLS"

	d := net.Dialer{Timeout: 30 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return serrors.E(op, err)
	}

	client, err := smtp.NewClient(conn, e.host)
	if err != nil {
		_ = conn.Close()
		return serrors.E(op, err)
	}
	defer func() {
		if quitErr := client.Quit(); quitErr != nil {
			composables.UseLogger(ctx).WithField("op", op).Warn("SMTP Quit error: " + quitErr.Error())
		}
	}()

	tlsConfig := &tls.Config{
		ServerName: e.host,
		MinVersion: tls.VersionTLS12,
	}
	if err = client.StartTLS(tlsConfig); err != nil {
		return serrors.E(op, err)
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return serrors.E(op, err)
		}
	}
	if err = client.Mail(from); err != nil {
		return serrors.E(op, err)
	}
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return serrors.E(op, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return serrors.E(op, err)
	}
	_, err = w.Write(msg)
	if err != nil {
		return serrors.E(op, err)
	}
	err = w.Close()
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

func (e *EmailOTPSender) getSubject(lang string) string {
	switch lang {
	case "ru":
		return "Ваш код подтверждения"
	case "uz":
		return "Tasdiqlash kodingiz"
	default:
		return "Your Verification Code"
	}
}

func (e *EmailOTPSender) buildMessage(code, lang string) string {
	switch lang {
	case "ru":
		return fmt.Sprintf("Ваш код подтверждения: %s\n\nЭтот код действителен в течение 10 минут.\n\nЕсли вы не запрашивали этот код, проигнорируйте это сообщение.", code)
	case "uz":
		return fmt.Sprintf("Tasdiqlash kodingiz: %s\n\nBu kod 10 daqiqa davomida amal qiladi.\n\nAgar bu kodni so'ramagan bo'lsangiz, bu xabarni e'tiborsiz qoldiring.", code)
	default:
		return fmt.Sprintf("Your verification code is: %s\n\nThis code is valid for 10 minutes.\n\nIf you did not request this code, please ignore this message.", code)
	}
}

// SMSOTPSender sends OTP codes via SMS using Twilio's REST API.
type SMSOTPSender struct {
	client     *twilio.RestClient
	fromNumber string
}

// NewSMSOTPSender creates an SMSOTPSender with the given Twilio credentials.
func NewSMSOTPSender(accountSID, authToken, fromNumber string) *SMSOTPSender {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: accountSID,
		Password: authToken,
	})

	return &SMSOTPSender{
		client:     client,
		fromNumber: fromNumber,
	}
}

// Send delivers an OTP code via SMS using Twilio.
func (s *SMSOTPSender) Send(ctx context.Context, req pkgtf.SendRequest) error {
	const op serrors.Op = "SMSOTPSender.Send"

	if req.Channel != pkgtf.ChannelSMS {
		return serrors.E(op, serrors.Invalid, fmt.Errorf("SMSOTPSender only supports SMS channel, got %s", req.Channel))
	}
	if req.Recipient == "" {
		return serrors.E(op, serrors.Invalid, errors.New("SMS recipient cannot be empty"))
	}
	if req.Code == "" {
		return serrors.E(op, serrors.Invalid, errors.New("OTP code cannot be empty"))
	}

	body := s.buildMessage(req.Code, req.LanguageCode)
	params := &twilioApi.CreateMessageParams{}
	params.SetTo(req.Recipient)
	params.SetFrom(s.fromNumber)
	params.SetBody(body)

	resp, err := s.client.Api.CreateMessage(params)
	if err != nil {
		return serrors.E(op, err)
	}
	if resp == nil || resp.Sid == nil {
		return serrors.E(op, serrors.Invalid, errors.New("invalid Twilio response"))
	}

	logger := composables.UseLogger(ctx)
	logger.WithField("twilio_sid", *resp.Sid).Info("OTP sent via SMS")

	return nil
}

func (s *SMSOTPSender) buildMessage(code, lang string) string {
	switch lang {
	case "ru":
		return fmt.Sprintf("Ваш код подтверждения: %s. Действителен 10 минут.", code)
	case "uz":
		return fmt.Sprintf("Tasdiqlash kodingiz: %s. 10 daqiqa amal qiladi.", code)
	default:
		return fmt.Sprintf("Your verification code: %s. Valid for 10 minutes.", code)
	}
}
