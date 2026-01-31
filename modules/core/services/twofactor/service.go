package twofactor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	pkgtf "github.com/iota-uz/iota-sdk/pkg/twofactor"
)

// TwoFactorService orchestrates two-factor authentication operations
// It acts as a facade over TOTP, OTP, and Recovery code services
type TwoFactorService struct {
	// Repositories
	otpRepo          twofactor.OTPRepository
	recoveryCodeRepo twofactor.RecoveryCodeRepository
	userRepo         user.Repository

	// Helper services
	totpService         *TOTPService
	otpService          *OTPService
	recoveryCodeService *RecoveryCodeService

	// Configuration
	issuer            string
	otpLength         int
	otpExpiry         time.Duration
	otpMaxAttempts    int
	totpSkew          uint
	recoveryCodeCount int
	setupExpiry       time.Duration
	qrCodeSize        int
	otpSender         pkgtf.OTPSender
	encryptor         pkgtf.SecretEncryptor

	// Setup challenges (in-memory cache, should be replaced with Redis in production)
	setupChallenges map[string]*setupChallengeData
	challengesMu    sync.RWMutex
}

// setupChallengeData holds temporary data for a setup challenge
type setupChallengeData struct {
	UserID      uint
	Method      pkgtf.Method
	Secret      string // For TOTP
	ExpiresAt   time.Time
	Destination string // For OTP
	AccountName string // For TOTP (user's email or identifier)
}

// NewTwoFactorService creates a new TwoFactorService with all dependencies
func NewTwoFactorService(
	otpRepo twofactor.OTPRepository,
	recoveryCodeRepo twofactor.RecoveryCodeRepository,
	userRepo user.Repository,
	opts ...ServiceOption,
) (*TwoFactorService, error) {
	// Default configuration
	svc := &TwoFactorService{
		otpRepo:           otpRepo,
		recoveryCodeRepo:  recoveryCodeRepo,
		userRepo:          userRepo,
		issuer:            "IOTA",
		otpLength:         defaultOTPLength,
		otpExpiry:         defaultOTPExpiry,
		otpMaxAttempts:    defaultOTPMaxAttempts,
		totpSkew:          defaultTOTPSkew,
		recoveryCodeCount: 8,
		setupExpiry:       15 * time.Minute,
		qrCodeSize:        defaultQRCodeSize,
		setupChallenges:   make(map[string]*setupChallengeData),
	}

	// Apply options
	for _, opt := range opts {
		opt(svc)
	}

	// Validate required dependencies BEFORE creating helper services
	if svc.encryptor == nil {
		return nil, serrors.E(serrors.Op("NewTwoFactorService"), serrors.Invalid, errors.New("encryptor is required (use WithSecretEncryptor option)"))
	}
	if svc.otpSender == nil {
		return nil, serrors.E(serrors.Op("NewTwoFactorService"), serrors.Invalid, errors.New("otpSender is required (use WithOTPSender option)"))
	}

	// Initialize helper services (only after validation passes)
	svc.totpService = NewTOTPService(
		svc.encryptor,
		svc.issuer,
		svc.totpSkew,
		svc.qrCodeSize,
	)

	svc.otpService = NewOTPService(
		svc.otpRepo,
		svc.otpSender,
		svc.otpLength,
		svc.otpExpiry,
		svc.otpMaxAttempts,
	)

	svc.recoveryCodeService = NewRecoveryCodeService(
		svc.recoveryCodeRepo,
	)

	return svc, nil
}

