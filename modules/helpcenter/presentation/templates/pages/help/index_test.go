package help

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/viewmodels"
	"github.com/stretchr/testify/require"
)

func TestCategoryDisplayTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"strips two-digit prefix", "01 CRM", "CRM"},
		{"strips two-digit prefix cyrillic", "02 Страхование", "Страхование"},
		{"keeps text without a numeric prefix", "Без номера", "Без номера"},
		{"keeps a single digit prefix", "1 Один разряд", "1 Один разряд"},
		{"keeps a four digit prefix", "2024 Обновления", "2024 Обновления"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, categoryDisplayTitle(tt.input))
		})
	}
}

func TestCategoryContainsActive(t *testing.T) {
	t.Parallel()

	category := viewmodels.CategoryNode{
		Title: "02 Страхование",
		Children: []viewmodels.CategoryNode{
			{Title: "Руководство", Path: "02-insurance/insurance-handbook.md"},
		},
	}

	require.True(t, categoryContainsActive(category, &viewmodels.DocView{
		Path: "02-insurance/insurance-handbook.md",
	}))
	require.False(t, categoryContainsActive(category, &viewmodels.DocView{
		Path: "01-crm/crm-handbook.md",
	}))
	require.False(t, categoryContainsActive(category, nil))
}
