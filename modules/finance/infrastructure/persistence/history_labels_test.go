package persistence_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/stretchr/testify/require"
)

func TestCounterpartyHistoryLabels_CaptionsKnownUUIDsOnly(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	created, err := persistence.NewCounterpartyRepository().Create(f.Ctx, counterparty.New(
		"Caption LLC", counterparty.Customer, counterparty.LLC, counterparty.WithTenantID(f.TenantID()),
	))
	require.NoError(t, err)

	labels, err := persistence.CounterpartyHistoryLabels(f.Ctx, []string{created.ID().String(), uuid.NewString(), "not-a-uuid"})
	require.NoError(t, err)
	require.Equal(t, map[string]string{created.ID().String(): "Caption LLC"}, labels)

	labels, err = persistence.CounterpartyHistoryLabels(f.Ctx, []string{"not-a-uuid"})
	require.NoError(t, err)
	require.Empty(t, labels)
}
