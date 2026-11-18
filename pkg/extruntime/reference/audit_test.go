package reference_test

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	xhtml "golang.org/x/net/html"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
	"github.com/iota-uz/iota-sdk/pkg/markdown"
)

// ---- field and guard leaks ----

// leakyLoyalty adds a hidden field that refers to a native entity the reader
// cannot see, so a leak shows up in the field list, the record's signature,
// schema and example, and in "Refers to".
func leakyLoyalty() catalog.ActiveVersion {
	av := loyalty()
	card := &av.Manifest.Resources[0]
	card.Fields = append(card.Fields, manifest.Field{
		ID: "referrer", Type: manifest.Type{Kind: manifest.KindRef, Entity: "crm.client", Optional: true},
	})
	card.Access.Fields["referrer"] = []string{"Loyalty.Audit"}
	return av
}

func leakyBuilder(t *testing.T) *reference.Builder {
	t.Helper()
	return reference.New(fakeSource{active: []catalog.ActiveVersion{leakyLoyalty()}, natives: nativeSet(t)})
}

func TestReaderWithoutAFieldPermissionLearnsNothingOfTheField(t *testing.T) {
	t.Parallel()
	hidden := []string{"secretNote", "referrer", "Loyalty.Audit"}
	view := func(who spi.Subject) (reference.Item, string, string) {
		m, err := leakyBuilder(t).Model(context.Background(), who, "en")
		require.NoError(t, err)
		raw, err := json.Marshal(m)
		require.NoError(t, err)
		it, ok := m.Lookup("acme.loyalty.card")
		require.True(t, ok)
		body, ok := reference.RenderNamespace(m, "acme.loyalty")
		require.True(t, ok)
		return it, string(raw), string(body)
	}

	item, wire, doc := view(reader("en", "Loyalty.ReadCards"))
	for _, h := range hidden {
		require.NotContains(t, wire, h, "the serialized model of a reader without the field permission")
		require.NotContains(t, doc, h, "the markdown reference")
	}
	require.NotContains(t, doc, "crm.client", "a hidden field's ref target is not announced under \"Refers to\"")
	require.Contains(t, item.Record.Signature, "number")
	require.NotContains(t, item.Record.Signature, "secretNote")
	var example map[string]any
	require.NoError(t, json.Unmarshal(item.Record.Example, &example))
	require.NotContains(t, example, "secretNote")
	require.NotContains(t, example, "referrer")
	for _, f := range item.Resource().Fields {
		require.NotContains(t, []string{"secretNote", "referrer"}, f.ID, "the resource the rest of the reference derives from")
	}
	require.NotContains(t, item.Resource().Access.Fields, "secretNote")

	// A holder of the permission and an administrator see the field everywhere.
	for _, who := range []spi.Subject{reader("en", "Loyalty.ReadCards", "Loyalty.Audit"), admin()} {
		item, wire, doc = view(who)
		for _, h := range hidden {
			require.Contains(t, wire, h)
			require.Contains(t, doc, h)
		}
		require.Contains(t, item.Record.Signature, "secretNote")
		require.Regexp(t, "Refers to: [^\\n]*`crm.client`", doc)
	}
}

func TestGuardsOfOtherPackagesStayWithExtensionsRead(t *testing.T) {
	t.Parallel()
	view := func(who spi.Subject) (string, string) {
		m, err := newBuilder(t).Model(context.Background(), who, "en")
		require.NoError(t, err)
		raw, err := json.Marshal(m)
		require.NoError(t, err)
		body, ok := reference.RenderNamespace(m, "acme.loyalty")
		require.True(t, ok)
		return string(raw), string(body)
	}
	wire, doc := view(reader("en", "Loyalty.Manage"))
	for _, secret := range []string{"awardGuard", "Guards:"} {
		require.NotContains(t, wire, secret)
		require.NotContains(t, doc, secret)
	}
	wire, doc = view(admin())
	require.Contains(t, wire, "awardGuard")
	require.Contains(t, doc, "Guards: `acme.loyalty.awardGuard` (input, `acme.loyalty`)")
}

// ---- package text in markdown ----

const evil = "[x](https://evil.example/a) <img src=x onerror=alert(1)> www.evil.example https://evil.example/b mail@evil.example"

