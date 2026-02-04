package testharness

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func WriteJSONReport(path string, report RunReport) error {
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func PrintConsoleSummary(w io.Writer, report RunReport) {
	fmt.Fprintf(w, "Tests: %d, Passed: %d, Failed: %d, Errors: %d\n",
		report.Summary.Total, report.Summary.Passed, report.Summary.Failed, report.Summary.Errored)

	if report.CacheKey != "" {
		suffix := ""
		if report.Cached {
			suffix = " (cached)"
		}
		fmt.Fprintf(w, "Cache key: %s%s\n", report.CacheKey, suffix)
	}

	failed := make([]string, 0)
	errored := make([]string, 0)
	for _, t := range report.Tests {
		switch t.Status {
		case TestStatusFailed:
			failed = append(failed, t.ID)
		case TestStatusError:
			errored = append(errored, t.ID)
		}
	}
	if len(failed) > 0 {
		fmt.Fprintf(w, "Failed: %s\n", strings.Join(failed, ", "))
	}
	if len(errored) > 0 {
		fmt.Fprintf(w, "Errors: %s\n", strings.Join(errored, ", "))
	}
}
