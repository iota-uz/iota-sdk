package pagination

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNextChunkURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current string
		page    int
		limit   int
		loaded  int
		want    string
	}{
		{
			name:    "full chunk points to the next page",
			current: "/finance/expenses",
			page:    1,
			limit:   25,
			loaded:  25,
			want:    "/finance/expenses?limit=25&page=2",
		},
		{
			name:    "keeps search and filters",
			current: "/finance/expenses?Search=rent&CreatedAt.From=2026-01-01&page=3&limit=10",
			page:    3,
			limit:   10,
			loaded:  10,
			want:    "/finance/expenses?CreatedAt.From=2026-01-01&Search=rent&limit=10&page=4",
		},
		{
			name:    "short chunk is the last one",
			current: "/finance/expenses",
			page:    2,
			limit:   25,
			loaded:  7,
			want:    "",
		},
		{
			name:    "empty chunk is the last one",
			current: "/finance/expenses",
			page:    1,
			limit:   25,
			loaded:  0,
			want:    "",
		},
		{
			name:    "no limit means no next chunk",
			current: "/finance/expenses",
			page:    1,
			limit:   0,
			loaded:  0,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			current, err := url.Parse(tt.current)
			require.NoError(t, err)
			require.Equal(t, tt.want, NextChunkURL(current, tt.page, tt.limit, tt.loaded))
		})
	}
}

func TestNextChunkURL_NilURL(t *testing.T) {
	t.Parallel()
	require.Empty(t, NextChunkURL(nil, 1, 25, 25))
}
