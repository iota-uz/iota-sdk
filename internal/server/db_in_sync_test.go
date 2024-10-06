package server

import (
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/test_utils"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../"); err != nil {
		panic(err)
	}
	db, err := test_utils.DbSetup()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	code := m.Run()

	os.Exit(code)
}

func TestCheckModels(t *testing.T) {
	db, err := test_utils.GormOpen(configuration.Use().DbOpts)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckModels(db); err != nil {
		t.Fatal(err)
	}
}
