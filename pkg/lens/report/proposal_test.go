package report_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens/report"
)

// False green: validating only the identity would accept a proposal whose rows break the composition rules.
func TestProposalValidation(t *testing.T) {
	t.Parallel()
	composition := report.Composition{ID: "board", Revision: "2", Title: "Board", Rows: [][]report.Placement{{{ID: "a", ReportID: "revenue"}}}}
	require.NoError(t, report.Proposal{ID: "p1", Base: "2:4", Composition: composition}.Validate())
	require.Error(t, report.Proposal{ID: "p1", Composition: composition}.Validate())
	require.Error(t, report.Proposal{ID: "bad id", Base: "2:4", Composition: composition}.Validate())
	composition.Rows[0][0].Span = 13
	require.Error(t, report.Proposal{ID: "p1", Base: "2:4", Composition: composition}.Validate())
}