// BeginSetup starts a 2FA setup flow for a user
// Returns a challenge that contains method-specific data (QR code for TOTP, expiry for OTP)
func (s *TwoFactorService) BeginSetup(ctx context.Context, userID uint, method pkgtf.Method) (*SetupChallenge, error) {
	const op serrors.Op = "TwoFactorService.BeginSetup"

	// Get user
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, serrors.E(op, serrors.NotFound, fmt.Errorf("failed to get user: %w", err))
	}

	// Check if 2FA is already enabled
	if u.Has2FAEnabled() {
		return nil, serrors.E(op, serrors.Invalid, errors.New("2FA is already enabled for this user"))
	}

	// Generate challenge ID
	challengeID := uuid.New().String()
	expiresAt := time.Now().Add(s.setupExpiry)

	var challenge *SetupChallenge

	switch method {
	case pkgtf.MethodTOTP:
		// Generate TOTP secret
		secret, err := s.totpService.GenerateSecret()
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to generate TOTP secret: %w", err))
		}

		// Generate QR code URL
		accountName := u.Email().Value()
		qrURL, err := s.totpService.GenerateQRCodeURL(accountName, secret)
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to generate QR URL: %w", err))
		}

		// Generate QR code PNG
		qrPNG, err := s.totpService.GenerateQRCodePNG(accountName, secret, s.qrCodeSize)
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to generate QR code: %w", err))
		}

		// Store challenge data
		s.challengesMu.Lock()
		s.setupChallenges[challengeID] = &setupChallengeData{
			UserID:      userID,
			Method:      method,
			Secret:      secret,
			ExpiresAt:   expiresAt,
			AccountName: accountName,
		}
		s.challengesMu.Unlock()

		challenge = &SetupChallenge{
			ChallengeID: challengeID,
			Method:      method,
			QRCodeURL:   qrURL,
			QRCodePNG:   qrPNG,
			ExpiresAt:   expiresAt,
		}

	case pkgtf.MethodSMS:
		// Get user's phone number
		phone := u.Phone()
		if phone.Value() == "" {
			return nil, serrors.E(op, serrors.Invalid, errors.New("user has no phone number configured"))
		}

		// Generate and send OTP
		_, otpExpiresAt, err := s.otpService.Generate(ctx, userID, pkgtf.ChannelSMS, phone.Value())
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to generate SMS OTP: %w", err))
		}

		// Store challenge data
		s.challengesMu.Lock()
		s.setupChallenges[challengeID] = &setupChallengeData{
			UserID:      userID,
			Method:      method,
			ExpiresAt:   expiresAt,
			Destination: phone.Value(),
		}
		s.challengesMu.Unlock()

		challenge = &SetupChallenge{
			ChallengeID: challengeID,
			Method:      method,
			ExpiresAt:   otpExpiresAt,
			Destination: phone.Value(),
		}

	case pkgtf.MethodEmail:
		// Get user's email
		email := u.Email().Value()
		if email == "" {
			return nil, serrors.E(op, serrors.Invalid, errors.New("user has no email configured"))
		}

		// Generate and send OTP
		_, otpExpiresAt, err := s.otpService.Generate(ctx, userID, pkgtf.ChannelEmail, email)
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to generate email OTP: %w", err))
		}

		// Store challenge data
		s.challengesMu.Lock()
		s.setupChallenges[challengeID] = &setupChallengeData{
			UserID:      userID,
			Method:      method,
			ExpiresAt:   expiresAt,
			Destination: email,
		}
		s.challengesMu.Unlock()

		challenge = &SetupChallenge{
			ChallengeID: challengeID,
			Method:      method,
			ExpiresAt:   otpExpiresAt,
			Destination: email,
		}

	case pkgtf.MethodBackupCodes:
		return nil, serrors.E(op, pkgtf.ErrMethodNotSupported, errors.New("backup codes cannot be used for initial setup"))

	default:
		return nil, serrors.E(op, pkgtf.ErrMethodNotSupported, fmt.Errorf("unsupported method: %s", method))
	}

	return challenge, nil
}

// preserveUserFields creates a new user instance with all fields from the original user preserved
// This helper ensures no fields are lost when updating user for 2FA operations
func preserveUserFields(u user.User, opts ...user.Option) user.User {
	baseOpts := []user.Option{
		user.WithID(u.ID()),
		user.WithTenantID(u.TenantID()),
		user.WithType(u.Type()),
		user.WithMiddleName(u.MiddleName()),
		user.WithPassword(u.Password()),
		user.WithAvatar(u.Avatar()),
		user.WithAvatarID(u.AvatarID()),
		user.WithRoles(u.Roles()),
		user.WithGroupIDs(u.GroupIDs()),
		user.WithPermissions(u.Permissions()),
		user.WithPhone(u.Phone()),
		user.WithLastIP(u.LastIP()),
		user.WithLastLogin(u.LastLogin()),
		user.WithLastAction(u.LastAction()),
		user.WithCreatedAt(u.CreatedAt()),
		user.WithUpdatedAt(u.UpdatedAt()),
		user.WithIsBlocked(u.IsBlocked()),
		user.WithBlockReason(u.BlockReason()),
		user.WithBlockedAt(u.BlockedAt()),
		user.WithBlockedBy(u.BlockedBy()),
		user.WithBlockedByTenantID(u.BlockedByTenantID()),
		user.WithTwoFactorMethod(u.TwoFactorMethod()),
		user.WithTwoFactorEnabledAt(u.TwoFactorEnabledAt()),
		user.WithTOTPSecretEncrypted(u.TOTPSecretEncrypted()),
	}
	// Append any additional options passed by caller (these will override base options)
	baseOpts = append(baseOpts, opts...)
	return user.New(
		u.FirstName(),
		u.LastName(),
		u.Email(),
		u.UILanguage(),
		baseOpts...,
	)
}

