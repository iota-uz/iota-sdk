package twofactor

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	pkgtf "github.com/iota-uz/iota-sdk/pkg/twofactor"
	"golang.org/x/crypto/bcrypt"
)

const (
	// defaultOTPLength is the default length of OTP codes
	defaultOTPLength = 6
	// defaultOTPExpiry is the default expiration time for OTP codes
	defaultOTPExpiry = 10 * time.Minute
	// defaultOTPMaxAttempts is the default maximum number of verification attempts
	defaultOTPMaxAttempts = 3
)

// OTPService handles OTP (One-Time Password) operations (internal helper)
type OTPService struct {
	repository   twofactor.OTPRepository
	sender       pkgtf.OTPSender
	length       int
	expiry       time.Duration
	maxAttempts  int
}

// NewOTPService creates a new OTPService
func NewOTPService(
	repo twofactor.OTPRepository,
	sender pkgtf.OTPSender,
	length int,
	expiry time.Duration,
	maxAttempts int,
) *OTPService {
	if length <= 0 {
		length = defaultOTPLength
	}
	if expiry <= 0 {
		expiry = defaultOTPExpiry
	}
	if maxAttempts <= 0 {
		maxAttempts = defaultOTPMaxAttempts
	}

	return &OTPService{
		repository:  repo,
		sender:      sender,
		length:      length,
		expiry:      expiry,
		maxAttempts: maxAttempts,
	}
}

// generateCode generates a random numeric OTP code
func (s *OTPService) generateCode() (string, error) {
	// Calculate max value (e.g., for 6 digits: 999999)
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(s.length)), nil)
	max.Sub(max, big.NewInt(1))

	// Generate random number
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("failed to generate random number: %w", err)
	}

	// Format with leading zeros
	format := fmt.Sprintf("%%0%dd", s.length)
	return fmt.Sprintf(format, n), nil
}

// hashCode hashes an OTP code using bcrypt
func (s *OTPService) hashCode(code string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(code), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash code: %w", err)
	}
	return string(hashed), nil
}

// Generate generates and sends an OTP code
func (s *OTPService) Generate(ctx context.Context, userID uint, channel pkgtf.OTPChannel, destination string) (string, time.Time, error) {
	if destination == "" {
		return "", time.Time{}, errors.New("destination cannot be empty")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to get tenant ID: %w", err)
	}

	// Generate code
	code, err := s.generateCode()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate code: %w", err)
	}

	// Hash code
	codeHash, err := s.hashCode(code)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to hash code: %w", err)
	}

	// Calculate expiration
	expiresAt := time.Now().Add(s.expiry)

	// Create OTP entity
	otp := twofactor.NewOTP(
		destination,
		codeHash,
		channel,
		expiresAt,
		tenantID,
		userID,
	)

	// Store in repository
	if err := s.repository.Create(ctx, otp); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to store OTP: %w", err)
	}

	// Send code via configured sender
	if s.sender != nil {
		sendReq := pkgtf.SendRequest{
			Channel:   channel,
			Recipient: destination,
			Code:      code,
			Metadata: map[string]string{
				"user_id": fmt.Sprintf("%d", userID),
			},
		}

		if err := s.sender.Send(ctx, sendReq); err != nil {
			// Log error but don't fail the operation
			// The code was already stored and can be used for verification
			return "", time.Time{}, fmt.Errorf("%w: %w", pkgtf.ErrSendFailed, err)
		}
	}

	return code, expiresAt, nil
}

// Validate validates an OTP code
func (s *OTPService) Validate(ctx context.Context, destination, code string) error {
	if destination == "" {
		return errors.New("destination cannot be empty")
	}
	if code == "" {
		return pkgtf.ErrInvalidCode
	}

	// Find OTP by identifier
	otp, err := s.repository.FindByIdentifier(ctx, destination)
	if err != nil {
		return pkgtf.ErrInvalidCode
	}

	// Check if expired
	if otp.IsExpired() {
		return pkgtf.ErrExpiredCode
	}

	// Check if already used
	if otp.IsUsed() {
		return pkgtf.ErrInvalidCode
	}

	// Check attempts
	if otp.Attempts() >= s.maxAttempts {
		return pkgtf.ErrTooManyAttempts
	}

	// Verify code
	if err := bcrypt.CompareHashAndPassword([]byte(otp.CodeHash()), []byte(code)); err != nil {
		// Increment attempts
		if err := s.repository.IncrementAttempts(ctx, otp.ID()); err != nil {
			return fmt.Errorf("failed to increment attempts: %w", err)
		}
		return pkgtf.ErrInvalidCode
	}

	// Mark as used
	if err := s.repository.MarkUsed(ctx, otp.ID()); err != nil {
		return fmt.Errorf("failed to mark OTP as used: %w", err)
	}

	return nil
}

// Resend resends an OTP to the same destination
func (s *OTPService) Resend(ctx context.Context, userID uint, channel pkgtf.OTPChannel, destination string) (time.Time, error) {
	// Generate and send new OTP
	_, expiresAt, err := s.Generate(ctx, userID, channel, destination)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to resend OTP: %w", err)
	}

	return expiresAt, nil
}

// CleanupExpired removes all expired OTP records
func (s *OTPService) CleanupExpired(ctx context.Context) (int64, error) {
	count, err := s.repository.DeleteExpired(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired OTPs: %w", err)
	}
	return count, nil
}
