package state

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNULEscapeIsTold(t *testing.T) {
	for raw, want := range map[string]bool{
		`"a\u0000b"`:        true,  // a NUL character
		`"\\u0000"`:         false, // a backslash and the text u0000
		`"\\\u0000"`:        true,  // a backslash and a NUL character
		`"plain"`:           false,
		`{"k":"x\u0000"}`:   true,
		`{"k":"\\\\u0000"}`: false,
	} {
		assert.Equal(t, want, hasNULEscape([]byte(raw)), raw)
	}
	_, err := Encode(json.RawMessage(`"\\u0000"`))
	require.NoError(t, err, "the text \\u0000 is a valid value")
	_, err = Encode(json.RawMessage(`"a\u0000b"`))
	var se *Error
	require.ErrorAs(t, err, &se)
	assert.Equal(t, CodeValueInvalid, se.Code)
}

// A lifetime too long for a duration is refused, not wrapped into a short one.
func TestDurationOfMsDoesNotWrap(t *testing.T) {
	assert.Equal(t, 20*time.Minute, DurationOfMs(20*60*1000))
	days := int64(213504) * 24 * 3600 * 1000 // wraps to about 25 minutes as a duration
	require.Error(t, ValidateTTL(DurationOfMs(days)))
	assert.NoError(t, ValidateTTL(DurationOfMs(int64(MaxTTL/time.Millisecond))))
}
