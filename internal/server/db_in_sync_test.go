package server

import (
	"github.com/iota-agency/iota-erp/internal/testutils"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestCheckModels(t *testing.T) { //nolint:paralleltest
	ctx := testutils.GetTestContext()
	if err := CheckModels(ctx.GormDB); err != nil {
		t.Fatal(err)
	}
}