// ConfirmSetup completes the 2FA setup by verifying the code
func (s *TwoFactorService) ConfirmSetup(ctx context.Context, userID uint, challengeID, code string) (*SetupResult, error) {
	const op serrors.Op = "TwoFactorService.ConfirmSetup"

	// Get challenge data
	s.challengesMu.RLock()
	challengeData, exists := s.setupChallenges[challengeID]
	s.challengesMu.RUnlock()

	if !exists {
		return nil, serrors.E(op, serrors.NotFound, errors.New("invalid or expired challenge"))
	}

	// Verify user ID matches
	if challengeData.UserID != userID {
		return nil, serrors.E(op, serrors.Invalid, errors.New("challenge does not belong to this user"))
	}

	// Check expiration
	if time.Now().After(challengeData.ExpiresAt) {
		s.challengesMu.Lock()
		delete(s.setupChallenges, challengeID)
		s.challengesMu.Unlock()
		return nil, serrors.E(op, serrors.Invalid, errors.New("challenge has expired"))
	}

	// Verify code based on method
	var encryptedSecret string
	switch challengeData.Method {
	case pkgtf.MethodTOTP:
		// Validate TOTP code
		valid, err := s.totpService.ValidateWithSkew(challengeData.Secret, code, s.totpSkew)
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to validate TOTP: %w", err))
		}
		if !valid {
			return nil, serrors.E(op, pkgtf.ErrInvalidCode)
		}

		// Encrypt secret for storage
		encrypted, err := s.totpService.EncryptSecret(ctx, challengeData.Secret)
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to encrypt secret: %w", err))
		}
		encryptedSecret = encrypted

	case pkgtf.MethodSMS, pkgtf.MethodEmail:
		// Validate OTP code
		if err := s.otpService.Validate(ctx, challengeData.Destination, code); err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to validate OTP: %w", err))
		}

	case pkgtf.MethodBackupCodes:
		return nil, serrors.E(op, pkgtf.ErrMethodNotSupported, errors.New("backup codes cannot be confirmed via this endpoint"))

	default:
		return nil, serrors.E(op, pkgtf.ErrMethodNotSupported, fmt.Errorf("unsupported method: %s", challengeData.Method))
	}

	// Generate recovery codes
	recoveryCodes, err := s.recoveryCodeService.Generate(s.recoveryCodeCount)
	if err != nil {
		return nil, serrors.E(op, fmt.Errorf("failed to generate recovery codes: %w", err))
	}

	// Start transaction to update user and store recovery codes
	var result *SetupResult
	if err := composables.InTx(ctx, func(txCtx context.Context) error {
		// Get user
		u, err := s.userRepo.GetByID(txCtx, userID)
		if err != nil {
			return serrors.E(op, serrors.NotFound, fmt.Errorf("failed to get user: %w", err))
		}

		// Update user with 2FA settings
		enabledAt := time.Now()

		// Preserve all user fields and update only 2FA-related fields
		updatedUser := preserveUserFields(u,
			user.WithTwoFactorMethod(challengeData.Method),
			user.WithTwoFactorEnabledAt(enabledAt),
			user.WithTOTPSecretEncrypted(encryptedSecret),
		)

		// Update user in repository
		if err := s.userRepo.Update(txCtx, updatedUser); err != nil {
			return serrors.E(op, fmt.Errorf("failed to update user: %w", err))
		}

		// Store recovery codes
		if err := s.recoveryCodeService.Store(txCtx, userID, recoveryCodes); err != nil {
			return serrors.E(op, fmt.Errorf("failed to store recovery codes: %w", err))
		}

		result = &SetupResult{
			Method:        challengeData.Method,
			EnabledAt:     enabledAt,
			RecoveryCodes: recoveryCodes,
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// Clean up challenge
	s.challengesMu.Lock()
	delete(s.setupChallenges, challengeID)
	s.challengesMu.Unlock()

	return result, nil
}

// BeginVerification starts a 2FA verification flow
func (s *TwoFactorService) BeginVerification(ctx context.Context, userID uint) (*VerifyChallenge, error) {
	const op serrors.Op = "TwoFactorService.BeginVerification"

	// Get user
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, serrors.E(op, serrors.NotFound, fmt.Errorf("failed to get user: %w", err))
	}

	// Check if 2FA is enabled
	if !u.Has2FAEnabled() {
		return nil, serrors.E(op, serrors.Invalid, errors.New("2FA is not enabled for this user"))
	}

	method := u.TwoFactorMethod()

	var challenge *VerifyChallenge

	switch method {
	case pkgtf.MethodTOTP:
		// TOTP doesn't require server-side challenge
		challenge = &VerifyChallenge{
			ChallengeID: uuid.New().String(),
			Method:      method,
		}

	case pkgtf.MethodSMS:
		// Get user's phone number
		phone := u.Phone().Value()
		if phone == "" {
			return nil, serrors.E(op, serrors.Invalid, errors.New("user has no phone number configured"))
		}

		// Generate and send OTP
		_, expiresAt, err := s.otpService.Generate(ctx, userID, pkgtf.ChannelSMS, phone)
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to generate SMS OTP: %w", err))
		}

		challenge = &VerifyChallenge{
			ChallengeID: uuid.New().String(),
			Method:      method,
			ExpiresAt:   &expiresAt,
			Destination: phone,
		}

	case pkgtf.MethodEmail:
		// Get user's email
		email := u.Email().Value()
		if email == "" {
			return nil, serrors.E(op, serrors.Invalid, errors.New("user has no email configured"))
		}

		// Generate and send OTP
		_, expiresAt, err := s.otpService.Generate(ctx, userID, pkgtf.ChannelEmail, email)
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to generate email OTP: %w", err))
		}

		challenge = &VerifyChallenge{
			ChallengeID: uuid.New().String(),
			Method:      method,
			ExpiresAt:   &expiresAt,
			Destination: email,
		}

	case pkgtf.MethodBackupCodes:
		return nil, serrors.E(op, pkgtf.ErrMethodNotSupported, errors.New("backup codes use different verification flow"))

	default:
		return nil, serrors.E(op, pkgtf.ErrMethodNotSupported, fmt.Errorf("unsupported method: %s", method))
	}

	return challenge, nil
}

