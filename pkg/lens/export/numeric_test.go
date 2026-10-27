package export

import (
	"context"
	"strconv"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// False green if exact decimals become text or the shared encoder rejects an
// established approximate Lens ratio: inspect both writer paths' saved cells.
func TestLensExactNumericAndApproximateRatioParity(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		t.Run(strconv.FormatBool(streamed), func(t *testing.T) {
			t.Parallel()
			req := streamingRequest(t)
			values := []any{decimal.RequireFromString("1.234"), float64(1) / 3, frame.Formula{Expression: "A2*2", Result: decimal.RequireFromString("2.468")}, nil, "0002714"}
			fields := []frame.Field{{Name: "exact"}, {Name: "ratio"}, {Name: "formula"}, {Name: "blank"}, {Name: "identifier"}}
			if streamed {
				req.EvidenceSources = map[string]EvidenceSource{"evidence": {Fields: fields, Rows: func(ctx context.Context, yield func([]any) error) error { return yield(values) }}}
			} else {
				for i := range fields {
					fields[i].Values = []any{values[i]}
				}
				req.Result.Datasets["evidence"] = &runtime.DatasetResult{Frames: mustFrames(t, "evidence", fields...)}
			}
			prepared, err := New().Prepare(context.Background(), req)
			require.NoError(t, err)
			defer func() { _ = prepared.Close() }()
			book, err := excelize.OpenFile(prepared.file.Name())
			require.NoError(t, err)
			defer func() { _ = book.Close() }()
			for coordinate, want := range map[string]string{"A2": "1.234", "D2": "", "E2": "0002714"} {
				got, err := book.GetCellValue("Evidence", coordinate)
				require.NoError(t, err)
				require.Equal(t, want, got)
			}
			typ, err := book.GetCellType("Evidence", "A2")
			require.NoError(t, err)
			require.Contains(t, []excelize.CellType{excelize.CellTypeNumber, excelize.CellTypeUnset}, typ)
			raw, err := book.GetCellValue("Evidence", "B2", excelize.Options{RawCellValue: true})
			require.NoError(t, err)
			ratio, err := strconv.ParseFloat(raw, 64)
			require.NoError(t, err)
			require.InEpsilon(t, float64(1)/3, ratio, 1e-15)
			result, err := book.CalcCellValue("Evidence", "C2")
			require.NoError(t, err)
			require.Equal(t, "2.468", result)
		})
	}
}