// hostile is a package whose every rendered string is an attack. The publish
// path refuses most of these values (see manifest.Validate); the renderer must
// hold on its own.
func hostile() catalog.ActiveVersion {
	av := loyalty()
	av.Version = "1.0.0 " + evil
	av.Manifest.Resources = append(av.Manifest.Resources,
		manifest.Resource{
			ID: "acme.loyalty.hostile", Kind: manifest.ResQuery, Description: evil, Label: manifest.Label{"en": evil},
			Permission: "A|" + evil, Input: &manifest.Type{Kind: manifest.KindObject}, Output: &manifest.Type{Kind: manifest.KindEnum, Values: []string{evil, "a|b"}},
		},
		manifest.Resource{
			ID: "acme.loyalty.hostileEntity", Kind: manifest.ResEntity,
			Fields: []manifest.Field{{ID: "f|" + evil, Label: manifest.Label{"en": evil}, Type: manifest.Type{Kind: manifest.KindString}, Default: json.RawMessage(`"a|` + evil + `"`)}},
		},
	)
	av.Manifest.Templates = []manifest.Template{{Export: "e", Notice: "acme.loyalty.reminder.v1", Channels: []string{evil, "a|b"}}}
	av.Manifest.Schedules = []manifest.Schedule{{ID: "acme.loyalty.s|1", Export: "e", Cron: "* * * * *|x", Timezone: evil + "|z"}}
	av.Manifest.Guards = []manifest.Guard{{ID: "acme.loyalty.g|1", Export: "e", Target: "acme.loyalty.hostile", Phase: manifest.GuardPhase(evil + "|p")}}
	av.Manifest.Endpoints = []manifest.Endpoint{{ID: "acme.loyalty.hook", Export: "e", Method: evil + "|m", Path: "/a|b/" + evil, Public: true}}
	return av
}

// articleRenderer is the Help Center's markdown renderer: the shared one with
// the class hooks of modules/helpcenter (component.go newArticleRenderer).
func articleRenderer() markdown.Renderer {
	return markdown.NewRendererWithOptions(markdown.WithAllowedClasses(
		regexp.MustCompile(`^help-(home-path|task-(grid|card|meta|title|description|outcome|footer))$`), "a", "div", "span"))
}

func hostileSource(t *testing.T) helpdocs.Source {
	t.Helper()
	admin := admin()
	src, err := reference.NewSource(reference.SourceConfig{
		Builder:      reference.New(fakeSource{active: []catalog.ActiveVersion{hostile()}, natives: nativeSet(t)}),
		Subject:      func(context.Context) (spi.Subject, error) { return admin, nil },
		ManageRoutes: []reference.APIRoute{{Method: "GET|" + evil, Path: "/_ext/manage/x|" + evil, Summary: evil, Permission: "P|" + evil, Alternative: evil}},
		APIServed:    true,
	})
	require.NoError(t, err)
	return src
}

// walk visits every node of the HTML tree.
func walk(n *xhtml.Node, visit func(*xhtml.Node)) {
	visit(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, visit)
	}
}

func textOf(n *xhtml.Node) string {
	var b strings.Builder
	walk(n, func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
		}
	})
	return b.String()
}

func TestPackageStringsRenderAsInertTextInTheHelpCenter(t *testing.T) {
	t.Parallel()
	src := hostileSource(t)
	renderer := articleRenderer()
	for _, docPath := range []string{"reference/index", "reference/acme.loyalty", "api"} {
		d, err := src.Get(context.Background(), "en", docPath)
		require.NoError(t, err, docPath)
		out, err := renderer.Render(d.Markdown)
		require.NoError(t, err)
		root, err := xhtml.Parse(strings.NewReader(string(out)))
		require.NoError(t, err)

		walk(root, func(n *xhtml.Node) {
			if n.Type != xhtml.ElementNode {
				return
			}
			require.NotEqual(t, "img", n.Data, "%s: an <img> from package text", docPath)
			require.NotEqual(t, "script", n.Data, docPath)
			for _, a := range n.Attr {
				require.NotContains(t, strings.ToLower(a.Key), "on", "%s: event handler attribute %s", docPath, a.Key)
				if n.Data == "a" && a.Key == "href" {
					require.NotContains(t, a.Val, "evil", "%s: a link from package text: %s", docPath, a.Val)
					require.NotContains(t, a.Val, "mailto:", docPath)
				}
			}
		})

		// Tables keep their shape: no pipe of a package string adds a cell.
		walk(root, func(n *xhtml.Node) {
			if n.Type != xhtml.ElementNode || n.Data != "table" {
				return
			}
			width := -1
			walk(n, func(r *xhtml.Node) {
				if r.Type != xhtml.ElementNode || r.Data != "tr" {
					return
				}
				cells := 0
				for c := r.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == xhtml.ElementNode && (c.Data == "td" || c.Data == "th") {
						cells++
					}
				}
				if width < 0 {
					width = cells
				}
				require.Equal(t, width, cells, "%s: a table row changed shape: %s", docPath, textOf(r))
			})
		})
	}

	// The attack is visible as text, not dropped: the document says what the
	// package declared.
	d, err := src.Get(context.Background(), "en", "reference/acme.loyalty")
	require.NoError(t, err)
	out, err := renderer.Render(d.Markdown)
	require.NoError(t, err)
	root, err := xhtml.Parse(strings.NewReader(string(out)))
	require.NoError(t, err)
	shown := textOf(root)
	for _, want := range []string{"[x](https://evil.example/a)", "<img src=x onerror=alert(1)>", "www.evil.example", "https://evil.example/b"} {
		require.Contains(t, shown, want)
	}
}

