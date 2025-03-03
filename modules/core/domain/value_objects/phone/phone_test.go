package phone_test

import (
	"errors"
	country2 "github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"testing"
)

func TestNewPhoneNumber(t *testing.T) {
	tests := []struct {
		phone   string
		country country2.Country
		err     error
	}{
		// ✅ Valid US numbers
		{"415-555-1234", country2.UnitedStates, nil},
		{"(415) 555-1234", country2.UnitedStates, nil},
		{"+1 415-555-1234", country2.UnitedStates, nil},
		{"415.555.1234", country2.UnitedStates, nil},
		{"14155551234", country2.UnitedStates, nil},
		{"800-555-1234", country2.UnitedStates, nil}, // Toll-free

		// ❌ Invalid US numbers
		{"911-555-1234", country2.UnitedStates, phone.ErrInvalidPhoneNumber},    // Exchange cannot be 911
		{"123-456-7890", country2.UnitedStates, phone.ErrInvalidPhoneNumber},    // Area code cannot start with 1
		{"000-555-1234", country2.UnitedStates, phone.ErrInvalidPhoneNumber},    // Invalid area code
		{"555-555-5555", country2.UnitedStates, phone.ErrInvalidPhoneNumber},    // Reserved 555 number
		{"415555", country2.UnitedStates, phone.ErrInvalidPhoneNumber},          // Too short
		{"415555123456789", country2.UnitedStates, phone.ErrInvalidPhoneNumber}, // Too long

		// ✅ Valid international numbers
		{"+1 415-555-1234", country2.UnitedStates, nil},
		{"+44 20 7946 0958", country2.UnitedKingdom, nil},
		{"+91-9876543210", country2.India, nil},
		{"08123456789", country2.Indonesia, nil},
		{"+81 90-1234-5678", country2.Japan, nil},
		{"+49 170 1234567", country2.Germany, nil},
		{"+33 6 12 34 56 78", country2.France, nil},
		{"+254712345678", country2.Kenya, nil},
		{"+61 400 123 456", country2.Australia, nil},
		{"+358 50 1234567", country2.Finland, nil},

		// ❌ Invalid international numbers
		{"001-555-234-5678", country2.UnitedStates, phone.ErrInvalidPhoneNumber},     // Invalid country code (001)
		{"+0 1234567890", country2.UnitedStates, phone.ErrInvalidPhoneNumber},        // Country code cannot start with 0
		{"123456", country2.UnitedStates, phone.ErrInvalidPhoneNumber},               // Too short
		{"99999999999999999999", country2.UnitedStates, phone.ErrInvalidPhoneNumber}, // Too long
		{"abcd-1234-5678", country2.UnitedStates, phone.ErrInvalidPhoneNumber},       // Non-numeric
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			_, err := phone.New(tt.phone, tt.country)
			if !errors.Is(err, tt.err) {
				t.Errorf("New(%q, %v) returned error %v, expected %v", tt.phone, tt.country, err, tt.err)
			}
		})
	}
}

func TestNewFromE164(t *testing.T) {
	tests := []struct {
		name          string
		phone         string
		expectedError error
	}{
		// North American Numbers (shared +1 code)
		{
			name:          "Valid US Number",
			phone:         "+14155551234",
			expectedError: nil,
		},
		{
			name:          "Valid Canadian Number",
			phone:         "+14165551234",
			expectedError: nil,
		},
		{
			name:          "Valid Caribbean Number (Jamaica)",
			phone:         "+18765551234",
			expectedError: nil,
		},

		// European Numbers
		{
			name:          "Valid UK Number",
			phone:         "+447911123456",
			expectedError: nil,
		},
		{
			name:          "Valid German Number",
			phone:         "+4917012345678",
			expectedError: nil,
		},
		{
			name:          "Valid French Number",
			phone:         "+33612345678",
			expectedError: nil,
		},

		// Asian Numbers
		{
			name:          "Valid Chinese Number",
			phone:         "+8613912345678",
			expectedError: nil,
		},
		{
			name:          "Valid Japanese Number",
			phone:         "+819012345678",
			expectedError: nil,
		},
		{
			name:          "Valid Indian Number",
			phone:         "+919876543210",
			expectedError: nil,
		},

		// Other Regions
		{
			name:          "Valid Australian Number",
			phone:         "+61412345678",
			expectedError: nil,
		},
		{
			name:          "Valid Brazilian Number",
			phone:         "+5511987654321",
			expectedError: nil,
		},
		{
			name:          "Valid South African Number",
			phone:         "+27821234567",
			expectedError: nil,
		},

		// Shared Codes
		// TODO: Russian & Kazakh numbers validation
		//		{
		//			name:          "Valid Russian Number",
		//			phone:         "+79123456789",
		//			expectedError: nil,
		//		},
		//		{
		//			name:          "Valid Kazakh Number",
		//			phone:         "+77771234567",
		//			expectedError: nil,
		//		},

		// Invalid Cases
		//		{
		//			name:          "Empty Number",
		//			phone:         "",
		//			expectedError: phone.ErrInvalidPhoneNumber,
		//		},
		//		{
		//			name:          "Invalid Format",
		//			phone:         "not-a-number",
		//			expectedError: phone.ErrUnknownCountry,
		//		},
		//		{
		//			name:          "Too Short",
		//			phone:         "+1234",
		//			expectedError: phone.ErrInvalidPhoneNumber,
		//		},
		//		{
		//			name:          "Too Long",
		//			phone:         "+123456789012345678",
		//			expectedError: phone.ErrInvalidPhoneNumber,
		//		},
		//		{
		//			name:          "Invalid Country Code",
		//			phone:         "+0123456789",
		//			expectedError: phone.ErrUnknownCountry,
		//		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := phone.NewFromE164(tt.phone)
			if !errors.Is(err, tt.expectedError) {
				t.Errorf("NewFromE164(%q) returned error %v, expected %v", tt.phone, err, tt.expectedError)
			}
		})
	}
}

