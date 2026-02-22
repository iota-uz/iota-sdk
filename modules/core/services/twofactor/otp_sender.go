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

// EmailOTPSender sends OTP codes via email using SMTP.
// Implements the OTPSender interface for email delivery with TLS encryption.
// Uses STARTTLS for secure SMTP communication and supports multi-language message templates.
type EmailOTPSender struct {
	host     string
	port     int
	username string
	password string
	from     string
}

// NewEmailOTPSender creates a new EmailOTPSender with SMTP configuration.
// Configures the SMTP client for sending OTP codes via email with TLS encryption.
// Parameters:
//   - host: SMTP server hostname (e.g., "smtp.gmail.com")
//   - port: SMTP server port (typically 587 for TLS)
//   - username: SMTP authentication username
//   - password: SMTP authentication password
//   - from: Sender email address
//
// Returns a configured EmailOTPSender instance.
func NewEmailOTPSender(host string, port int, username, password, from string) *EmailOTPSender {
	return &EmailOTPSender{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// Send delivers an OTP code via email using SMTP with TLS.
// Validates the request, formats the email message with localized template,
// and sends it using STARTTLS encryption for secure credential transmission.
// Parameters:
//   - ctx: Request context for cancellation
//   - req: Send request with channel, recipient, code, and language
//
// Returns an error if validation or sending fails.
func (e *EmailOTPSender) Send(ctx context.Context, req pkgtf.SendRequest) error {
	const op serrors.Op = "EmailOTPSender.Send"

	// Validate channel
	if req.Channel != pkgtf.ChannelEmail {
		return serrors.E(op, serrors.Invalid, fmt.Errorf("EmailOTPSender only supports email channel, got %s", req.Channel))
	}

	// Validate recipient
	if req.Recipient == "" {
		return serrors.E(op, serrors.Invalid, errors.New("email recipient cannot be empty"))
	}

	// Validate code
	if req.Code == "" {
		return serrors.E(op, serrors.Invalid, errors.New("OTP code cannot be empty"))
	}

	// Build email message with template
	subject := e.getSubject(req.LanguageCode)
	body := e.buildMessage(req.Code, req.LanguageCode)

	// Setup SMTP authentication
	auth := smtp.PlainAuth("", e.username, e.password, e.host)

	// Build email in RFC 822 format
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		e.from, req.Recipient, subject, body)

	// Send email with TLS
	addr := fmt.Sprintf("%s:%d", e.host, e.port)
	err := e.sendWithTLS(ctx, addr, auth, e.from, []string{req.Recipient}, []byte(msg))
	if err != nil {
		return serrors.E(op, err)
	}

	logger := composables.UseLogger(ctx)
	logger.Info("OTP sent via email")

	return nil
}

// sendWithTLS sends email using SMTP with explicit STARTTLS enforcement
func (e *EmailOTPSender) sendWithTLS(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	const op serrors.Op = "EmailOTPSender.sendWithTLS"

	d := net.Dialer{Timeout: 30 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return serrors.E(op, err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			// Connection close errors are logged but not returned to avoid masking the main error
			_ = closeErr
		}
	}()

	// Create SMTP client
	client, err := smtp.NewClient(conn, e.host)
	if err != nil {
		return serrors.E(op, err)
	}
	defer func() {
		if quitErr := client.Quit(); quitErr != nil {
			// SMTP quit errors are logged but not returned to avoid masking the main error
			_ = quitErr
		}
	}()

	// Start TLS encryption (CRITICAL: enforces encrypted connection)
	tlsConfig := &tls.Config{
		ServerName: e.host,
		MinVersion: tls.VersionTLS12,
	}
	if err = client.StartTLS(tlsConfig); err != nil {
		return serrors.E(op, err)
	}

	// Authenticate (credentials now sent over encrypted connection)
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return serrors.E(op, err)
		}
	}

	// Set sender
	if err = client.Mail(from); err != nil {
		return serrors.E(op, err)
	}

	// Set recipients
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return serrors.E(op, err)
		}
	}

	// Send message body
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

