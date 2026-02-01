package twofactor

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type RecoveryCodeField int

const (
	RecoveryCodeFieldCreatedAt RecoveryCodeField = iota
	RecoveryCodeFieldUsedAt
)

type RecoveryCodeSortByField = repo.SortByField[RecoveryCodeField]
type RecoveryCodeSortBy = repo.SortBy[RecoveryCodeField]

type RecoveryCodeFindParams struct {
	UserID uint
	Limit  int
	Offset int
	SortBy RecoveryCodeSortBy
}

// RecoveryCodeRepository defines the interface for recovery code persistence operations
type RecoveryCodeRepository interface {
	// Create stores new recovery code entities in the repository
	// The codeHashes slice contains pre-hashed recovery codes for security
	Create(ctx context.Context, userID uint, codeHashes []string) error

	// FindUnused retrieves all unused recovery codes for a given user
	// Recovery codes are considered unused if UsedAt is nil
	// Returns a slice of recovery codes and an error on failure
	FindUnused(ctx context.Context, userID uint) ([]RecoveryCode, error)

	// MarkUsed marks a recovery code as used by setting the UsedAt timestamp
	// This should be called when a recovery code has been successfully used for authentication
	MarkUsed(ctx context.Context, id uint) error

	// DeleteAll removes all recovery codes associated with a given user
	// This is useful when regenerating recovery codes or disabling 2FA
	DeleteAll(ctx context.Context, userID uint) error

	// CountRemaining returns the count of unused recovery codes for a given user
	// This can be used to determine if the user should be prompted to generate new codes
	CountRemaining(ctx context.Context, userID uint) (int, error)
}
