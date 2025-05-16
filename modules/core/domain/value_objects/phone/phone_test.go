package phone_test

import (
	"errors"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
)

func TestParseCountry(t *testing.T) {
	tests := []struct {
		name            string
		phone           string
		expectedError   error
		expectedCountry country.Country
	}{
		// North America
		{
			name:            "US Number",
			phone:           "+14155551234",
			expectedError:   nil,
			expectedCountry: country.UnitedStates,
		},
		{
			name:            "Canadian Number",
			phone:           "+14165551234",
			expectedError:   nil,
			expectedCountry: country.Canada,
		},

		// Europe
		{
			name:            "UK Number",
			phone:           "+447911123456",
			expectedError:   nil,
			expectedCountry: country.UnitedKingdom,
		},
		{
			name:            "German Number",
			phone:           "+4917012345678",
			expectedError:   nil,
			expectedCountry: country.Germany,
		},

		// Asia
		{
			name:            "Chinese Number",
			phone:           "+8613912345678",
			expectedError:   nil,
			expectedCountry: country.China,
		},
		{
			name:            "Japanese Number",
			phone:           "+819012345678",
			expectedError:   nil,
			expectedCountry: country.Japan,
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
			expectedCountry: country.NilCountry,
		},
		{
			name:            "Invalid Country Code",
			phone:           "+0123456789",
			expectedError:   phone.ErrUnknownCountry,
			expectedCountry: country.NilCountry,
		},
		{
			name:            "Non-numeric",
			phone:           "+abcdefghijk",
			expectedError:   phone.ErrUnknownCountry,
			expectedCountry: country.NilCountry,
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

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		phone    string
		country  country.Country
		expected string
	}{
		// Uzbekistan Phone Numbers
		{"UZ Local Number", "993309090", country.Uzbekistan, "+998993309090"},
		{"UZ With Country Code", "998993309090", country.Uzbekistan, "+998993309090"},
		{"UZ Formatted", "99 330 90 90", country.Uzbekistan, "+998993309090"},

		// US Phone Numbers
		{"US Local Number", "4155551234", country.UnitedStates, "+14155551234"},
		{"US With Country Code", "14155551234", country.UnitedStates, "+14155551234"},
		{"US Formatted", "(415) 555-1234", country.UnitedStates, "+14155551234"},

		// Already E.164 formatted numbers should remain unchanged in content (just normalized)
		{"Already E.164 US", "+14155551234", country.UnitedStates, "+14155551234"},
		{"Already E.164 UZ", "+998993309090", country.Uzbekistan, "+998993309090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phoneObj, err := phone.Parse(tt.phone, tt.country)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if phoneObj.E164() != tt.expected {
				t.Errorf("Parse(%q).Value() = %q, expected %q", tt.phone, phoneObj.E164(), tt.expected)
			}
		})
	}
}
