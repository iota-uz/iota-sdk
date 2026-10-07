package notification_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/stretchr/testify/require"
)

func TestNew_ActionURL(t *testing.T) {
	for _, tc := range []struct {
		url   string
		valid bool
	}{{"", true}, {"/orders/1?tab=details", true}, {"https://example.com", false}, {"//example.com", false}, {"/%2fexample.com", false}, {"/\\example.com", false}, {"/%5cexample.com", false}, {"/%0aexample.com", false}, {"javascript:alert(1)", false}} {
		t.Run(tc.url, func(t *testing.T) {
			_, err := notification.New(1, "Order", "", notification.WithActionURL(tc.url))
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
func TestNew_RequiredAndBounds(t *testing.T) {
	_, err := notification.New(0, "Title", "")
	require.Error(t, err)
	_, err = notification.New(1, " ", "")
	require.Error(t, err)
	p := (notification.FindParams{Limit: 10000, Offset: -1}).Bounded()
	require.Equal(t, 100, p.Limit)
	require.Zero(t, p.Offset)
}

func TestNotificationLevelAndCursor(t *testing.T) {
	n, err := notification.New(1, "Title", "Body", notification.WithLevel(notification.LevelWarning))
	require.NoError(t, err)
	require.Equal(t, notification.LevelWarning, n.Level())
	at, id, err := notification.ParseCursor(notification.CursorFor(n))
	require.NoError(t, err)
	require.True(t, at.Equal(n.CreatedAt()))
	require.Equal(t, n.ID(), id)
	require.Error(t, notification.ValidateCursor("invalid"))
	_, err = notification.New(1, "Title", "Body", notification.WithLevel("invalid"))
	require.Error(t, err)
}
