package jsonspec

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/stretchr/testify/require"
)

func TestTextResolvesStringAndLocaleMap(t *testing.T) {
	t.Parallel()

	var single Text
	require.NoError(t, single.UnmarshalJSON([]byte(`"Sales Overview"`)))
	require.Equal(t, "Sales Overview", single.Resolve("ru"))

	var translated Text
	require.NoError(t, translated.UnmarshalJSON([]byte(`{"en":"Sales Overview","ru":"Обзор продаж"}`)))
	require.Equal(t, "Обзор продаж", translated.Resolve("ru"))
	require.Equal(t, "Sales Overview", translated.Resolve("uz"))
}

func TestDurationParsesString(t *testing.T) {
	t.Parallel()

	var d Duration
	require.NoError(t, d.UnmarshalJSON([]byte(`"168h"`)))
	require.Equal(t, 168*time.Hour, d.Std())
}

func TestLoadCubeResolvesLocaleRefsAndTemplates(t *testing.T) {
	t.Parallel()

	spec, err := LoadCube([]byte(`{
		"version": 1,
		"id": "finance-overview",
		"title": {"en":"Finance Overview","ru":"Финансы"},
		"description": "Tenant {{tenantLabel}}",
		"dataMode": "sql",
		"dataSource": "primary",
		"fromSQL": "finance.transactions t",
		"params": {
			"tenant_id": {"Literal": {"$ref": "tenant_id"}},
			"locale": {"Literal": {"$ref": "locale"}}
		},
		"where": [
			"(t.tenant_id = @tenant_id OR t.tenant_id IS NULL)"
		],
		"joins": [
			{"name":"accounts","sql":"LEFT JOIN finance.accounts a ON a.id = t.account_id"}
		],
		"dimensions": [
			{
				"name":"account",
				"label":{"en":"Account","ru":"Счёт"},
				"column":"COALESCE(a.id::text, '')",
				"labelColumn":"COALESCE(a.name, '{{unknownLabel}}')",
				"panelKind":"bar",
				"requiresJoin":["accounts"]
			}
		],
		"measures": [
			{
				"name":"total_amount",
				"label":{"en":"Revenue","ru":"Выручка"},
				"column":"COALESCE(t.amount, 0)::float8",
				"aggregation":"sum",
				"accentColor":"#10b981",
				"action":{
					"kind":"navigate",
					"method":"GET",
					"url":"{{basePath}}/transactions",
					"preserveQuery":true,
					"params":[
						{"name":"tenant_id","source":{"kind":"literal","value":{"$ref":"tenant_id"}}}
					]
				}
			}
		],
		"defaultDimension":"account",
		"leaf":{"url":"{{basePath}}/transactions"}
	}`), ResolveOptions{
		Locale: "ru",
		Values: map[string]any{
			"tenant_id":    "tenant-123",
			"tenantLabel":  "ACME",
			"unknownLabel": "Неизвестно",
			"basePath":     "/finance",
			"locale":       "ru",
		},
	})
	require.NoError(t, err)

	require.Equal(t, cube.DataModeSQL, spec.DataMode)
	require.Equal(t, "Финансы", spec.Title)
	require.Equal(t, "Tenant ACME", spec.Description)
	require.Equal(t, "tenant-123", spec.Params["tenant_id"].Literal)
	require.Equal(t, "ru", spec.Params["locale"].Literal)
	require.Equal(t, "/finance/transactions", spec.Leaf.URL)
	require.Equal(t, "Счёт", spec.Dimensions[0].Label)
	require.Contains(t, spec.Dimensions[0].LabelColumn, "Неизвестно")
	require.NotNil(t, spec.Measures[0].Action)
	require.Equal(t, "/finance/transactions", spec.Measures[0].Action.URL)
	require.Equal(t, "tenant-123", spec.Measures[0].Action.Params[0].Source.Value)
}

func TestLoadCubeSupportsDatasetRef(t *testing.T) {
	t.Parallel()

	frameSet, err := frame.FromRows("crm_policies", frame.Row{
		"policy_id": "pol-1",
		"value":     1,
	})
	require.NoError(t, err)

	spec, err := LoadCube([]byte(`{
		"version": 1,
		"id": "crm-sales-report",
		"title": "CRM Sales",
		"dataMode": "dataset",
		"dataRef": "base_dataset",
		"dimensions": [
			{
				"name":"product",
				"label":"Product",
				"field":"policy_id"
			}
		],
		"measures": [
			{
				"name":"total_policies",
				"label":"Policies",
				"aggregation":"count"
			}
		]
	}`), ResolveOptions{
		Values: map[string]any{
			"base_dataset": frameSet,
		},
	})
	require.NoError(t, err)
	require.Equal(t, cube.DataModeDataset, spec.DataMode)
	require.Same(t, frameSet, spec.Data)
}

func TestLoadCubeResolvesVariableDefaults(t *testing.T) {
	t.Parallel()

	spec, err := LoadCube([]byte(`{
		"version": 1,
		"id": "date-filtered",
		"title": "Date Filtered",
		"dataMode": "sql",
		"dataSource": "primary",
		"fromSQL": "finance.transactions t",
		"variables": [
			{
				"name":"range",
				"label":"Range",
				"kind":"date_range",
				"requestKeys":["range","range_start","range_end"],
				"default":{"mode":"default"},
				"defaultDuration":"168h",
				"allowAllTime":true
			}
		],
		"dimensions":[{"name":"account","label":"Account","column":"t.id::text"}],
		"measures":[{"name":"total","label":"Total","column":"1","aggregation":"count"}]
	}`), ResolveOptions{})
	require.NoError(t, err)
	require.Len(t, spec.Variables, 1)
	require.Equal(t, lens.VariableDateRange, spec.Variables[0].Kind)
	require.Equal(t, 168*time.Hour, spec.Variables[0].DefaultDuration)
}
