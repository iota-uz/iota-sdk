package reference_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
)

func newSource(t *testing.T, who *spi.Subject) helpdocs.Source {
	t.Helper()
	src, err := reference.NewSource(reference.SourceConfig{
		Builder: newBuilder(t),
		Subject: func(context.Context) (spi.Subject, error) {
			if who == nil {
				return spi.Subject{}, errors.New("no user")
			}
			return *who, nil
		},
		ManageRoutes: []reference.APIRoute{{Method: "GET", Path: "/_ext/manage/packages", Summary: "List packages", Permission: reference.PermRead}},
		Ref:          "v9.9.9",
	})
	require.NoError(t, err)
	return src
}

func paths(nodes []helpdocs.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Path)
		out = append(out, paths(n.Children)...)
	}
	return out
}

func TestSourceMountAndTree(t *testing.T) {
	t.Parallel()
	a := admin()
	src := newSource(t, &a)
	require.Equal(t, "extensions", src.Mount())
	tree, err := src.Tree(context.Background(), "en")
	require.NoError(t, err)
	require.Equal(t, []string{
		"guides", "guides/authoring", "guides/consumer",
		"reference", "reference/index", "reference/acme.loyalty", "reference/core", "reference/crm",
		"api",
	}, paths(tree))

	o := reader("en")
	tree, err = newSource(t, &o).Tree(context.Background(), "en")
	require.NoError(t, err)
	require.Equal(t, []string{
		"guides", "guides/authoring", "guides/consumer",
		"reference", "reference/index", "reference/acme.loyalty", "reference/crm",
		"api",
	}, paths(tree), "namespaces the reader cannot see are not listed")
}

func TestSourceHidesEverythingFromAnUnknownReader(t *testing.T) {
	t.Parallel()
	src := newSource(t, nil)
	tree, err := src.Tree(context.Background(), "en")
	require.NoError(t, err)
	require.Empty(t, tree)
	_, err = src.Get(context.Background(), "en", "guides/authoring")
	require.ErrorIs(t, err, helpdocs.ErrNotFound)
}

func TestSourceGetServesGuidesReferenceAndAPI(t *testing.T) {
	t.Parallel()
	a := admin()
	src := newSource(t, &a)
	get := func(p string) helpdocs.Document {
		d, err := src.Get(context.Background(), "en", p)
		require.NoError(t, err, p)
		require.Equal(t, "Developers", d.Audience)
		require.Equal(t, p, d.Path)
		require.NotEmpty(t, d.Title)
		return d
	}

	authoring := string(get("guides/authoring").Markdown)
	require.Contains(t, authoring, "](../guides/consumer)", "links between guides are document links")
	require.Contains(t, authoring, "(https://github.com/iota-uz/iota-sdk/blob/v9.9.9/docs/extension-runtime.md)")
	require.Contains(t, authoring, "(https://github.com/iota-uz/iota-sdk/tree/v9.9.9/docs/extensions/example)")
	consumer := string(get("guides/consumer").Markdown)
	require.Contains(t, consumer, "](../guides/authoring)")
	require.Equal(t, "Writing extension packages", get("guides/authoring").Title)

	index := string(get("reference/index").Markdown)
	require.Contains(t, index, "](../reference/acme.loyalty)")
	require.Contains(t, index, "](../api")
	require.Contains(t, index, "/_ext/openapi.json")

	ns := string(get("reference/acme.loyalty").Markdown)
	for _, want := range []string{
		"# `acme.loyalty`", "active version **1.2.0**", "## Commands", "### `acme.loyalty.award` — Award points",
		"permission `Loyalty.Manage`", "{ card: ref<acme.loyalty.card>, points: int }", "JSON Schema:", `"additionalProperties": false`,
		"[`acme.loyalty.notRegistered`](../reference/acme.loyalty)", "Refers to:", "## Endpoints", "`/_ext/endpoints/acme.loyalty/webhooks/stripe`",
		"public (runs as the trigger owner)", "## Schedules", "0 3 * * *", "## Subscriptions", "## Guards", "Template channels: email, sms.",
		"| `number` | `string(16)` | yes | Number; unique |",
	} {
		require.Contains(t, ns, want)
	}
	for _, forbidden := range []string{"STRIPE_SECRET_KEY", "stripe.com", "SecretExport", "reminderTemplates"} {
		require.NotContains(t, ns, forbidden)
	}

	crm := string(get("reference/crm").Markdown)
	require.Contains(t, crm, "Extended by: [`acme.loyalty.clientExtra`](../reference/acme.loyalty).")
	require.Contains(t, crm, "| `lastName` | `string(255) \\| null` |", "pipes in a type stay inside the table cell")

	api := string(get("api").Markdown)
	for _, want := range []string{"/_ext/openapi.json", "same-origin", "`acme.loyalty.award`", "## Package endpoints", "## Management API", "/_ext/manage/packages"} {
		require.Contains(t, api, want)
	}
	require.NotContains(t, api, "](../", "the API document is at the top level")
}

