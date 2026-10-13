package report

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/filtercontext"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Catalog is a code-authored host. Stored/user-authored definitions can use the same Host boundary.
type Catalog struct {
	Definitions []Definition
	Sources     []Source
}

func (c Catalog) Report(_ context.Context, id string) (Definition, error) {
	for _, definition := range c.Definitions {
		if definition.ID == id {
			return clone(definition), nil
		}
	}
	return Definition{}, serrors.New(serrors.NotFound, "report is not registered").WithOp("lens.report.Catalog")
}

func (c Catalog) Source(_ context.Context, id string) (Source, error) {
	for _, source := range c.Sources {
		if source.Descriptor().ID == id {
			return source, nil
		}
	}
	return nil, serrors.New(serrors.NotFound, "source is not registered").WithOp("lens.report.Catalog")
}

type SourceFunc struct {
	BuildContextSpec func(context.Context, Definition, Parameters, filtercontext.EffectiveContext) (lens.DashboardSpec, error)
	Metadata         SourceDescriptor
	BuildSpec        func(context.Context, Definition, Parameters) (lens.DashboardSpec, error)
}

func (s SourceFunc) Descriptor() SourceDescriptor { return clone(s.Metadata) }
func (s SourceFunc) Build(ctx context.Context, definition Definition, params Parameters) (lens.DashboardSpec, error) {
	if s.BuildSpec == nil {
		return lens.DashboardSpec{}, invalid("source builder is required")
	}
	return s.BuildSpec(ctx, definition, params)
}

func (s SourceFunc) BuildContext(ctx context.Context, definition Definition, params Parameters, effective filtercontext.EffectiveContext) (lens.DashboardSpec, error) {
	if s.BuildContextSpec == nil {
		return lens.DashboardSpec{}, invalid("contextual source builder is required")
	}
	return s.BuildContextSpec(ctx, definition, params, effective)
}
