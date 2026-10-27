package resource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBudgetReplaceManyIsAtomicAndDeduplicates(t *testing.T) {
	t.Parallel()
	budget := NewBudget(100)
	one, err := budget.Reserve(30)
	require.NoError(t, err)
	two, err := budget.Reserve(40)
	require.NoError(t, err)
	_, err = budget.ReplaceMany([]*Reservation{one, two}, 101)
	require.ErrorIs(t, err, ErrLimit)
	require.EqualValues(t, 70, budget.Used())
	replacement, err := budget.ReplaceMany([]*Reservation{one, two, one}, 90)
	require.NoError(t, err)
	require.EqualValues(t, 90, budget.Used())
	one.Release()
	two.Release()
	require.EqualValues(t, 90, budget.Used())
	replacement.Release()
	require.Zero(t, budget.Used())
}

func TestBudgetReplaceManyRejectsForeignAndRetiredReservations(t *testing.T) {
	t.Parallel()
	budget := NewBudget(100)
	one, err := budget.Reserve(30)
	require.NoError(t, err)
	foreign, err := NewBudget(100).Reserve(20)
	require.NoError(t, err)
	_, err = budget.ReplaceMany([]*Reservation{one, foreign}, 40)
	require.Error(t, err)
	require.EqualValues(t, 30, budget.Used())
	one.Release()
	_, err = budget.ReplaceMany([]*Reservation{one}, 10)
	require.Error(t, err)
	require.Zero(t, budget.Used())
	foreign.Release()
}

func TestLimitStatusIsIndependentOfResourceSpelling(t *testing.T) {
	t.Parallel()
	status, ok := StatusCode(NewAdmissionError("renamed queue", 4, 3))
	require.True(t, ok)
	require.Equal(t, 503, status)
	status, ok = StatusCode(NewLimitError("report_build_queue", 4, 3))
	require.True(t, ok)
	require.Equal(t, 413, status)
}
