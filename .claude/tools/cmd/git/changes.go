package git

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/sdk-tools/internal/git"
	"github.com/iota-uz/iota-sdk/sdk-tools/internal/output"
	"github.com/spf13/cobra"
)

// Category represents a file category
type Category struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
	Count       int      `json:"count"`
}

var (
	changesJSONFlag bool
	changesOnlyFlag string
)

var changesCmd = &cobra.Command{
	Use:   "changes",
	Short: "Categorize changed files by type",
	Long:  `Categorize changed files by type for better commit grouping.`,
	RunE:  runChanges,
}

func init() {
	changesCmd.Flags().BoolVar(&changesJSONFlag, "json", false, "Output as JSON")
	changesCmd.Flags().StringVar(&changesOnlyFlag, "only", "", "Filter by category (comma-separated)")
}

func runChanges(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	formatter := output.New(os.Stdout, changesJSONFlag)

	files, err := git.GetChangedFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to get changed files: %w", err)
	}

	if len(files) == 0 {
		if formatter.IsJSON() {
			return formatter.PrintJSON(map[string]string{"message": "No changes detected"})
		}
		return formatter.PrintTextLn("No changes detected")
	}

	categories := categorizeFiles(files)

	if changesOnlyFlag != "" {
		categories = filterCategories(categories, changesOnlyFlag)
	}

	if formatter.IsJSON() {
		return formatter.PrintJSON(map[string]interface{}{
			"total":      len(files),
			"categories": categories,
		})
	}

	return printTextCategories(formatter, categories)
}

func categorizeFiles(files []string) []Category {
	categories := []Category{
		{Name: "Backend", Description: "Backend Code"},
		{Name: "Tests", Description: "Tests"},
		{Name: "Templates", Description: "Templates"},
		{Name: "Generated", Description: "Generated (must commit)"},
		{Name: "Migrations", Description: "Migrations"},
		{Name: "Translations", Description: "Translations"},
		{Name: "Config", Description: "Config/CI"},
		{Name: "Documentation", Description: "Documentation"},
		{Name: "Claude", Description: "Claude Code Config (use cc: prefix)"},
		{Name: "ShouldNotCommit", Description: "WARNING: Should NOT Commit"},
		{Name: "Other", Description: "Other"},
	}

	backendPattern := regexp.MustCompile(`\.go$`)
	testPattern := regexp.MustCompile(`_test\.go$`)
	templGeneratedPattern := regexp.MustCompile(`_templ\.go$`)
	templPattern := regexp.MustCompile(`\.templ$`)
	migrationPattern := regexp.MustCompile(`migrations/.*\.sql$`)
	translationPattern := regexp.MustCompile(`\.toml$`)
	configPattern := regexp.MustCompile(`\.(yml|yaml|json)$|Dockerfile|Makefile|go\.mod|go\.sum`)
	docsPattern := regexp.MustCompile(`\.md$`)
	claudePattern := regexp.MustCompile(`^\.claude/`)
	shouldNotCommitPattern := regexp.MustCompile(`\.(env|out|test|log|swp|swo|swn|dump|csv)$|^(FOLLOW_UP_ISSUES|PR-.*-REVIEW)\.md$`)

	for _, file := range files {
		if shouldNotCommitPattern.MatchString(file) {
			categories[9].Files = append(categories[9].Files, file)
			categories[9].Count++
			continue
		}

		if claudePattern.MatchString(file) {
			categories[8].Files = append(categories[8].Files, file)
			categories[8].Count++
			continue
		}

		if migrationPattern.MatchString(file) {
			categories[4].Files = append(categories[4].Files, file)
			categories[4].Count++
			continue
		}

		if backendPattern.MatchString(file) {
			if testPattern.MatchString(file) {
				categories[1].Files = append(categories[1].Files, file)
				categories[1].Count++
			} else if templGeneratedPattern.MatchString(file) {
				categories[3].Files = append(categories[3].Files, file)
				categories[3].Count++
			} else {
				categories[0].Files = append(categories[0].Files, file)
				categories[0].Count++
			}
			continue
		}

		if templPattern.MatchString(file) {
			categories[2].Files = append(categories[2].Files, file)
			categories[2].Count++
			continue
		}

		if translationPattern.MatchString(file) {
			categories[5].Files = append(categories[5].Files, file)
			categories[5].Count++
			continue
		}

		if configPattern.MatchString(file) {
			categories[6].Files = append(categories[6].Files, file)
			categories[6].Count++
			continue
		}

		if docsPattern.MatchString(file) && !claudePattern.MatchString(file) {
			categories[7].Files = append(categories[7].Files, file)
			categories[7].Count++
			continue
		}

		categories[10].Files = append(categories[10].Files, file)
		categories[10].Count++
	}

	result := make([]Category, 0)
	for _, cat := range categories {
		if cat.Count > 0 {
			result = append(result, cat)
		}
	}

	return result
}

func filterCategories(categories []Category, filter string) []Category {
	filters := strings.Split(strings.ToLower(filter), ",")
	filterMap := make(map[string]bool)
	for _, f := range filters {
		filterMap[strings.TrimSpace(f)] = true
	}

	result := make([]Category, 0)
	for _, cat := range categories {
		if filterMap[strings.ToLower(cat.Name)] {
			result = append(result, cat)
		}
	}

	return result
}

func printTextCategories(formatter *output.Formatter, categories []Category) error {
	if err := formatter.PrintTextLn(output.Bold("Changed Files by Category")); err != nil {
		return err
	}

	if err := formatter.PrintTextLn(output.Bold("=========================")); err != nil {
		return err
	}

	if err := formatter.PrintTextLn(""); err != nil {
		return err
	}

	for _, cat := range categories {
		var color func(string) string

		switch cat.Name {
		case "Backend":
			color = output.Cyan
		case "Tests":
			color = output.Green
		case "Templates":
			color = output.Blue
		case "Generated":
			color = output.Magenta
		case "Migrations":
			color = output.Yellow
		case "Translations":
			color = output.Blue
		case "Config":
			color = output.Yellow
		case "Documentation":
			color = output.Cyan
		case "Claude":
			color = output.Magenta
		case "ShouldNotCommit":
			color = output.Red
		default:
			color = func(s string) string { return s }
		}

		title := fmt.Sprintf("%s (%d files):", cat.Description, cat.Count)
		if err := formatter.PrintTextLn(color(title)); err != nil {
			return err
		}

		fileList := output.FormatFileList(cat.Files, 10)
		if err := formatter.PrintText(fileList); err != nil {
			return err
		}

		if err := formatter.PrintTextLn(""); err != nil {
			return err
		}
	}

	return nil
}