// Verify verifies a 2FA code (TOTP or OTP)
func (s *TwoFactorService) Verify(ctx context.Context, userID uint, code string) error {
	const op serrors.Op = "TwoFactorService.Verify"

	// Get user
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return serrors.E(op, serrors.NotFound, fmt.Errorf("failed to get user: %w", err))
	}

	// Check if 2FA is enabled
	if !u.Has2FAEnabled() {
		return serrors.E(op, serrors.Invalid, errors.New("2FA is not enabled for this user"))
	}

	method := u.TwoFactorMethod()

	switch method {
	case pkgtf.MethodTOTP:
		// Decrypt secret
		secret, err := s.totpService.DecryptSecret(ctx, u.TOTPSecretEncrypted())
		if err != nil {
			return serrors.E(op, fmt.Errorf("failed to decrypt TOTP secret: %w", err))
		}

		// Validate TOTP code
		valid, err := s.totpService.ValidateWithSkew(secret, code, s.totpSkew)
		if err != nil {
			return serrors.E(op, fmt.Errorf("failed to validate TOTP: %w", err))
		}
		if !valid {
			return serrors.E(op, pkgtf.ErrInvalidCode)
		}

	case pkgtf.MethodSMS:
		// Validate SMS OTP
		phone := u.Phone().Value()
		if err := s.otpService.Validate(ctx, phone, code); err != nil {
			return serrors.E(op, err)
		}

	case pkgtf.MethodEmail:
		// Validate email OTP
		email := u.Email().Value()
		if err := s.otpService.Validate(ctx, email, code); err != nil {
			return serrors.E(op, err)
		}

	case pkgtf.MethodBackupCodes:
		return serrors.E(op, pkgtf.ErrMethodNotSupported, errors.New("backup codes use VerifyRecovery method"))

	default:
		return serrors.E(op, pkgtf.ErrMethodNotSupported, fmt.Errorf("unsupported method: %s", method))
	}

	return nil
}

