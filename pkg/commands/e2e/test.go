package e2e

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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
	baseURL := "http://" + net.JoinHostPort(E2E_SERVER_HOST, E2E_SERVER_PORT)
	logger.Info("Checking if e2e server is running...", "url", baseURL)

	// Try to connect to the server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("e2e server is not running on %s. Please start the e2e development server first using: make e2e dev", baseURL)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("e2e server is not healthy (status %d). Please check the server logs or restart using: make e2e dev", resp.StatusCode)
	}

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

	// Run Playwright tests
	e2eDir := filepath.Join(projectRoot, "e2e")
	testCtx, testCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer testCancel()

	testCmd := exec.CommandContext(testCtx, "npm", "run", "test")
	testCmd.Dir = e2eDir
	testCmd.Env = append(os.Environ(), "BASE_URL="+baseURL)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("e2e tests failed: %w", err)
	}

	logger.Info("E2E tests completed successfully!")
	return nil
}
