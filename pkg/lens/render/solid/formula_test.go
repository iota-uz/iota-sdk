package solid

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens/formula"
)

func TestFormulaComponentRendersMarkupAndValues(t *testing.T) {
	var out bytes.Buffer
	err := Formula(FormulaProps{
		Spec:   formula.Spec{Latex: `\frac{\htmlData{var=N}{N}}{E}`, Variables: []formula.Variable{{Key: "N", Label: "Net"}}},
		Label:  "ROE",
		Value:  "12%",
		Values: map[string]string{"N": "300"},
		Mode:   FormulaBlock,
	}).Render(context.Background(), &out)
	require.NoError(t, err)
	html := out.String()
	require.Contains(t, html, "<lens-formula")
	require.Contains(t, html, `mode="block"`)
	require.Contains(t, html, "<mfrac>")
	require.Contains(t, html, "300")
}

func TestFormulaComponentCarriesTheNote(t *testing.T) {
	render := func(note string) string {
		var out bytes.Buffer
		err := Formula(FormulaProps{Spec: formula.Spec{Latex: `a`}, Label: "ROE", Note: note}).Render(context.Background(), &out)
		require.NoError(t, err)
		return out.String()
	}
	require.Contains(t, render("Net of reinsurance."), `note="Net of reinsurance."`)
	require.NotContains(t, render(""), "note=")
}
