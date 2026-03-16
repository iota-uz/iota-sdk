package drill

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	state := Parse(url.Values{
		QueryFilter: []string{"product:OSAGO", "region:Tashkent"},
	})
	require.NotNil(t, state)
	require.Len(t, state.Filters, 2)
	require.Equal(t, "product", state.Filters[0].Dimension)
	require.Equal(t, "OSAGO", state.Filters[0].Value)
}

func TestStrip(t *testing.T) {
	values := Strip(url.Values{
		QueryFilter: []string{"product:OSAGO"},
		"foo":       []string{"bar"},
	})
	require.Empty(t, values.Get(QueryFilter))
	require.Equal(t, "bar", values.Get("foo"))
}
