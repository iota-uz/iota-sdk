package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/lens/color"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	root, err := findModuleRoot()
	if err != nil {
		return err
	}
	kinds := document.PanelKinds()
	panelVariants := make(map[string][]string, len(kinds))
	for kind, metadata := range kinds {
		panelVariants[string(kind)] = metadata.ConfigFields
	}
	files, err := generate(config{
		dir:            root,
		packagePattern: "./pkg/lens/document",
		rootType:       "DashboardDocument",
		additionalTypes: []string{
			"QueryRequest", "QueryResponse", "QueryErrorResponse",
			"PanelRequest", "PanelResponse", "PanelBatchRequest", "PanelBatchResponse", "PanelBatchStreamEvent",
			"DrawerResolveRequest", "DrawerResolveResponse",
		},
		versionConstant: "ContractVersion",
		palette: paletteConfig{
			series:  color.Series(),
			neutral: color.Neutral,
		},
		discriminatedTypes: map[string]discriminatedTypeConfig{
			"Panel": {field: "kind", variants: panelVariants},
		},
	})
	if err != nil {
		return fmt.Errorf("generate Lens contract: %w", err)
	}
	outputDir := filepath.Join(root, "web", "lens", "src", "contract")
	if err := writeGeneratedDirectory(outputDir, files); err != nil {
		return fmt.Errorf("write Lens contract: %w", err)
	}
	return nil
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}
