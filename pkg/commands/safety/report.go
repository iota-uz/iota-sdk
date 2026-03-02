package safety

import (
	"fmt"
	"io"
	"strings"
)

type CredentialStatus struct {
	Label    string
	Email    string
	Password string
	Status   string
}

func PrintPreflight(out io.Writer, res PreflightResult, opts RunOptions) {
	if out == nil {
		return
	}
	_, _ = fmt.Fprintf(out, "\n== Seed/DB Safety Preflight ==\n")
	_, _ = fmt.Fprintf(out, "operation: %s\n", res.Operation)
	_, _ = fmt.Fprintf(out, "environment: %s\n", emptyFallback(res.Target.Environment, "development"))
	_, _ = fmt.Fprintf(out, "db target: host=%s port=%s name=%s user=%s password=%s\n",
		emptyFallback(res.Target.Host, "<empty>"),
		emptyFallback(res.Target.Port, "<empty>"),
		emptyFallback(res.Target.Name, "<empty>"),
		emptyFallback(res.Target.User, "<empty>"),
		res.Target.Password,
	)
	_, _ = fmt.Fprintf(out, "db local host: %t\n", res.DBState.IsLocalHost)
	_, _ = fmt.Fprintf(out, "db non-empty: %t\n", res.DBState.IsNonEmpty)
	if len(res.DBState.CheckedTables) > 0 {
		parts := make([]string, 0, len(res.DBState.CheckedTables))
		for _, tc := range res.DBState.CheckedTables {
			parts = append(parts, fmt.Sprintf("%s=%d", tc.Table, tc.Count))
		}
		_, _ = fmt.Fprintf(out, "row counts: %s\n", strings.Join(parts, ", "))
	}
	_, _ = fmt.Fprintf(out, "backup markers detected: %t\n", res.DBState.LooksLikeBackup)
	if len(res.Risks) > 0 {
		_, _ = fmt.Fprintf(out, "risks:\n")
		for _, risk := range res.Risks {
			_, _ = fmt.Fprintf(out, "- [%s] %s: %s\n", strings.ToUpper(risk.Severity), risk.Code, risk.Message)
		}
	} else {
		_, _ = fmt.Fprintf(out, "risks: none\n")
	}
	_, _ = fmt.Fprintf(out, "planned actions:\n")
	for _, action := range res.Actions {
		_, _ = fmt.Fprintf(out, "- %s\n", action)
	}
	if opts.DryRun {
		_, _ = fmt.Fprintf(out, "mode: DRY RUN (no mutations will be executed)\n")
	}
}

func PrintCredentialSummary(out io.Writer, items []CredentialStatus, dryRun bool) {
	if out == nil {
		return
	}
	if len(items) == 0 {
		return
	}
	if dryRun {
		_, _ = fmt.Fprintln(out, "\n== Credential Summary (dry-run) ==")
	} else {
		_, _ = fmt.Fprintln(out, "\n== Credential Summary ==")
	}
	for _, item := range items {
		line := fmt.Sprintf("- %s: email=%s status=%s", item.Label, item.Email, item.Status)
		if strings.TrimSpace(item.Password) != "" {
			line += fmt.Sprintf(" password=%s", item.Password)
		}
		_, _ = fmt.Fprintln(out, line)
	}
}

func PrintOutcomeSummary(out io.Writer, title string, executed bool, dryRun bool) {
	if out == nil {
		return
	}
	status := "executed"
	if !executed || dryRun {
		status = "skipped"
	}
	_, _ = fmt.Fprintf(out, "\n== Summary ==\n%s: %s\n", title, status)
}

func emptyFallback(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
