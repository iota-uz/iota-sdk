// Package commands provides this package.
package commands

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"

	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// CheckTrKeys validates translation key consistency across the locale files
// shipped by the given components. It does NOT boot the application — no
// database, Redis, NATS, S3 or DI container is initialized — so it is safe
// to run as a fast pre-commit / CI check.
//
// Locale files are discovered through the optional composition.LocaleSource
// interface on each component. Components that do not implement it are
// skipped silently (they ship no locales for this check to validate).
//
// The check performs three things:
//  1. Parses every embedded locale file. The underlying go-i18n parser
//     surfaces an error if a TOML/JSON table mixes reserved plural keys
//     (one/other/description/hash/…) with regular sub-keys.
//  2. Reports keys that exist in some locales but are missing from others.
//  3. Walks the current working directory for T()/TSafe() call sites and
//     reports any translation key that is used in code but never defined
//     in a locale file.
func CheckTrKeys(allowedLanguages []string, components ...composition.Component) error {
	logger := newDefaultLogger()

	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	if err := loadComponentLocales(bundle, components); err != nil {
		return err
	}

	messages := bundle.Messages()
	if len(messages) == 0 {
		return fmt.Errorf("no locale files were discovered from the supplied components — ensure each component implements composition.LocaleSource")
	}

	allowedLocales, err := parseAllowedLanguages(allowedLanguages, messages)
	if err != nil {
		return err
	}

	allKeys, locales := collectKeys(messages, allowedLocales)
	if len(locales) == 0 {
		return fmt.Errorf("no locales found in the application bundle")
	}

	if missing := reportMissingKeys(logger, allKeys, locales); missing {
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

// loadComponentLocales parses every locale file contributed by the
// LocaleSource components into the supplied bundle. Each component's locale
// files are loaded in registration order; duplicate filenames across
// components are tolerated (go-i18n merges them into the same locale).
func loadComponentLocales(bundle *i18n.Bundle, components []composition.Component) error {
	for _, comp := range components {
		src, ok := comp.(composition.LocaleSource)
		if !ok {
			continue
		}
		for _, embedFS := range src.LocaleFS() {
			if embedFS == nil {
				continue
			}
			if err := loadEmbedFSIntoBundle(bundle, embedFS); err != nil {
				return fmt.Errorf("component %q: %w", componentName(comp), err)
			}
		}
	}
	return nil
}

func loadEmbedFSIntoBundle(bundle *i18n.Bundle, embedFS *embed.FS) error {
	return fs.WalkDir(embedFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".toml" && ext != ".json" {
			return nil
		}
		data, err := embedFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if _, err := bundle.ParseMessageFileBytes(data, filepath.Base(path)); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		return nil
	})
}

func componentName(comp composition.Component) string {
	defer func() { _ = recover() }()
	return comp.Descriptor().Name
}

func parseAllowedLanguages(allowed []string, messages map[language.Tag]map[string]*i18n.MessageTemplate) (map[string]language.Tag, error) {
	out := make(map[string]language.Tag, len(allowed))
	for _, code := range allowed {
		tag, err := language.Parse(code)
		if err != nil {
			return nil, fmt.Errorf("invalid language code in whitelist: %s: %w", code, err)
		}
		if messages[tag] == nil {
			return nil, fmt.Errorf("language %s (%s) is in whitelist but not found in bundle", code, tag)
		}
		out[code] = tag
	}
	return out, nil
}

func collectKeys(
	messages map[language.Tag]map[string]*i18n.MessageTemplate,
	allowedLocales map[string]language.Tag,
) (map[string]map[language.Tag]bool, []language.Tag) {
	allKeys := make(map[string]map[language.Tag]bool)
	locales := make([]language.Tag, 0)

	for locale, message := range messages {
		if message == nil {
			continue
		}
		if len(allowedLocales) > 0 {
			isAllowed := false
			for _, tag := range allowedLocales {
				if locale == tag {
					isAllowed = true
					break
				}
			}
			if !isAllowed {
				continue
			}
		}

		locales = append(locales, locale)
		for key := range message {
			if allKeys[key] == nil {
				allKeys[key] = make(map[language.Tag]bool)
			}
			allKeys[key][locale] = true
		}
	}
	return allKeys, locales
}

func reportMissingKeys(logger *logrus.Logger, allKeys map[string]map[language.Tag]bool, locales []language.Tag) bool {
	missing := false
	for key, localeMap := range allKeys {
		if len(localeMap) == len(locales) {
			continue
		}
		missing = true
		present := make([]string, 0, len(locales))
		absent := make([]string, 0, len(locales))
		for _, locale := range locales {
			if localeMap[locale] {
				present = append(present, locale.String())
			} else {
				absent = append(absent, locale.String())
			}
		}
		logger.WithFields(logrus.Fields{
			"key":          key,
			"present_in":   strings.Join(present, ", "),
			"missing_from": strings.Join(absent, ", "),
		}).Error("Translation key mismatch")
	}
	return missing
}

// newDefaultLogger returns a stdout logger so the command does not depend on
// configuration.Use() (which loads .env and validates rate-limit / 2FA
// settings — neither relevant to a static key-parity check).
func newDefaultLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(os.Stdout)
	l.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: false,
		DisableQuote:  true,
	})
	return l
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
// outputPath is created or overwritten. projectRoot is typically the application root (e.g. where go.mod lives).
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
