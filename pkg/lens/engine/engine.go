// Package engine compiles and executes Lens dashboards against runtime inputs.
package engine

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	lenscompile "github.com/iota-uz/iota-sdk/pkg/lens/compile"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	lensspec "github.com/iota-uz/iota-sdk/pkg/lens/spec"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type Request struct {
	Runtime lensruntime.Request
	Scope   lensruntime.Scope
}

type Result struct {
	Compiled  lenscompile.CompiledDocument
	Dashboard *lensruntime.DashboardResult
}

type Executor interface {
	Execute(context.Context, lens.DashboardSpec, lensruntime.Request, lensruntime.Scope) (*lensruntime.DashboardResult, error)
}

type Engine struct{ executor Executor }

func New(executor Executor) *Engine {
	return &Engine{executor: executor}
}

func (e *Engine) Executor() Executor { return e.executor }

func Prepare(doc lensspec.Document, opts lenscompile.Options) (lenscompile.CompiledDocument, error) {
	op := serrors.Op("lens/engine.Prepare")
	compiled, err := lenscompile.Document(doc, opts)
	if err != nil {
		return lenscompile.CompiledDocument{}, serrors.E(op, err)
	}
	return compiled, nil
}

func (e *Engine) RunPrepared(ctx context.Context, compiled lenscompile.CompiledDocument, req Request) (*Result, error) {
	op := serrors.Op("lens/engine.RunPrepared")
	if e == nil || e.executor == nil {
		return nil, serrors.E(op, fmt.Errorf("lens executor is required"))
	}
	result, err := e.executor.Execute(ctx, compiled.Spec, req.Runtime, req.Scope)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return &Result{
		Compiled:  compiled,
		Dashboard: result,
	}, nil
}

func (e *Engine) Run(ctx context.Context, doc lensspec.Document, compileOpts lenscompile.Options, req Request) (*Result, error) {
	op := serrors.Op("lens/engine.Run")
	compiled, err := Prepare(doc, compileOpts)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	result, err := e.RunPrepared(ctx, compiled, req)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return result, nil
}
