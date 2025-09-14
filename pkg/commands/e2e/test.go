package e2e

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// Test runs the complete e2e test suite with proper server lifecycle management
func Test() error {
	conf := configuration.Use()
	logger := conf.Logger()

	logger.Info("Starting e2e test runner...")

	// Setup cleanup handler
	var serverCmd *exec.Cmd
	cleanup := func() {
		if serverCmd != nil && serverCmd.Process != nil {
			logger.Info("Shutting down e2e server...", "pid", serverCmd.Process.Pid)

			// Try graceful shutdown first
			if err := serverCmd.Process.Signal(syscall.SIGTERM); err != nil {
				logger.Warn("Failed to send SIGTERM", "error", err)
			}

			// Wait for graceful shutdown with timeout
			done := make(chan error, 1)
			go func() {
				done <- serverCmd.Wait()
			}()

			select {
			case <-time.After(10 * time.Second):
				logger.Warn("Server didn't shutdown gracefully, force killing...")
				if err := serverCmd.Process.Kill(); err != nil {
					logger.Error("Failed to kill server process", "error", err)
				}
				// Wait a bit more for the kill to take effect
				<-done
			case err := <-done:
				if err != nil {
					logger.Debug("Server process exited", "error", err)
				} else {
					logger.Info("Server shutdown gracefully")
				}
			}
		}

		// Additional cleanup: kill any processes still using the port
		if cmd := exec.Command("lsof", "-t", "-i:"+E2E_SERVER_PORT); cmd != nil {
			if out, err := cmd.Output(); err == nil && len(out) > 0 {
				pids := string(out)
				logger.Info("Killing remaining processes on port", "port", E2E_SERVER_PORT, "pids", pids)
				if killCmd := exec.Command("kill", "-TERM", string(out[:len(out)-1])); killCmd != nil {
					_ = killCmd.Run()
				}
			}
		}
	}

	// Handle interrupt signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logger.Info("Received interrupt signal, cleaning up...")
		cleanup()
		os.Exit(1)
	}()

	// Setup e2e environment
	logger.Info("Setting up e2e environment...")
	if err := Setup(); err != nil {
		return fmt.Errorf("failed to setup e2e environment: %w", err)
	}

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

	// Start e2e server
	logger.Info("Starting e2e server...", "port", E2E_SERVER_PORT)
	serverCmd = exec.Command("go", "run", "cmd/server/main.go")
	serverCmd.Dir = projectRoot
	serverCmd.Env = append(os.Environ(),
		"DB_NAME="+E2E_DB_NAME,
		"PORT="+E2E_SERVER_PORT,
		"ORIGIN=http://"+E2E_SERVER_HOST+":"+E2E_SERVER_PORT,
	)

	if err := serverCmd.Start(); err != nil {
		return fmt.Errorf("failed to start e2e server: %w", err)
	}

	logger.Info("E2E server started", "pid", serverCmd.Process.Pid)

	// Wait for server to be healthy
	baseURL := fmt.Sprintf("http://%s:%s", E2E_SERVER_HOST, E2E_SERVER_PORT)
	logger.Info("Waiting for server to be healthy...", "url", baseURL)

	healthy := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(baseURL)
		if err == nil && resp.StatusCode < 500 {
			resp.Body.Close()
			healthy = true
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}

	if !healthy {
		cleanup()
		return fmt.Errorf("server failed to become healthy within 30 seconds")
	}

	logger.Info("Server is healthy, running tests...")

	// Run cypress tests
	e2eDir := filepath.Join(projectRoot, "e2e")
	testCmd := exec.Command("npm", "run", "test")
	testCmd.Dir = e2eDir
	testCmd.Env = append(os.Environ(), "CYPRESS_BASE_URL="+baseURL)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	testErr := testCmd.Run()

	// Always cleanup server
	cleanup()

	if testErr != nil {
		return fmt.Errorf("e2e tests failed: %w", testErr)
	}

	logger.Info("E2E tests completed successfully!")
	return nil
}
