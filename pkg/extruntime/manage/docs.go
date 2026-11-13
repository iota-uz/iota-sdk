package manage

import (
	"context"
	"fmt"
	"sort"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// checkDocPermissions verifies, against the catalog, the permissions the
// package's documents name that the package does not declare itself: a
// permission of the catalog (a native contract's or another package's), or a
// native RBAC name, which only the application's roles can vouch for and is
// accepted as written. The structure of the documents (paths, titles, the
// package's own permissions) is manifest.Validate's.
func (s *Service) checkDocPermissions(ctx context.Context, m manifest.Manifest) ([]build.Diagnostic, error) {
	diags := make([]build.Diagnostic, 0, len(m.Docs))
	checked := map[string]bool{}
	for _, d := range m.Docs {
		p := d.Permission
		if p == "" || checked[p] || m.DeclaresPermission(p) || manifest.NativeDocPermission(p) {
			continue
		}
		checked[p] = true
		cur, found, err := s.catalog.Resolve(ctx, p)
		if err != nil {
			return nil, err
		}
		if found && cur.Resource.Kind == manifest.ResPermission {
			continue
		}
		diags = append(diags, build.Diagnostic{
			File: manifest.DocFile(d), Line: 1, Severity: build.SeverityError, Code: "docs.unknownPermission",
			Message: fmt.Sprintf("permission %q is not declared by the package or the catalog", p),
		})
	}
	sort.SliceStable(diags, func(a, b int) bool { return diags[a].File+diags[a].Message < diags[b].File+diags[b].Message })
	return diags, nil
}

// checkDocFiles verifies that the stored artifact holds exactly the documents
// the manifest lists. The build guarantees it; a version that did not come
// from the build must not be able to activate otherwise.
func (s *Service) checkDocFiles(ctx context.Context, artifactHash string, m manifest.Manifest) error {
	if len(m.Docs) == 0 {
		return nil
	}
	tree, err := s.arts.Tree(ctx, artifactHash)
	if err != nil {
		return err
	}
	if err := manifest.ValidateDocFiles(m, tree); err != nil {
		return serrors.New(serrors.FailedPrecondition, "documentation: "+err.Error()).WithOp("extruntime.manage.checkDocFiles")
	}
	return nil
}
