package engine

import (
	"context"

	lenscompile "github.com/iota-uz/iota-sdk/pkg/lens/compile"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	lensspec "github.com/iota-uz/iota-sdk/pkg/lens/spec"
)

type Request struct {
	Runtime lensruntime.Request
	Scope   lensruntime.Scope
}

type Result struct {
	Compiled  lenscompile.CompiledDocument
	Dashboard *lensruntime.DashboardResult
}

func Prepare(doc lensspec.Document, opts lenscompile.Options) (lenscompile.CompiledDocument, error) {
	return lenscompile.Document(doc, opts)
}

func RunPrepared(ctx context.Context, compiled lenscompile.CompiledDocument, req Request) (*Result, error) {
	runtimeReq := req.Runtime
	if runtimeReq.Cache == nil {
		runtimeReq.Cache = lensruntime.NewMemoryCache()
	}
	result, err := lensruntime.RunScope(ctx, compiled.Spec, runtimeReq, req.Scope)
	if err != nil {
		return nil, err
	}
	return &Result{
		Compiled:  compiled,
		Dashboard: result,
	}, nil
}

func Run(ctx context.Context, doc lensspec.Document, compileOpts lenscompile.Options, req Request) (*Result, error) {
	compiled, err := Prepare(doc, compileOpts)
	if err != nil {
		return nil, err
	}
	return RunPrepared(ctx, compiled, req)
}
