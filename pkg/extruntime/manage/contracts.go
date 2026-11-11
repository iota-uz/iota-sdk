package manage

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func contractDiag(code, export, msg string) build.Diagnostic {
	return build.Diagnostic{File: "package.json", Severity: build.SeverityError, Code: code, Message: fmt.Sprintf("implementation %q: %s", export, msg)}
}

func describe(r manifest.Resource) string {
	if r.Kind == manifest.ResCommand && r.Atomic {
		return "atomic command"
	}
	return string(r.Kind)
}

// checkImplementations validates every implementation of the manifest against
// the contract it implements. A foreign contract (ID outside the package
// namespace) is looked up in the catalog: its kind and atomic flag must match
// and the contract hash the package was built against (the kit snapshot) must
// equal the catalog's current hash. It returns the hashes of the foreign
// contracts, recorded with the version.
func (s *Service) checkImplementations(ctx context.Context, ns string, m manifest.Manifest, built map[string]string) ([]build.Diagnostic, map[string]string, error) {
	var diags []build.Diagnostic
	foreign := map[string]string{}
	for _, i := range m.Implementations {
		if ownsID(ns, i.Implements) {
			res, ok := m.Find(i.Implements)
			switch {
			case !ok:
				diags = append(diags, contractDiag("contract.unknown", i.Export, "implements "+i.Implements+", which the package does not declare"))
			case !implMatches(i, res):
				diags = append(diags, contractDiag("contract.kindMismatch", i.Export, fmt.Sprintf("is a %s implementation but %s is a %s", i.Kind, res.ID, describe(res))))
			}
			continue
		}
		cur, found, err := s.catalog.Resolve(ctx, i.Implements)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			diags = append(diags, contractDiag("contract.unknown", i.Export, "implements "+i.Implements+", which is not in the catalog"))
			continue
		}
		if !implMatches(i, cur.Resource) {
			diags = append(diags, contractDiag("contract.kindMismatch", i.Export, fmt.Sprintf("is a %s implementation but %s is a %s", i.Kind, cur.Resource.ID, describe(cur.Resource))))
			continue
		}
		hash := cur.Resource.ContractHash
		if hash == "" {
			hash = manifest.ContractHashOf(cur.Resource)
		}
		if built[i.Implements] != hash {
			diags = append(diags, contractDiag("contract.hashMismatch", i.Export, fmt.Sprintf("was built against a different version of %s than the catalog's current one", i.Implements)))
			continue
		}
		foreign[i.Implements] = hash
	}
	sort.SliceStable(diags, func(a, b int) bool { return diags[a].Code+diags[a].Message < diags[b].Code+diags[b].Message })
	return diags, foreign, nil
}

// verifyForeignContracts re-checks, at activation, that the foreign contracts
// a version implements still have the hashes it was built against.
func (s *Service) verifyForeignContracts(ctx context.Context, tx pgx.Tx, versionID any) error {
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT foreign_contracts FROM extruntime.versions WHERE id = $1`, versionID).Scan(&raw); err != nil {
		return serrors.FromDB("extruntime.manage.verifyForeignContracts", err)
	}
	var recorded map[string]string
	if err := json.Unmarshal(raw, &recorded); err != nil {
		return serrors.Wrap("extruntime.manage.verifyForeignContracts", err)
	}
	ids := make([]string, 0, len(recorded))
	for id := range recorded {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		cur, found, err := s.catalog.Resolve(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return serrors.New(serrors.FailedPrecondition, "activation blocked: contract "+id+" is no longer in the catalog")
		}
		hash := cur.Resource.ContractHash
		if hash == "" {
			hash = manifest.ContractHashOf(cur.Resource)
		}
		if hash != recorded[id] {
			return serrors.New(serrors.FailedPrecondition, "activation blocked: contract "+id+" changed since the version was built")
		}
	}
	return nil
}
