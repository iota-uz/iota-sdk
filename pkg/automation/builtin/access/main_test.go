package access_test

import (
	"os"
	"testing"
)

// The migrator of the PostgreSQL tests reads ./migrations relative to the working directory.
func TestMain(m *testing.M) {
	if err := os.Chdir("../../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