// VerifyRecovery verifies a recovery code
func (s *TwoFactorService) VerifyRecovery(ctx context.Context, userID uint, code string) error {
	const op serrors.Op = "TwoFactorService.VerifyRecovery"

	// Get user
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return serrors.E(op, serrors.NotFound, fmt.Errorf("failed to get user: %w", err))
	}

	// Check if 2FA is enabled
	if !u.Has2FAEnabled() {
		return serrors.E(op, serrors.Invalid, errors.New("2FA is not enabled for this user"))
	}

	// Validate recovery code
	if err := s.recoveryCodeService.Validate(ctx, userID, code); err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// GetStatus returns the current 2FA status for a user
func (s *TwoFactorService) GetStatus(ctx context.Context, userID uint) (*Status, error) {
	const op serrors.Op = "TwoFactorService.GetStatus"

	// Get user
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, serrors.E(op, serrors.NotFound, fmt.Errorf("failed to get user: %w", err))
	}

	status := &Status{
		Enabled:   u.Has2FAEnabled(),
		Method:    u.TwoFactorMethod(),
		EnabledAt: u.TwoFactorEnabledAt(),
	}

	// Get remaining recovery codes if 2FA is enabled
	if status.Enabled {
		remaining, err := s.recoveryCodeService.Remaining(ctx, userID)
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to count recovery codes: %w", err))
		}
		status.RemainingRecoveryCodes = remaining
	}

	return status, nil
}

// Disable disables 2FA for a user
func (s *TwoFactorService) Disable(ctx context.Context, userID uint) error {
	const op serrors.Op = "TwoFactorService.Disable"

	return composables.InTx(ctx, func(txCtx context.Context) error {
		// Get user
		u, err := s.userRepo.GetByID(txCtx, userID)
		if err != nil {
			return serrors.E(op, serrors.NotFound, fmt.Errorf("failed to get user: %w", err))
		}

		// Check if 2FA is enabled
		if !u.Has2FAEnabled() {
			return serrors.E(op, serrors.Invalid, errors.New("2FA is not enabled for this user"))
		}

		// Update user to disable 2FA (preserve all fields, clear only 2FA settings)
		updatedUser := preserveUserFields(u,
			user.WithTwoFactorMethod(""),
			user.WithTwoFactorEnabledAt(time.Time{}),
			user.WithTOTPSecretEncrypted(""),
		)

		if err := s.userRepo.Update(txCtx, updatedUser); err != nil {
			return serrors.E(op, fmt.Errorf("failed to update user: %w", err))
		}

		// Delete all recovery codes
		if err := s.recoveryCodeService.DeleteAll(txCtx, userID); err != nil {
			return serrors.E(op, fmt.Errorf("failed to delete recovery codes: %w", err))
		}

		return nil
	})
}

// RegenerateRecoveryCodes generates new recovery codes and replaces existing ones
func (s *TwoFactorService) RegenerateRecoveryCodes(ctx context.Context, userID uint) ([]string, error) {
	const op serrors.Op = "TwoFactorService.RegenerateRecoveryCodes"

	// Get user
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, serrors.E(op, serrors.NotFound, fmt.Errorf("failed to get user: %w", err))
	}

	// Check if 2FA is enabled
	if !u.Has2FAEnabled() {
		return nil, serrors.E(op, serrors.Invalid, errors.New("2FA is not enabled for this user"))
	}

	// Regenerate recovery codes
	codes, err := s.recoveryCodeService.Regenerate(ctx, userID, s.recoveryCodeCount)
	if err != nil {
		return nil, serrors.E(op, fmt.Errorf("failed to regenerate recovery codes: %w", err))
	}

	return codes, nil
}

