package builtin_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

func TestRegisterBuildsACheckedCatalog(t *testing.T) {
	reg := catalog.NewRegistry(nil)
	if err := reg.RegisterTriggers(engine.Triggers()); err != nil {
		t.Fatal(err)
	}
	if err := builtin.Register(reg, builtin.Deps{}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	v, err := reg.ForTenant(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("ForTenant: %v", err)
	}
	for _, typ := range []string{"set", "code", "http", "start_automation", "run_automation"} {
		if _, ok := v.Catalog.Nodes[typ]; !ok {
			t.Errorf("%s is not in the catalog (unavailable: %v)", typ, v.Unavailable)
		}
	}
	if err := builtin.Register(reg, builtin.Deps{}); err == nil {
		t.Fatal("a second registration was accepted")
	}
}
