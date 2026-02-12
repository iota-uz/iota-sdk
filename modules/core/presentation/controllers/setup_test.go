package controllers_test

import (
	"os"
	"testing"
)

// TestMain sets up the test environment before any tests run.
// This includes changing to the project root directory and setting up
// consistent UPLOADS_PATH for upload controller tests.
// The UPLOADS_PATH setup is necessary because configuration.Use() is a singleton
// that caches the UPLOADS_PATH value from the first initialization.
func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}

	// Set a consistent uploads path for all tests to avoid singleton caching issues
	_ = os.Setenv("UPLOADS_PATH", "test-uploads")

	// Create the base directory
	_ = os.MkdirAll("test-uploads", 0755)

	// Run tests
	code := m.Run()

	// Cleanup
	_ = os.RemoveAll("test-uploads")
	os.Exit(code)
}
