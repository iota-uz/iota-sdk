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

// OTPService handles OTP (One-Time Password) operations (internal helper).
// Provides generation, validation, and delivery of time-limited numeric codes for SMS and email 2FA.
// OTP codes are hashed with bcrypt before storage and have configurable expiry and attempt limits.
type OTPService struct {
	repository  twofactor.OTPRepository
	sender      pkgtf.OTPSender
	length      int
	expiry      time.Duration
	maxAttempts int
}

// NewOTPService creates a new OTPService.
// Configures OTP generation, expiry, and validation behavior.
// Parameters:
//   - repo: Repository for storing and retrieving OTP records
//   - sender: OTP delivery mechanism (SMS, email, etc.)
//   - length: Number of digits in generated codes (default: 6)
//   - expiry: How long codes remain valid (default: 10 minutes)
//   - maxAttempts: Maximum verification attempts before lockout (default: 3)
//
// Returns a configured OTPService instance.
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

// Generate generates and sends an OTP code.
// Creates a cryptographically random numeric code, hashes it with bcrypt, stores it in the database,
// and delivers it to the user via the specified channel (SMS or email).
// Parameters:
//   - ctx: Request context with tenant ID
//   - userID: The user ID requesting the OTP
//   - channel: Delivery method (SMS, email, etc.)
//   - destination: Where to send the code (phone number or email address)
//
// Returns the generated code (for testing), expiration time, and an error if generation or sending fails.
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
				"user_id":     fmt.Sprintf("%d", userID),
				"ttl_seconds": fmt.Sprintf("%d", int(s.expiry.Seconds())),
			},
		}

		if err := s.sender.Send(ctx, sendReq); err != nil {
			return "", time.Time{}, serrors.E(op, fmt.Errorf("%w: %w", pkgtf.ErrSendFailed, err))
		}
	}

	return code, expiresAt, nil
}

// Validate validates an OTP code.
// Verifies the code against the hashed value in the database, checks expiration, usage status,
// and attempt limits. Increments the attempt counter on failure and marks as used on success.
// Parameters:
//   - ctx: Request context
//   - destination: The destination (phone/email) where the OTP was sent
//   - code: The code entered by the user
//
// Returns an error if validation fails (invalid code, expired, too many attempts, etc.).
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

// Resend resends an OTP to the same destination.
// Generates a new OTP code and sends it to the previously used destination.
// Used when users don't receive the initial code or it expires.
// Parameters:
//   - ctx: Request context with tenant ID
//   - userID: The user ID requesting the resend
//   - channel: Delivery method (SMS, email, etc.)
//   - destination: Where to send the code (phone number or email address)
//
// Returns the new expiration time and an error if generation or sending fails.
func (s *OTPService) Resend(ctx context.Context, userID uint, channel pkgtf.OTPChannel, destination string) (time.Time, error) {
	const op serrors.Op = "OTPService.Resend"

	// Generate and send new OTP
	_, expiresAt, err := s.Generate(ctx, userID, channel, destination)
	if err != nil {
		return time.Time{}, serrors.E(op, err)
	}

	return expiresAt, nil
}

// CleanupExpired removes all expired OTP records.
// Should be called periodically (e.g., via cron job) to prevent database bloat.
// Deletes OTP records where the expiration time has passed.
// Parameters:
//   - ctx: Request context
//
// Returns the number of deleted records and an error if cleanup fails.
func (s *OTPService) CleanupExpired(ctx context.Context) (int64, error) {
	const op serrors.Op = "OTPService.CleanupExpired"

	count, err := s.repository.DeleteExpired(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}