func TestParseCountry(t *testing.T) {
	tests := []struct {
		name            string
		phone           string
		expectedError   error
		expectedCountry country2.Country
	}{
		// North America
		{
			name:            "US Number",
			phone:           "+14155551234",
			expectedError:   nil,
			expectedCountry: country2.UnitedStates,
		},
		{
			name:            "Canadian Number",
			phone:           "+14165551234",
			expectedError:   nil,
			expectedCountry: country2.Canada,
		},

		// Europe
		{
			name:            "UK Number",
			phone:           "+447911123456",
			expectedError:   nil,
			expectedCountry: country2.UnitedKingdom,
		},
		{
			name:            "German Number",
			phone:           "+4917012345678",
			expectedError:   nil,
			expectedCountry: country2.Germany,
		},

		// Asia
		{
			name:            "Chinese Number",
			phone:           "+8613912345678",
			expectedError:   nil,
			expectedCountry: country2.China,
		},
		{
			name:            "Japanese Number",
			phone:           "+819012345678",
			expectedError:   nil,
			expectedCountry: country2.Japan,
		},

		// Shared Codes
		// TODO: Russian & Kazakh numbers handling
		//		{
		//			name:            "Russian Number",
		//			phone:           "+79123456789",
		//			expectedError:   nil,
		//			expectedCountry: country.Russia,
		//		},
		//		{
		//			name:            "Kazakh Number",
		//			phone:           "+77771234567",
		//			expectedError:   nil,
		//			expectedCountry: country.Kazakhstan,
		//		},

		// Edge Cases
		{
			name:            "Empty Number",
			phone:           "",
			expectedError:   phone.ErrUnknownCountry,
			expectedCountry: country2.NilCountry,
		},
		{
			name:            "Invalid Country Code",
			phone:           "+0123456789",
			expectedError:   phone.ErrUnknownCountry,
			expectedCountry: country2.NilCountry,
		},
		{
			name:            "Non-numeric",
			phone:           "+abcdefghijk",
			expectedError:   phone.ErrUnknownCountry,
			expectedCountry: country2.NilCountry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			country, err := phone.ParseCountry(tt.phone)
			if !errors.Is(err, tt.expectedError) {
				t.Errorf("ParseCountry(%q) returned error %v, expected %v", tt.phone, err, tt.expectedError)
			}
			if err == nil && country != tt.expectedCountry {
				t.Errorf(
					"ParseCountry(%q) returned country %v, expected %v",
					tt.phone,
					country,
					tt.expectedCountry,
				)
			}
		})
	}
}

func TestStrip(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"+1 (415) 555-1234", "14155551234"},
		{"415.555.1234", "4155551234"},
		{"415-555-1234", "4155551234"},
		{"(415) 555-1234", "4155551234"},
		{"1-415-555-1234", "14155551234"},
		{"+44 20 7946 0958", "442079460958"},
		{"++1-415-555-1234", "14155551234"},
		{"abc123def456", "123456"},
		{"", ""},
		{"123", "123"},
		{"!@#$%^&*()", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := phone.Strip(tt.input)
			if result != tt.expected {
				t.Errorf("Strip(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
