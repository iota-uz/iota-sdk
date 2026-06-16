package table

import (
	"net/url"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSortURL(t *testing.T) {
	tests := []struct {
		name             string
		baseURL          string
		fieldKey         string
		currentSortField string
		currentSortOrder string
		expectedURL      string
	}{
		{
			name:             "First click on column - should sort ASC",
			baseURL:          "/users",
			fieldKey:         "name",
			currentSortField: "",
			currentSortOrder: "",
			expectedURL:      "/users?order=asc&sort=name",
		},
		{
			name:             "Second click on same column - should sort DESC",
			baseURL:          "/users",
			fieldKey:         "name",
			currentSortField: "name",
			currentSortOrder: "asc",
			expectedURL:      "/users?order=desc&sort=name",
		},
		{
			name:             "Third click on same column - should reset sorting",
			baseURL:          "/users",
			fieldKey:         "name",
			currentSortField: "name",
			currentSortOrder: "desc",
			expectedURL:      "/users",
		},
		{
			name:             "Click on different column - should sort ASC",
			baseURL:          "/users",
			fieldKey:         "email",
			currentSortField: "name",
			currentSortOrder: "asc",
			expectedURL:      "/users?order=asc&sort=email",
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
		name             string
		baseURL          string
		fieldKey         string
		currentSortField string
		currentSortOrder string
		existingParams   url.Values
		expectedURL      string
	}{
		{
			name:             "First click with search parameter",
			baseURL:          "/users",
			fieldKey:         "name",
			currentSortField: "",
			currentSortOrder: "",
			existingParams:   url.Values{"Search": []string{"john"}},
			expectedURL:      "/users?Search=john&order=asc&sort=name",
		},
		{
			name:             "Reset sort with existing parameters",
			baseURL:          "/users",
			fieldKey:         "name",
			currentSortField: "name",
			currentSortOrder: "desc",
			existingParams:   url.Values{"Search": []string{"john"}, "status": []string{"active"}},
			expectedURL:      "/users?Search=john&status=active",
		},
		{
			name:             "Change sort field with existing parameters",
			baseURL:          "/users",
			fieldKey:         "email",
			currentSortField: "name",
			currentSortOrder: "asc",
			existingParams:   url.Values{"Search": []string{"john"}},
			expectedURL:      "/users?Search=john&order=asc&sort=email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSortURLWithParams(tt.baseURL, tt.fieldKey, tt.currentSortField, tt.currentSortOrder, tt.existingParams)

			// Parse both URLs to compare parameters
			resultURL, err := url.Parse(result)
			require.NoError(t, err)

			expectedURL, err := url.Parse(tt.expectedURL)
			require.NoError(t, err)

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
		expectedDirection SortDirection
	}{
		{
			name:              "Field is currently sorted ASC",
			fieldKey:          "name",
			currentSortField:  "name",
			currentSortOrder:  "asc",
			expectedDirection: SortDirectionAsc,
		},
		{
			name:              "Field is currently sorted DESC",
			fieldKey:          "name",
			currentSortField:  "name",
			currentSortOrder:  "desc",
			expectedDirection: SortDirectionDesc,
		},
		{
			name:              "Field is not currently sorted",
			fieldKey:          "email",
			currentSortField:  "name",
			currentSortOrder:  "asc",
			expectedDirection: SortDirectionNone,
		},
		{
			name:              "No sorting applied",
			fieldKey:          "name",
			currentSortField:  "",
			currentSortOrder:  "",
			expectedDirection: SortDirectionNone,
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
		WithSortable(),
		WithSortDir(SortDirectionAsc),
		WithSortURL("/users?sort=name&order=desc"),
	)

	assert.Equal(t, "name", col.Key())
	assert.Equal(t, "Name", col.Label())
	assert.True(t, col.Sortable())
	assert.Equal(t, SortDirectionAsc, col.SortDir())
	assert.Equal(t, "/users?sort=name&order=desc", col.SortURL())
}

func TestColumnDefaultBehavior(t *testing.T) {
	col := Column("actions", "Actions")

	assert.Equal(t, "actions", col.Key())
	assert.Equal(t, "Actions", col.Label())
	assert.False(t, col.Sortable()) // Should be false by default
	assert.Equal(t, SortDirectionNone, col.SortDir())
	assert.Empty(t, col.SortURL())
}

func TestColumnWithSortable(t *testing.T) {
	col := Column("name", "Name", WithSortable())

	assert.Equal(t, "name", col.Key())
	assert.Equal(t, "Name", col.Label())
	assert.True(t, col.Sortable()) // Should be true with WithSortable
	assert.Equal(t, SortDirectionNone, col.SortDir())
	assert.Empty(t, col.SortURL())
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

func TestWithTruncate(t *testing.T) {
	t.Run("default width", func(t *testing.T) {
		col := Column("description", "Description", WithTruncate())
		assert.True(t, col.Truncate())
		assert.Equal(t, DefaultTruncateWidth, col.TruncateWidth())
	})

	t.Run("explicit width", func(t *testing.T) {
		col := Column("description", "Description", WithTruncate(320))
		assert.True(t, col.Truncate())
		assert.Equal(t, 320, col.TruncateWidth())
	})

	t.Run("not set by default", func(t *testing.T) {
		col := Column("description", "Description")
		assert.False(t, col.Truncate())
		assert.Equal(t, 0, col.TruncateWidth())
	})
}

func TestWithTruncateDefault(t *testing.T) {
	t.Run("applies to non-sticky columns lacking own setting", func(t *testing.T) {
		cfg := NewTableConfig("t", "/x", WithTruncateDefault())
		col := Column("description", "Description")
		on, width := cfg.resolvedTruncate(col)
		assert.True(t, on)
		assert.Equal(t, DefaultTruncateWidth, width)
	})

	t.Run("custom default width", func(t *testing.T) {
		cfg := NewTableConfig("t", "/x", WithTruncateDefault(180))
		col := Column("description", "Description")
		on, width := cfg.resolvedTruncate(col)
		assert.True(t, on)
		assert.Equal(t, 180, width)
	})

	t.Run("skips sticky columns", func(t *testing.T) {
		cfg := NewTableConfig("t", "/x", WithTruncateDefault())
		col := Column("actions", "Actions", WithSticky(StickyPositionRight))
		on, _ := cfg.resolvedTruncate(col)
		assert.False(t, on)
	})

	t.Run("column override wins over default width", func(t *testing.T) {
		cfg := NewTableConfig("t", "/x", WithTruncateDefault(180))
		col := Column("description", "Description", WithTruncate(400))
		on, width := cfg.resolvedTruncate(col)
		assert.True(t, on)
		assert.Equal(t, 400, width)
	})

	t.Run("no default leaves non-truncated columns alone", func(t *testing.T) {
		cfg := NewTableConfig("t", "/x")
		col := Column("name", "Name")
		on, width := cfg.resolvedTruncate(col)
		assert.False(t, on)
		assert.Equal(t, 0, width)
	})
}

func TestWithPriority(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		expected int
	}{
		{"unset", 0, 0},
		{"always visible", 1, 1},
		{"tablet hide", 2, 2},
		{"desktop hide", 3, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := Column("created_at", "Created", WithPriority(tt.priority))
			assert.Equal(t, tt.expected, col.Priority())
		})
	}

	t.Run("sticky columns exempt", func(t *testing.T) {
		cfg := NewTableConfig("t", "/x")
		col := Column("actions", "Actions", WithPriority(3), WithSticky(StickyPositionRight))
		assert.Equal(t, 0, cfg.resolvedPriority(col))
	})

	t.Run("non-sticky keeps priority", func(t *testing.T) {
		cfg := NewTableConfig("t", "/x")
		col := Column("created_at", "Created", WithPriority(2))
		assert.Equal(t, 2, cfg.resolvedPriority(col))
	})
}

func TestPriorityClassMapping(t *testing.T) {
	assert.Empty(t, priorityCellClass(0))
	assert.Empty(t, priorityCellClass(1))
	assert.Equal(t, "max-md:hidden", priorityCellClass(2))
	assert.Equal(t, "max-lg:hidden", priorityCellClass(3))
	assert.Equal(t, "max-lg:hidden", priorityCellClass(4))
}

func TestWithDrawerAttrs(t *testing.T) {
	row := Row().ApplyOpts(WithDrawer("/finance/expense-categories/abc/drawer"))
	attrs := row.Attrs()

	assert.Equal(t, "/finance/expense-categories/abc/drawer", attrs["hx-get"])
	assert.Equal(t, "#view-drawer", attrs["hx-target"])
	assert.Equal(t, "innerHTML", attrs["hx-swap"])
	assert.Equal(t, "0", attrs["tabindex"])
	assert.Equal(t, "true", attrs["data-row-drawer"])
	assert.Equal(t, "button", attrs["role"])
	assert.Equal(t, "dialog", attrs["aria-haspopup"])

	class, ok := attrs["class"].(string)
	require.True(t, ok)
	assert.Contains(t, class, "cursor-pointer")
	assert.Contains(t, class, "focus-visible:ring-2")
}

func TestToBaseTableColumnsPropagatesTruncateAndPriority(t *testing.T) {
	cfg := NewTableConfig("t", "/x")
	cfg.AddCols(
		Column("name", "Name"),
		Column("description", "Description", WithTruncate(320)),
		Column("created_at", "Created", WithPriority(2)),
	)

	cols := toBaseTableColumns(cfg, types.PageContext(nil))
	require.Len(t, cols, 3)

	assert.Equal(t, 0, cols[0].TruncateWidth)
	assert.Equal(t, 0, cols[0].Priority)

	assert.Equal(t, 320, cols[1].TruncateWidth)
	assert.Equal(t, 0, cols[1].Priority)

	assert.Equal(t, 0, cols[2].TruncateWidth)
	assert.Equal(t, 2, cols[2].Priority)
}

func TestToBaseTableColumnsAppliesTruncateDefault(t *testing.T) {
	cfg := NewTableConfig("t", "/x", WithTruncateDefault(200))
	cfg.AddCols(
		Column("description", "Description"),
		Column("actions", "Actions", WithSticky(StickyPositionRight)),
	)

	cols := toBaseTableColumns(cfg, types.PageContext(nil))
	require.Len(t, cols, 2)
	assert.Equal(t, 200, cols[0].TruncateWidth, "non-sticky gets default truncate width")
	assert.Equal(t, 0, cols[1].TruncateWidth, "sticky column is skipped")
}
