package ui

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/listext"
)

// Columns returns the ColumnSource over activated column resources.
func (r *Runtime) Columns() listext.ColumnSource { return columnSource{r} }

type columnSource struct{ r *Runtime }

func (s columnSource) Columns(ctx context.Context, list string) ([]listext.Contribution, error) {
	ix, err := s.r.Index(ctx)
	if err != nil {
		return nil, err
	}
	return append([]listext.Contribution(nil), ix.Columns[list]...), nil
}
