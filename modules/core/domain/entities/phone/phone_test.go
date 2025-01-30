package phone_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
)

func TestNewPhoneNumber(t *testing.T) {
	tests := []struct {
		phone   string
		country country.Country
		err     error
	}{
		// ✅ Valid US numbers
		{"415-555-1234", country.UnitedStates, nil},
		{"(415) 555-1234", country.UnitedStates, nil},
		{"+1 415-555-1234", country.UnitedStates, nil},
		{"415.555.1234", country.UnitedStates, nil},
		{"14155551234", country.UnitedStates, nil},
		{"800-555-1234", country.UnitedStates, nil}, // Toll-free

		// ❌ Invalid US numbers
		{"911-555-1234", country.UnitedStates, phone.ErrInvalidPhoneNumber},    // Exchange cannot be 911
		{"123-456-7890", country.UnitedStates, phone.ErrInvalidPhoneNumber},    // Area code cannot start with 1
		{"000-555-1234", country.UnitedStates, phone.ErrInvalidPhoneNumber},    // Invalid area code
		{"555-555-5555", country.UnitedStates, phone.ErrInvalidPhoneNumber},    // Reserved 555 number
		{"415555", country.UnitedStates, phone.ErrInvalidPhoneNumber},          // Too short
		{"415555123456789", country.UnitedStates, phone.ErrInvalidPhoneNumber}, // Too long

		// ✅ Valid international numbers
		{"+1 415-555-1234", country.UnitedStates, nil},
		{"+44 20 7946 0958", country.UnitedKingdom, nil},
		{"+91-9876543210", country.India, nil},
		{"08123456789", country.Indonesia, nil},
		{"+81 90-1234-5678", country.Japan, nil},
		{"+49 170 1234567", country.Germany, nil},
		{"+33 6 12 34 56 78", country.France, nil},
		{"+254712345678", country.Kenya, nil},
		{"+61 400 123 456", country.Australia, nil},
		{"+358 50 1234567", country.Finland, nil},

		// ❌ Invalid international numbers
		{"001-555-234-5678", country.UnitedStates, phone.ErrInvalidPhoneNumber},     // Invalid country code (001)
		{"+0 1234567890", country.UnitedStates, phone.ErrInvalidPhoneNumber},        // Country code cannot start with 0
		{"123456", country.UnitedStates, phone.ErrInvalidPhoneNumber},               // Too short
		{"99999999999999999999", country.UnitedStates, phone.ErrInvalidPhoneNumber}, // Too long
		{"abcd-1234-5678", country.UnitedStates, phone.ErrInvalidPhoneNumber},       // Non-numeric
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			_, err := phone.New(tt.phone, tt.country)
			if err != tt.err {
				t.Errorf("New(%q, %v) returned error %v, expected %v", tt.phone, tt.country, err, tt.err)
			}
		})
	}
}
