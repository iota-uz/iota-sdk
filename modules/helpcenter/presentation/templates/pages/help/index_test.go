package help

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/helpcenter/presentation/viewmodels"
	"github.com/stretchr/testify/require"
)

func TestCategoryDisplayTitle(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"01 CRM":          "CRM",
		"02 Страхование":  "Страхование",
		"Без номера":      "Без номера",
		"1 Один разряд":   "1 Один разряд",
		"2024 Обновления": "2024 Обновления",
	}
	for input, expected := range tests {
		require.Equal(t, expected, categoryDisplayTitle(input))
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
