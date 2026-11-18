package mcptools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pkgdocs"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Scopes a credential may be granted for the extension tools. They narrow RBAC
// and never replace it.
const (
	ScopeRead   = "extensions:read"
	ScopeManage = "extensions:manage"
	ScopeInvoke = "extensions:invoke"
)

// MetaPermissions is the Spec.Meta key listing the RBAC permissions the
// service requires for a tool, for clients that explain refusals.
const MetaPermissions = "iota.permissions"

// Invoker runs a root invocation of a command or query contract; *host.Host
// implements it.
type Invoker interface {
	Invoke(ctx context.Context, req host.Request) (host.Result, error)
}

// Deps are what the tools adapt.
type Deps struct {
	// Manage is the management service. Required.
	Manage *manage.Service
	// Catalog serves the discoverability tools. Default Manage.Catalog().
	Catalog *catalog.Catalog
	// Invoker enables ext_invoke; nil leaves the tool out.
	Invoker Invoker
	// Reference serves ext_reference. Default: built over Catalog.
	Reference *reference.Builder
	// Docs enables ext_docs, the documentation packages ship for their
	// users; nil leaves the tool out.
	Docs *pkgdocs.Library
}

// Tools builds the tool set.
func Tools(d Deps) ([]mcp.Tool, error) {
	const op = serrors.Op("extruntime.mcptools.Tools")
	if d.Manage == nil {
		return nil, serrors.New(serrors.Invalid, "extruntime mcptools: Manage is required").WithOp(op)
	}
	if d.Catalog == nil {
		d.Catalog = d.Manage.Catalog()
	}
	if d.Reference == nil {
		d.Reference = reference.New(d.Catalog)
	}
	r := &registrar{}
	packageTools(r, d)
	lifecycleTools(r, d)
	workflowTools(r, d)
	catalogTools(r, d)
	referenceTool(r, d.Reference)
	if d.Docs != nil {
		docsTool(r, d.Docs)
	}
	if d.Invoker != nil {
		invokeTool(r, d)
	}
	if r.err != nil {
		return nil, serrors.Wrap(op, r.err)
	}
	return r.tools, nil
}

// Register adds the tools to a registry.
func Register(reg *mcp.Registry, d Deps) error {
	if reg == nil {
		return serrors.New(serrors.Invalid, "extruntime mcptools: registry is required").WithOp("extruntime.mcptools.Register")
	}
	tools, err := Tools(d)
	if err != nil {
		return err
	}
	return reg.Register(tools...)
}

// Extension attaches the tools to a runtime; add it to runtime.Config.Extensions.
type Extension struct {
	Registry *mcp.Registry
}

var _ runtime.Extension = Extension{}

func (Extension) Name() string { return "extruntime-mcp" }

// Attach implements runtime.Extension.
func (e Extension) Attach(_ context.Context, rt *runtime.Runtime) (func(context.Context) error, error) {
	d := Deps{Manage: rt.Manage, Catalog: rt.Catalog, Reference: rt.Reference, Docs: rt.Docs}
	if rt.Host != nil {
		d.Invoker = rt.Host
	}
	return nil, Register(e.Registry, d)
}

// registrar collects tools and the first construction error.
type registrar struct {
	tools []mcp.Tool
	err   error
}

func (r *registrar) add(t mcp.Tool, err error) {
	if err != nil {
		r.err = errors.Join(r.err, err)
		return
	}
	r.tools = append(r.tools, t)
}

func readPolicy(perm string) mcp.Policy {
	return mcp.Policy{Scopes: []string{ScopeRead}, Permissions: []string{perm}}
}

// managePolicy leaves the permission to the service (see the package comment).
func managePolicy() mcp.Policy { return mcp.Policy{Scopes: []string{ScopeManage}} }

func meta(perms ...string) map[string]any {
	return map[string]any{MetaPermissions: perms}
}

func describe(summary string, perms ...string) string {
	if len(perms) == 0 {
		return summary
	}
	return fmt.Sprintf("%s Requires %s.", summary, strings.Join(perms, " and "))
}
