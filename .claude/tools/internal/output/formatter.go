package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
)

// Formatter handles output formatting for both text and JSON
type Formatter struct {
	writer io.Writer
	json   bool
}

// New creates a new Formatter
func New(writer io.Writer, asJSON bool) *Formatter {
	return &Formatter{
		writer: writer,
		json:   asJSON,
	}
}

// PrintJSON prints data as JSON
func (f *Formatter) PrintJSON(data interface{}) error {
	if !f.json {
		return fmt.Errorf("formatter not in JSON mode")
	}

	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// PrintText prints formatted text
func (f *Formatter) PrintText(text string) error {
	if f.json {
		return fmt.Errorf("formatter in JSON mode")
	}

	_, err := fmt.Fprint(f.writer, text)
	return err
}

// PrintTextLn prints formatted text with a newline
func (f *Formatter) PrintTextLn(text string) error {
	if f.json {
		return fmt.Errorf("formatter in JSON mode")
	}

	_, err := fmt.Fprintln(f.writer, text)
	return err
}

// IsJSON returns true if the formatter is in JSON mode
func (f *Formatter) IsJSON() bool {
	return f.json
}

// Color helpers for terminal output

// Bold returns bold text
func Bold(text string) string {
	return color.New(color.Bold).Sprint(text)
}

// Green returns green text
func Green(text string) string {
	return color.GreenString(text)
}

// Yellow returns yellow text
func Yellow(text string) string {
	return color.YellowString(text)
}

// Red returns red text
func Red(text string) string {
	return color.RedString(text)
}

// Cyan returns cyan text
func Cyan(text string) string {
	return color.CyanString(text)
}

// Blue returns blue text
func Blue(text string) string {
	return color.BlueString(text)
}

// Magenta returns magenta text
func Magenta(text string) string {
	return color.MagentaString(text)
}

// FormatFileList formats a list of files with indentation and truncation
func FormatFileList(files []string, maxDisplay int) string {
	if len(files) == 0 {
		return ""
	}

	var result strings.Builder

	displayCount := len(files)
	if maxDisplay > 0 && displayCount > maxDisplay {
		displayCount = maxDisplay
	}

	for i := 0; i < displayCount; i++ {
		result.WriteString("  ")
		result.WriteString(files[i])
		result.WriteString("\n")
	}

	if maxDisplay > 0 && len(files) > maxDisplay {
		remaining := len(files) - maxDisplay
		result.WriteString(fmt.Sprintf("  ... and %d more\n", remaining))
	}

	return result.String()
}
