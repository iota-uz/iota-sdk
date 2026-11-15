package acceptance_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

var day = time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)

func newDocument(opts ...acceptance.Option) acceptance.Document {
	return acceptance.New(uuid.New(), acceptance.KindAct, "A-1", day, money.New(40000, "USD"), opts...)
}

func TestNew_SigningDaySignsTheDocument(t *testing.T) {
	t.Parallel()

	require.Equal(t, acceptance.StatusDraft, newDocument().Status())
	require.True(t, newDocument().SignedOn().IsZero())

	signed := newDocument(acceptance.WithSignedOn(day.AddDate(0, 0, 3)))
	require.Equal(t, acceptance.StatusSigned, signed.Status())
	require.Equal(t, day.AddDate(0, 0, 3), signed.SignedOn())
}

func TestNew_SignedWithoutDayFallsBackToDocumentDate(t *testing.T) {
	t.Parallel()

	legacy := newDocument(acceptance.WithStatus(acceptance.StatusSigned))
	require.Equal(t, day, legacy.SignedOn())
}

func TestUpdateStatus_SigningSetsTodayOnce(t *testing.T) {
	t.Parallel()

	signed := newDocument().UpdateStatus(acceptance.StatusSigned)
	require.False(t, signed.SignedOn().IsZero())

	earlier := newDocument(acceptance.WithSignedOn(day)).UpdateStatus(acceptance.StatusSigned)
	require.Equal(t, day, earlier.SignedOn(), "an existing signing day is kept")

	cancelled := signed.UpdateStatus(acceptance.StatusCancelled)
	require.Equal(t, signed.SignedOn(), cancelled.SignedOn(), "a cancelled document remembers when it was signed")
}

func TestRevise_SigningDayDecidesStatus(t *testing.T) {
	t.Parallel()
	revision := acceptance.Revision{
		Kind: acceptance.KindProtocol, Number: "P-2", Date: day, Amount: money.New(50000, "EUR"), Description: "Phase two",
	}

	draft := newDocument().Revise(revision)
	require.Equal(t, acceptance.StatusDraft, draft.Status())
	require.Equal(t, acceptance.KindProtocol, draft.Kind())
	require.Equal(t, "P-2", draft.Number())
	require.Equal(t, int64(50000), draft.Amount().Amount())
	require.Equal(t, "Phase two", draft.Description())

	revision.SignedOn = day
	signed := draft.Revise(revision)
	require.Equal(t, acceptance.StatusSigned, signed.Status())

	revision.SignedOn = time.Time{}
	require.Equal(t, acceptance.StatusDraft, signed.Revise(revision).Status(), "clearing the day takes the amount out of revenue")
}

func TestRevise_CancelledStaysCancelled(t *testing.T) {
	t.Parallel()

	cancelled := newDocument().UpdateStatus(acceptance.StatusCancelled)
	revised := cancelled.Revise(acceptance.Revision{Kind: acceptance.KindAct, Number: "A-9", Date: day, SignedOn: day, Amount: money.New(1, "USD")})
	require.Equal(t, acceptance.StatusCancelled, revised.Status())
}

func TestValidKind(t *testing.T) {
	t.Parallel()

	for _, kind := range acceptance.DefaultKinds() {
		require.True(t, acceptance.ValidKind(kind.Value), kind.Value)
	}
	require.True(t, acceptance.ValidKind("COMPLETION_V2"))
	for _, kind := range []acceptance.Kind{"", "act", "1ACT", "ACT-1", "ACT X", "A_VERY_LONG_KIND_NAME_THAT_DOES_NOT_FIT"} {
		require.False(t, acceptance.ValidKind(kind), kind)
	}
}
