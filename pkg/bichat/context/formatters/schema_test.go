package formatters

import (
	"testing"
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
		got := abbreviateCount(tt.input)
		if got != tt.expected {
			t.Errorf("abbreviateCount(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
