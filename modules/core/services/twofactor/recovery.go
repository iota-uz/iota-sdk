package twofactor

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	pkgtf "github.com/iota-uz/iota-sdk/pkg/twofactor"
	"golang.org/x/crypto/bcrypt"
)

const (
	// recoveryCodeLength is the total length of a recovery code (excluding dashes)
	recoveryCodeLength = 12
	// recoveryCodeSegments is the number of segments in a formatted code (XXXX-XXXX-XXXX)
	recoveryCodeSegments = 3
	// recoveryCodeSegmentLength is the length of each segment
	recoveryCodeSegmentLength = 4
	// bcryptCost is the cost factor for bcrypt hashing
	bcryptCost = 10
)

// RecoveryCodeService handles recovery code operations (internal helper).
// Provides generation, storage, validation, and regeneration of backup codes for account recovery.
// Recovery codes are formatted as XXXX-XXXX-XXXX and hashed with bcrypt before storage.
type RecoveryCodeService struct {
	repository twofactor.RecoveryCodeRepository
}

// NewRecoveryCodeService creates a new RecoveryCodeService.
// Parameters:
//   - repo: Repository for storing and retrieving recovery codes
//
// Returns a configured RecoveryCodeService instance.
func NewRecoveryCodeService(repo twofactor.RecoveryCodeRepository) *RecoveryCodeService {
	return &RecoveryCodeService{
		repository: repo,
	}
}

// Generate generates N recovery codes in the format XXXX-XXXX-XXXX.
// Creates cryptographically random base32-encoded codes formatted with dashes for readability.
// These codes are displayed once to the user and must be stored securely.
// Parameters:
//   - count: Number of recovery codes to generate (must be between 1 and 100)
//
// Returns a slice of formatted recovery codes and an error if generation fails.
func (s *RecoveryCodeService) Generate(count int) ([]string, error) {
	const op serrors.Op = "RecoveryCodeService.Generate"
	if count < 1 || count > 100 {
		return nil, serrors.E(op, serrors.Invalid, errors.New("recovery code count must be between 1 and 100"))
	}
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code, err := s.generateSingleCode()
		if err != nil {
			return nil, serrors.E(op, err)
		}
		codes[i] = code
	}
	return codes, nil
}

// generateSingleCode generates a single recovery code
func (s *RecoveryCodeService) generateSingleCode() (string, error) {
	const op serrors.Op = "RecoveryCodeService.generateSingleCode"
	// Generate random bytes (more than needed to ensure base32 length)
	bytes := make([]byte, 10)
	if _, err := rand.Read(bytes); err != nil {
		return "", serrors.E(op, err)
	}

	// Encode to base32 and take first 12 characters
	encoded := base32.StdEncoding.EncodeToString(bytes)
	code := strings.ToUpper(encoded[:recoveryCodeLength])

	// Format as XXXX-XXXX-XXXX
	return formatRecoveryCode(code), nil
}

// formatRecoveryCode formats a 12-character code as XXXX-XXXX-XXXX
func formatRecoveryCode(code string) string {
	var segments []string
	for i := 0; i < recoveryCodeSegments; i++ {
		start := i * recoveryCodeSegmentLength
		end := start + recoveryCodeSegmentLength
		segments = append(segments, code[start:end])
	}
	return strings.Join(segments, "-")
}

// normalizeRecoveryCode removes dashes and converts to uppercase
func normalizeRecoveryCode(code string) string {
	normalized := strings.ReplaceAll(code, "-", "")
	return strings.ToUpper(normalized)
}

// hashCode hashes a recovery code using bcrypt
func (s *RecoveryCodeService) hashCode(code string) (string, error) {
	const op serrors.Op = "RecoveryCodeService.hashCode"
	hashed, err := bcrypt.GenerateFromPassword([]byte(code), bcryptCost)
	if err != nil {
		return "", serrors.E(op, err)
	}
	return string(hashed), nil
}

