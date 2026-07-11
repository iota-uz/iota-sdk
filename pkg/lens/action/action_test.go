package action

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSafeRelativeURL(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"/analytics/drill?token=signed#result",
		"analytics/drill",
		"?token=signed",
		"#result",
	} {
		t.Run("accepts "+raw, func(t *testing.T) {
			actual, ok := SafeRelativeURL(raw)
			require.True(t, ok)
			require.Equal(t, raw, actual)
		})
	}

	for _, raw := range []string{
		"javascript:alert(1)",
		"data:text/html,pwned",
		"//evil.example/steal",
		"https://evil.example/steal",
		`\\evil.example\steal`,
	} {
		t.Run("rejects "+raw, func(t *testing.T) {
			_, ok := SafeRelativeURL(raw)
			require.False(t, ok)
		})
	}
}
