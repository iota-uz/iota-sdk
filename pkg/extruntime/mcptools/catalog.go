package mcptools

import (
	"context"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type resourceListIn struct {
	Kind    string `json:"kind,omitempty" jsonschema:"entity, overlay, query, command, operation, event, workflow, notice, route, page and so on"`
	Package string `json:"package,omitempty" jsonschema:"only resources of this package namespace; use native for Go-defined core resources"`
	Query   string `json:"query,omitempty" jsonschema:"case-insensitive text matched against ID, label and description"`
	Limit   int    `json:"limit,omitempty"`
}

type resourceSummary struct {
	ID           string            `json:"id"`
	Kind         string            `json:"kind"`
	Package      string            `json:"package,omitempty" jsonschema:"owning package; empty for native resources"`
	Version      string            `json:"version,omitempty"`
	Native       bool              `json:"native"`
	Label        map[string]string `json:"label,omitempty"`
	Description  string            `json:"description,omitempty"`
	Permission   string            `json:"permission,omitempty"`
	Atomic       bool              `json:"atomic,omitempty"`
	ContractHash string            `json:"contract_hash,omitempty"`
}

type resourceList struct {
	Resources []resourceSummary `json:"resources"`
	// Truncated is set when Limit cut the list.
	Truncated bool `json:"truncated"`
}

type resourceGetIn struct {
	ID string `json:"id" jsonschema:"resource ID, for example acme.insurance.renewPolicy"`
}

type resourceDoc struct {
	ID           string            `json:"id"`
	Kind         string            `json:"kind"`
	Package      string            `json:"package,omitempty"`
	Version      string            `json:"version,omitempty"`
	Native       bool              `json:"native"`
	Label        map[string]string `json:"label,omitempty"`
	Description  string            `json:"description,omitempty"`
	Permission   string            `json:"permission,omitempty" jsonschema:"RBAC permission the host requires to call it"`
	Atomic       bool              `json:"atomic,omitempty"`
	ContractHash string            `json:"contract_hash,omitempty"`
	Resource     manifest.Resource `json:"resource" jsonschema:"the declared contract exactly as published"`
	// Invocable says whether ext_invoke may call it.
	Invocable     bool           `json:"invocable"`
	InputSchema   map[string]any `json:"input_schema,omitempty" jsonschema:"JSON Schema of the input (query, command, operation)"`
	OutputSchema  map[string]any `json:"output_schema,omitempty"`
	ParamsSchema  map[string]any `json:"params_schema,omitempty" jsonschema:"JSON Schema of route params, domain error params or notice params"`
	PayloadSchema map[string]any `json:"payload_schema,omitempty" jsonschema:"JSON Schema of an event payload"`
	RowSchema     map[string]any `json:"row_schema,omitempty" jsonschema:"JSON Schema of a row of an entity or overlay"`
	Signals       map[string]any `json:"signals,omitempty" jsonschema:"JSON Schema per signal name of a workflow"`
}

func summaryOf(rr spi.ResolvedResource) resourceSummary {
	r := rr.Resource
	return resourceSummary{
		ID: r.ID, Kind: string(r.Kind), Package: rr.Package, Version: rr.Version, Native: rr.Native,
		Label: r.Label, Description: r.Description, Permission: r.Permission, Atomic: r.Atomic, ContractHash: r.ContractHash,
	}
}

// resources lists the active catalog: native contracts and the resources of
// every activated package version.
func resources(ctx context.Context, cat *catalog.Catalog) ([]spi.ResolvedResource, error) {
	natives := cat.Natives().Resources()
	active, err := cat.Active(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]spi.ResolvedResource, 0, len(natives))
	for _, r := range natives {
		out = append(out, spi.ResolvedResource{Resource: r, Native: true})
	}
	for _, av := range active {
		for _, r := range av.Manifest.Resources {
			if r.ContractHash == "" {
				r.ContractHash = manifest.ContractHashOf(r)
			}
			out = append(out, spi.ResolvedResource{Resource: r, Package: av.Namespace, Version: av.Version})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Resource.ID < out[j].Resource.ID })
	return out, nil
}

