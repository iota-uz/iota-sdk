package server

import (
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/testutils"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../"); err != nil {
		panic(err)
	}
	db, err := testutils.DBSetup()
	if err != nil {
		panic(err)
	}

	code := m.Run()

	if err := db.Close(); err != nil {
		panic(err)
	}

	os.Exit(code)
}

func TestCheckModels(t *testing.T) { //nolint:paralleltest
	db, err := testutils.GormOpen(configuration.Use().DBOpts)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckModels(db); err != nil {
		t.Fatal(err)
	}
}
