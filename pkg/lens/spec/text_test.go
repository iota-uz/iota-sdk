package spec

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextResolvePrefersExactLocaleThenFallbacks(t *testing.T) {
	t.Parallel()

	text := Text{
		Translations: map[string]string{
			"en":    "Hello",
			"uz":    "Salom",
			"uz-oz": "Assalomu alaykum",
		},
	}

	testCases := []struct {
		name     string
		locale   string
		expected string
	}{
		{name: "exact locale after normalization", locale: "uz_OZ", expected: "Assalomu alaykum"},
		{name: "language fallback", locale: "uz-Cyrl", expected: "Salom"},
		{name: "base language fallback", locale: "en-US", expected: "Hello"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.expected, text.Resolve(testCase.locale))
		})
	}
}

func TestDurationUnmarshal(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		payload  string
		expected time.Duration
	}{
		{name: "supports string", payload: `"48h"`, expected: 48 * time.Hour},
		{name: "treats numbers as seconds", payload: `90`, expected: 90 * time.Second},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var duration Duration
			require.NoError(t, json.Unmarshal([]byte(testCase.payload), &duration))
			assert.Equal(t, testCase.expected, duration.Std())
		})
	}
}

func TestTextMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		text     Text
		expected string
	}{
		{
			name:     "literal text marshals as string",
			text:     LiteralText("Sales report"),
			expected: `"Sales report"`,
		},
		{
			name: "translated text marshals as locale map",
			text: Text{
				Translations: map[string]string{
					"en": "Sales report",
					"ru": "Otchet",
				},
			},
			expected: `{"en":"Sales report","ru":"Otchet"}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			payload, err := json.Marshal(testCase.text)
			require.NoError(t, err)
			assert.JSONEq(t, testCase.expected, string(payload))

			var roundTrip Text
			require.NoError(t, json.Unmarshal(payload, &roundTrip))
			assert.Equal(t, testCase.text.Resolve("en"), roundTrip.Resolve("en"))
		})
	}
}

func TestDurationMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(Duration(48 * time.Hour))
	require.NoError(t, err)
	assert.JSONEq(t, `"48h0m0s"`, string(payload))

	var duration Duration
	require.NoError(t, json.Unmarshal(payload, &duration))
	assert.Equal(t, 48*time.Hour, duration.Std())
}
