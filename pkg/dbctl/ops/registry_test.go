package ops

import "testing"

func TestRegistry_ContainsExpectedOperations(t *testing.T) {
	expected := []string{
		"seed.main",
		"seed.superadmin",
		"seed.e2e",
		"db.e2e.create",
		"db.e2e.drop",
		"db.e2e.reset",
		"db.e2e.migrate",
	}

	registry := Registry()
	for _, name := range expected {
		spec, ok := registry[name]
		if !ok {
			t.Fatalf("missing operation %q", name)
		}
		if spec.Name != name {
			t.Fatalf("operation %q has wrong spec name %q", name, spec.Name)
		}
		if len(spec.Steps) == 0 {
			t.Fatalf("operation %q has no steps", name)
		}
		for _, step := range spec.Steps {
			if step.Handler == nil {
				t.Fatalf("operation %q step %q has nil handler", name, step.ID)
			}
		}
	}
}
