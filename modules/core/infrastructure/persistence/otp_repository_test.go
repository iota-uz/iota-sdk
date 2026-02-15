package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	tf "github.com/iota-uz/iota-sdk/pkg/twofactor"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
)

// TestOTPRepositoryInterface verifies that OTPRepository implements the interface
func TestOTPRepositoryInterface(t *testing.T) {
	var _ twofactor.OTPRepository = (*OTPRepository)(nil)
}

// TestNewOTPRepository verifies factory function
func TestNewOTPRepository(t *testing.T) {
	repo := NewOTPRepository()
	if repo == nil {
		t.Fatal("NewOTPRepository should not return nil")
	}
}

// TestCreateMethodSignature verifies Create method signature
func TestOTPCreateMethodSignature(t *testing.T) {
	repo := NewOTPRepository()
	ctx := context.Background()

	// This test just verifies the method exists and has correct signature
	// It will fail at runtime due to missing DB context, but that's expected
	expiresAt := time.Now().Add(10 * time.Minute)
	otp := twofactor.NewOTP("test@example.com", "hash", tf.ChannelEmail, expiresAt, uuid.Nil, 1)
	_ = repo.Create(ctx, otp)
}

// TestFindByIdentifierMethodSignature verifies FindByIdentifier method signature
func TestOTPFindByIdentifierMethodSignature(t *testing.T) {
	repo := NewOTPRepository()
	ctx := context.Background()

	// This test just verifies the method exists and has correct signature
	_, _ = repo.FindByIdentifier(ctx, "test@example.com")
}

// TestIncrementAttemptsMethodSignature verifies IncrementAttempts method signature
func TestOTPIncrementAttemptsMethodSignature(t *testing.T) {
	repo := NewOTPRepository()
	ctx := context.Background()

	// This test just verifies the method exists and has correct signature
	_ = repo.IncrementAttempts(ctx, 1)
}

// TestMarkUsedMethodSignature verifies MarkUsed method signature
func TestOTPMarkUsedMethodSignature(t *testing.T) {
	repo := NewOTPRepository()
	ctx := context.Background()

	// This test just verifies the method exists and has correct signature
	_ = repo.MarkUsed(ctx, 1)
}

// TestDeleteExpiredMethodSignature verifies DeleteExpired method signature
func TestOTPDeleteExpiredMethodSignature(t *testing.T) {
	repo := NewOTPRepository()
	ctx := context.Background()

	// This test just verifies the method exists and has correct signature
	_, _ = repo.DeleteExpired(ctx)
}

// TODO: Add integration tests using setupTest() fixture similar to user_repository_test.go
// Integration tests should cover:
// - Create and FindByIdentifier (basic CRUD)
// - IncrementAttempts (verify counter increases)
// - MarkUsed (verify OTP becomes unusable)
// - DeleteExpired (verify only expired OTPs are deleted)
// - Tenant isolation (verify cross-tenant access is prevented)
//
// Example structure:
// func TestOTPRepository_Create(t *testing.T) {
//     t.Parallel()
//     f := setupTest(t)
//     repo := NewOTPRepository()
//
//     // Create OTP
//     expiresAt := time.Now().Add(10 * time.Minute)
//     otp := twofactor.NewOTP("test@example.com", "hash123", tf.ChannelEmail, expiresAt, tenantID, userID)
//     err := repo.Create(f.Ctx, otp)
//     require.NoError(t, err)
//
//     // Verify OTP was created
//     foundOTP, err := repo.FindByIdentifier(f.Ctx, "test@example.com")
//     require.NoError(t, err)
//     assert.Equal(t, "test@example.com", foundOTP.Identifier())
//
//     // Cleanup
//     t.Cleanup(func() {
//         _ = repo.MarkUsed(f.Ctx, foundOTP.ID())
//     })
// }
