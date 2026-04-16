// Package commands provides this package.
package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"

	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
)

// CheckTrKeys validates translation key consistency across all configured locales.
// cfg and logger are resolved by the caller (typically a cobra RunE).
// When logger is nil a default logrus logger is used.
func CheckTrKeys(cfg *dbconfig.Config, logger *logrus.Logger, components ...composition.Component) error {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	app, pool, err := common.NewApplicationWithDefaults(cfg, logger, components...)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}
	defer pool.Close()

	messages := app.Bundle().Messages()

	// Store all keys for each locale
	allKeys := make(map[string]map[language.Tag]bool)
	locales := make([]language.Tag, 0)

	// First pass: collect all keys from locales
	for locale, message := range messages {
		if message == nil {
			continue
		}

		locales = append(locales, locale)

		for key := range message {
			if allKeys[key] == nil {
				allKeys[key] = make(map[language.Tag]bool)
			}
			allKeys[key][locale] = true
		}
	}

	// No locales found
	if len(locales) == 0 {
		return fmt.Errorf("no locales found in the application bundle")
	}

	// Second pass: check for missing keys
	missingKeys := false
	for key, localeMap := range allKeys {
		if len(localeMap) != len(locales) {
			missingKeys = true

			present := make([]string, 0)
			missing := make([]string, 0)

			for _, locale := range locales {
				if localeMap[locale] {
					present = append(present, locale.String())
				} else {
					missing = append(missing, locale.String())
				}
			}

			logger.WithFields(logrus.Fields{
				"key":          key,
				"present_in":   strings.Join(present, ", "),
				"missing_from": strings.Join(missing, ", "),
			}).Error("Translation key mismatch")
		}
	}

	if missingKeys {
		return fmt.Errorf("some translation keys are not consistent across all locales, see logs for details")
	}

	logger.WithFields(logrus.Fields{
		"locale_count": len(locales),
		"key_count":    len(allKeys),
	}).Info("All translation keys are consistent across all locales")

	if err := checkForUndefinedKeys(allKeys, logger); err != nil {
		return err
	}

	return nil
}

func extractKeysFromGoFile(path string, rootPath string, fset *token.FileSet) (map[string][]string, error) {
	keysWithLocations := make(map[string][]string)

	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		methodName := sel.Sel.Name
		if methodName != "T" && methodName != "TSafe" {
			return true
		}

		if len(call.Args) == 0 {
			return true
		}

		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			key := strings.Trim(lit.Value, `"`)
			relPath, _ := filepath.Rel(rootPath, path)
			position := fset.Position(call.Pos())
			location := fmt.Sprintf("%s:%d", relPath, position.Line)
			keysWithLocations[key] = append(keysWithLocations[key], location)
		}

		return true
	})

	return keysWithLocations, nil
}

func extractKeysFromTemplFile(path string, rootPath string) (map[string][]string, error) {
	keysWithLocations := make(map[string][]string)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	tCallRegex := regexp.MustCompile(`\.T(?:Safe)?\s*\(\s*["']([^"']+)["']`)

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		matches := tCallRegex.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				key := match[1]
				relPath, _ := filepath.Rel(rootPath, path)
				location := fmt.Sprintf("%s:%d", relPath, lineNum)
				keysWithLocations[key] = append(keysWithLocations[key], location)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return keysWithLocations, nil
}

func extractTranslationKeys(rootPath string) (map[string][]string, error) {
	keysWithLocations := make(map[string][]string)
	fset := token.NewFileSet()

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		var fileKeys map[string][]string
		var fileErr error

		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			fileKeys, fileErr = extractKeysFromGoFile(path, rootPath, fset)
		} else if strings.HasSuffix(path, ".templ") {
			fileKeys, fileErr = extractKeysFromTemplFile(path, rootPath)
		} else {
			return nil
		}

		if fileErr != nil {
			return fileErr
		}

		for key, locations := range fileKeys {
			keysWithLocations[key] = append(keysWithLocations[key], locations...)
		}

		return nil
	})

	return keysWithLocations, err
}

// WriteRequiredKeysFile extracts T/TSafe translation keys from the codebase under projectRoot
// and writes the unique, sorted list to outputPath as a JSON array of strings.
func WriteRequiredKeysFile(projectRoot, outputPath string) error {
	keysWithLocations, err := extractTranslationKeys(projectRoot)
	if err != nil {
		return fmt.Errorf("extract translation keys: %w", err)
	}
	keys := make([]string, 0, len(keysWithLocations))
	for key := range keysWithLocations {
		if strings.HasSuffix(key, ".") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal required keys: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}
	return nil
}

func checkForUndefinedKeys(allKeys map[string]map[language.Tag]bool, logger *logrus.Logger) error {
	logger.Info("Scanning codebase for T() and TSafe() calls...")

	rootPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	usedKeys, err := extractTranslationKeys(rootPath)
	if err != nil {
		return fmt.Errorf("failed to extract translation keys from codebase: %w", err)
	}

	undefinedKeys := false
	for key, locations := range usedKeys {
		if strings.HasSuffix(key, ".") {
			continue
		}

		if _, exists := allKeys[key]; !exists {
			undefinedKeys = true
			logger.WithFields(logrus.Fields{
				"key":     key,
				"used_in": strings.Join(locations, ", "),
			}).Error("Translation key used in code but not defined in any locale file")
		}
	}

	if undefinedKeys {
		return fmt.Errorf("some translation keys are used in code but not defined in any locale files, see logs for details")
	}

	logger.WithFields(logrus.Fields{
		"used_keys_count": len(usedKeys),
	}).Info("All used translation keys are properly defined")

	return nil
}
