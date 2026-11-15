package toast

import (
	"bytes"
	"net/url"
	"strings"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// Same contract as navtabs: the dismiss label is localized when it can be, and
// falls back to English rather than panicking when the page context carries no
// localizer.
func TestContainer_LabelsEveryDismissButton(t *testing.T) {
	t.Parallel()

	bundle := i18n.NewBundle(language.Russian)
	bundle.MustAddMessages(language.Russian, &i18n.Message{
		ID: "Common.CloseNotification", Other: "Закрыть уведомление",
	})

	for _, tt := range []struct {
		name      string
		localizer *i18n.Localizer
		want      string
	}{
		{name: "localized", localizer: i18n.NewLocalizer(bundle, "ru"), want: `aria-label="Закрыть уведомление"`},
		{name: "no localizer", localizer: nil, want: `aria-label="Close notification"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := composables.WithPageCtx(
				t.Context(),
				types.NewPageContext(language.English, &url.URL{Path: "/"}, tt.localizer),
			)

			var buf bytes.Buffer
			require.NoError(t, Container().Render(ctx, &buf))
			// One per toast variant: success, error, warning, info.
			require.Equal(t, 4, strings.Count(buf.String(), tt.want))
		})
	}
}

// The global HTMX error listener reads its texts from the container, so the
// container must always carry them, even without a localizer.
func TestContainer_ExposesErrorTextsForHTMXFailures(t *testing.T) {
	t.Parallel()

	ctx := composables.WithPageCtx(
		t.Context(),
		types.NewPageContext(language.English, &url.URL{Path: "/"}, nil),
	)
	var buf bytes.Buffer
	require.NoError(t, Container().Render(ctx, &buf))
	html := buf.String()
	for _, want := range []string{
		`data-toast-container`,
		`data-error-load-failed="Could not load data"`,
		`data-error-save-failed="Could not save changes"`,
		`data-error-network="No connection to the server"`,
	} {
		require.Contains(t, html, want)
	}
}
