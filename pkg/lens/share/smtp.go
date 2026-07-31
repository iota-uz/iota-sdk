package share

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	netmail "net/mail"
	"net/smtp"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"
)

type SMTPMailer struct {
	host     string
	port     int
	username string
	password string
	from     string
	timeout  time.Duration
}

func NewSMTPMailer(host string, port int, username, password, from string) (*SMTPMailer, error) {
	host = strings.TrimSpace(host)
	if host == "" || strings.TrimSpace(from) == "" {
		return nil, fmt.Errorf("SMTP host and sender are required")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("SMTP port is invalid")
	}
	from, err := validateMailbox(from)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP sender: %w", err)
	}
	return &SMTPMailer{host: host, port: port, username: username, password: password, from: from, timeout: 30 * time.Second}, nil
}

func (m *SMTPMailer) Send(ctx context.Context, mail Mail) error {
	recipients := make([]string, len(mail.Recipients))
	for index, recipient := range mail.Recipients {
		validated, err := validateMailbox(recipient)
		if err != nil {
			return fmt.Errorf("invalid SMTP recipient: %w", err)
		}
		recipients[index] = validated
	}
	mail.Recipients = recipients
	message, err := buildSMTPMessage(m.from, mail)
	if err != nil {
		return err
	}
	address := net.JoinHostPort(m.host, fmt.Sprintf("%d", m.port))
	connection, err := (&net.Dialer{Timeout: m.timeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("dial SMTP: %w", err)
	}
	deadline := time.Now().Add(m.timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		_ = connection.Close()
		return fmt.Errorf("set SMTP deadline: %w", err)
	}
	client, err := smtp.NewClient(connection, m.host)
	if err != nil {
		_ = connection.Close()
		return fmt.Errorf("open SMTP client: %w", err)
	}
	defer func() { _ = client.Close() }()
	if err := client.StartTLS(&tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("start SMTP TLS: %w", err)
	}
	if m.username != "" {
		if err := client.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil {
			return fmt.Errorf("authenticate SMTP: %w", err)
		}
	}
	if err := client.Mail(m.from); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	for _, recipient := range mail.Recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("set SMTP recipient: %w", err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit SMTP: %w", err)
	}
	return nil
}

func buildSMTPMessage(from string, mail Mail) ([]byte, error) {
	if len(mail.Recipients) == 0 || len(mail.Attachment.Content) == 0 {
		return nil, fmt.Errorf("recipients and workbook attachment are required")
	}
	filename := filepath.Base(strings.ReplaceAll(strings.ReplaceAll(mail.Attachment.Filename, "\r", ""), "\n", ""))
	if filename == "." || filename == "" {
		return nil, fmt.Errorf("workbook filename is required")
	}
	from, err := validateMailbox(from)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP sender: %w", err)
	}
	recipients := make([]string, len(mail.Recipients))
	for index, recipient := range mail.Recipients {
		validated, validateErr := validateMailbox(recipient)
		if validateErr != nil {
			return nil, fmt.Errorf("invalid SMTP recipient: %w", validateErr)
		}
		recipients[index] = validated
	}
	var body bytes.Buffer
	mixed := multipart.NewWriter(&body)
	writeHeader := func(name, value string) { _, _ = fmt.Fprintf(&body, "%s: %s\r\n", name, value) }
	writeHeader("From", from)
	writeHeader("To", strings.Join(recipients, ", "))
	writeHeader("Subject", mime.QEncoding.Encode("UTF-8", strings.ReplaceAll(strings.ReplaceAll(mail.Subject, "\r", ""), "\n", "")))
	writeHeader("MIME-Version", "1.0")
	writeHeader("Content-Type", `multipart/mixed; boundary="`+mixed.Boundary()+`"`)
	_, _ = body.WriteString("\r\n")
	textHeader := make(textproto.MIMEHeader)
	textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	textHeader.Set("Content-Transfer-Encoding", "8bit")
	textPart, err := mixed.CreatePart(textHeader)
	if err != nil {
		return nil, err
	}
	if _, err := textPart.Write([]byte(mail.Body)); err != nil {
		return nil, err
	}
	attachmentHeader := make(textproto.MIMEHeader)
	attachmentHeader.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	attachmentHeader.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	attachmentHeader.Set("Content-Transfer-Encoding", "base64")
	attachment, err := mixed.CreatePart(attachmentHeader)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(mail.Attachment.Content)
	for len(encoded) > 0 {
		lineLength := min(76, len(encoded))
		if _, err := fmt.Fprintf(attachment, "%s\r\n", encoded[:lineLength]); err != nil {
			return nil, err
		}
		encoded = encoded[lineLength:]
	}
	if err := mixed.Close(); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func validateMailbox(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("mailbox is required and cannot contain line breaks")
	}
	parsed, err := netmail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return "", fmt.Errorf("%q is not a plain email address", value)
	}
	return parsed.Address, nil
}

var _ Mailer = (*SMTPMailer)(nil)
