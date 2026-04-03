package spec

import (
	"encoding/json"
	"testing"
	"time"

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

	require.Equal(t, "Assalomu alaykum", text.Resolve("uz_OZ"))
	require.Equal(t, "Salom", text.Resolve("uz-Cyrl"))
	require.Equal(t, "Hello", text.Resolve("en-US"))
}

func TestDurationUnmarshalSupportsString(t *testing.T) {
	t.Parallel()

	var duration Duration
	require.NoError(t, json.Unmarshal([]byte(`"48h"`), &duration))
	require.Equal(t, 48*time.Hour, duration.Std())
}
