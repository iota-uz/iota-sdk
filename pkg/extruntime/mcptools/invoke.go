package mcptools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type invokeIn struct {
	Resource string          `json:"resource" jsonschema:"ID of a command or query; see ext_resource_get for its input schema"`
	Input    json.RawMessage `json:"input,omitempty" jsonschema:"input matching the resource's input_schema"`
}

type invokeOut struct {
	InvocationID string          `json:"invocation_id"`
	Output       json.RawMessage `json:"output"`
	Package      string          `json:"package" jsonschema:"package whose implementation served the call"`
	Version      string          `json:"version"`
	ContractHash string          `json:"contract_hash,omitempty"`
}

func invokeTool(r *registrar, d Deps) {
	cat, invoker := d.Catalog, d.Invoker

	add(r, mcp.Spec{
		Name: "ext_invoke",
		Description: "Call an active command or query as YOU: the host runs it with your current rights, exactly like the " +
			"same call from the UI, and records an invocation with trigger mcp. The resource's own permission, input " +
			"validation and guards apply; nothing here widens them. Operations are not callable. A command may change data.",
		Annotations: mcp.Annotations{Title: "Invoke command or query", OpenWorld: true},
		Policy:      mcp.Policy{Scopes: []string{ScopeInvoke}}, Meta: meta(),
	}, func(ctx context.Context, actor spi.Subject, in invokeIn) (invokeOut, error) {
		rr, ok, err := cat.Resolve(ctx, in.Resource)
		if err != nil {
			return invokeOut{}, err
		}
		if !ok {
			return invokeOut{}, serrors.New(serrors.NotFound, "resource "+in.Resource+" not found")
		}
		if !invocable(rr.Resource) {
			return invokeOut{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "only commands and queries can be invoked",
				mcp.Issue{Path: "resource", Code: "kind", Message: in.Resource + " is a " + string(rr.Resource.Kind)})
		}
		input := in.Input
		if len(input) == 0 {
			input = json.RawMessage("null")
		}
		res, err := invoker.Invoke(ctx, host.Request{
			Contract: in.Resource, Input: input, Subject: actor,
			Trigger: host.Trigger{Kind: host.TriggerMCP, Ref: actor.Credential + " ext_invoke " + mcp.RequestIDFrom(ctx)},
		})
		if err != nil {
			return invokeOut{InvocationID: res.InvocationID}, invokeFailure(err, res.InvocationID)
		}
		out := res.Output
		if len(out) == 0 {
			out = json.RawMessage("null")
		}
		return invokeOut{
			InvocationID: res.InvocationID, Output: out, Package: res.Binding.Package, Version: res.Binding.Version,
			ContractHash: res.ContractHash,
		}, nil
	})
}

// invokeFailure maps what the host and the package code report onto the MCP
// contract without losing the error code, params or issues.
func invokeFailure(err error, invocation string) error {
	var we *pool.WireError
	switch {
	case errors.As(err, &we):
		issues := make([]mcp.Issue, 0, len(we.Issues))
		for _, is := range we.Issues {
			msg := is.Source
			if len(is.Params) > 0 {
				msg = string(is.Params)
			}
			issues = append(issues, mcp.Issue{Path: string(is.Path), Code: is.Code, Message: msg})
		}
		switch we.Kind {
		case pool.ErrKindDenied:
			return mcp.Fail(mcp.CodeForbidden, mcp.RecoveryRequestAccess, orDefault(we.Message, "permission denied"), issues...).WithOperation(invocation)
		case pool.ErrKindValidation:
			return mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, orDefault(we.Message, "input rejected"), issues...).WithOperation(invocation)
		case pool.ErrKindDomain:
			msg := we.Code
			if len(we.Params) > 0 {
				msg += " " + string(we.Params)
			}
			return mcp.Fail(mcp.CodeConflict, mcp.RecoveryReadThenPrepare, msg, issues...).WithOperation(invocation)
		case pool.ErrKindInternal, pool.ErrKindSuspend:
		}
		return mcp.Fail(mcp.CodeDependencyUnavailable, mcp.RecoveryRetryLater, "the implementation failed").WithOperation(invocation)
	case errors.Is(err, host.ErrNoBinding):
		return mcp.Fail(mcp.CodeConflict, mcp.RecoveryNone, "no active implementation")
	}
	return err
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
