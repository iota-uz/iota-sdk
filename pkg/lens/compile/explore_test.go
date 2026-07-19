package compile

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensspec "github.com/iota-uz/iota-sdk/pkg/lens/spec"
	"github.com/stretchr/testify/require"
)

func TestDocument_CompilesMetricExplorers(t *testing.T) {
	t.Parallel()

	doc := lensspec.Document{
		ID:    "overview",
		Title: lensspec.LiteralText("Overview"),
		Explorers: []lensspec.ExplorerSpec{{
			ID:           "{{ explorer_id }}",
			HostPanelID:  "host",
			ExpandedSpan: 12,
			Branches: []lensspec.ExplorerBranch{{
				Key:                "segment",
				Label:              lensspec.Text{Translations: map[string]string{"en": "Segment", "ru": "Сегмент"}},
				DefaultPerspective: "composition",
				Perspectives: []lensspec.ExplorerPerspective{{
					Key:       "composition",
					Label:     lensspec.LiteralText("Composition"),
					Semantics: string(explore.SemanticsPartition),
					RootNode:  "root",
					Nodes: []lensspec.ExplorerNode{{
						Key:   "root",
						Label: lensspec.LiteralText("Root"),
						Panel: &lensspec.PanelSpec{
							ID:      "detail",
							Title:   lensspec.LiteralText("Detail"),
							Kind:    panel.KindPie,
							Dataset: "data",
							Fields:  lensspec.FieldMappingSpec{Label: "label", Value: "value", ID: "id"},
						},
						Edges: []lensspec.ExplorerEdge{{
							PointKey: "leaf",
							Action:   actionSpecPtr(action.Navigate("{{ portfolio_url }}")),
						}},
					}},
				}},
			}},
		}},
	}

	compiled, err := Document(doc, Options{
		Locale: "ru",
		Values: map[string]any{
			"explorer_id":   "premium",
			"portfolio_url": "/portfolio",
		},
	})
	require.NoError(t, err)
	require.Len(t, compiled.Spec.Explorers, 1)
	require.Equal(t, "premium", compiled.Spec.Explorers[0].ID)
	require.Equal(t, "Сегмент", compiled.Spec.Explorers[0].Branches[0].Label)
	require.Equal(t, "/portfolio", compiled.Spec.Explorers[0].Branches[0].Perspectives[0].Nodes[0].Edges[0].Action.URL)
}

func actionSpecPtr(spec action.Spec) *action.Spec { return &spec }
