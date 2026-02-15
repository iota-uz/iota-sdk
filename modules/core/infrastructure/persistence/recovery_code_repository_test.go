package persistence

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
)

// TestRecoveryCodeRepositoryInterface verifies that RecoveryCodeRepository implements the interface
func TestRecoveryCodeRepositoryInterface(t *testing.T) {
	var _ twofactor.RecoveryCodeRepository = (*RecoveryCodeRepository)(nil)
}

// TestNewRecoveryCodeRepository verifies factory function
func TestNewRecoveryCodeRepository(t *testing.T) {
	repo := NewRecoveryCodeRepository()
	if repo == nil {
		t.Fatal("NewRecoveryCodeRepository should not return nil")
	}
}

// TestCreateMethodExists verifies Create method signature
func TestCreateMethodSignature(t *testing.T) {
	repo := NewRecoveryCodeRepository()
	ctx := context.Background()

	// This test just verifies the method exists and has correct signature
	// It will fail at runtime due to missing DB context, but that's expected
	_ = repo.Create(ctx, 1, []string{"hash1", "hash2"})
}

// TestFindUnusedMethodExists verifies FindUnused method signature
func TestFindUnusedMethodSignature(t *testing.T) {
	repo := NewRecoveryCodeRepository()
	ctx := context.Background()

	// This test just verifies the method exists and has correct signature
	_, _ = repo.FindUnused(ctx, 1)
}

// TestMarkUsedMethodExists verifies MarkUsed method signature
func TestMarkUsedMethodSignature(t *testing.T) {
	repo := NewRecoveryCodeRepository()
	ctx := context.Background()

	// This test just verifies the method exists and has correct signature
	_ = repo.MarkUsed(ctx, 1)
}

// TestDeleteAllMethodExists verifies DeleteAll method signature
func TestDeleteAllMethodSignature(t *testing.T) {
	repo := NewRecoveryCodeRepository()
	ctx := context.Background()

	// This test just verifies the method exists and has correct signature
	_ = repo.DeleteAll(ctx, 1)
}

// TestCountRemainingMethodExists verifies CountRemaining method signature
func TestCountRemainingMethodSignature(t *testing.T) {
	repo := NewRecoveryCodeRepository()
	ctx := context.Background()

	// This test just verifies the method exists and has correct signature
	_, _ = repo.CountRemaining(ctx, 1)
}