// Store generates and stores hashed recovery codes for a user.
// Hashes each code with bcrypt before storing in the database.
// The plaintext codes are not stored and must be saved by the caller for display to the user.
// Parameters:
//   - ctx: Request context with tenant ID
//   - userID: The user ID for whom to store recovery codes
//   - codes: The plaintext recovery codes to hash and store
//
// Returns an error if hashing or storage fails.
func (s *RecoveryCodeService) Store(ctx context.Context, userID uint, codes []string) error {
	const op serrors.Op = "RecoveryCodeService.Store"
	_, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	// Hash all codes
	hashes := make([]string, len(codes))
	for i, code := range codes {
		normalized := normalizeRecoveryCode(code)
		hash, err := s.hashCode(normalized)
		if err != nil {
			return serrors.E(op, err)
		}
		hashes[i] = hash
	}

	// Store in repository
	if err := s.repository.Create(ctx, userID, hashes); err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// Validate validates a recovery code and marks it as used if valid.
// Normalizes the input (removes dashes, converts to uppercase), compares against hashed codes,
// and marks the matching code as used to prevent reuse.
// Parameters:
//   - ctx: Request context
//   - userID: The user ID attempting recovery
//   - code: The recovery code entered by the user (with or without dashes)
//
// Returns an error if no matching code is found or validation fails.
func (s *RecoveryCodeService) Validate(ctx context.Context, userID uint, code string) error {
	const op serrors.Op = "RecoveryCodeService.Validate"
	// Normalize the input code
	normalized := normalizeRecoveryCode(code)

	// Get all unused codes for the user
	codes, err := s.repository.FindUnused(ctx, userID)
	if err != nil {
		return serrors.E(op, err)
	}

	if len(codes) == 0 {
		return pkgtf.ErrInvalidCode
	}

	// Try to match against each unused code
	for _, rc := range codes {
		if err := bcrypt.CompareHashAndPassword([]byte(rc.CodeHash()), []byte(normalized)); err == nil {
			// Code matches, mark as used
			if err := s.repository.MarkUsed(ctx, rc.ID()); err != nil {
				return serrors.E(op, err)
			}
			return nil
		}
	}

	return pkgtf.ErrInvalidCode
}

// Remaining returns the count of remaining unused recovery codes.
// Used to warn users when they are running low on backup codes.
// Parameters:
//   - ctx: Request context
//   - userID: The user ID to check
//
// Returns the count of unused recovery codes and an error if the query fails.
func (s *RecoveryCodeService) Remaining(ctx context.Context, userID uint) (int, error) {
	const op serrors.Op = "RecoveryCodeService.Remaining"
	count, err := s.repository.CountRemaining(ctx, userID)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

// Regenerate deletes all existing codes and generates new ones.
// Used when users run out of recovery codes or want to refresh them for security.
// The operation is atomic - deletes old codes and creates new ones in a transaction.
// Parameters:
//   - ctx: Request context with tenant ID
//   - userID: The user ID for whom to regenerate codes
//   - count: Number of new recovery codes to generate (must be between 1 and 100)
//
// Returns the new plaintext recovery codes (to display to user) and an error if the operation fails.
func (s *RecoveryCodeService) Regenerate(ctx context.Context, userID uint, count int) ([]string, error) {
	const op serrors.Op = "RecoveryCodeService.Regenerate"
	// Validate count
	if count < 1 || count > 100 {
		return nil, serrors.E(op, serrors.Invalid, errors.New("recovery code count must be between 1 and 100"))
	}

	_, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Generate codes ONCE before transaction starts
	// This ensures the codes returned to the user match exactly what was stored
	codes, err := s.Generate(count)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Start transaction
	if err := composables.InTx(ctx, func(txCtx context.Context) error {
		// Delete all existing codes
		if err := s.repository.DeleteAll(txCtx, userID); err != nil {
			return serrors.E(op, err)
		}

		// Hash and store the codes that were generated above
		hashes := make([]string, len(codes))
		for i, code := range codes {
			normalized := normalizeRecoveryCode(code)
			hash, err := s.hashCode(normalized)
			if err != nil {
				return serrors.E(op, err)
			}
			hashes[i] = hash
		}

		if err := s.repository.Create(txCtx, userID, hashes); err != nil {
			return serrors.E(op, err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// Return the originally generated codes that were hashed and stored
	return codes, nil
}

// DeleteAll removes all recovery codes for a user.
// Called when disabling 2FA to clean up all associated recovery codes.
// Parameters:
//   - ctx: Request context
//   - userID: The user ID for whom to delete recovery codes
//
// Returns an error if deletion fails.
func (s *RecoveryCodeService) DeleteAll(ctx context.Context, userID uint) error {
	const op serrors.Op = "RecoveryCodeService.DeleteAll"
	if err := s.repository.DeleteAll(ctx, userID); err != nil {
		return serrors.E(op, err)
	}
	return nil
}