// getSubject returns the email subject based on language
func (e *EmailOTPSender) getSubject(lang string) string {
	// TODO: Integrate with i18n localizer for proper translation support
	switch lang {
	case "ru":
		return "Ваш код подтверждения"
	case "uz":
		return "Tasdiqlash kodingiz"
	default:
		return "Your Verification Code"
	}
}

// buildMessage creates the email body with the OTP code
func (e *EmailOTPSender) buildMessage(code, lang string) string {
	// TODO: Integrate with i18n localizer for proper translation support
	// TODO: Use HTML templates for better formatting
	switch lang {
	case "ru":
		return fmt.Sprintf("Ваш код подтверждения: %s\n\nЭтот код действителен в течение 10 минут.\n\nЕсли вы не запрашивали этот код, проигнорируйте это сообщение.", code)
	case "uz":
		return fmt.Sprintf("Tasdiqlash kodingiz: %s\n\nBu kod 10 daqiqa davomida amal qiladi.\n\nAgar bu kodni so'ramagan bo'lsangiz, bu xabarni e'tiborsiz qoldiring.", code)
	default:
		return fmt.Sprintf("Your verification code is: %s\n\nThis code is valid for 10 minutes.\n\nIf you did not request this code, please ignore this message.", code)
	}
}

// SMSOTPSender sends OTP codes via SMS using Twilio.
// Implements the OTPSender interface for SMS delivery via Twilio's REST API.
// Supports multi-language message templates for international users.
type SMSOTPSender struct {
	client     *twilio.RestClient
	fromNumber string
}

// NewSMSOTPSender creates a new SMSOTPSender with Twilio credentials.
// Configures the Twilio REST client for sending OTP codes via SMS.
// Parameters:
//   - accountSID: Twilio account SID from dashboard
//   - authToken: Twilio authentication token from dashboard
//   - fromNumber: Twilio phone number to send from (E.164 format, e.g., "+15551234567")
//
// Returns a configured SMSOTPSender instance.
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
// Validates the request, formats the SMS message with localized template,
// and sends it via Twilio's REST API.
// Parameters:
//   - ctx: Request context for cancellation
//   - req: Send request with channel, recipient, code, and language
//
// Returns an error if validation or sending fails.
func (s *SMSOTPSender) Send(ctx context.Context, req pkgtf.SendRequest) error {
	const op serrors.Op = "SMSOTPSender.Send"

	// Validate channel
	if req.Channel != pkgtf.ChannelSMS {
		return serrors.E(op, serrors.Invalid, fmt.Errorf("SMSOTPSender only supports SMS channel, got %s", req.Channel))
	}

	// Validate recipient
	if req.Recipient == "" {
		return serrors.E(op, serrors.Invalid, errors.New("SMS recipient cannot be empty"))
	}

	// Validate code
	if req.Code == "" {
		return serrors.E(op, serrors.Invalid, errors.New("OTP code cannot be empty"))
	}

	// Build SMS message with template
	body := s.buildMessage(req.Code, req.LanguageCode)

	// Send SMS via Twilio
	params := &twilioApi.CreateMessageParams{}
	params.SetTo(req.Recipient)
	params.SetFrom(s.fromNumber)
	params.SetBody(body)

	resp, err := s.client.Api.CreateMessage(params)
	if err != nil {
		return serrors.E(op, err)
	}

	// Validate response
	if resp == nil || resp.Sid == nil {
		return serrors.E(op, serrors.Invalid, errors.New("invalid Twilio response"))
	}

	logger := composables.UseLogger(ctx)
	logger.Info("OTP sent via SMS", "twilio_sid", *resp.Sid)

	return nil
}

// buildMessage creates the SMS body with the OTP code
func (s *SMSOTPSender) buildMessage(code, lang string) string {
	// TODO: Integrate with i18n localizer for proper translation support
	switch lang {
	case "ru":
		return fmt.Sprintf("Ваш код подтверждения: %s. Действителен 10 минут.", code)
	case "uz":
		return fmt.Sprintf("Tasdiqlash kodingiz: %s. 10 daqiqa amal qiladi.", code)
	default:
		return fmt.Sprintf("Your verification code: %s. Valid for 10 minutes.", code)
	}
}
