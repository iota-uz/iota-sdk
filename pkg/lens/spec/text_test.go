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