// ResendSetupOTP resends an OTP for a setup challenge
func (s *TwoFactorService) ResendSetupOTP(ctx context.Context, challengeID string) (time.Time, error) {
	const op serrors.Op = "TwoFactorService.ResendSetupOTP"

	// Validate input
	if challengeID == "" {
		return time.Time{}, serrors.E(op, serrors.Invalid, errors.New("challenge ID cannot be empty"))
	}

	// Get challenge data
	s.challengesMu.RLock()
	challengeData, exists := s.setupChallenges[challengeID]
	s.challengesMu.RUnlock()

	if !exists {
		return time.Time{}, serrors.E(op, serrors.NotFound, errors.New("invalid or expired challenge"))
	}

	// Check expiration
	if time.Now().After(challengeData.ExpiresAt) {
		s.challengesMu.Lock()
		delete(s.setupChallenges, challengeID)
		s.challengesMu.Unlock()
		return time.Time{}, serrors.E(op, serrors.Invalid, errors.New("challenge has expired"))
	}

	// Only OTP methods support resend
	var channel pkgtf.OTPChannel
	switch challengeData.Method {
	case pkgtf.MethodSMS:
		channel = pkgtf.ChannelSMS
	case pkgtf.MethodEmail:
		channel = pkgtf.ChannelEmail
	default:
		return time.Time{}, serrors.E(op, serrors.Invalid, fmt.Errorf("resend not supported for method: %s", challengeData.Method))
	}

	// Resend OTP
	expiresAt, err := s.otpService.Resend(ctx, challengeData.UserID, channel, challengeData.Destination)
	if err != nil {
		return time.Time{}, serrors.E(op, fmt.Errorf("failed to resend OTP: %w", err))
	}

	return expiresAt, nil
}

// GetSetupChallenge retrieves the challenge data for a setup flow
// This is used by the presentation layer to display method-specific data (QR code for TOTP, destination for OTP)
func (s *TwoFactorService) GetSetupChallenge(challengeID string) (*SetupChallenge, error) {
	const op serrors.Op = "TwoFactorService.GetSetupChallenge"

	// Validate input
	if challengeID == "" {
		return nil, serrors.E(op, serrors.Invalid, errors.New("challenge ID cannot be empty"))
	}

	// Get challenge data
	s.challengesMu.RLock()
	challengeData, exists := s.setupChallenges[challengeID]
	s.challengesMu.RUnlock()

	if !exists {
		return nil, serrors.E(op, serrors.NotFound, errors.New("invalid or expired challenge"))
	}

	// Check expiration
	if time.Now().After(challengeData.ExpiresAt) {
		s.challengesMu.Lock()
		delete(s.setupChallenges, challengeID)
		s.challengesMu.Unlock()
		return nil, serrors.E(op, serrors.Invalid, errors.New("challenge has expired"))
	}

	// Handle different methods
	switch challengeData.Method {
	case pkgtf.MethodTOTP:
		// Regenerate QR code PNG for display
		secret := challengeData.Secret
		accountName := challengeData.AccountName

		qrPNG, err := s.totpService.GenerateQRCodePNG(accountName, secret, s.qrCodeSize)
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to generate QR code: %w", err))
		}

		qrURL, err := s.totpService.GenerateQRCodeURL(accountName, secret)
		if err != nil {
			return nil, serrors.E(op, fmt.Errorf("failed to generate QR URL: %w", err))
		}

		return &SetupChallenge{
			ChallengeID: challengeID,
			Method:      challengeData.Method,
			QRCodeURL:   qrURL,
			QRCodePNG:   qrPNG,
			ExpiresAt:   challengeData.ExpiresAt,
		}, nil

	case pkgtf.MethodSMS, pkgtf.MethodEmail:
		// Return OTP challenge data with destination
		return &SetupChallenge{
			ChallengeID: challengeID,
			Method:      challengeData.Method,
			ExpiresAt:   challengeData.ExpiresAt,
			Destination: challengeData.Destination,
		}, nil

	default:
		return nil, serrors.E(op, serrors.Invalid, fmt.Errorf("unsupported method: %s", challengeData.Method))
	}
}
