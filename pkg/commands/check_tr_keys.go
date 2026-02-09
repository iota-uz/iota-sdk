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

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func CheckTrKeys(allowedLanguages []string, mods ...application.Module) error {
	conf := configuration.Use()
	app, pool, err := common.NewApplicationWithDefaults(mods...)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}
	defer pool.Close()

	messages := app.Bundle().Messages()

	// If allowedLanguages is provided, create a whitelist map for validation
	var allowedLocales map[string]language.Tag
	if len(allowedLanguages) > 0 {
		allowedLocales = make(map[string]language.Tag)
		for _, code := range allowedLanguages {
			// Parse language code to tag
			tag, err := language.Parse(code)
			if err != nil {
				return fmt.Errorf("invalid language code in whitelist: %s: %w", code, err)
			}
			allowedLocales[code] = tag
		}

		// Validate that all allowed languages exist in the bundle
		for code, tag := range allowedLocales {
			if messages[tag] == nil {
				return fmt.Errorf("language %s (%s) is in whitelist but not found in bundle", code, tag)
			}
		}
	}

	// Store all keys for each locale
	allKeys := make(map[string]map[language.Tag]bool)
	locales := make([]language.Tag, 0)

	// First pass: collect all keys from locales (filtered by allowedLanguages if provided)
	for locale, message := range messages {
		if message == nil {
			continue
		}

		// If allowedLanguages is specified, only process those locales
		if len(allowedLocales) > 0 {
			isAllowed := false
			for _, allowedTag := range allowedLocales {
				if locale == allowedTag {
					isAllowed = true
					break
				}
			}
			if !isAllowed {
				continue
			}
		}

		locales = append(locales, locale)

		// Process keys in this locale
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
		// If the key is not present in all locales
		if len(localeMap) != len(locales) {
			missingKeys = true

			// Find which locales have the key
			present := make([]string, 0)
			missing := make([]string, 0)

			for _, locale := range locales {
				if localeMap[locale] {
					present = append(present, locale.String())
				} else {
					missing = append(missing, locale.String())
				}
			}

			// Log detailed error about the missing key using WithFields
			conf.Logger().WithFields(logrus.Fields{
				"key":          key,
				"present_in":   strings.Join(present, ", "),
				"missing_from": strings.Join(missing, ", "),
			}).Error("Translation key mismatch")
		}
	}

	if missingKeys {
		return fmt.Errorf("some translation keys are not consistent across all locales, see logs for details")
	}

	conf.Logger().WithFields(logrus.Fields{
		"locale_count": len(locales),
		"key_count":    len(allKeys),
	}).Info("All translation keys are consistent across all locales")

	if err := checkForUndefinedKeys(allKeys, conf.Logger()); err != nil {
		return err
	}

	return nil
}

func extractKeysFromGoFile(path string, rootPath string, fset *token.FileSet) (map[string][]string, error) {
	keysWithLocations := make(map[string][]string)

	// Parse the Go file
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	// Visit all nodes in the AST
	ast.Inspect(node, func(n ast.Node) bool {
		// Look for function calls
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if it's a method call (selector expression)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// Check if method name is T or TSafe
		methodName := sel.Sel.Name
		if methodName != "T" && methodName != "TSafe" {
			return true
		}

		// Extract the first argument (the translation key)
		if len(call.Args) == 0 {
			return true
		}

		// Check if first argument is a string literal
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			// Remove quotes from string literal
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

	// Regex to match .T("key") or .TSafe("key") calls
	// Matches both single and double quotes
	tCallRegex := regexp.MustCompile(`\.T(?:Safe)?\s*\(\s*["']([^"']+)["']`)

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Find all matches in this line
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

		// Skip non-relevant directories
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		var fileKeys map[string][]string
		var fileErr error

		// Process Go files (excluding tests)
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			fileKeys, fileErr = extractKeysFromGoFile(path, rootPath, fset)
		} else if strings.HasSuffix(path, ".templ") {
			// Process templ files
			fileKeys, fileErr = extractKeysFromTemplFile(path, rootPath)
		} else {
			return nil
		}

		if fileErr != nil {
			return fileErr
		}

		// Merge keys from this file
		for key, locations := range fileKeys {
			keysWithLocations[key] = append(keysWithLocations[key], locations...)
		}

		return nil
	})

	return keysWithLocations, err
}

// WriteRequiredKeysFile extracts T/TSafe translation keys from the codebase under projectRoot
// and writes the unique, sorted list to outputPath as a JSON array of strings.
// outputPath is created or overwritten. projectRoot is typically the application root (e.g. where go.mod lives).
func WriteRequiredKeysFile(projectRoot, outputPath string) error {
	keysWithLocations, err := extractTranslationKeys(projectRoot)
	if err != nil {
		return fmt.Errorf("extract translation keys: %w", err)
	}
	keys := make([]string, 0, len(keysWithLocations))
	for key := range keysWithLocations {
		// Skip dynamic key suffixes (e.g. "Countries.") same as checkForUndefinedKeys
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

	// Get current working directory as root path for scanning
	rootPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	usedKeys, err := extractTranslationKeys(rootPath)
	if err != nil {
		return fmt.Errorf("failed to extract translation keys from codebase: %w", err)
	}

	// Check for keys used in code but not in any locale
	undefinedKeys := false
	for key, locations := range usedKeys {
		// Skip keys that end with "." - these are likely dynamic keys built with string concatenation
		// Example: pageCtx.T("Countries." + countryCode) will be detected as "Countries."
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
