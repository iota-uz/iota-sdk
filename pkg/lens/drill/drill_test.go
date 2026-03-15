package drill

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		path         string
		values       url.Values
		currentTitle string
		assert       func(t *testing.T, state *State)
	}{
		{
			name:         "empty values build current crumb from path and title",
			path:         "/reports/sales",
			values:       nil,
			currentTitle: "Sales report",
			assert: func(t *testing.T, state *State) {
				t.Helper()
				require.NotNil(t, state)
				assert.Empty(t, state.Trail)
				assert.Equal(t, "/reports/sales", state.Current.URL)
				assert.Equal(t, "Sales report", state.Current.Title)
				assert.Equal(t, encodeTrail([]Crumb{{URL: "/reports/sales", Title: "Sales report"}}), state.NextTrailEncoded())
			},
		},
		{
			name: "reserved keys stay in current scope but are stripped from current url",
			path: "/reports/sales/drill",
			values: url.Values{
				QueryTrail:      []string{encodeTrail([]Crumb{{URL: "/reports/sales", Title: "Sales"}})},
				QueryPageTitle:  []string{"Product detail"},
				QueryScopeLabel: []string{"Product"},
				QueryScopeValue: []string{"OSAGO"},
				"issue_at_from": []string{"2026-03-01"},
				"issue_at_to":   []string{"2026-03-15"},
			},
			currentTitle: "Fallback title",
			assert: func(t *testing.T, state *State) {
				t.Helper()
				require.NotNil(t, state)
				require.Len(t, state.Trail, 1)
				assert.Equal(t, "Sales", state.Trail[0].Title)
				assert.Equal(t, "/reports/sales/drill?issue_at_from=2026-03-01&issue_at_to=2026-03-15", state.Current.URL)
				assert.Equal(t, "Product detail", state.Current.Title)
				assert.Equal(t, "Product", state.Current.ScopeLabel)
				assert.Equal(t, "OSAGO", state.Current.ScopeValue)
			},
		},
		{
			name: "invalid trail payload is ignored",
			path: "/reports/sales",
			values: url.Values{
				QueryTrail: []string{"not-valid"},
			},
			currentTitle: "Sales report",
			assert: func(t *testing.T, state *State) {
				t.Helper()
				require.NotNil(t, state)
				assert.Empty(t, state.Trail)
				assert.Equal(t, "Sales report", state.Current.Title)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := Parse(tt.path, tt.values, tt.currentTitle)
			tt.assert(t, state)
		})
	}
}

func TestStripRemovesReservedKeys(t *testing.T) {
	t.Parallel()

	values := url.Values{
		QueryTrail:       []string{"trail"},
		QueryPageTitle:   []string{"title"},
		QueryScopeLabel:  []string{"Product"},
		QueryScopeValue:  []string{"OSAGO"},
		QueryDestination: []string{"raw"},
		"issue_at_from":  []string{"2026-03-01"},
		"products":       []string{"osago"},
	}

	clean := Strip(values)

	assert.Equal(t, "2026-03-01", clean.Get("issue_at_from"))
	assert.Equal(t, "osago", clean.Get("products"))
	assert.Empty(t, clean.Get(QueryTrail))
	assert.Empty(t, clean.Get(QueryPageTitle))
	assert.Empty(t, clean.Get(QueryScopeLabel))
	assert.Empty(t, clean.Get(QueryScopeValue))
	assert.Empty(t, clean.Get(QueryDestination))
}

func TestHiddenFieldsIncludesOnlyNonEmptyReservedValues(t *testing.T) {
	t.Parallel()

	fields := HiddenFields(url.Values{
		QueryTrail:       []string{"trail"},
		QueryPageTitle:   []string{"Detail"},
		QueryScopeLabel:  []string{"Product"},
		QueryScopeValue:  []string{"OSAGO"},
		QueryDestination: []string{"raw"},
		"products":       []string{"osago"},
	})

	require.Len(t, fields, 5)
	assert.Equal(t, "trail", fields[QueryTrail])
	assert.Equal(t, "Detail", fields[QueryPageTitle])
	assert.Equal(t, "Product", fields[QueryScopeLabel])
	assert.Equal(t, "OSAGO", fields[QueryScopeValue])
	assert.Equal(t, "raw", fields[QueryDestination])
}

func TestParse_RoundTripsTrailEncoding(t *testing.T) {
	t.Parallel()

	initial := Parse("/reports/sales", url.Values{
		"issue_at_from": []string{"2026-03-01"},
	}, "Sales")
	nextValues := url.Values{
		QueryTrail:      []string{initial.NextTrailEncoded()},
		QueryPageTitle:  []string{"Product detail"},
		QueryScopeLabel: []string{"Product"},
		QueryScopeValue: []string{"OSAGO"},
		"products":      []string{"osago"},
	}

	state := Parse("/reports/sales/drill/product", nextValues, "Fallback")

	require.Len(t, state.Trail, 1)
	assert.Equal(t, "/reports/sales?issue_at_from=2026-03-01", state.Trail[0].URL)
	assert.Equal(t, "Sales", state.Trail[0].Title)
	assert.Equal(t, "/reports/sales/drill/product?products=osago", state.Current.URL)
	assert.Equal(t, "Product detail", state.Current.Title)
	assert.Equal(t, "OSAGO", state.Current.ScopeValue)
}
