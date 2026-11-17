package history_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/stretchr/testify/require"
)

func TestLifecycle_LabelsTheResourceAfterItsKeyPrefix(t *testing.T) {
	t.Parallel()
	defs := history.Lifecycle("hrm.contract", "Contracts.History", history.Resource{Name: "hrm.contract"})
	require.Len(t, defs, 3)
	for _, d := range defs {
		require.Equal(t, "Contracts.History.Resource", d.Resources[0].LabelKey)
	}

	own := history.Lifecycle("hrm.contract", "Contracts.History", history.Resource{Name: "hrm.contract", LabelKey: "Contracts.Plural"})
	require.Equal(t, "Contracts.Plural", own[0].Resources[0].LabelKey)
}

func TestRegistry_ResourceLabelComesFromAnyDefinitionThatSetsIt(t *testing.T) {
	t.Parallel()
	registry := history.NewRegistry()
	refOnly := history.Definition{
		Type: "hrm.contract.signed", Version: 1, Category: businessevent.CategoryBusiness,
		TitleKey: "Contracts.History.Signed.Title", DescriptionKey: "Contracts.History.Signed.Summary",
		Resources: []history.Resource{{Name: "hrm.contract"}},
	}
	require.NoError(t, registry.Register(refOnly))
	require.NoError(t, registry.Register(history.Lifecycle("hrm.contract", "Contracts.History", history.Resource{Name: "hrm.contract"})...))

	resources := registry.Resources()
	require.Len(t, resources, 1)
	require.Equal(t, "Contracts.History.Resource", resources[0].LabelKey)
}

func TestRegistry_RejectsTwoLabelsForOneResource(t *testing.T) {
	t.Parallel()
	registry := history.NewRegistry()
	require.NoError(t, registry.Register(history.Lifecycle("hrm.contract", "Contracts.History", history.Resource{Name: "hrm.contract"})...))

	conflicting := history.Definition{
		Type: "hrm.contract.signed", Version: 1, Category: businessevent.CategoryBusiness,
		TitleKey: "Contracts.History.Signed.Title", DescriptionKey: "Contracts.History.Signed.Summary",
		Resources: []history.Resource{{Name: "hrm.contract", LabelKey: "Contracts.Other"}},
	}
	require.Error(t, registry.Register(conflicting))
	_, registered := registry.Get("hrm.contract.signed", 1)
	require.False(t, registered)
}
