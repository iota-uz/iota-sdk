package internet_test

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"testing"
)

func TestNewEmail(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:    "valid email",
			value:   "test@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with dots",
			value:   "first.last@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with plus",
			value:   "user+tag@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with numbers",
			value:   "user123@example123.com",
			wantErr: false,
		},
		{
			name:    "valid email with subdomain",
			value:   "user@sub.example.com",
			wantErr: false,
		},
		{
			name:    "empty string",
			value:   "",
			wantErr: true,
		},
		{
			name:    "missing @",
			value:   "userexample.com",
			wantErr: true,
		},
		{
			name:    "missing domain",
			value:   "user@.com",
			wantErr: true,
		},
		{
			name:    "missing TLD",
			value:   "user@example",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			value:   "user!#$%@example.com",
			wantErr: true,
		},
		{
			name:    "double @",
			value:   "user@@example.com",
			wantErr: true,
		},
		{
			name:    "single letter TLD",
			value:   "user@example.a",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, err := internet.NewEmail(tt.value)

			if tt.wantErr {
				if err == nil {
					t.Error("NewEmail() error = nil, wantErr true")
				}
				if err != internet.ErrInvalidEmail {
					t.Errorf("NewEmail() error = %v, want %v", err, internet.ErrInvalidEmail)
				}
				return
			}

			if err != nil {
				t.Errorf("NewEmail() unexpected error = %v", err)
				return
			}

			if got := email.Value(); got != tt.value {
				t.Errorf("Email.Value() = %v, want %v", got, tt.value)
			}
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		// All previous test cases but formatted for IsValidEmail
		{
			name:  "valid email",
			value: "test@example.com",
			want:  true,
		},
		// ... add all other test cases here
		{
			name:  "email with multiple dots in domain",
			value: "user@multiple.dots.example.com",
			want:  true,
		},
		{
			name:  "email with hyphen in domain",
			value: "user@my-example.com",
			want:  true,
		},
		{
			name:  "spaces in email",
			value: "user name@example.com",
			want:  false,
		},
		{
			name:  "consecutive dots",
			value: "user..name@example.com",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := internet.IsValidEmail(tt.value); got != tt.want {
				t.Errorf("IsValidEmail(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestEmailDomain(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "gmail.com domain from email",
			value: "test@gmail.com",
			want:  "gmail.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, err := internet.NewEmail(tt.value)
			if err != nil {
				t.Errorf("NewEmail() unexpected error = %v", err)
				return
			}
			if got := email.Domain(); got != tt.want {
				t.Errorf("Email.Domain() = %v, want = %v", email.Domain(), tt.want)
			}
		})
	}
}

func TestEmailUsername(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "test@gmail.com username equals to test",
			value: "test@gmail.com",
			want:  "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, err := internet.NewEmail(tt.value)
			if err != nil {
				t.Errorf("NewEmail() unexpected error = %v", err)
				return
			}
			if got := email.Username(); got != tt.want {
				t.Errorf("Email.Username() = %v, want = %v", email.Username(), tt.want)
			}
		})
	}
}
