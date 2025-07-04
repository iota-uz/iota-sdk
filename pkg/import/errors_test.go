package importpkg

import (
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestErrorFormatting(t *testing.T) {
	factory := NewDefaultErrorFactory()

	t.Run("InvalidCellError_ExcelFormat", func(t *testing.T) {
		// Test with Excel-style column letter
		err := factory.NewInvalidCellError("C", 2)

		cellErr, ok := err.(*InvalidCellError)
		require.True(t, ok)
		assert.Equal(t, "C", cellErr.Col)
		assert.Equal(t, uint(2), cellErr.Row)
		assert.Equal(t, "ERR_INVALID_CELL", cellErr.Code)
	})

	t.Run("ValidationError_ExcelFormat", func(t *testing.T) {
		// Test with Excel-style column letter
		err := factory.NewValidationError("D", "abc", 5, "Must be a valid number")

		valErr, ok := err.(*ValidationError)
		require.True(t, ok)
		assert.Equal(t, "D", valErr.Col)
		assert.Equal(t, "abc", valErr.Value)
		assert.Equal(t, uint(5), valErr.RowNum)
		assert.Equal(t, "Must be a valid number", valErr.Message)
		assert.Equal(t, "ERR_VALIDATION", valErr.Code)
	})

	t.Run("Localization_ExcelFormat", func(t *testing.T) {
		// Create a mock localizer with test messages
		bundle := i18n.NewBundle(language.English)
		if err := bundle.AddMessages(language.English, &i18n.Message{
			ID:    "ERR_INVALID_CELL",
			Other: "{{.Col}}:{{.Row}} - This field is required and cannot be empty",
		}, &i18n.Message{
			ID:    "Error.ERR_VALIDATION",
			Other: "{{.Col}}:{{.RowNum}} - {{.Message}} (found value: '{{.Value}}')",
		}); err != nil {
			t.Fatal(err)
		}

		localizer := i18n.NewLocalizer(bundle, "en")

		// Test InvalidCellError localization
		cellErr := factory.NewInvalidCellError("C", 2).(*InvalidCellError)
		localized := cellErr.Localize(localizer)
		assert.Equal(t, "C:2 - This field is required and cannot be empty", localized)

		// Test ValidationError localization
		valErr := factory.NewValidationError("D", "abc", 5, "Must be a valid number").(*ValidationError)
		localized = valErr.Localize(localizer)
		assert.Equal(t, "D:5 - Must be a valid number (found value: 'abc')", localized)
	})
}
