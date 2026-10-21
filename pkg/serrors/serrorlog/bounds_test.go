package serrorlog

import (
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAttributesBoundLongTrace(t *testing.T) {
	var err error = serrors.NewInternal("private").WithReason(serrors.Reason(strings.Repeat("界", 200)))
	for range 100 {
		err = serrors.Wrap(serrors.Op(strings.Repeat("界", 200)), err)
	}
	attrs := Attributes(err, strings.Repeat("界", 200))
	for _, attr := range attrs {
		if attr.Key == "error.trace" {
			operations := attr.Value.Any().([]string)
			require.Len(t, operations, maxTraceOperations)
			for _, op := range operations {
				require.LessOrEqual(t, len(op), maxAttributeBytes)
				require.True(t, utf8.ValidString(op))
			}
		} else {
			require.LessOrEqual(t, len(attr.Value.String()), maxAttributeBytes)
			require.True(t, utf8.ValidString(attr.Value.String()))
		}
	}
}
