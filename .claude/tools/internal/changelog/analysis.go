package changelog

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/sdk-tools/internal/git"
)

var (
	// Layer classification patterns
	presentationPattern   = regexp.MustCompile(`(_controller\.go|_viewmodel\.go|\.templ|\.toml)$`)
	businessPattern       = regexp.MustCompile(`_service\.go$`)
	infrastructurePattern = regexp.MustCompile(`(_repository\.go|migrations/)`)
	migrationPattern      = regexp.MustCompile(`migrations/.*\.sql$`)
	testPattern           = regexp.MustCompile(`_test\.go$`)
	docsPattern           = regexp.MustCompile(`\.md$`)
)

// AnalyzeChanges analyzes changed files to determine if CHANGELOG update is required
func AnalyzeChanges(ctx context.Context) (*CheckResult, error) {
	// Get changed files from git
	files, err := git.GetChangedFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	return AnalyzeFiles(files), nil
}

// AnalyzeFiles analyzes a list of files and returns a CheckResult
func AnalyzeFiles(files []string) *CheckResult {
	stats := FileStats{
		ChangedFiles: files,
		AllFiles:     len(files),
	}

	// Classify files by layer
	nonTestFiles := 0
	nonDocFiles := 0

	for _, file := range files {
		// Count by layer
		if presentationPattern.MatchString(file) {
			stats.Presentation++
		}
		if businessPattern.MatchString(file) {
			stats.Business++
		}
		if infrastructurePattern.MatchString(file) {
			stats.Infrastructure++
		}
		if migrationPattern.MatchString(file) {
			stats.Migrations++
		}

		// Track test and doc files
		if !testPattern.MatchString(file) {
			nonTestFiles++
		}
		if !docsPattern.MatchString(file) {
			nonDocFiles++
		}
	}

	// Count layers with changes
	if stats.Presentation > 0 {
		stats.Layers++
	}
	if stats.Business > 0 {
		stats.Layers++
	}
	if stats.Infrastructure > 0 {
		stats.Layers++
	}

	// Calculate total files in layers
	stats.Total = stats.Presentation + stats.Business + stats.Infrastructure

	// Determine if only tests or docs changed
	stats.TestsOnly = nonTestFiles == 0
	stats.DocsOnly = nonDocFiles == 0

	// Apply decision logic
	result := &CheckResult{
		Stats: stats,
	}

	// MUST: Database migrations detected
	if stats.Migrations > 0 {
		result.Recommendation = "MUST"
		result.Reason = "Database migration detected"
		return result
	}

	// SKIP: Only test files changed
	if stats.TestsOnly {
		result.Recommendation = "SKIP"
		result.Reason = "Only test files changed"
		return result
	}

	// SKIP: Only documentation changed
	if stats.DocsOnly {
		result.Recommendation = "SKIP"
		result.Reason = "Only documentation changed"
		return result
	}

	// MUST: Multi-layer feature (≥2 layers AND ≥3 files)
	if stats.Layers >= 2 && stats.Total >= 3 {
		result.Recommendation = "MUST"
		result.Reason = fmt.Sprintf("Multi-layer feature (%d layers, %d files)", stats.Layers, stats.Total)
		return result
	}

	// SHOULD: Significant scope (≥5 files in single layer)
	if stats.Total >= 5 {
		result.Recommendation = "SHOULD"
		result.Reason = fmt.Sprintf("Significant scope (%d files in single layer)", stats.Total)
		return result
	}

	// SHOULD: Business logic service changes
	if stats.Business > 0 {
		result.Recommendation = "SHOULD"
		result.Reason = "Business logic service changes"
		return result
	}

	// SKIP: Minor changes
	result.Recommendation = "SKIP"
	layerText := "layer"
	if stats.Layers > 1 {
		layerText = "layers"
	}
	result.Reason = fmt.Sprintf("Minor changes (%d files, %d %s)", stats.Total, stats.Layers, layerText)
	return result
}

// FormatStats returns a human-readable summary of file statistics
func FormatStats(stats FileStats) string {
	return fmt.Sprintf("Changed: %d presentation, %d business, %d infrastructure (%d files, %d layers, %d migrations)",
		stats.Presentation, stats.Business, stats.Infrastructure, stats.Total, stats.Layers, stats.Migrations)
}

// FormatCheckResult returns a human-readable result with explanation
func FormatCheckResult(result *CheckResult, explain bool) string {
	var b strings.Builder

	// Always show stats
	b.WriteString(FormatStats(result.Stats))
	b.WriteString("\n\n")

	// Show recommendation
	b.WriteString("RECOMMENDATION: ")
	b.WriteString(result.Recommendation)
	b.WriteString(" update CHANGELOG\n")
	b.WriteString("Reason: ")
	b.WriteString(result.Reason)

	// Show detailed explanation if requested
	if explain {
		b.WriteString("\n\nExplanation:\n")
		b.WriteString(getExplanation(result))
	}

	return b.String()
}

// getExplanation returns a detailed explanation of the recommendation
func getExplanation(result *CheckResult) string {
	var b strings.Builder

	b.WriteString("The recommendation is based on the following analysis:\n\n")

	// Show file breakdown
	b.WriteString(fmt.Sprintf("Files changed: %d total, %d in tracked layers\n", result.Stats.AllFiles, result.Stats.Total))
	b.WriteString(fmt.Sprintf("  - Presentation layer: %d files\n", result.Stats.Presentation))
	b.WriteString(fmt.Sprintf("  - Business layer: %d files\n", result.Stats.Business))
	b.WriteString(fmt.Sprintf("  - Infrastructure layer: %d files\n", result.Stats.Infrastructure))
	b.WriteString(fmt.Sprintf("  - Migrations: %d files\n", result.Stats.Migrations))
	b.WriteString(fmt.Sprintf("  - Layers affected: %d\n\n", result.Stats.Layers))

	// Explain decision logic
	b.WriteString("Decision logic:\n")

	if result.Stats.Migrations > 0 {
		b.WriteString("✓ MUST - Database migrations are always significant changes\n")
	} else {
		b.WriteString("✗ No database migrations\n")
	}

	if result.Stats.TestsOnly {
		b.WriteString("✓ SKIP - Only test files changed\n")
	} else {
		b.WriteString("✗ Non-test files changed\n")
	}

	if result.Stats.DocsOnly {
		b.WriteString("✓ SKIP - Only documentation changed\n")
	} else {
		b.WriteString("✗ Non-documentation files changed\n")
	}

	if result.Stats.Layers >= 2 && result.Stats.Total >= 3 {
		b.WriteString("✓ MUST - Multi-layer feature (≥2 layers AND ≥3 files)\n")
	} else {
		b.WriteString("✗ Not a multi-layer feature\n")
	}

	if result.Stats.Total >= 5 {
		b.WriteString("✓ SHOULD - Significant scope (≥5 files)\n")
	} else {
		b.WriteString("✗ Scope not significant enough\n")
	}

	if result.Stats.Business > 0 {
		b.WriteString("✓ SHOULD - Business logic service changes\n")
	} else {
		b.WriteString("✗ No business logic changes\n")
	}

	return b.String()
}
