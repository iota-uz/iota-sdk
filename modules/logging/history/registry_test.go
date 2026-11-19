package history_test

import (
	"context"
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

func TestRegistry_ValueTranslationOnlyForCodes(t *testing.T) {
	t.Parallel()
	define := func(f history.Field) history.Definition {
		return history.Definition{
			Type: "hrm.contract.signed", Version: 1, Category: businessevent.CategoryBusiness,
			TitleKey: "Contracts.History.Signed.Title", DescriptionKey: "Contracts.History.Signed.Summary",
			Resources: []history.Resource{{Name: "hrm.contract", Fields: []history.Field{f}}},
		}
	}
	require.NoError(t, history.NewRegistry().Register(define(history.Field{Name: "status", LabelKey: "Contracts.Status", Type: history.ValueString, ValueKeyPrefix: "Contracts.Statuses"})))
	require.NoError(t, history.NewRegistry().Register(define(history.Field{Name: "grade", LabelKey: "Contracts.Grade", Type: history.ValueInteger,
		ValueLabel: func(context.Context, string) string { return "" }})))
	require.Error(t, history.NewRegistry().Register(define(history.Field{Name: "amount", LabelKey: "Contracts.Amount", Type: history.ValueMoney, ValueKeyPrefix: "Contracts.Amounts"})))
}

func TestWithResource_ConfiguresOneResourceOfEveryDefinition(t *testing.T) {
	t.Parallel()
	labels := func(context.Context, []string) (map[string]string, error) { return nil, nil }
	visible := func(context.Context) (history.Visibility, error) { return history.VisibleIDs("1"), nil }
	defs := history.Lifecycle("hrm.contract", "Contracts.History", history.Resource{Name: "hrm.contract"})
	configured := history.WithResource(defs, "hrm.contract", func(r *history.Resource) {
		r.Labels, r.Visible = labels, visible
	})
	for i := range defs {
		require.Nil(t, defs[i].Resources[0].Labels, "the input is not modified")
		require.NotNil(t, configured[i].Resources[0].Labels)
	}

	registry := history.NewRegistry()
	require.NoError(t, registry.Register(configured...))
	resources := registry.Resources()
	require.Len(t, resources, 1)
	require.NotNil(t, resources[0].Labels)
	v, err := resources[0].Visible(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"1"}, v.IDs)
}
