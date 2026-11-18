package mcptools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/pkgdocs"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/helpdocs"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type docsIn struct {
	Namespace string `json:"namespace,omitempty" jsonschema:"package namespace, for example acme.loyalty; omit to list the documents of every package you may read"`
	Path      string `json:"path,omitempty" jsonschema:"document path within the package, for example getting-started; omit to list the package's documents"`
	Locale    string `json:"locale,omitempty" jsonschema:"locale of the document, for example ru; falls back to en; default is the user's"`
}

type docsOut struct {
	Namespace string `json:"namespace,omitempty"`
	Path      string `json:"path,omitempty"`
	Title     string `json:"title,omitempty"`
	// Markdown is the document, or the index of the documents you may read.
	Markdown string            `json:"markdown"`
	Packages []pkgdocs.Listing `json:"packages" jsonschema:"the documents you may read, per package"`
}

// docsTool serves the end-user documentation the packages ship: the same
// documents, with the same visibility, as the Help Center's "apps" area. The
// permissions decide the content, not the tool: a user sees the documents of
// the packages they may use.
func docsTool(r *registrar, lib *pkgdocs.Library) {
	add(r, mcp.Spec{
		Name: "ext_docs",
		Description: "Read the user documentation that installed extension packages ship, as markdown (the Help Center's Applications area). " +
			"Omit namespace to list the documents of every package you may read; give namespace to list one package's; give namespace and path to read a document. " +
			"Shows only what your permissions allow.",
		Annotations: mcp.Annotations{Title: "Package documentation", ReadOnly: true, Idempotent: true},
		Policy:      mcp.Policy{Scopes: []string{ScopeRead}},
	}, func(ctx context.Context, actor spi.Subject, in docsIn) (docsOut, error) {
		locale := in.Locale
		if locale == "" {
			locale = actor.Locale
		}
		listings, err := lib.List(ctx, actor, locale)
		if err != nil {
			return docsOut{}, err
		}
		out := docsOut{Packages: listings}
		if out.Packages == nil {
			out.Packages = []pkgdocs.Listing{}
		}
		if in.Namespace != "" && in.Path != "" {
			page, err := lib.Read(ctx, actor, locale, in.Namespace, in.Path)
			if errors.Is(err, helpdocs.ErrNotFound) {
				// A document the caller may not read is indistinguishable from an absent one.
				return docsOut{}, serrors.New(serrors.NotFound, "document "+in.Namespace+"/"+in.Path+" not found")
			}
			if err != nil {
				return docsOut{}, err
			}
			out.Namespace, out.Path, out.Title, out.Markdown = in.Namespace, page.Path, page.Title, string(page.Markdown)
			return out, nil
		}
		if in.Path != "" {
			return docsOut{}, serrors.New(serrors.Invalid, "path needs a namespace")
		}
		if in.Namespace != "" {
			kept := out.Packages[:0:0]
			for _, l := range out.Packages {
				if l.Namespace == in.Namespace {
					kept = append(kept, l)
				}
			}
			if len(kept) == 0 {
				return docsOut{}, serrors.New(serrors.NotFound, "no documents for "+in.Namespace)
			}
			out.Namespace, out.Packages = in.Namespace, kept
		}
		out.Markdown = indexMarkdown(out.Packages)
		return out, nil
	})
}

func indexMarkdown(listings []pkgdocs.Listing) string {
	if len(listings) == 0 {
		return "No package documentation is available to you.\n"
	}
	var b strings.Builder
	for _, l := range listings {
		fmt.Fprintf(&b, "## %s %s\n\n", l.Namespace, l.Version)
		for _, d := range l.Docs {
			fmt.Fprintf(&b, "- `%s` — %s\n", d.Path, d.Title)
		}
		b.WriteString("\n")
	}
	return b.String()
}
