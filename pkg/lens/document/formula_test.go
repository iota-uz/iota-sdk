package document

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens/formula"
)

func TestBuildFormula(t *testing.T) {
	require.Nil(t, BuildFormula("p", nil))
	got := BuildFormula("p", &formula.Spec{Latex: `\frac{a}{b}`, Variables: []formula.Variable{{Key: "a", Label: "A", Ref: &formula.Ref{Kind: formula.RefField, Name: "a"}}}})
	require.False(t, got.Error)
	require.Contains(t, got.Markup, "<mfrac>")
	require.NotEmpty(t, got.Alt)
	require.Equal(t, FormulaRefField, got.Variables[0].Ref.Kind)
}

func TestBuildFormulaInvalidDoesNotFail(t *testing.T) {
	got := BuildFormula("p", &formula.Spec{Latex: `\frac{a}`})
	require.True(t, got.Error)
	require.Empty(t, got.Markup)
}