func TestSourceGetRefusesWhatTheReaderCannotSee(t *testing.T) {
	t.Parallel()
	o := reader("en")
	src := newSource(t, &o)
	_, err := src.Get(context.Background(), "en", "reference/core")
	require.ErrorIs(t, err, helpdocs.ErrNotFound)
	_, err = src.Get(context.Background(), "en", "reference/nope")
	require.ErrorIs(t, err, helpdocs.ErrNotFound)
	_, err = src.Get(context.Background(), "en", "guides/nope")
	require.ErrorIs(t, err, helpdocs.ErrNotFound)

	d, err := src.Get(context.Background(), "en", "reference/acme.loyalty")
	require.NoError(t, err)
	body := string(d.Markdown)
	require.NotContains(t, body, "acme.loyalty.award`", "gated by Loyalty.Manage")
	require.NotContains(t, body, "### `acme.loyalty.card`", "its own read rule hides the entity itself")
	require.NotContains(t, body, "## Schedules")
	require.Contains(t, body, "acme.loyalty.tiers")

	api, err := src.Get(context.Background(), "en", "api")
	require.NoError(t, err)
	require.NotContains(t, string(api.Markdown), "Management API", "the management API is described to Extensions.Read holders only")
}

func TestPackageTextCannotInjectMarkdownOrHTML(t *testing.T) {
	t.Parallel()
	got := string(renderEvil(t))
	require.NotContains(t, got, "<script>")
	require.NotContains(t, got, "[click](javascript:")
	require.Contains(t, got, "&lt;script&gt;")
}

func TestRewriteGuideLinks(t *testing.T) {
	t.Parallel()
	in := strings.Join([]string{
		"See [consumer](consumer.md#setup) and [`runtime`](../extension-runtime.md).",
		"Inline `[not](a-link.md)` stays; so does [abs](https://example.com/a.md) and [anchor](#x).",
		"```",
		"[in fence](other.md)",
		"```",
		"[example](example) and [site](/help)",
		"[ref]: ../x.md",
	}, "\n")
	got := string(reference.RewriteGuideLinks([]byte(in), "main"))
	require.Equal(t, strings.Join([]string{
		"See [consumer](../guides/consumer#setup) and [`runtime`](https://github.com/iota-uz/iota-sdk/blob/main/docs/extension-runtime.md).",
		"Inline `[not](a-link.md)` stays; so does [abs](https://example.com/a.md) and [anchor](#x).",
		"```",
		"[in fence](other.md)",
		"```",
		"[example](https://github.com/iota-uz/iota-sdk/tree/main/docs/extensions/example) and [site](/help)",
		"[ref]: https://github.com/iota-uz/iota-sdk/blob/main/docs/x.md",
	}, "\n"), got)
}

func TestSourceRefIsTheReleaseTag(t *testing.T) {
	t.Parallel()
	require.Regexp(t, `^v\d+\.\d+\.\d+`, reference.SourceRef())
}

func TestNewSourceNeedsItsGuides(t *testing.T) {
	t.Parallel()
	_, err := reference.NewSource(reference.SourceConfig{Builder: newBuilder(t), Subject: func(context.Context) (spi.Subject, error) { return spi.Subject{}, nil }, Guides: fstest.MapFS{}})
	require.Error(t, err)
	_, err = reference.NewSource(reference.SourceConfig{})
	require.Error(t, err)
}
