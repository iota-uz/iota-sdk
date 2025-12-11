package logging

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractMethodName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple function",
			input:    "main.handleRequest",
			expected: "handleRequest",
		},
		{
			name:     "method with pointer receiver",
			input:    "github.com/iota-uz/shy-eld/modules/shyona/services.(*CostTrackingService).GetSessionCost",
			expected: "GetSessionCost",
		},
		{
			name:     "method with value receiver",
			input:    "github.com/iota-uz/shy-eld/modules/finance/services.TransactionImporter.ImportFromExcel",
			expected: "ImportFromExcel",
		},
		{
			name:     "closure func1",
			input:    "github.com/iota-uz/shy-eld/modules/shyona/services.(*chatService).triggerAsyncTitleGeneration.func1",
			expected: "closure",
		},
		{
			name:     "nested closure",
			input:    "main.main.func1.func2",
			expected: "closure",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "unknown",
		},
		{
			name:     "package-level function",
			input:    "github.com/iota-uz/iota-sdk/pkg/logging.extractSource",
			expected: "extractSource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMethodName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractModuleAndService(t *testing.T) {
	tests := []struct {
		name            string
		filePath        string
		expectedModule  string
		expectedService string
	}{
		{
			name:            "modules path - services",
			filePath:        "/Users/x/Projects/sdk/shy-eld/modules/shyona/services/cost_tracking_service.go",
			expectedModule:  "shyona",
			expectedService: "cost_tracking_service",
		},
		{
			name:            "modules path - controllers",
			filePath:        "/Users/x/Projects/sdk/shy-eld/modules/finance/presentation/controllers/statements_controller.go",
			expectedModule:  "finance",
			expectedService: "statements_controller",
		},
		{
			name:            "modules path - repositories",
			filePath:        "/Users/x/Projects/sdk/shy-eld/modules/logistics/infrastructure/persistence/driver_repository.go",
			expectedModule:  "logistics",
			expectedService: "driver_repository",
		},
		{
			name:            "pkg path - nested",
			filePath:        "/Users/x/Projects/sdk/iota-sdk/pkg/logging/logger.go",
			expectedModule:  "pkg/logging",
			expectedService: "logger",
		},
		{
			name:            "pkg path - composables",
			filePath:        "/Users/x/Projects/sdk/iota-sdk/pkg/composables/request.go",
			expectedModule:  "pkg/composables",
			expectedService: "request",
		},
		{
			name:            "cmd path",
			filePath:        "/Users/x/Projects/sdk/shy-eld/cmd/server/main.go",
			expectedModule:  "cmd",
			expectedService: "server",
		},
		{
			name:            "cmd path - superadmin",
			filePath:        "/Users/x/Projects/sdk/shy-eld/cmd/superadmin/main.go",
			expectedModule:  "cmd",
			expectedService: "superadmin",
		},
		{
			name:            "unknown path",
			filePath:        "/some/random/path/file.go",
			expectedModule:  "unknown",
			expectedService: "file",
		},
		{
			name:            "empty path",
			filePath:        "",
			expectedModule:  "unknown",
			expectedService: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, service := extractModuleAndService(tt.filePath)
			assert.Equal(t, tt.expectedModule, module, "module mismatch")
			assert.Equal(t, tt.expectedService, service, "service mismatch")
		})
	}
}

func TestSourceHookFire(t *testing.T) {
	// Create a logger with JSON formatter and our hook
	logger := logrus.New()
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.DebugLevel)
	logger.AddHook(NewSourceHook())

	// Log a message
	logger.Info("test message")

	// Parse the JSON output
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err, "Failed to parse log JSON")

	// Verify all source fields are present
	assert.Contains(t, logEntry, "source", "source field should be present")
	assert.Contains(t, logEntry, "module", "module field should be present")
	assert.Contains(t, logEntry, "service", "service field should be present")
	assert.Contains(t, logEntry, "method", "method field should be present")

	// Verify the method is TestSourceHookFire (this test function)
	method, ok := logEntry["method"].(string)
	assert.True(t, ok, "method should be a string")
	assert.Equal(t, "TestSourceHookFire", method, "method should be the test function name")

	// Verify the message
	msg, ok := logEntry["msg"].(string)
	assert.True(t, ok, "msg should be a string")
	assert.Equal(t, "test message", msg)
}

func TestSourceHookWithFields(t *testing.T) {
	// Create a logger with JSON formatter and our hook
	logger := logrus.New()
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.DebugLevel)
	logger.AddHook(NewSourceHook())

	// Log a message with additional fields
	logger.WithField("request_id", "abc123").WithField("tenant_id", "tenant1").Info("test with fields")

	// Parse the JSON output
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err, "Failed to parse log JSON")

	// Verify source fields are present alongside user fields
	assert.Contains(t, logEntry, "source")
	assert.Contains(t, logEntry, "module")
	assert.Contains(t, logEntry, "service")
	assert.Contains(t, logEntry, "method")

	// Verify user fields are preserved
	assert.Equal(t, "abc123", logEntry["request_id"])
	assert.Equal(t, "tenant1", logEntry["tenant_id"])

	// Verify method is correct
	assert.Equal(t, "TestSourceHookWithFields", logEntry["method"])
}

func TestSourceHookWithEntry(t *testing.T) {
	// Create a logger with JSON formatter and our hook
	logger := logrus.New()
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.DebugLevel)
	logger.AddHook(NewSourceHook())

	// Create an entry and log with it (simulates composables.UseLogger pattern)
	entry := logrus.NewEntry(logger)
	entry.Info("test with entry")

	// Parse the JSON output
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err, "Failed to parse log JSON")

	// Verify source fields are present
	assert.Contains(t, logEntry, "source")
	assert.Contains(t, logEntry, "module")
	assert.Contains(t, logEntry, "service")
	assert.Contains(t, logEntry, "method")

	// Verify method is correct
	assert.Equal(t, "TestSourceHookWithEntry", logEntry["method"])
}

func TestIsInternalFrame(t *testing.T) {
	tests := []struct {
		filePath string
		expected bool
	}{
		{"/go/pkg/mod/github.com/sirupsen/logrus@v1.9.3/entry.go", true},
		{"/go/pkg/mod/github.com/sirupsen/logrus@v1.9.3/logger.go", true},
		{"/Users/x/Projects/sdk/iota-sdk/pkg/logging/source_hook.go", true},
		{"/Users/x/Projects/sdk/iota-sdk/pkg/logging/source_hook_test.go", false},
		{"/Users/x/Projects/sdk/shy-eld/modules/shyona/services/cost_tracking_service.go", false},
		{"/Users/x/Projects/sdk/shy-eld/modules/finance/presentation/controllers/statements_controller.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			result := isInternalFrame(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function to test source extraction from a known location
func helperFunctionForSourceTest() SourceInfo {
	return extractSource(1) // Skip just this function
}

func TestExtractSourceFromKnownLocation(t *testing.T) {
	source := helperFunctionForSourceTest()

	// The method should be helperFunctionForSourceTest
	assert.Equal(t, "helperFunctionForSourceTest", source.Method)

	// The service should be source_hook_test (this file)
	assert.Equal(t, "source_hook_test", source.Service)

	// The module should be pkg/logging
	assert.Equal(t, "pkg/logging", source.Module)
}

func BenchmarkSourceHook(b *testing.B) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.AddHook(NewSourceHook())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message")
	}
}

func BenchmarkLogrusWithoutHook(b *testing.B) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{})
	logger.SetFormatter(&logrus.JSONFormatter{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message")
	}
}
