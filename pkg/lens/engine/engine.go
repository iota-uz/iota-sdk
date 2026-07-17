// Package engine compiles and executes Lens dashboards against runtime inputs.
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

type Engine struct{ runtime *lensruntime.Runtime }

func New(runtime *lensruntime.Runtime) *Engine {
	if runtime == nil {
		runtime = lensruntime.New(lensruntime.Options{})
	}
	return &Engine{runtime: runtime}
}

func (e *Engine) Runtime() *lensruntime.Runtime { return e.runtime }

func Prepare(doc lensspec.Document, opts lenscompile.Options) (lenscompile.CompiledDocument, error) {
	return lenscompile.Document(doc, opts)
}

func (e *Engine) RunPrepared(ctx context.Context, compiled lenscompile.CompiledDocument, req Request) (*Result, error) {
	result, err := e.runtime.Execute(ctx, compiled.Spec, req.Runtime, req.Scope)
	if err != nil {
		return nil, err
	}
	return &Result{
		Compiled:  compiled,
		Dashboard: result,
	}, nil
}

func (e *Engine) Run(ctx context.Context, doc lensspec.Document, compileOpts lenscompile.Options, req Request) (*Result, error) {
	compiled, err := Prepare(doc, compileOpts)
	if err != nil {
		return nil, err
	}
	return e.RunPrepared(ctx, compiled, req)
}
