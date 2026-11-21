package toast

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
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
			// One variant-agnostic toast template carries the single dismiss button.
			require.Equal(t, 1, strings.Count(buf.String(), tt.want))
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

// The action slot and the politeness split are part of the markup contract
// shared with the Solid toast.
func TestContainer_RendersActionSlotAndLiveRoles(t *testing.T) {
	t.Parallel()

	ctx := composables.WithPageCtx(
		t.Context(),
		types.NewPageContext(language.English, &url.URL{Path: "/"}, nil),
	)
	var buf bytes.Buffer
	require.NoError(t, Container().Render(ctx, &buf))
	html := buf.String()
	for _, want := range []string{
		`data-toast-region`,
		`:role="polite ? &#39;status&#39; : &#39;alert&#39;"`,
		`:aria-live="polite ? &#39;polite&#39; : false"`,
		`data-toast-action`,
		`@click="runAction()"`,
		`:href="action.href"`,
		`Math.max(duration, 8000)`,
		`@focusin=`,
		`@mouseenter=`,
	} {
		require.Contains(t, html, want)
	}
	// The region no longer interrupts: politeness is per toast.
	require.NotContains(t, html, `aria-live="assertive"`)
}

// Regression guard for templ parity: the Solid toast uses the same class
// strings, so a drift on either side shows up here before the VR run.
func TestContainer_ClassesMatchSolidToast(t *testing.T) {
	t.Parallel()

	solid, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "solid-ui", "src", "overlays", "Toast.tsx"))
	require.NoError(t, err)
	source := string(solid)
	for name, class := range map[string]string{
		"surface": surfaceClass,
		"action":  actionClass,
		"close":   closeClass,
	} {
		require.Containsf(t, source, "'"+class+"'", "Solid toast must use the templ %s classes verbatim", name)
	}
	for _, part := range strings.Split(regionClass, " ") {
		require.Containsf(t, source, part, "Solid toast region must use the templ region class %q", part)
	}
}
