package builders

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// ErrorHandler provides consistent error handling for CLI commands
type ErrorHandler struct {
	cmd *cobra.Command
}

// NewErrorHandler creates a new error handler for a command
func NewErrorHandler(cmd *cobra.Command) *ErrorHandler {
	return &ErrorHandler{cmd: cmd}
}

// Handle processes an error with context and exits appropriately
func (e *ErrorHandler) Handle(err error, context string) {
	if err == nil {
		return
	}

	var message string
	if context != "" {
		message = fmt.Sprintf("Failed to %s: %v", context, err)
	} else {
		message = fmt.Sprintf("Command failed: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
	os.Exit(1)
}

// HandleWithStack processes an error with stack trace for debugging
func (e *ErrorHandler) HandleWithStack(err error, context string) {
	if err == nil {
		return
	}

	stack := string(debug.Stack())
	var message string
	if context != "" {
		message = fmt.Sprintf("Failed to %s: %v", context, err)
	} else {
		message = fmt.Sprintf("Command failed: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Error: %s\n\nStack trace:\n%s", message, stack)
	os.Exit(1)
}

// WrapRunE wraps a RunE function with consistent error handling
func WrapRunE(fn func(cmd *cobra.Command, args []string) error, context string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := fn(cmd, args); err != nil {
			handler := NewErrorHandler(cmd)
			handler.Handle(err, context)
		}
		return nil
	}
}

// WrapRun wraps a Run function with consistent error handling
func WrapRun(fn func() error, context string) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := fn(); err != nil {
			handler := NewErrorHandler(cmd)
			handler.Handle(err, context)
		}
	}
}
