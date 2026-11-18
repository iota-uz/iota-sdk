package listext_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/shellext"
)

func ptr(s string) *string { return &s }

func TestValueWireJSONIsWhatTheBrowserRendererReceives(t *testing.T) {
	cases := []struct {
		name string
		v    listext.Value
		want any
	}{
		{"null", listext.Value{Type: manifest.Type{Kind: manifest.KindInt}}, nil},
		{"int is a number", listext.Value{Type: manifest.Type{Kind: manifest.KindInt}, Raw: ptr("30")}, int64(30)},
		{"bool is a bool", listext.Value{Type: manifest.Type{Kind: manifest.KindBool}, Raw: ptr("true")}, true},
		{"int64 stays exact text", listext.Value{Type: manifest.Type{Kind: manifest.KindInt64}, Raw: ptr("9007199254740993")}, "9007199254740993"},
		{"decimal stays exact text", listext.Value{Type: manifest.Type{Kind: manifest.KindDecimal}, Raw: ptr("12.50")}, "12.50"},
		{"enum is its text", listext.Value{Type: manifest.Type{Kind: manifest.KindEnum}, Raw: ptr("gold")}, "gold"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assert.Equal(t, c.want, c.v.WireJSON()) })
	}
}

func TestViewCellsShareTheRowForRenderersAndKeepTheRendererOfTheColumn(t *testing.T) {
	tier := listext.Column{Contribution: listext.Contribution{ID: "acme.tier", Cell: &shellext.Module{ID: "acme.tierCell"}}, Type: manifest.Type{Kind: manifest.KindEnum}}
	score := listext.Column{Contribution: listext.Contribution{ID: "acme.score"}, Type: manifest.Type{Kind: manifest.KindInt}}
	view := listext.NewView(&listext.Plan{Columns: []listext.Column{tier, score}, Native: "crm.client"}, listext.Request{}, "en")
	require.NotNil(t, view)
	view.SetRow("5", []listext.Value{{Type: tier.Type, Raw: ptr("gold")}, {Type: score.Type, Raw: ptr("30")}})

	cells := view.Cells("5")
	require.Len(t, cells, 2)
	require.NotNil(t, cells[0].Column.Cell)
	assert.Equal(t, "acme.tierCell", cells[0].Column.Cell.ID)
	assert.Nil(t, cells[1].Column.Cell, "a column without a renderer keeps the standard cell")
	assert.Equal(t, map[string]any{
		"ref":        map[string]string{"entity": "crm.client", "key": "5"},
		"acme.tier":  "gold",
		"acme.score": int64(30),
	}, cells[0].Row)

	// A row without values keeps the shape, with nulls.
	empty := view.Cells("missing")
	assert.Nil(t, empty[1].Row["acme.score"])
}
