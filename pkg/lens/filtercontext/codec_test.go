package filtercontext_test

import (
	"encoding/base64"
	"github.com/iota-uz/iota-sdk/pkg/lens/filtercontext"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
	"testing"
)

// False green: structural JSON acceptance alone would forward unknown fields or unsupported leaf constraints.
func TestContextCodecAndOperationCapabilities(t *testing.T) {
	calendar := filtercontext.ContextCalendar{Timezone: "Asia/Tashkent", AsOf: "2024-02-29T10:00:00Z", DefaultPeriod: filtercontext.ContextPeriod{Mode: "relative", Months: 12}}
	input := filtercontext.ContextInput{Calendar: calendar, SupportedDimensions: []string{"product"}, OperationDimensions: map[string][]string{"drill": {}, "export": {"product"}}, Layers: []filtercontext.ContextLayer{{Origin: filtercontext.ContextOrigin{Scope: "card", ID: "card"}, Filters: filtercontext.ContextFilters{Constraints: []filtercontext.ContextConstraint{{Dimension: "product", Values: []string{"A"}}}}}}}
	effective, err := filtercontext.Resolve(input)
	require.NoError(t, err)
	require.True(t, effective.Ready())
	require.Len(t, effective.UnsupportedOperations["drill"], 1)
	require.Empty(t, effective.UnsupportedOperations["export"])
	require.Equal(t, "2023-03-01", effective.Period.Start)
	require.Equal(t, "2024-02-29", effective.Period.End)
	encoded, err := filtercontext.Encode(input)
	require.NoError(t, err)
	decoded, err := filtercontext.Decode(encoded)
	require.NoError(t, err)
	frozen, err := filtercontext.Resolve(decoded)
	require.NoError(t, err)
	require.Equal(t, effective.Identity, frozen.Identity)
	board := filtercontext.ContextBoard{Calendar: calendar, Dashboard: filtercontext.ContextFilters{Period: &filtercontext.ContextPeriod{Mode: "reports"}}, Cards: map[string]filtercontext.ContextFilters{"card": input.Layers[0].Filters}}
	value, err := filtercontext.EncodeBoard(board)
	require.NoError(t, err)
	roundtrip, err := filtercontext.DecodeBoard(value)
	require.NoError(t, err)
	require.Equal(t, board, roundtrip)
	_, err = filtercontext.DecodeBoard(base64.RawURLEncoding.EncodeToString([]byte(`{"calendar":{},"dashboard":{},"sql":"select 1"}`)))
	require.Error(t, err)
	require.True(t, serrors.HasCode(err, serrors.Invalid))
	// Even an overridden invalid period must not survive inside an immutable context.
	invalid := filtercontext.ContextPeriod{Mode: "fixed", Start: "broken", End: "2026-06-30"}
	input.Layers = append([]filtercontext.ContextLayer{{Origin: filtercontext.ContextOrigin{Scope: "report", ID: "base"}, Filters: filtercontext.ContextFilters{Period: &invalid}}}, input.Layers...)
	input.Layers[1].Filters.Period = &filtercontext.ContextPeriod{Mode: "all"}
	_, err = filtercontext.Resolve(input)
	require.Error(t, err)
	require.True(t, serrors.HasCode(err, serrors.Invalid))
}