func matches(rr spi.ResolvedResource, in resourceListIn) bool {
	r := rr.Resource
	if in.Kind != "" && string(r.Kind) != in.Kind {
		return false
	}
	switch in.Package {
	case "":
	case "native":
		if !rr.Native {
			return false
		}
	default:
		if rr.Package != in.Package {
			return false
		}
	}
	if in.Query == "" {
		return true
	}
	q := strings.ToLower(in.Query)
	if strings.Contains(strings.ToLower(r.ID), q) || strings.Contains(strings.ToLower(r.Description), q) {
		return true
	}
	for _, l := range r.Label {
		if strings.Contains(strings.ToLower(l), q) {
			return true
		}
	}
	return false
}

// invocable reports whether ext_invoke may call the resource: queries and
// commands, like the browser; operations are internal building blocks.
func invocable(r manifest.Resource) bool {
	return r.Kind == manifest.ResQuery || r.Kind == manifest.ResCommand
}

func docOf(rr spi.ResolvedResource) resourceDoc {
	r := rr.Resource
	sum := summaryOf(rr)
	doc := resourceDoc{
		ID: sum.ID, Kind: sum.Kind, Package: sum.Package, Version: sum.Version, Native: sum.Native, Label: sum.Label,
		Description: sum.Description, Permission: sum.Permission, Atomic: sum.Atomic, ContractHash: sum.ContractHash,
		Resource: r, Invocable: invocable(r),
	}
	if r.Input != nil {
		doc.InputSchema = SchemaOfType(*r.Input)
	}
	if r.Output != nil {
		doc.OutputSchema = SchemaOfType(*r.Output)
	}
	if r.Params != nil {
		doc.ParamsSchema = SchemaOfType(*r.Params)
	}
	if r.Payload != nil {
		doc.PayloadSchema = SchemaOfType(*r.Payload)
	}
	if r.Kind == manifest.ResEntity || r.Kind == manifest.ResOverlay {
		doc.RowSchema = rowSchema(r)
	}
	if len(r.Signals) > 0 {
		doc.Signals = map[string]any{}
		for _, name := range sortedKeys(r.Signals) {
			doc.Signals[name] = SchemaOfType(r.Signals[name])
		}
	}
	return doc
}

func catalogTools(r *registrar, d Deps) {
	cat := d.Catalog

	add(r, mcp.Spec{
		Name: "ext_resource_list",
		Description: describe("Discover the contracts of the active catalog: entities, commands, queries, operations, events, "+
			"workflows, notices and routes, native (core) and from activated packages, with descriptions. "+
			"Drafts are not in the catalog until activated; use ext_build for a draft's manifest.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "List resources", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in resourceListIn) (resourceList, error) {
		all, err := resources(ctx, cat)
		if err != nil {
			return resourceList{}, err
		}
		limit := in.Limit
		if limit <= 0 || limit > 500 {
			limit = 200
		}
		out := resourceList{Resources: []resourceSummary{}}
		for _, rr := range all {
			if !matches(rr, in) {
				continue
			}
			if len(out.Resources) == limit {
				out.Truncated = true
				break
			}
			out.Resources = append(out.Resources, summaryOf(rr))
		}
		return out, nil
	})

	add(r, mcp.Spec{
		Name: "ext_resource_get",
		Description: describe("Read one contract with its documentation and JSON Schemas of its input, output, params, payload "+
			"and rows, as callers must send them. Authoring code imports the same contract from the developer kit "+
			"(@iota-kit/<namespace>).", manage.PermRead),
		Annotations: mcp.Annotations{Title: "Get resource", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in resourceGetIn) (resourceDoc, error) {
		rr, ok, err := cat.Resolve(ctx, in.ID)
		if err != nil {
			return resourceDoc{}, err
		}
		if !ok {
			return resourceDoc{}, serrors.New(serrors.NotFound, "resource "+in.ID+" not found")
		}
		if rr.Resource.ContractHash == "" {
			rr.Resource.ContractHash = manifest.ContractHashOf(rr.Resource)
		}
		return docOf(rr), nil
	})
}
