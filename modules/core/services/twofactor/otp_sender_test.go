package twofactor

import (
	"context"
	"testing"

	pkgtf "github.com/iota-uz/iota-sdk/pkg/twofactor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailOTPSender_Send_InvalidChannel(t *testing.T) {
	t.Parallel()

	sender := NewEmailOTPSender("localhost", 1025, "test", "test", "test@example.com")

	req := pkgtf.SendRequest{
		Channel:      pkgtf.ChannelSMS, // Wrong channel
		Recipient:    "user@example.com",
		Code:         "123456",
		LanguageCode: "en",
	}

	err := sender.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only supports email channel")
}

func TestEmailOTPSender_Send_EmptyRecipient(t *testing.T) {
	t.Parallel()

	sender := NewEmailOTPSender("localhost", 1025, "test", "test", "test@example.com")

	req := pkgtf.SendRequest{
		Channel:      pkgtf.ChannelEmail,
		Recipient:    "", // Empty recipient
		Code:         "123456",
		LanguageCode: "en",
	}

	err := sender.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "recipient cannot be empty")
}

func TestEmailOTPSender_Send_EmptyCode(t *testing.T) {
	t.Parallel()

	sender := NewEmailOTPSender("localhost", 1025, "test", "test", "test@example.com")

	req := pkgtf.SendRequest{
		Channel:      pkgtf.ChannelEmail,
		Recipient:    "user@example.com",
		Code:         "", // Empty code
		LanguageCode: "en",
	}

	err := sender.Send(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code cannot be empty")
}

func TestEmailOTPSender_MessageTemplates(t *testing.T) {
	t.Parallel()

	sender := NewEmailOTPSender("localhost", 1025, "test", "test", "test@example.com")

	tests := []struct {
		name         string
		languageCode string
		expectedSubj string
		codeInBody   string
	}{
		{
			name:         "English template",
			languageCode: "en",
			expectedSubj: "Your Verification Code",
			codeInBody:   "123456",
		},
		{
			name:         "Russian template",
			languageCode: "ru",
			expectedSubj: "Ваш код подтверждения",
			codeInBody:   "654321",
		},
		{
			name:         "Uzbek template",
			languageCode: "uz",
			expectedSubj: "Tasdiqlash kodingiz",
			codeInBody:   "999888",
		},
		{
			name:         "Default (unknown language)",
			languageCode: "fr",
			expectedSubj: "Your Verification Code",
			codeInBody:   "111222",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject := sender.getSubject(tt.languageCode)
			assert.Equal(t, tt.expectedSubj, subject)

			body := sender.buildMessage(tt.codeInBody, tt.languageCode)
			assert.Contains(t, body, tt.codeInBody, "Message body should contain the OTP code")
		})
	}
}

func TestSMSOTPSender_Send_InvalidChannel(t *testing.T) {
	t.Parallel()

	sender := NewSMSOTPSender("test_sid", "test_token", "+1234567890")

	req := pkgtf.SendRequest{
		Channel:      pkgtf.ChannelEmail, // Wrong channel
		Recipient:    "+0987654321",
		Code:         "123456",
		LanguageCode: "en",
	}

	err := sender.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only supports SMS channel")
}

func TestSMSOTPSender_Send_EmptyRecipient(t *testing.T) {
	t.Parallel()

	sender := NewSMSOTPSender("test_sid", "test_token", "+1234567890")

	req := pkgtf.SendRequest{
		Channel:      pkgtf.ChannelSMS,
		Recipient:    "", // Empty recipient
		Code:         "123456",
		LanguageCode: "en",
	}

	err := sender.Send(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recipient cannot be empty")
}

func TestSMSOTPSender_Send_EmptyCode(t *testing.T) {
	t.Parallel()

	sender := NewSMSOTPSender("test_sid", "test_token", "+1234567890")

	req := pkgtf.SendRequest{
		Channel:      pkgtf.ChannelSMS,
		Recipient:    "+0987654321",
		Code:         "", // Empty code
		LanguageCode: "en",
	}

	err := sender.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "code cannot be empty")
}

func TestSMSOTPSender_MessageTemplates(t *testing.T) {
	t.Parallel()

	sender := NewSMSOTPSender("test_sid", "test_token", "+1234567890")

	tests := []struct {
		name         string
		languageCode string
		code         string
	}{
		{
			name:         "English template",
			languageCode: "en",
			code:         "123456",
		},
		{
			name:         "Russian template",
			languageCode: "ru",
			code:         "654321",
		},
		{
			name:         "Uzbek template",
			languageCode: "uz",
			code:         "999888",
		},
		{
			name:         "Default (unknown language)",
			languageCode: "fr",
			code:         "111222",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := sender.buildMessage(tt.code, tt.languageCode)
			assert.Contains(t, body, tt.code, "SMS body should contain the OTP code")
		})
	}
}

func TestNewEmailOTPSender(t *testing.T) {
	t.Parallel()

	sender := NewEmailOTPSender("smtp.example.com", 587, "user", "pass", "from@example.com")

	require.NotNil(t, sender)
	assert.Equal(t, "smtp.example.com", sender.host)
	assert.Equal(t, 587, sender.port)
	assert.Equal(t, "user", sender.username)
	assert.Equal(t, "pass", sender.password)
	assert.Equal(t, "from@example.com", sender.from)
}

func TestNewSMSOTPSender(t *testing.T) {
	t.Parallel()

	sender := NewSMSOTPSender("AC123", "token123", "+15551234567")

	require.NotNil(t, sender)
	assert.NotNil(t, sender.client)
	assert.Equal(t, "+15551234567", sender.fromNumber)
}
