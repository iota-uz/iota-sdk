package document

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func syntheticMapDocument() *DashboardDocument {
	doc := testDocument()
	doc.Panels[0] = Panel{
		ID: "regions", Kind: PanelKindMap, Title: "Regions", Semantics: SemanticsSeries,
		Frame: "panel:regions", Encoding: Encoding{ID: "region_code", Label: "region_name", Value: "premium"},
		Format: map[string]FieldFormat{}, Actions: []Action{}, Terminal: true,
		Map: &MapConfig{
			Source: GeoJSONSource{Inline: &GeoJSONFeatureCollection{
				Type: "FeatureCollection",
				Features: []GeoJSONFeature{
					{Type: "Feature", Properties: map[string]any{"code": "north", "name": "North", "name_en": "North", "name_ru": "Север"}, Geometry: map[string]any{"type": "Polygon", "coordinates": []any{[]any{}}}},
					{Type: "Feature", Properties: map[string]any{"code": "south", "name": "South", "name_en": "South", "name_ru": "Юг"}, Geometry: map[string]any{"type": "MultiPolygon", "coordinates": []any{[]any{}}}},
				},
			}},
			FeatureProperty: "code", LabelProperty: "name", LabelProperties: map[string]string{"en": "name_en", "ru": "name_ru"},
		},
	}
	doc.Layout.Rows[0].Panels[0].PanelID = "regions"
	doc.Frames = map[FrameRef]Frame{
		"panel:regions": {
			Columns: []Column{{Name: "region_code", Type: ColumnString}, {Name: "region_name", Type: ColumnString}, {Name: "premium", Type: ColumnNumber}},
			Rows:    [][]any{{"north", "North", 42.0}, {"south", "South", 18.0}},
		},
	}
	return doc
}

func TestDashboardDocumentValidate_MapContract(t *testing.T) {
	t.Parallel()
	require.NoError(t, syntheticMapDocument().Validate())

	t.Run("requires config", func(t *testing.T) {
		doc := syntheticMapDocument()
		doc.Panels[0].Map = nil
		require.ErrorContains(t, doc.Validate(), "map kind requires map config")
	})
	t.Run("requires exactly one source", func(t *testing.T) {
		doc := syntheticMapDocument()
		doc.Panels[0].Map.Source.URL = "/maps/synthetic.geojson"
		doc.Panels[0].Map.Source.MaxBytes = 1024
		require.ErrorContains(t, doc.Validate(), "exactly one of inline or url")
	})
	t.Run("rejects cross-origin and unbounded urls", func(t *testing.T) {
		doc := syntheticMapDocument()
		doc.Panels[0].Map.Source = GeoJSONSource{URL: "https://example.com/map.geojson", MaxBytes: 1024}
		require.ErrorContains(t, doc.Validate(), "same-origin absolute path")

		doc.Panels[0].Map.Source = GeoJSONSource{URL: "/maps/synthetic.geojson"}
		require.ErrorContains(t, doc.Validate(), "maxBytes must be between")
	})
	t.Run("rejects unsupported geometry", func(t *testing.T) {
		doc := syntheticMapDocument()
		doc.Panels[0].Map.Source.Inline.Features[0].Geometry["type"] = "Point"
		require.ErrorContains(t, doc.Validate(), "geometry must be Polygon or MultiPolygon")
	})
	t.Run("bounds localized label mapping", func(t *testing.T) {
		doc := syntheticMapDocument()
		doc.Panels[0].Map.LabelProperties["../ru"] = "name_ru"
		require.ErrorContains(t, doc.Validate(), "invalid locale")

		doc = syntheticMapDocument()
		doc.Panels[0].Map.LabelProperties["ru"] = strings.Repeat("x", 121)
		require.ErrorContains(t, doc.Validate(), "must contain 1 to 120 characters")

		doc = syntheticMapDocument()
		delete(doc.Panels[0].Map.Source.Inline.Features[0].Properties, "name_ru")
		require.ErrorContains(t, doc.Validate(), "label property \"name_ru\" must be a non-empty string")
	})
	t.Run("requires unique feature and frame keys", func(t *testing.T) {
		doc := syntheticMapDocument()
		doc.Panels[0].Map.Source.Inline.Features[1].Properties["code"] = "north"
		require.ErrorContains(t, doc.Validate(), "duplicate GeoJSON feature key")

		doc = syntheticMapDocument()
		doc.Frames["panel:regions"] = Frame{
			Columns: doc.Frames["panel:regions"].Columns,
			Rows:    [][]any{{"north", "North", 42.0}, {"north", "North again", 18.0}},
		}
		require.ErrorContains(t, doc.Validate(), "duplicate region key")
	})
	t.Run("requires every data key in inline geometry", func(t *testing.T) {
		doc := syntheticMapDocument()
		frame := doc.Frames["panel:regions"]
		frame.Rows[1][0] = "west"
		doc.Frames["panel:regions"] = frame
		require.ErrorContains(t, doc.Validate(), "has no GeoJSON feature")
	})
	t.Run("bounds inline geometry", func(t *testing.T) {
		doc := syntheticMapDocument()
		doc.Panels[0].Map.Source.Inline.Features[0].Properties["payload"] = strings.Repeat("x", MaxMapGeoJSONBytes)
		require.ErrorContains(t, doc.Validate(), "inline map source cannot exceed")
	})
}
