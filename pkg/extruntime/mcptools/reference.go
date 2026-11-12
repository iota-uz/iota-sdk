package mcptools

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type referenceIn struct {
	Namespace string `json:"namespace,omitempty" jsonschema:"namespace to read, for example acme.loyalty or core; omit for the overview of every namespace you may read"`
	Locale    string `json:"locale,omitempty" jsonschema:"locale of the labels, for example ru; default is the user's"`
}

type referenceOut struct {
	Namespace string `json:"namespace,omitempty"`
	// Markdown is the reference document: the same text the Help Center shows.
	Markdown   string   `json:"markdown"`
	Namespaces []string `json:"namespaces" jsonschema:"every namespace the caller may read"`
}

// referenceTool serves the live API reference as markdown, filtered to what
// the caller may see. It needs a read scope and a signed-in user; the
// permissions decide the content, not the tool, so a user without
// Extensions.Read gets the contracts open to them.
func referenceTool(r *registrar, ref *reference.Builder) {
	add(r, mcp.Spec{
		Name: "ext_reference",
		Description: "Read the live API reference of everything installed (native contracts and activated packages) as markdown: " +
			"entities, commands, queries, events, workflows, endpoints with their JSON Schemas and examples. " +
			"Shows only what your permissions allow: a holder of " + reference.PermRead + " sees all. Omit the namespace for the overview.",
		Annotations: mcp.Annotations{Title: "API reference", ReadOnly: true, Idempotent: true},
		Policy:      mcp.Policy{Scopes: []string{ScopeRead}},
	}, func(ctx context.Context, actor spi.Subject, in referenceIn) (referenceOut, error) {
		m, err := ref.Model(ctx, actor, in.Locale)
		if err != nil {
			return referenceOut{}, err
		}
		out := referenceOut{Namespaces: []string{}}
		for _, ns := range m.Namespaces {
			out.Namespaces = append(out.Namespaces, ns.Name)
		}
		if in.Namespace == "" {
			out.Markdown = string(reference.RenderIndex(m, false))
			return out, nil
		}
		body, ok := reference.RenderNamespace(m, in.Namespace)
		if !ok {
			// A namespace the caller may not read is indistinguishable from an absent one.
			return referenceOut{}, serrors.New(serrors.NotFound, "namespace "+in.Namespace+" not found")
		}
		out.Namespace, out.Markdown = in.Namespace, string(body)
		return out, nil
	})
}
