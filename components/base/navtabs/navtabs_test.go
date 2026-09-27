package navtabs

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// A base component renders wherever a host places it, including page contexts
// built without a localizer (component tests, non-HTTP renders). A decorative
// a11y label must degrade to its English default there, never take the render
// down with it.
func TestList_LabelsTheTablist(t *testing.T) {
	t.Parallel()

	bundle := i18n.NewBundle(language.Russian)
	bundle.MustAddMessages(language.Russian, &i18n.Message{
		ID: "Common.TabNavigation", Other: "Навигация по вкладкам",
	})

	for _, tt := range []struct {
		name      string
		localizer *i18n.Localizer
		want      string
	}{
		{name: "localized", localizer: i18n.NewLocalizer(bundle, "ru"), want: `aria-label="Навигация по вкладкам"`},
		{name: "no localizer", localizer: nil, want: `aria-label="Tab navigation"`},
		{
			name:      "localizer without the key",
			localizer: i18n.NewLocalizer(i18n.NewBundle(language.English), "en"),
			want:      `aria-label="Tab navigation"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := composables.WithPageCtx(
				t.Context(),
				types.NewPageContext(language.English, &url.URL{Path: "/"}, tt.localizer),
			)

			var buf bytes.Buffer
			require.NoError(t, List("").Render(ctx, &buf))
			require.Contains(t, buf.String(), tt.want)
		})
	}
}
