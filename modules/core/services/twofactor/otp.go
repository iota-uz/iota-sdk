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
	"github.com/iota-uz/iota-sdk/pkg/serrors"
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
	repository  twofactor.OTPRepository
	sender      pkgtf.OTPSender
	length      int
	expiry      time.Duration
	maxAttempts int
}

// NewOTPService creates a new OTPService
func NewOTPService(
	repo twofactor.OTPRepository,
	sender pkgtf.OTPSender,
	length int,
	expiry time.Duration,
	maxAttempts int,
) *OTPService {
	if repo == nil {
		panic("OTPRepository cannot be nil")
	}
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
	const op serrors.Op = "OTPService.generateCode"

	// Calculate max value (e.g., for 6 digits: 10^6 = 1000000)
	// rand.Int returns [0, max) so we get [0, 999999] for 6 digits
	maxVal := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(s.length)), nil)

	// Generate random number
	n, err := rand.Int(rand.Reader, maxVal)
	if err != nil {
		return "", serrors.E(op, err)
	}

	// Format with leading zeros
	format := fmt.Sprintf("%%0%dd", s.length)
	return fmt.Sprintf(format, n), nil
}

// hashCode hashes an OTP code using bcrypt
func (s *OTPService) hashCode(code string) (string, error) {
	const op serrors.Op = "OTPService.hashCode"

	hashed, err := bcrypt.GenerateFromPassword([]byte(code), bcryptCost)
	if err != nil {
		return "", serrors.E(op, err)
	}
	return string(hashed), nil
}

// Generate generates and sends an OTP code
func (s *OTPService) Generate(ctx context.Context, userID uint, channel pkgtf.OTPChannel, destination string) (string, time.Time, error) {
	const op serrors.Op = "OTPService.Generate"

	if destination == "" {
		return "", time.Time{}, serrors.E(op, serrors.Invalid, errors.New("destination cannot be empty"))
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return "", time.Time{}, serrors.E(op, err)
	}

	// Generate code
	code, err := s.generateCode()
	if err != nil {
		return "", time.Time{}, serrors.E(op, err)
	}

	// Hash code
	codeHash, err := s.hashCode(code)
	if err != nil {
		return "", time.Time{}, serrors.E(op, err)
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
		return "", time.Time{}, serrors.E(op, err)
	}

	// Send the OTP
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
			return "", time.Time{}, serrors.E(op, fmt.Errorf("%w: %w", pkgtf.ErrSendFailed, err))
		}
	}

	return code, expiresAt, nil
}

// Validate validates an OTP code
func (s *OTPService) Validate(ctx context.Context, destination, code string) error {
	const op serrors.Op = "OTPService.Validate"

	if destination == "" {
		return serrors.E(op, serrors.Invalid, errors.New("destination cannot be empty"))
	}
	if code == "" {
		return serrors.E(op, pkgtf.ErrInvalidCode)
	}

	// Find OTP by identifier
	otp, err := s.repository.FindByIdentifier(ctx, destination)
	if err != nil {
		return serrors.E(op, pkgtf.ErrInvalidCode)
	}

	// Check if expired
	if otp.IsExpired() {
		return serrors.E(op, pkgtf.ErrExpiredCode)
	}

	// Check if already used
	if otp.IsUsed() {
		return serrors.E(op, pkgtf.ErrInvalidCode)
	}

	// Check attempts
	if otp.Attempts() >= s.maxAttempts {
		return serrors.E(op, pkgtf.ErrTooManyAttempts)
	}

	// Verify code
	if err := bcrypt.CompareHashAndPassword([]byte(otp.CodeHash()), []byte(code)); err != nil {
		// Increment attempts
		if err := s.repository.IncrementAttempts(ctx, otp.ID()); err != nil {
			return serrors.E(op, err)
		}
		return serrors.E(op, pkgtf.ErrInvalidCode)
	}

	// Mark as used
	if err := s.repository.MarkUsed(ctx, otp.ID()); err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// Resend resends an OTP to the same destination
func (s *OTPService) Resend(ctx context.Context, userID uint, channel pkgtf.OTPChannel, destination string) (time.Time, error) {
	const op serrors.Op = "OTPService.Resend"

	// Generate and send new OTP
	_, expiresAt, err := s.Generate(ctx, userID, channel, destination)
	if err != nil {
		return time.Time{}, serrors.E(op, err)
	}

	return expiresAt, nil
}

// CleanupExpired removes all expired OTP records
func (s *OTPService) CleanupExpired(ctx context.Context) (int64, error) {
	const op serrors.Op = "OTPService.CleanupExpired"

	count, err := s.repository.DeleteExpired(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}
