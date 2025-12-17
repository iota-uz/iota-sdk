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

// Fire is called for every log entry. It walks the call stack to find
// the first caller outside logrus internals and adds source location fields.
func (h *SourceHook) Fire(entry *logrus.Entry) error {
	source := extractSource()

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

// extractSource walks up the call stack to find the first caller
// outside of logrus and this logging package.
func extractSource() SourceInfo {
	for skip := 2; skip <= 20; skip++ {
		pc, file, _, ok := runtime.Caller(skip)
		if !ok {
			break
		}

		if isInternalFrame(file) {
			continue
		}

		fn := runtime.FuncForPC(pc)
		method := "unknown"
		if fn != nil {
			method = extractMethodName(fn.Name())
		}

		module, service := extractModuleAndService(file)

		fullPath := service + "." + method
		if module != "unknown" {
			fullPath = module + "/" + fullPath
		}

		return SourceInfo{
			FullPath: fullPath,
			Module:   module,
			Service:  service,
			Method:   method,
		}
	}

	return SourceInfo{
		FullPath: "unknown",
		Module:   "unknown",
		Service:  "unknown",
		Method:   "unknown",
	}
}

// isInternalFrame checks if a file path is from logrus or this package's internals.
func isInternalFrame(filePath string) bool {
	return strings.Contains(filePath, "sirupsen/logrus") ||
		strings.HasSuffix(filePath, "source_hook.go")
}

// extractMethodName extracts method name from full function name.
// Handles: package.Function, package.(*Type).Method, package.Type.Method, closures
func extractMethodName(fullName string) string {
	if fullName == "" {
		return "unknown"
	}

	parts := strings.Split(fullName, ".")
	method := parts[len(parts)-1]

	if strings.HasPrefix(method, "func") {
		return "closure"
	}

	return method
}

// extractModuleAndService parses file path to extract module and service.
// Supports: /modules/{module}/..., /pkg/..., /cmd/...
func extractModuleAndService(filePath string) (string, string) {
	if filePath == "" {
		return "unknown", "unknown"
	}

	if idx := strings.Index(filePath, "/modules/"); idx != -1 {
		parts := strings.Split(filePath[idx+9:], "/")
		module := parts[0]
		service := "unknown"
		if len(parts) >= 2 {
			service = strings.TrimSuffix(parts[len(parts)-1], ".go")
		}
		return module, service
	}

	if idx := strings.Index(filePath, "/pkg/"); idx != -1 {
		parts := strings.Split(filePath[idx+5:], "/")
		module := "pkg"
		if len(parts) > 1 {
			module = "pkg/" + parts[0]
		}
		service := strings.TrimSuffix(parts[len(parts)-1], ".go")
		return module, service
	}

	if idx := strings.Index(filePath, "/cmd/"); idx != -1 {
		parts := strings.Split(filePath[idx+5:], "/")
		return "cmd", parts[0]
	}

	// Fallback: use filename as service
	parts := strings.Split(filePath, "/")
	if fileName := parts[len(parts)-1]; fileName != "" {
		return "unknown", strings.TrimSuffix(fileName, ".go")
	}

	return "unknown", "unknown"
}