func TestMarkdownTextIsSingleLineAndStartsNoBlock(t *testing.T) {
	t.Parallel()
	av := loyalty()
	av.Manifest.Resources = append(av.Manifest.Resources, manifest.Resource{
		ID: "acme.loyalty.blocks", Kind: manifest.ResQuery, Description: "- not a list\n# not a heading\n\n> not a quote\n1. not an ordered list",
		Input: &manifest.Type{Kind: manifest.KindObject}, Output: &manifest.Type{Kind: manifest.KindObject},
	})
	m, err := reference.New(fakeSource{active: []catalog.ActiveVersion{av}, natives: nativeSet(t)}).Model(context.Background(), admin(), "en")
	require.NoError(t, err)
	body, _ := reference.RenderNamespace(m, "acme.loyalty")
	out, err := articleRenderer().Render(body)
	require.NoError(t, err)
	root, err := xhtml.Parse(strings.NewReader(string(out)))
	require.NoError(t, err)
	walk(root, func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			require.NotContains(t, []string{"ul", "ol", "blockquote"}, n.Data, "package text started a block")
			if n.Data == "h1" || n.Data == "h2" || n.Data == "h3" {
				require.NotContains(t, textOf(n), "not a heading")
			}
		}
	})
}

// ---- the HTTP API documents ----

func TestOpenAPILinkIsRenderedOnlyWhereItIsServed(t *testing.T) {
	t.Parallel()
	build := func(served bool) helpdocs.Source {
		admin := admin()
		src, err := reference.NewSource(reference.SourceConfig{
			Builder: newBuilder(t), Subject: func(context.Context) (spi.Subject, error) { return admin, nil },
			APIServed: served,
		})
		require.NoError(t, err)
		return src
	}
	get := func(src helpdocs.Source, p string) string {
		d, err := src.Get(context.Background(), "en", p)
		require.NoError(t, err)
		return string(d.Markdown)
	}

	served := build(true)
	require.Contains(t, get(served, "reference/index"), reference.OpenAPIPath)
	require.Contains(t, get(served, "api"), reference.OpenAPIPath)

	off := build(false)
	for _, p := range []string{"reference/index", "api"} {
		body := get(off, p)
		require.NotContains(t, body, reference.OpenAPIPath, p)
		require.NotContains(t, body, "OpenAPI 3.1 document", p)
		require.Contains(t, body, "is not served by this deployment", p)
	}
	require.NotContains(t, get(off, "api"), "Calling commands and queries")
	require.Contains(t, get(off, "reference/acme.loyalty"), "acme.loyalty.award", "the reference itself is unaffected")
}

// ---- the Help Center source ----

type failingSource struct{ fakeSource }

func (failingSource) Active(context.Context) ([]catalog.ActiveVersion, error) {
	return nil, errors.New("catalog down")
}

