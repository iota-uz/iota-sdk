package twofactor_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/twofactor"
)

// TestNoopEncryptor verifies the NoopEncryptor implementation.
func TestNoopEncryptor(t *testing.T) {
	ctx := context.Background()
	encryptor := twofactor.NewNoopEncryptor()

	secret := "JBSWY3DPEHPK3PXP"

	// Test encryption (should return plaintext)
	encrypted, err := encryptor.Encrypt(ctx, secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if encrypted != secret {
		t.Errorf("Expected encrypted to equal plaintext, got %s", encrypted)
	}

	// Test decryption (should return plaintext)
	decrypted, err := encryptor.Decrypt(ctx, encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != secret {
		t.Errorf("Expected decrypted to equal plaintext, got %s", decrypted)
	}
}

// TestCompositeSender verifies the CompositeSender routing logic.
func TestCompositeSender(t *testing.T) {
	ctx := context.Background()

	// Create mock senders
	smsSender := &mockSender{channel: twofactor.ChannelSMS}
	emailSender := &mockSender{channel: twofactor.ChannelEmail}

	// Create composite sender
	sender := twofactor.NewCompositeSender(map[twofactor.OTPChannel]twofactor.OTPSender{
		twofactor.ChannelSMS:   smsSender,
		twofactor.ChannelEmail: emailSender,
	})

	// Test SMS routing
	err := sender.Send(ctx, twofactor.SendRequest{
		Channel:   twofactor.ChannelSMS,
		Recipient: "+1234567890",
		Code:      "123456",
	})
	if err != nil {
		t.Fatalf("SMS send failed: %v", err)
	}
	if !smsSender.called {
		t.Error("Expected SMS sender to be called")
	}

	// Test email routing
	err = sender.Send(ctx, twofactor.SendRequest{
		Channel:   twofactor.ChannelEmail,
		Recipient: "user@example.com",
		Code:      "654321",
	})
	if err != nil {
		t.Fatalf("Email send failed: %v", err)
	}
	if !emailSender.called {
		t.Error("Expected email sender to be called")
	}

	// Test unavailable channel
	err = sender.Send(ctx, twofactor.SendRequest{
		Channel:   twofactor.ChannelVoice,
		Recipient: "+1234567890",
		Code:      "123456",
	})
	if err == nil {
		t.Error("Expected error for unavailable channel")
	}
}

// TestCompositeSenderRegister verifies dynamic sender registration.
func TestCompositeSenderRegister(t *testing.T) {
	ctx := context.Background()

	sender := twofactor.NewCompositeSender(nil)

	// Initially, no senders are registered
	err := sender.Send(ctx, twofactor.SendRequest{
		Channel: twofactor.ChannelSMS,
		Code:    "123456",
	})
	if err == nil {
		t.Error("Expected error when no sender is registered")
	}

	// Register a sender
	smsSender := &mockSender{channel: twofactor.ChannelSMS}
	sender.Register(twofactor.ChannelSMS, smsSender)

	// Now the send should succeed
	err = sender.Send(ctx, twofactor.SendRequest{
		Channel: twofactor.ChannelSMS,
		Code:    "123456",
	})
	if err != nil {
		t.Errorf("Send failed after registration: %v", err)
	}
	if !smsSender.called {
		t.Error("Expected SMS sender to be called")
	}
}

// TestAuthAttemptTypes verifies AuthAttempt struct construction.
func TestAuthAttemptTypes(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	attempt := twofactor.AuthAttempt{
		UserID:            userID,
		Method:            twofactor.AuthMethodPassword,
		IPAddress:         "192.168.1.1",
		UserAgent:         "Mozilla/5.0",
		Timestamp:         time.Now(),
		SessionID:         &sessionID,
		DeviceFingerprint: "abc123",
	}

	if attempt.UserID != userID {
		t.Error("UserID mismatch")
	}
	if attempt.Method != twofactor.AuthMethodPassword {
		t.Error("Method mismatch")
	}
	if attempt.SessionID == nil || *attempt.SessionID != sessionID {
		t.Error("SessionID mismatch")
	}
}

// TestErrorTypes verifies all error types are defined.
func TestErrorTypes(t *testing.T) {
	errors := []error{
		twofactor.ErrInvalidCode,
		twofactor.ErrExpiredCode,
		twofactor.ErrTooManyAttempts,
		twofactor.ErrInvalidSecret,
		twofactor.ErrChannelUnavailable,
		twofactor.ErrSendFailed,
		twofactor.ErrMethodNotSupported,
		twofactor.ErrEncryptionFailed,
		twofactor.ErrDecryptionFailed,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Expected error to be defined")
		}
		if err.Error() == "" {
			t.Error("Expected error to have a message")
		}
	}
}

// mockSender is a test implementation of OTPSender.
type mockSender struct {
	channel twofactor.OTPChannel
	called  bool
}

func (m *mockSender) Send(ctx context.Context, req twofactor.SendRequest) error {
	m.called = true
	return nil
}

// mockPolicy is a test implementation of TwoFactorPolicy.
type mockPolicy struct {
	requireFunc func(context.Context, twofactor.AuthAttempt) (bool, error)
}

func (m *mockPolicy) Requires(ctx context.Context, attempt twofactor.AuthAttempt) (bool, error) {
	if m.requireFunc != nil {
		return m.requireFunc(ctx, attempt)
	}
	return false, nil
}

// TestPolicyInterface verifies the TwoFactorPolicy interface.
func TestPolicyInterface(t *testing.T) {
	ctx := context.Background()

	// Create a policy that always requires 2FA
	policy := &mockPolicy{
		requireFunc: func(ctx context.Context, attempt twofactor.AuthAttempt) (bool, error) {
			return true, nil
		},
	}

	attempt := twofactor.AuthAttempt{
		UserID:    uuid.New(),
		Method:    twofactor.AuthMethodPassword,
		IPAddress: "192.168.1.1",
		Timestamp: time.Now(),
	}

	required, err := policy.Requires(ctx, attempt)
	if err != nil {
		t.Fatalf("Policy.Requires failed: %v", err)
	}
	if !required {
		t.Error("Expected policy to require 2FA")
	}
}
