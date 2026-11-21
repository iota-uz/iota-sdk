package base

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func renderProgress(t *testing.T, props ProgressProps) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, Progress(props).Render(t.Context(), &buf))
	return buf.String()
}

// Without a name the bar keeps its earlier, purely visual markup: an unnamed
// role="progressbar" would be an accessibility violation for existing callers.
func TestProgress_WithoutLabelIsPresentational(t *testing.T) {
	t.Parallel()

	html := renderProgress(t, ProgressProps{Value: 3, Target: 10})
	require.NotContains(t, html, `role="progressbar"`)
	require.NotContains(t, html, `aria-value`)
}

// The accessible value follows the visible fill: an overfilled bar is
// clamped to 100 % and a zero target is an empty bar, never a division by zero.
func TestProgress_LabelledValueMatchesTheFill(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		props ProgressProps
		now   string
		text  string
		width string
	}{
		{name: "partial", props: ProgressProps{Value: 68, Target: 100, Label: "Collected premiums"}, now: `aria-valuenow="68"`, text: `aria-valuetext="68 / 100"`, width: "width: 68.000000%"},
		{name: "overfilled", props: ProgressProps{Value: 120, Target: 100, Label: "Seats"}, now: `aria-valuenow="100"`, text: `aria-valuetext="120 / 100"`, width: "width: 100.000000%"},
		{name: "zero target", props: ProgressProps{Value: 0, Target: 0, Label: "Import"}, now: `aria-valuenow="0"`, text: `aria-valuetext="0 / 0"`, width: "width: 0.000000%"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			html := renderProgress(t, tt.props)
			require.Contains(t, html, `role="progressbar"`)
			require.Contains(t, html, `aria-label="`+tt.props.Label+`"`)
			require.Contains(t, html, `aria-valuemin="0"`)
			require.Contains(t, html, `aria-valuemax="100"`)
			require.Contains(t, html, tt.now)
			require.Contains(t, html, tt.text)
			require.Contains(t, html, tt.width)
		})
	}
}
