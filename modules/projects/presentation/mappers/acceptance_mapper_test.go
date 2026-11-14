package mappers_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/mappers"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/stretchr/testify/require"
)

// Opening a document and saving an unrelated field must not change its amount,
// even when the amount has more digits than a float64 holds exactly.
func TestAcceptanceForm_AmountRoundTripIsExact(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)

	for _, minor := range []int64{12345678901234567, 9007199254740993, 150050, 1} {
		document := acceptance.New(uuid.New(), acceptance.KindAct, "A-1", day, money.New(minor, "USD"))

		form := mappers.AcceptanceDocumentToForm(document)
		form.Description = "an unrelated change"
		// The browser posts the amount as text; shared.ParseAmount is what the form decoder uses.
		form.Amount = shared.ParseAmount(form.Amount.Decimal().String())

		revised := form.ToRevision().Amount
		require.Equal(t, minor, revised.Amount(), "minor units %d", minor)
		require.Equal(t, "USD", revised.Currency().Code)
	}
}
