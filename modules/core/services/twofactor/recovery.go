package twofactor

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
	"github.com/iota-uz/iota-sdk/pkg/composables"
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

// RecoveryCodeService handles recovery code operations (internal helper)
type RecoveryCodeService struct {
	repository twofactor.RecoveryCodeRepository
}

// NewRecoveryCodeService creates a new RecoveryCodeService
func NewRecoveryCodeService(repo twofactor.RecoveryCodeRepository) *RecoveryCodeService {
	return &RecoveryCodeService{
		repository: repo,
	}
}

// Generate generates N recovery codes in the format XXXX-XXXX-XXXX
func (s *RecoveryCodeService) Generate(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code, err := s.generateSingleCode()
		if err != nil {
			return nil, fmt.Errorf("failed to generate recovery code: %w", err)
		}
		codes[i] = code
	}
	return codes, nil
}

// generateSingleCode generates a single recovery code
func (s *RecoveryCodeService) generateSingleCode() (string, error) {
	// Generate random bytes (more than needed to ensure base32 length)
	bytes := make([]byte, 10)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
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
	hashed, err := bcrypt.GenerateFromPassword([]byte(code), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash code: %w", err)
	}
	return string(hashed), nil
}

// Store generates and stores hashed recovery codes for a user
func (s *RecoveryCodeService) Store(ctx context.Context, userID uint, codes []string) error {
	_, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant ID: %w", err)
	}

	// Hash all codes
	hashes := make([]string, len(codes))
	for i, code := range codes {
		normalized := normalizeRecoveryCode(code)
		hash, err := s.hashCode(normalized)
		if err != nil {
			return fmt.Errorf("failed to hash recovery code: %w", err)
		}
		hashes[i] = hash
	}

	// Store in repository
	if err := s.repository.Create(ctx, userID, hashes); err != nil {
		return fmt.Errorf("failed to store recovery codes: %w", err)
	}

	return nil
}

// Validate validates a recovery code and marks it as used if valid
func (s *RecoveryCodeService) Validate(ctx context.Context, userID uint, code string) error {
	// Normalize the input code
	normalized := normalizeRecoveryCode(code)

	// Get all unused codes for the user
	codes, err := s.repository.FindUnused(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to retrieve recovery codes: %w", err)
	}

	if len(codes) == 0 {
		return pkgtf.ErrInvalidCode
	}

	// Try to match against each unused code
	for _, rc := range codes {
		if err := bcrypt.CompareHashAndPassword([]byte(rc.CodeHash()), []byte(normalized)); err == nil {
			// Code matches, mark as used
			if err := s.repository.MarkUsed(ctx, rc.ID()); err != nil {
				return fmt.Errorf("failed to mark recovery code as used: %w", err)
			}
			return nil
		}
	}

	return pkgtf.ErrInvalidCode
}

// Remaining returns the count of remaining unused recovery codes
func (s *RecoveryCodeService) Remaining(ctx context.Context, userID uint) (int, error) {
	count, err := s.repository.CountRemaining(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to count recovery codes: %w", err)
	}
	return count, nil
}

// Regenerate deletes all existing codes and generates new ones
func (s *RecoveryCodeService) Regenerate(ctx context.Context, userID uint, count int) ([]string, error) {
	// Validate count
	if count < 1 || count > 100 {
		return nil, errors.New("recovery code count must be between 1 and 100")
	}

	_, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant ID: %w", err)
	}

	// Start transaction
	if err := composables.InTx(ctx, func(txCtx context.Context) error {
		// Delete all existing codes
		if err := s.repository.DeleteAll(txCtx, userID); err != nil {
			return fmt.Errorf("failed to delete existing codes: %w", err)
		}

		// Generate new codes
		codes, err := s.Generate(count)
		if err != nil {
			return fmt.Errorf("failed to generate codes: %w", err)
		}

		// Hash and store new codes
		hashes := make([]string, len(codes))
		for i, code := range codes {
			normalized := normalizeRecoveryCode(code)
			hash, err := s.hashCode(normalized)
			if err != nil {
				return fmt.Errorf("failed to hash code: %w", err)
			}
			hashes[i] = hash
		}

		if err := s.repository.Create(txCtx, userID, hashes); err != nil {
			return fmt.Errorf("failed to store new codes: %w", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// Generate fresh codes to return (these won't be hashed)
	return s.Generate(count)
}

// DeleteAll removes all recovery codes for a user
func (s *RecoveryCodeService) DeleteAll(ctx context.Context, userID uint) error {
	if err := s.repository.DeleteAll(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete recovery codes: %w", err)
	}
	return nil
}
