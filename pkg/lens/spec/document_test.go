package spec

import (
	"encoding/json"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

func TestLoadRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	_, err := Load([]byte(`{
		"id": "sales-report",
		"title": "Sales report",
		"unknownField": true
	}`))
	require.Error(t, err)
	require.ErrorContains(t, err, "lens.spec.Load")
	require.ErrorContains(t, err, "unknown field")
}

func TestLoadParsesVariableComponentOverride(t *testing.T) {
	t.Parallel()

	doc, err := Load([]byte(`{
		"id": "sales-report",
		"title": "Sales report",
		"variables": [
			{
				"name": "product",
				"label": "Product",
				"kind": "single_select",
				"component": "text_input"
			}
		]
	}`))
	require.NoError(t, err)
	require.Len(t, doc.Variables, 1)
	require.Equal(t, string(lens.VariableComponentTextInput), doc.Variables[0].Component)
}

func TestRowSpecMarshal_OmitsEmptyHeading(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(RowSpec{ //nolint:musttag // RowSpec is the canonical Lens JSON payload under test.
		Panels: []PanelSpec{
			{
				ID:   "total",
				Kind: panel.KindStat,
			},
		},
	})
	require.NoError(t, err)
	require.NotContains(t, string(payload), `"heading"`)

	payload, err = json.Marshal(RowSpec{ //nolint:musttag // RowSpec is the canonical Lens JSON payload under test.
		Heading: LiteralText("Summary"),
	})
	require.NoError(t, err)
	require.Contains(t, string(payload), `"heading":"Summary"`)
}

func TestPanelSpecMarshal_UsesDrillTreeJSONContract(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(PanelSpec{ //nolint:musttag // PanelSpec is the canonical Lens JSON payload under test.
		ID:   "premium",
		Kind: panel.KindPie,
		DrillTree: &panel.DrillTree{ExpandedSpan: 12, Branches: []panel.DrillBranch{{
			TriggerKey: "earned",
			Label:      "Earned premium",
			View: &panel.DrillLevelView{
				LegendPosition: panel.LegendRight,
				LegendWidthPx:  300,
			},
			Children: []panel.DrillNode{{Key: "year:2026", Label: "2026", Value: 100}},
		}}},
	})
	require.NoError(t, err)
	require.Contains(t, string(payload), `"drillTree":{"branches":[{"triggerKey":"earned","label":"Earned premium","view":{"legendPosition":"right","legendWidthPx":300},"children":[{"key":"year:2026","label":"2026","value":100}]}]`) //nolint:lll // exact public JSON contract
	require.Contains(t, string(payload), `"expandedSpan":12`)
	require.NotContains(t, string(payload), `"Branches"`)
}

func TestDocumentValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		doc     Document
		wantErr string
	}{
		{
			name: "rejects blank translated title",
			doc: Document{
				ID:    "sales-report",
				Title: Text{Translations: map[string]string{"en": " ", "ru": "\t"}},
			},
			wantErr: "document title is required",
		},
		{
			name: "rejects invalid body position",
			doc: Document{
				ID:           "sales-report",
				Title:        LiteralText("Sales"),
				BodyPosition: BodyPosition("preprend"),
			},
			wantErr: `unsupported bodyPosition "preprend"`,
		},
		{
			name: "accepts supported body position",
			doc: Document{
				ID:           "sales-report",
				Title:        LiteralText("Sales"),
				BodyPosition: BodyPositionPrepend,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.doc.Validate()
			if testCase.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.EqualError(t, err, testCase.wantErr)
		})
	}
}
