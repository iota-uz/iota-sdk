package e2e

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// Test runs the e2e test suite with complete database setup
// The server must be started separately using `make e2e dev`
func Test() error {
	conf := configuration.Use()
	logger := conf.Logger()

	logger.Info("Starting e2e test runner...")

	// First, ensure database is set up with fresh data
	logger.Info("Setting up e2e database...")
	if err := Setup(); err != nil {
		return fmt.Errorf("failed to setup e2e database: %w", err)
	}

	// Check if server is already running
	baseURL := fmt.Sprintf("http://%s:%s", E2E_SERVER_HOST, E2E_SERVER_PORT)
	logger.Info("Checking if e2e server is running...", "url", baseURL)

	// Try to connect to the server
	resp, err := http.Get(baseURL)
	if err != nil {
		return fmt.Errorf("e2e server is not running on %s. Please start the e2e development server first using: make e2e dev", baseURL)
	}

	if resp.StatusCode >= 500 {
		resp.Body.Close()
		return fmt.Errorf("e2e server is not healthy (status %d). Please check the server logs or restart using: make e2e dev", resp.StatusCode)
	}

	resp.Body.Close()
	logger.Info("Server is running and healthy, proceeding with tests...")

	// Get project root directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			return fmt.Errorf("could not find project root with go.mod")
		}
		projectRoot = parent
	}

	// Run cypress tests
	e2eDir := filepath.Join(projectRoot, "e2e")
	testCmd := exec.Command("npm", "run", "test")
	testCmd.Dir = e2eDir
	testCmd.Env = append(os.Environ(), "CYPRESS_BASE_URL="+baseURL)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("e2e tests failed: %w", err)
	}

	logger.Info("E2E tests completed successfully!")
	return nil
}
