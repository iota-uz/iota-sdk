package ops

import (
	"fmt"
	"sort"
)

func Registry() map[string]OperationSpec {
	return map[string]OperationSpec{
		"seed.main":       SeedMainOperation(),
		"seed.superadmin": SeedSuperadminOperation(),
		"seed.e2e":        SeedE2EOperation(),
		"db.e2e.create":   E2ECreateOperation(),
		"db.e2e.drop":     E2EDropOperation(),
		"db.e2e.reset":    E2EResetOperation(),
		"db.e2e.migrate":  E2EMigrateOperation(),
	}
}

func Get(name string) (OperationSpec, error) {
	spec, ok := Registry()[name]
	if !ok {
		return OperationSpec{}, fmt.Errorf("unknown operation %q", name)
	}
	return spec, nil
}

func Names() []string {
	names := make([]string, 0, len(Registry()))
	for name := range Registry() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
