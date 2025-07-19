package table

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateSortURL(t *testing.T) {
	tests := []struct {
		name              string
		baseURL           string
		fieldKey          string
		currentSortField  string
		currentSortOrder  string
		expectedURL       string
	}{
		{
			name:              "First click on column - should sort ASC",
			baseURL:           "/users",
			fieldKey:          "name",
			currentSortField:  "",
			currentSortOrder:  "",
			expectedURL:       "/users?order=asc&sort=name",
		},
		{
			name:              "Second click on same column - should sort DESC",
			baseURL:           "/users",
			fieldKey:          "name",
			currentSortField:  "name",
			currentSortOrder:  "asc",
			expectedURL:       "/users?order=desc&sort=name",
		},
		{
			name:              "Third click on same column - should reset sorting",
			baseURL:           "/users",
			fieldKey:          "name",
			currentSortField:  "name",
			currentSortOrder:  "desc",
			expectedURL:       "/users",
		},
		{
			name:              "Click on different column - should sort ASC",
			baseURL:           "/users",
			fieldKey:          "email",
			currentSortField:  "name",
			currentSortOrder:  "asc",
			expectedURL:       "/users?order=asc&sort=email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSortURL(tt.baseURL, tt.fieldKey, tt.currentSortField, tt.currentSortOrder)
			assert.Equal(t, tt.expectedURL, result)
		})
	}
}

func TestGenerateSortURLWithParams(t *testing.T) {
	tests := []struct {
		name              string
		baseURL           string
		fieldKey          string
		currentSortField  string
		currentSortOrder  string
		existingParams    url.Values
		expectedURL       string
	}{
		{
			name:              "First click with search parameter",
			baseURL:           "/users",
			fieldKey:          "name",
			currentSortField:  "",
			currentSortOrder:  "",
			existingParams:    url.Values{"Search": []string{"john"}},
			expectedURL:       "/users?Search=john&order=asc&sort=name",
		},
		{
			name:              "Reset sort with existing parameters",
			baseURL:           "/users",
			fieldKey:          "name",
			currentSortField:  "name",
			currentSortOrder:  "desc",
			existingParams:    url.Values{"Search": []string{"john"}, "status": []string{"active"}},
			expectedURL:       "/users?Search=john&status=active",
		},
		{
			name:              "Change sort field with existing parameters",
			baseURL:           "/users",
			fieldKey:          "email",
			currentSortField:  "name",
			currentSortOrder:  "asc",
			existingParams:    url.Values{"Search": []string{"john"}},
			expectedURL:       "/users?Search=john&order=asc&sort=email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSortURLWithParams(tt.baseURL, tt.fieldKey, tt.currentSortField, tt.currentSortOrder, tt.existingParams)
			
			// Parse both URLs to compare parameters
			resultURL, err := url.Parse(result)
			assert.NoError(t, err)
			
			expectedURL, err := url.Parse(tt.expectedURL)
			assert.NoError(t, err)
			
			// Compare base path
			assert.Equal(t, expectedURL.Path, resultURL.Path)
			
			// Compare query parameters
			assert.Equal(t, expectedURL.Query(), resultURL.Query())
		})
	}
}

func TestGetSortDirection(t *testing.T) {
	tests := []struct {
		name              string
		fieldKey          string
		currentSortField  string
		currentSortOrder  string
		expectedDirection string
	}{
		{
			name:              "Field is currently sorted ASC",
			fieldKey:          "name",
			currentSortField:  "name",
			currentSortOrder:  "asc",
			expectedDirection: "asc",
		},
		{
			name:              "Field is currently sorted DESC",
			fieldKey:          "name",
			currentSortField:  "name",
			currentSortOrder:  "desc",
			expectedDirection: "desc",
		},
		{
			name:              "Field is not currently sorted",
			fieldKey:          "email",
			currentSortField:  "name",
			currentSortOrder:  "asc",
			expectedDirection: "",
		},
		{
			name:              "No sorting applied",
			fieldKey:          "name",
			currentSortField:  "",
			currentSortOrder:  "",
			expectedDirection: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSortDirection(tt.fieldKey, tt.currentSortField, tt.currentSortOrder)
			assert.Equal(t, tt.expectedDirection, result)
		})
	}
}

func TestColumnWithSortingOptions(t *testing.T) {
	col := Column("name", "Name",
		WithSortable(true),
		WithSortDir("asc"),
		WithSortURL("/users?sort=name&order=desc"),
	)

	assert.Equal(t, "name", col.Key())
	assert.Equal(t, "Name", col.Label())
	assert.True(t, col.Sortable())
	assert.Equal(t, "asc", col.SortDir())
	assert.Equal(t, "/users?sort=name&order=desc", col.SortURL())
}

func TestColumnWithoutSorting(t *testing.T) {
	col := Column("actions", "Actions")

	assert.Equal(t, "actions", col.Key())
	assert.Equal(t, "Actions", col.Label())
	assert.False(t, col.Sortable()) // Default should be false
	assert.Equal(t, "", col.SortDir())
	assert.Equal(t, "", col.SortURL())
}

func TestSortURLCycling(t *testing.T) {
	baseURL := "/users"
	fieldKey := "name"

	// First click: no sort -> ASC
	url1 := GenerateSortURL(baseURL, fieldKey, "", "")
	assert.Contains(t, url1, "sort=name")
	assert.Contains(t, url1, "order=asc")

	// Second click: ASC -> DESC
	url2 := GenerateSortURL(baseURL, fieldKey, "name", "asc")
	assert.Contains(t, url2, "sort=name")
	assert.Contains(t, url2, "order=desc")

	// Third click: DESC -> no sort
	url3 := GenerateSortURL(baseURL, fieldKey, "name", "desc")
	assert.Equal(t, baseURL, url3) // Should be just base URL without params

	// Fourth click: no sort -> ASC (cycle repeats)
	url4 := GenerateSortURL(baseURL, fieldKey, "", "")
	assert.Equal(t, url1, url4) // Should be same as first click
}