package twofactor

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// OTPChannel represents the delivery channel for OTP codes.
type OTPChannel string

const (
	// ChannelSMS delivers OTP codes via SMS text message.
	ChannelSMS OTPChannel = "sms"

	// ChannelEmail delivers OTP codes via email.
	ChannelEmail OTPChannel = "email"

	// ChannelVoice delivers OTP codes via voice call.
	ChannelVoice OTPChannel = "voice"

	// ChannelPush delivers OTP codes via push notification.
	ChannelPush OTPChannel = "push"
)

// SendRequest encapsulates the parameters for sending an OTP code.
type SendRequest struct {
	// Channel specifies the delivery method (SMS, email, etc.).
	Channel OTPChannel

	// Recipient is the destination address (phone number, email address, etc.).
	Recipient string

	// Code is the OTP code to send.
	Code string

	// LanguageCode is the language for the message template (e.g., "en", "ru", "uz").
	LanguageCode string

	// Metadata contains additional context for the send operation (e.g., user ID, IP address).
	Metadata map[string]string
}

// OTPSender is a channel-agnostic interface for sending OTP codes.
//
// Implementations are responsible for:
//   - Formatting the message according to the channel and language
//   - Interfacing with the delivery provider (Twilio, SendGrid, etc.)
//   - Handling delivery failures and retries
//   - Rate limiting and abuse prevention
type OTPSender interface {
	// Send delivers an OTP code through the configured channel.
	// Returns an error if the send operation fails.
	Send(ctx context.Context, req SendRequest) error
}

// CompositeSender routes OTP send requests to the appropriate sender based on the channel.
//
// This allows the application to support multiple OTP delivery methods (SMS, email, etc.)
// while presenting a unified interface to callers.
type CompositeSender struct {
	mu      sync.RWMutex
	senders map[OTPChannel]OTPSender
}

type testOTPEntry struct {
	code      string
	expiresAt time.Time
}

var testOTPCache = struct {
	mu    sync.RWMutex
	codes map[string]testOTPEntry
}{
	codes: make(map[string]testOTPEntry),
}

// StoreTestOTPCode stores a plaintext OTP for test retrieval.
func StoreTestOTPCode(key, code string, ttl time.Duration) {
	if key == "" || code == "" {
		return
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	testOTPCache.mu.Lock()
	cleanupExpiredLocked(time.Now())
	expiresAt := time.Now().Add(ttl)
	for _, candidateKey := range otpCacheKeys(key) {
		testOTPCache.codes[candidateKey] = testOTPEntry{
			code:      code,
			expiresAt: expiresAt,
		}
	}
	testOTPCache.mu.Unlock()
}

// GetTestOTPCode returns the latest plaintext OTP for a key if not expired.
func GetTestOTPCode(key string) (string, bool) {
	if key == "" {
		return "", false
	}
	now := time.Now()
	for _, candidateKey := range otpCacheKeys(key) {
		testOTPCache.mu.RLock()
		entry, ok := testOTPCache.codes[candidateKey]
		testOTPCache.mu.RUnlock()
		if ok && now.Before(entry.expiresAt) {
			return entry.code, true
		}
		if ok {
			testOTPCache.mu.Lock()
			if current, exists := testOTPCache.codes[candidateKey]; exists && !now.Before(current.expiresAt) {
				delete(testOTPCache.codes, candidateKey)
			}
			testOTPCache.mu.Unlock()
		}
	}
	return "", false
}

func otpCacheKeys(key string) []string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return nil
	}

	keys := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	add := func(v string) {
		if v == "" {
			return
		}
		if _, exists := seen[v]; exists {
			return
		}
		seen[v] = struct{}{}
		keys = append(keys, v)
	}

	add(trimmed)
	if !isPhoneLikeOTPKey(trimmed) {
		return keys
	}

	add(strings.TrimPrefix(trimmed, "+"))

	digitsOnly := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, trimmed)
	add(digitsOnly)
	if digitsOnly != "" {
		add("+" + digitsOnly)
	}

	return keys
}

func cleanupExpiredLocked(now time.Time) {
	for key, entry := range testOTPCache.codes {
		if !now.Before(entry.expiresAt) {
			delete(testOTPCache.codes, key)
		}
	}
}

func isPhoneLikeOTPKey(key string) bool {
	if strings.Contains(key, "@") {
		return false
	}
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '+', '-', '(', ')', '.', ' ':
			continue
		default:
			return false
		}
	}
	return true
}

func sortedOTPTestCacheKeys() []string {
	testOTPCache.mu.RLock()
	defer testOTPCache.mu.RUnlock()

	keys := make([]string, 0, len(testOTPCache.codes))
	for key := range testOTPCache.codes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// NewCompositeSender creates a new CompositeSender with the given channel-to-sender mappings.
func NewCompositeSender(senders map[OTPChannel]OTPSender) *CompositeSender {
	if senders == nil {
		senders = make(map[OTPChannel]OTPSender)
	}
	return &CompositeSender{
		senders: senders,
	}
}

// Register adds or updates a sender for the specified channel.
func (c *CompositeSender) Register(channel OTPChannel, sender OTPSender) {
	if sender == nil {
		panic("Sender cannot be nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.senders[channel] = sender
}

// Send routes the request to the appropriate sender based on the channel.
// Returns ErrChannelUnavailable if no sender is registered for the requested channel.
func (c *CompositeSender) Send(ctx context.Context, req SendRequest) error {
	c.mu.RLock()
	sender, ok := c.senders[req.Channel]
	c.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrChannelUnavailable, req.Channel)
	}
	return sender.Send(ctx, req)
}

// NoopSender is a no-op implementation that logs OTP codes instead of sending them.
//
// WARNING: This implementation provides NO DELIVERY and should ONLY be used for:
//   - Local development and testing
//   - Prototyping and demonstrations
//   - Non-production environments
//
// NEVER use NoopSender in production environments. Always use a proper sender
// implementation (Twilio for SMS, SendGrid for email, etc.) to deliver OTP codes.
//
// In development, OTP codes are logged to stdout for manual testing.
type NoopSender struct{}

// NewNoopSender creates a new NoopSender instance.
func NewNoopSender() *NoopSender {
	return &NoopSender{}
}

// Send logs the OTP code to stdout instead of sending it.
func (n *NoopSender) Send(_ context.Context, req SendRequest) error {
	StoreTestOTPCode(req.Recipient, req.Code, 10*time.Minute)
	if userID, ok := req.Metadata["user_id"]; ok {
		StoreTestOTPCode(userID, req.Code, 10*time.Minute)
	}

	// In development, log the OTP code for manual testing
	// TODO: Replace with proper structured logging in production
	fmt.Fprintf(os.Stderr, "[NoopSender] OTP Code: %s | Channel: %s | Recipient: %s\n", req.Code, req.Channel, req.Recipient)
	return nil
}
