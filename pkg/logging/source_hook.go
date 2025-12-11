package logging

import (
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// SourceHook injects source location fields into every log entry.
// It automatically adds module, service, method, and source (combined path) fields
// to help identify where each log message originated.
type SourceHook struct{}

// NewSourceHook creates a new SourceHook
func NewSourceHook() *SourceHook {
	return &SourceHook{}
}

// Fire is called for every log entry. It extracts caller information
// and adds source location fields to the entry.
func (h *SourceHook) Fire(entry *logrus.Entry) error {
	// Skip depth accounts for:
	// 0: runtime.Caller
	// 1: extractSource
	// 2: SourceHook.Fire (this function)
	// 3: logrus entry.fireHooks
	// 4: logrus entry.log
	// 5: logrus entry.Log
	// 6: logrus entry.Info/Error/etc OR entry.Logf
	// 7: actual caller (may vary based on call pattern)
	//
	// We try multiple skip depths to find the first caller outside logrus internals
	source := extractSourceWithFallback()

	entry.Data["source"] = source.FullPath
	entry.Data["module"] = source.Module
	entry.Data["service"] = source.Service
	entry.Data["method"] = source.Method

	return nil
}

// Levels returns all log levels - this hook fires for every log message
func (h *SourceHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// SourceInfo contains parsed caller information
type SourceInfo struct {
	FullPath string // "shyona/cost_tracking_service.GetSessionCost"
	Module   string // "shyona"
	Service  string // "cost_tracking_service"
	Method   string // "GetSessionCost"
}

// extractSourceWithFallback walks up the call stack to find the first caller
// outside of logrus and this logging package internals
func extractSourceWithFallback() SourceInfo {
	// Walk up the stack starting from skip=2 (skip runtime.Caller and this function)
	// Look for the first frame that's not in logrus or this package
	for skip := 2; skip <= 20; skip++ {
		pc, file, _, ok := runtime.Caller(skip)
		if !ok {
			break
		}

		// Check if this is a logrus internal or our hook
		if isInternalFrame(file) {
			continue
		}

		// Found a valid caller, extract source info
		fn := runtime.FuncForPC(pc)
		method := "unknown"
		if fn != nil {
			method = extractMethodName(fn.Name())
		}

		module, service := extractModuleAndService(file)

		var fullPath string
		if module != "unknown" {
			fullPath = module + "/" + service + "." + method
		} else {
			fullPath = service + "." + method
		}

		return SourceInfo{
			FullPath: fullPath,
			Module:   module,
			Service:  service,
			Method:   method,
		}
	}

	// Fallback if nothing found
	return SourceInfo{
		FullPath: "unknown",
		Module:   "unknown",
		Service:  "unknown",
		Method:   "unknown",
	}
}

// isInternalFrame checks if a file path is from logrus or our logging package internals
func isInternalFrame(filePath string) bool {
	// Check for logrus package
	if strings.Contains(filePath, "sirupsen/logrus") {
		return true
	}
	// Check for our source_hook.go (but not the test file)
	if strings.HasSuffix(filePath, "source_hook.go") {
		return true
	}
	return false
}

// extractSource parses runtime.Caller info into SourceInfo.
// Never panics - returns "unknown" for any extraction failure.
func extractSource(skip int) SourceInfo {
	pc, file, _, ok := runtime.Caller(skip)
	if !ok {
		return SourceInfo{
			FullPath: "unknown",
			Module:   "unknown",
			Service:  "unknown",
			Method:   "unknown",
		}
	}

	// Extract method name from function
	fn := runtime.FuncForPC(pc)
	method := "unknown"
	if fn != nil {
		method = extractMethodName(fn.Name())
	}

	// Extract module and service from file path
	module, service := extractModuleAndService(file)

	// Build full path
	var fullPath string
	if module != "unknown" {
		fullPath = module + "/" + service + "." + method
	} else {
		fullPath = service + "." + method
	}

	return SourceInfo{
		FullPath: fullPath,
		Module:   module,
		Service:  service,
		Method:   method,
	}
}

// extractMethodName extracts method name from full function name.
// Handles: package.Function, package.(*Type).Method, package.Type.Method, closures
func extractMethodName(fullName string) string {
	if fullName == "" {
		return "unknown"
	}

	// Split by '.' and get last part
	parts := strings.Split(fullName, ".")
	if len(parts) == 0 {
		return "unknown"
	}

	method := parts[len(parts)-1]

	// Handle closures: "func1", "func2.1" -> "closure"
	if strings.HasPrefix(method, "func") {
		return "closure"
	}

	return method
}

// extractModuleAndService parses file path to extract module and service.
// Handles: /modules/{module}/..., /pkg/..., /cmd/...
func extractModuleAndService(filePath string) (module, service string) {
	module = "unknown"
	service = "unknown"

	// Handle empty path
	if filePath == "" {
		return module, service
	}

	// Try /modules/ first (most common case for application code)
	if idx := strings.Index(filePath, "/modules/"); idx != -1 {
		remainder := filePath[idx+9:] // after "/modules/"
		parts := strings.Split(remainder, "/")
		if len(parts) >= 1 {
			module = parts[0] // "shyona", "finance", etc.
		}
		if len(parts) >= 2 {
			fileName := parts[len(parts)-1]
			service = strings.TrimSuffix(fileName, ".go")
		}
		return module, service
	}

	// Handle /pkg/ paths (SDK and shared packages)
	if idx := strings.Index(filePath, "/pkg/"); idx != -1 {
		remainder := filePath[idx+5:] // after "/pkg/"
		parts := strings.Split(remainder, "/")
		module = "pkg"
		if len(parts) >= 1 {
			// For nested packages like pkg/logging/logger.go
			// Use the package name, not just the file
			if len(parts) > 1 {
				module = "pkg/" + parts[0] // e.g., "pkg/logging"
			}
			fileName := parts[len(parts)-1]
			service = strings.TrimSuffix(fileName, ".go")
		}
		return module, service
	}

	// Handle /cmd/ paths (command-line tools)
	if idx := strings.Index(filePath, "/cmd/"); idx != -1 {
		remainder := filePath[idx+5:] // after "/cmd/"
		parts := strings.Split(remainder, "/")
		module = "cmd"
		if len(parts) >= 1 {
			service = parts[0] // cmd name like "server", "superadmin"
		}
		return module, service
	}

	// Fallback: use filename as service
	parts := strings.Split(filePath, "/")
	if len(parts) > 0 {
		fileName := parts[len(parts)-1]
		if fileName != "" {
			service = strings.TrimSuffix(fileName, ".go")
		}
	}

	return module, service
}
