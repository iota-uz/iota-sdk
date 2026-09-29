package user

import (
	"crypto/rand"
	"errors"
	"math/big"
	"time"
	"unicode/utf8"
)

type Status string

const (
	StatusActive            Status = "active"
	StatusPendingOnboarding Status = "pending_onboarding"
)

const (
	TemporaryPasswordTTL         = 72 * time.Hour
	MaxTemporaryPasswordAttempts = 5
	MinPasswordLength            = 8
	// MaxPasswordBytes is the bcrypt input limit.
	MaxPasswordBytes = 72

	generatedPasswordLength   = 16
	generatedPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
)

var (
	ErrPasswordTooShort           = errors.New("password is too short")
	ErrPasswordTooLong            = errors.New("password is too long")
	ErrNotPendingOnboarding       = errors.New("user is not pending onboarding")
	ErrPasswordReusesTemporary    = errors.New("new password must differ from the temporary password")
	ErrTemporaryPasswordExpired   = errors.New("temporary password expired")
	ErrTemporaryPasswordExhausted = errors.New("temporary password attempts exhausted")
)

func (s Status) IsValid() bool {
	return s == StatusActive || s == StatusPendingOnboarding
}

// ValidatePassword applies the SDK password policy.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(password) > MaxPasswordBytes {
		return ErrPasswordTooLong
	}
	return nil
}

func GenerateTemporaryPassword() (string, error) {
	out := make([]byte, generatedPasswordLength)
	limit := big.NewInt(int64(len(generatedPasswordAlphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		out[i] = generatedPasswordAlphabet[n.Int64()]
	}
	return string(out), nil
}
