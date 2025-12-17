package pr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iota-uz/iota-sdk/sdk-tools/internal/git"
	"github.com/iota-uz/iota-sdk/sdk-tools/internal/output"
	"github.com/spf13/cobra"
)

var (
	baseBranch string
	jsonOutput bool
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Analyze PR context",
	Long:  `Analyze PR context to understand what has changed.`,
	RunE:  runContext,
}

type contextOutput struct {
	Base       string   `json:"base"`
	Modules    []string `json:"modules"`
	Services   []string `json:"services"`
	Migrations []string `json:"migrations"`
}

func init() {
	contextCmd.Flags().StringVar(&baseBranch, "base", "staging", "Base branch to compare against")
	contextCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
}

func runContext(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	formatter := output.New(os.Stdout, jsonOutput)

	base, err := git.GetMergeBase(ctx, baseBranch)
	if err != nil {
		return fmt.Errorf("failed to get merge base: %w", err)
	}

	files, err := git.GetChangedFilesSince(ctx, base)
	if err != nil {
		return fmt.Errorf("failed to get changed files: %w", err)
	}

	modules := extractModules(files)
	services := extractServices(files)
	migrations := extractMigrations(files)

	if formatter.IsJSON() {
		return formatter.PrintJSON(contextOutput{
			Base:       base,
			Modules:    modules,
			Services:   services,
			Migrations: migrations,
		})
	}

	printSection(formatter, "Changed modules", modules, 10)
	printSection(formatter, "Changed services", services, 10)
	printSection(formatter, "Migrations", migrations, -1)

	return nil
}

func extractModules(files []string) []string {
	moduleMap := make(map[string]bool)

	for _, file := range files {
		if strings.HasPrefix(file, "modules/") {
			parts := strings.Split(file, string(filepath.Separator))
			if len(parts) >= 2 {
				module := parts[1]
				moduleMap[module] = true
			}
		}
	}

	modules := make([]string, 0, len(moduleMap))
	for module := range moduleMap {
		modules = append(modules, module)
	}

	return modules
}

func extractServices(files []string) []string {
	var services []string

	for _, file := range files {
		if strings.HasSuffix(file, "_service.go") {
			services = append(services, file)
		}
	}

	return services
}

func extractMigrations(files []string) []string {
	var migrations []string

	for _, file := range files {
		if strings.Contains(file, "migrations/") && strings.HasSuffix(file, ".sql") {
			migrations = append(migrations, file)
		}
	}

	return migrations
}

func printSection(formatter *output.Formatter, title string, items []string, limit int) {
	formatter.PrintTextLn("")
	formatter.PrintTextLn(output.Bold(title + ":"))

	if len(items) == 0 {
		formatter.PrintTextLn("  (none)")
		return
	}

	displayItems := items
	if limit > 0 && len(items) > limit {
		displayItems = items[:limit]
	}

	for _, item := range displayItems {
		formatter.PrintTextLn("  " + item)
	}

	if limit > 0 && len(items) > limit {
		formatter.PrintTextLn(fmt.Sprintf("  ... and %d more", len(items)-limit))
	}
}