func TestGuidesAreServedWithoutTheCatalog(t *testing.T) {
	t.Parallel()
	a := admin()
	src, err := reference.NewSource(reference.SourceConfig{
		Builder: reference.New(failingSource{fakeSource{natives: nativeSet(t)}}),
		Subject: func(context.Context) (spi.Subject, error) { return a, nil },
	})
	require.NoError(t, err)
	for _, p := range []string{"guides/authoring", "guides/consumer"} {
		d, err := src.Get(context.Background(), "en", p)
		require.NoError(t, err, "%s must not depend on the catalog", p)
		require.NotEmpty(t, d.Markdown)
	}
	_, err = src.Get(context.Background(), "en", "reference/index")
	require.Error(t, err, "the live reference does need it")
	require.NotErrorIs(t, err, helpdocs.ErrNotFound)
}

func TestMountDisappearsForAReaderWithNothingToSee(t *testing.T) {
	t.Parallel()
	nothing := reference.New(fakeSource{natives: catalog.Natives(nil)})
	for _, who := range []spi.Subject{reader("en"), reader("en", "Something.Else")} {
		src, err := reference.NewSource(reference.SourceConfig{
			Builder: nothing, Subject: func(context.Context) (spi.Subject, error) { return who, nil },
		})
		require.NoError(t, err)
		tree, err := src.Tree(context.Background(), "en")
		require.NoError(t, err)
		require.Empty(t, tree)
		for _, p := range []string{"reference/index", "guides/authoring", "api"} {
			_, err = src.Get(context.Background(), "en", p)
			require.ErrorIs(t, err, helpdocs.ErrNotFound, p)
		}
	}

	// An administrator still has the guides and the (empty) reference.
	a := admin()
	src, err := reference.NewSource(reference.SourceConfig{
		Builder: nothing, Subject: func(context.Context) (spi.Subject, error) { return a, nil }, Guides: fstest.MapFS{
			"authoring.md": {Data: []byte("# A\n")}, "consumer.md": {Data: []byte("# C\n")},
		},
	})
	require.NoError(t, err)
	tree, err := src.Tree(context.Background(), "en")
	require.NoError(t, err)
	require.Contains(t, paths(tree), "guides/authoring")
	require.Contains(t, paths(tree), "reference/index")
}

// ---- OpenAPI contracts ----

func TestRPCErrorKindsAreThoseOfThePool(t *testing.T) {
	t.Parallel()
	doc, _ := contractDoc(t, admin())
	kinds := sub(t, doc, "components", "schemas", "RpcError", "properties", "kind", "enum").([]any)
	for _, k := range pool.ErrorKinds() {
		require.Contains(t, kinds, string(k))
	}
	require.Contains(t, kinds, "suspend")
}

func TestRPCAndEndpointsDeclareTheirOwnDefaultErrors(t *testing.T) {
	t.Parallel()
	doc, _ := contractDoc(t, admin())
	rpc := sub(t, doc, "paths", "/_ext/rpc", "post", "responses").(map[string]any)
	require.Equal(t, map[string]any{"$ref": "#/components/schemas/RpcFailure"},
		sub(t, rpc, "default", "content", "application/json", "schema"), "the RPC error envelope, not the management API's")
	hook := sub(t, doc, "paths", "/_ext/endpoints/acme.loyalty/webhooks/stripe", "post", "responses").(map[string]any)
	require.NotContains(t, hook, "default", "a package endpoint's failures are whatever its handler returns")
	require.NotContains(t, sub(t, doc, "components"), "responses")
}

func TestRPCRequestOfAContractWithoutInputTakesNull(t *testing.T) {
	t.Parallel()
	av := loyalty()
	av.Manifest.Resources = append(av.Manifest.Resources, manifest.Resource{
		ID: "acme.loyalty.ping", Kind: manifest.ResQuery, Output: &manifest.Type{Kind: manifest.KindString},
	})
	m, err := reference.New(fakeSource{active: []catalog.ActiveVersion{av}, natives: nativeSet(t)}).Model(context.Background(), admin(), "en")
	require.NoError(t, err)
	b := newOpenAPIBuilder()
	reference.AddContracts(b, m)
	raw, err := b.JSON()
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	sch := compileAt(t, doc, "#/components/schemas/RpcRequest")
	// the handler takes an absent input and an explicit null (ui/rpc.go)
	require.NoError(t, validates(t, sch, `{"kind":"query","resource":"acme.loyalty.ping"}`))
	require.NoError(t, validates(t, sch, `{"kind":"query","resource":"acme.loyalty.ping","input":null}`))
	require.Error(t, validates(t, sch, `{"kind":"query","resource":"acme.loyalty.ping","input":{}}`))
}
