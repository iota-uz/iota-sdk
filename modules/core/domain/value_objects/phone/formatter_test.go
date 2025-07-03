package phone_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
)

func TestFormatForDisplay(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Uzbekistan phone number",
			input:    "+998993303030",
			expected: "+998(99)330-30-30",
		},
		{
			name:     "US phone number",
			input:    "+14155551234",
			expected: "+1(415)555-1234",
		},
		{
			name:     "Canadian phone number",
			input:    "+14165551234",
			expected: "+1(416)555-1234",
		},
		{
			name:     "UK phone number",
			input:    "+447911123456",
			expected: "+44(79)1112-3456",
		},
		{
			name:     "German phone number",
			input:    "+4917012345678",
			expected: "+49(170)123-45678",
		},
		{
			name:     "French phone number",
			input:    "+33123456789",
			expected: "+33(12)34-56-789",
		},
		{
			name:     "Russian phone number",
			input:    "+79031234567",
			expected: "+7(903)123-45-67",
		},
		{
			name:     "Empty phone number",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input == "" {
				_, err := phone.NewFromE164("")
				require.Error(t, err)
				return
			}
			
			phoneObj, err := phone.NewFromE164(tt.input)
			require.NoError(t, err)
			
			result := phone.FormatForDisplay(phoneObj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatWithStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		style    phone.DisplayStyle
		expected string
	}{
		{
			name:     "Uzbekistan parentheses style",
			input:    "+998993303030",
			style:    phone.StyleParentheses,
			expected: "+998(99)330-30-30",
		},
		{
			name:     "Uzbekistan dashes style",
			input:    "+998993303030",
			style:    phone.StyleDashes,
			expected: "+998-99-330-30-30",
		},
		{
			name:     "Uzbekistan spaces style",
			input:    "+998993303030",
			style:    phone.StyleSpaces,
			expected: "+998 99 330 30 30",
		},
		{
			name:     "US parentheses style",
			input:    "+14155551234",
			style:    phone.StyleParentheses,
			expected: "+1(415)555-1234",
		},
		{
			name:     "US dashes style",
			input:    "+14155551234",
			style:    phone.StyleDashes,
			expected: "+1-415-555-1234",
		},
		{
			name:     "US spaces style",
			input:    "+14155551234",
			style:    phone.StyleSpaces,
			expected: "+1 415 555 1234",
		},
		{
			name:     "UK parentheses style",
			input:    "+447911123456",
			style:    phone.StyleParentheses,
			expected: "+44(79)1112-3456",
		},
		{
			name:     "UK dashes style",
			input:    "+447911123456",
			style:    phone.StyleDashes,
			expected: "+44-79-1112-3456",
		},
		{
			name:     "UK spaces style",
			input:    "+447911123456",
			style:    phone.StyleSpaces,
			expected: "+44 79 1112 3456",
		},
		{
			name:     "Unknown country defaults to spaces",
			input:    "+35987654321",
			style:    phone.StyleSpaces,
			expected: "+359 876 543 21",
		},
		{
			name:     "Unknown country defaults to dashes",
			input:    "+35987654321",
			style:    phone.StyleDashes,
			expected: "+359-876-543-21",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phoneObj, err := phone.NewFromE164(tt.input)
			require.NoError(t, err)
			
			result := phone.FormatWithStyle(phoneObj, tt.style)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Valid Uzbekistan phone",
			input:    "+998993303030",
			expected: "+998(99)330-30-30",
		},
		{
			name:     "Valid US phone",
			input:    "+14155551234",
			expected: "+1(415)555-1234",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Invalid phone format returns original",
			input:    "invalid",
			expected: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := phone.FormatString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatByCountry(t *testing.T) {
	tests := []struct {
		name     string
		phone    string
		country  country.Country
		style    phone.DisplayStyle
		expected string
	}{
		{
			name:     "Uzbekistan with parentheses",
			phone:    "998993303030",
			country:  country.Uzbekistan,
			style:    phone.StyleParentheses,
			expected: "+998(99)330-30-30",
		},
		{
			name:     "US with parentheses",
			phone:    "14155551234",
			country:  country.UnitedStates,
			style:    phone.StyleParentheses,
			expected: "+1(415)555-1234",
		},
		{
			name:     "Canada with parentheses",
			phone:    "14165551234",
			country:  country.Canada,
			style:    phone.StyleParentheses,
			expected: "+1(416)555-1234",
		},
		{
			name:     "UK with dashes",
			phone:    "447911123456",
			country:  country.UnitedKingdom,
			style:    phone.StyleDashes,
			expected: "+44-79-1112-3456",
		},
		{
			name:     "Germany with spaces",
			phone:    "4917012345678",
			country:  country.Germany,
			style:    phone.StyleSpaces,
			expected: "+49 170 123 45678",
		},
		{
			name:     "France with parentheses",
			phone:    "33123456789",
			country:  country.France,
			style:    phone.StyleParentheses,
			expected: "+33(12)34-56-789",
		},
		{
			name:     "Russia with dashes",
			phone:    "79031234567",
			country:  country.Russia,
			style:    phone.StyleDashes,
			expected: "+7-903-123-45-67",
		},
		{
			name:     "Invalid Uzbekistan phone returns with plus",
			phone:    "12345",
			country:  country.Uzbekistan,
			style:    phone.StyleParentheses,
			expected: "+12345",
		},
		{
			name:     "Invalid US phone returns with plus",
			phone:    "123",
			country:  country.UnitedStates,
			style:    phone.StyleParentheses,
			expected: "+123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is testing the internal formatByCountry function indirectly
			phoneObj, err := phone.NewFromE164("+"+tt.phone)
			require.NoError(t, err)
			
			result := phone.FormatWithStyle(phoneObj, tt.style)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDisplayStyles(t *testing.T) {
	tests := []struct {
		name  string
		style phone.DisplayStyle
		value int
	}{
		{
			name:  "StyleParentheses",
			style: phone.StyleParentheses,
			value: 0,
		},
		{
			name:  "StyleDashes",
			style: phone.StyleDashes,
			value: 1,
		},
		{
			name:  "StyleSpaces",
			style: phone.StyleSpaces,
			value: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.value, int(tt.style))
		})
	}
}

func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Very short number",
			input:    "+1",
			expected: "+1",
		},
		{
			name:     "Long unknown number",
			input:    "+123456789012345",
			expected: "+123 456 789 012 345",
		},
		{
			name:     "Number with leading zeros after country code",
			input:    "+998001234567",
			expected: "+998(00)123-45-67",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := phone.FormatString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}