package formatters

import (
	"fmt"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbbreviateCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    int64
		expected string
	}{
		{0, "-"},
		{-1, "-"},
		{1, "~1"},
		{5, "~5"},
		{9, "~9"},
		{10, "~10"},
		{54, "~50"},
		{99, "~90"},
		{100, "~100"},
		{500, "~500"},
		{999, "~900"},
		{1000, "~1K"},
		{1234, "~1.2K"},
		{1900, "~1.9K"},
		{5000, "~5K"},
		{9999, "~9.9K"},
		{10000, "~10K"},
		{54000, "~54K"},
		{100000, "~100K"},
		{999999, "~999K"},
		{1000000, "~1M"},
		{1200000, "~1.2M"},
		{1243230, "~1.2M"},
		{9999999, "~9.9M"},
		{10000000, "~10M"},
		{12000000, "~12M"},
		{100000000, "~100M"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input_%d", tt.input), func(t *testing.T) {
			got := abbreviateCount(tt.input)
			assert.Equal(t, tt.expected, got, "abbreviateCount(%d)", tt.input)
		})
	}
}

func TestSchemaDescribeBatchFormatter_RendersHeadersAndSeparator(t *testing.T) {
	t.Parallel()

	f := NewSchemaDescribeBatchFormatter()
	payload := types.SchemaDescribeBatchPayload{
		Tables: []types.SchemaDescribeBatchEntry{
			{
				Requested: "public.users",
				Name:      "users",
				Schema:    "public",
				Columns: []types.SchemaDescribeColumn{
					{Name: "id", Type: "integer", Nullable: false},
					{Name: "email", Type: "text", Nullable: true},
				},
			},
			{
				Requested: "crm.clients",
				Name:      "clients",
				Schema:    "crm",
				Columns: []types.SchemaDescribeColumn{
					{Name: "client_id", Type: "uuid", Nullable: false, Description: "primary key"},
				},
			},
		},
	}

	out, err := f.Format(payload, types.FormatOptions{})
	require.NoError(t, err)

	assert.Contains(t, out, "## public.users")
	assert.Contains(t, out, "| id | integer |")
	assert.Contains(t, out, "## crm.clients")
	assert.Contains(t, out, "| client_id | uuid |")
	assert.Contains(t, out, "primary key")
	assert.Contains(t, out, "\n\n---\n\n", "section separator between entries")
	// Header should never appear before the first entry's "##".
	idxSep := strings.Index(out, "---")
	idxFirst := strings.Index(out, "## public.users")
	assert.Less(t, idxFirst, idxSep, "first header precedes the separator")
}

func TestSchemaDescribeBatchFormatter_RendersPerEntryError(t *testing.T) {
	t.Parallel()

	f := NewSchemaDescribeBatchFormatter()
	payload := types.SchemaDescribeBatchPayload{
		Tables: []types.SchemaDescribeBatchEntry{
			{
				Requested: "good",
				Name:      "good",
				Schema:    "public",
				Columns: []types.SchemaDescribeColumn{
					{Name: "id", Type: "integer"},
				},
			},
			{
				Requested: "bad",
				Error:     "connection reset",
			},
		},
	}

	out, err := f.Format(payload, types.FormatOptions{})
	require.NoError(t, err)

	assert.Contains(t, out, "## good")
	assert.Contains(t, out, "| id | integer |")
	assert.Contains(t, out, "## bad")
	assert.Contains(t, out, "error: connection reset")
}

func TestSchemaDescribeBatchFormatter_FallsBackToSchemaQualifiedName(t *testing.T) {
	t.Parallel()

	f := NewSchemaDescribeBatchFormatter()
	payload := types.SchemaDescribeBatchPayload{
		Tables: []types.SchemaDescribeBatchEntry{
			{
				Name:    "orders",
				Schema:  "sales",
				Columns: []types.SchemaDescribeColumn{{Name: "id", Type: "integer"}},
			},
		},
	}

	out, err := f.Format(payload, types.FormatOptions{})
	require.NoError(t, err)
	assert.Contains(t, out, "## sales.orders")
}

func TestSchemaDescribeBatchFormatter_RejectsWrongPayload(t *testing.T) {
	t.Parallel()

	f := NewSchemaDescribeBatchFormatter()
	_, err := f.Format(types.SchemaDescribePayload{}, types.FormatOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected SchemaDescribeBatchPayload")
}
