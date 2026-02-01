package twofactor

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type OTPField int

const (
	OTPFieldExpiresAt OTPField = iota
	OTPFieldCreatedAt
	OTPFieldAttempts
)

type OTPSortByField = repo.SortByField[OTPField]
type OTPSortBy = repo.SortBy[OTPField]

type OTPFindParams struct {
	Identifier string
	Limit      int
	Offset     int
	SortBy     OTPSortBy
}

// OTPRepository defines the interface for OTP persistence operations
type OTPRepository interface {
	// Create stores a new OTP entity in the repository
	Create(ctx context.Context, otp OTP) error

	// FindByIdentifier retrieves an OTP by its identifier (phone number, email address, etc.)
	// Returns the OTP if found, or an error if not found or on query failure
	FindByIdentifier(ctx context.Context, identifier string) (OTP, error)

	// IncrementAttempts increments the failed attempt counter for an OTP
	// This is typically called when a user provides an invalid code
	IncrementAttempts(ctx context.Context, id uint) error

	// MarkUsed marks an OTP as used by setting the UsedAt timestamp
	// This should be called when the OTP code has been successfully verified
	MarkUsed(ctx context.Context, id uint) error

	// DeleteExpired removes all OTP records that have expired
	// Returns the count of deleted records or an error on failure
	DeleteExpired(ctx context.Context) (int64, error)
}
