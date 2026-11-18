package manage

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

var _ workflow.PackageGate = implementations{}

// PackageActive implements workflow.PackageGate: a workflow instance runs only
// while its package has an active version. Deactivating the package blocks its
// live instances at their next run (diagnostic `packageDeactivated`); they
// continue with Resume after the package is activated again. Instances pinned
// to an older version of a package that is still active keep running.
func (i implementations) PackageActive(ctx context.Context, pkg string) (bool, error) {
	active, err := i.s.catalog.Active(ctx)
	if err != nil {
		return false, err
	}
	for _, av := range active {
		if av.Namespace == pkg {
			return true, nil
		}
	}
	return false, nil
}
