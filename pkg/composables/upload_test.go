package composables_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func TestUseUploadSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "returns general when not set",
			ctx:      context.Background(),
			expected: "general",
		},
		{
			name:     "returns set value",
			ctx:      composables.WithUploadSource(context.Background(), "custom"),
			expected: "custom",
		},
		{
			name:     "returns general when empty string",
			ctx:      composables.WithUploadSource(context.Background(), ""),
			expected: "general",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := composables.UseUploadSource(tt.ctx)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCheckUploadSourceAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		ctx         context.Context
		source      string
		expectError bool
	}{
		{
			name:        "allows access when no checker",
			ctx:         context.Background(),
			source:      "general",
			expectError: false,
		},
		{
			name:        "allows access when checker allows",
			ctx:         composables.WithUploadAccessChecker(context.Background(), &mockAccessChecker{allowAccess: true}),
			source:      "general",
			expectError: false,
		},
		{
			name:        "denies access when checker denies",
			ctx:         composables.WithUploadAccessChecker(context.Background(), &mockAccessChecker{allowAccess: false}),
			source:      "restricted",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := composables.CheckUploadSourceAccess(tt.ctx, tt.source, nil)
			if (err != nil) != tt.expectError {
				t.Errorf("expected error=%v, got error=%v", tt.expectError, err != nil)
			}
		})
	}
}

func TestCheckUploadToSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		ctx         context.Context
		source      string
		expectError bool
	}{
		{
			name:        "allows upload when no checker",
			ctx:         context.Background(),
			source:      "general",
			expectError: false,
		},
		{
			name:        "allows upload when checker allows",
			ctx:         composables.WithUploadAccessChecker(context.Background(), &mockAccessChecker{allowUpload: true}),
			source:      "general",
			expectError: false,
		},
		{
			name:        "denies upload when checker denies",
			ctx:         composables.WithUploadAccessChecker(context.Background(), &mockAccessChecker{allowUpload: false}),
			source:      "restricted",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := composables.CheckUploadToSource(tt.ctx, tt.source, nil)
			if (err != nil) != tt.expectError {
				t.Errorf("expected error=%v, got error=%v", tt.expectError, err != nil)
			}
		})
	}
}

// mockAccessChecker is a mock implementation for testing
type mockAccessChecker struct {
	allowAccess bool
	allowUpload bool
}

func (m *mockAccessChecker) CanAccessSource(r *http.Request, source string) error {
	if !m.allowAccess {
		return errors.New("access denied")
	}
	return nil
}

func (m *mockAccessChecker) CanUploadToSource(r *http.Request, source string) error {
	if !m.allowUpload {
		return errors.New("upload denied")
	}
	return nil
}
