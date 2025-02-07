package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
	"gopkg.in/yaml.v3"
)

var JSONLinter = &analysis.Analyzer{
	Name: "iotalinter",
	Doc:  "validates that all .json files are valid JSON",
	Run:  run,
}

type LintError struct {
	File    string
	Line    int
	Message string
}

func (e *LintError) Error() string {
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Message)
}

type Config struct {
	ExcludeDirs        []string `yaml:"exclude-dirs"`
	CheckZeroByteFiles bool     `yaml:"check-zero-byte-files"`
}

type LinterConfig struct {
	LintersSettings struct {
		IotaSDK Config `yaml:"iotalinter"`
	} `yaml:"linters-settings"`
}

func loadConfig() (*Config, error) {
	configFiles := []string{".golangci.yml", ".golangci.yaml"}
	var config LinterConfig

	for _, file := range configFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("error parsing config file %s: %v", file, err)
		}
		if !config.LintersSettings.IotaSDK.CheckZeroByteFiles &&
			!strings.Contains(string(data), "check-zero-byte-files") {
			config.LintersSettings.IotaSDK.CheckZeroByteFiles = true
		}
		return &config.LintersSettings.IotaSDK, nil
	}

	// Return default config if no config file found
	return &Config{
		ExcludeDirs:        []string{"apex"},
		CheckZeroByteFiles: true,
	}, nil
}

// Add a mutex to protect our key operations
type KeyStore struct {
	sync.Mutex
	store map[string][]JSONKeys
}

var keyStore = &KeyStore{
	store: make(map[string][]JSONKeys),
}

type JSONKeys struct {
	Keys map[string]bool
	Path string
}

func collectKeys(obj interface{}, prefix string, keys map[string]bool) {
	switch v := obj.(type) {
	case map[string]interface{}:
		for k, val := range v {
			fullKey := k
			if prefix != "" {
				fullKey = prefix + "." + k
			}
			keys[fullKey] = true
			collectKeys(val, fullKey, keys)
		}
	case []interface{}:
		for _, val := range v {
			collectKeys(val, prefix, keys)
		}
	}
}

func lint(file string, content []byte, config *Config) ([]LintError, error) {
	var errors []LintError

	if len(content) == 0 {
		if config.CheckZeroByteFiles {
			errors = append(errors, LintError{
				File:    file,
				Line:    1,
				Message: "empty file",
			})
		}
		return errors, nil
	}

	var js interface{}
	if err := json.Unmarshal(content, &js); err != nil {
		lineNum := 1
		if jsonErr, ok := err.(*json.SyntaxError); ok {
			lineNum = bytes.Count(content[:jsonErr.Offset], []byte("\n")) + 1
		}
		errors = append(errors, LintError{
			File:    file,
			Line:    lineNum,
			Message: err.Error(),
		})
		return errors, nil
	}

	// Collect keys for this file
	keys := make(map[string]bool)
	collectKeys(js, "", keys)

	// Group files by directory for comparison
	dir := filepath.Dir(file)

	keyStore.Lock()
	defer keyStore.Unlock()

	// Get or initialize the slice for this directory
	dirKeys := keyStore.store[dir]

	// Append new keys
	dirKeys = append(dirKeys, JSONKeys{Keys: keys, Path: file})
	keyStore.store[dir] = dirKeys

	// Compare with other files in same directory
	if len(dirKeys) > 1 {
		for k := range keys {
			for _, other := range dirKeys {
				if other.Path != file && !other.Keys[k] {
					errors = append(errors, LintError{
						File:    file,
						Line:    1,
						Message: fmt.Sprintf("key '%s' exists here but is missing in %s", k, other.Path),
					})
				}
			}
		}
	}

	return errors, nil
}

func run(pass *analysis.Pass) (interface{}, error) {
	config, err := loadConfig()
	if err != nil {
		return nil, err
	}

	// Reset the store
	keyStore.Lock()
	keyStore.store = make(map[string][]JSONKeys)
	keyStore.Unlock()

	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Skip excluded directories
			for _, excludeDir := range config.ExcludeDirs {
				if strings.Contains(path, excludeDir) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Skip if not a .json file
		if filepath.Ext(path) != ".json" {
			return nil
		}

		// Read file contents
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading file %s: %v", path, err)
		}

		// Use lint function to check the file
		if lintErrors, err := lint(path, data, config); err != nil {
			return err
		} else {
			for _, e := range lintErrors {
				pass.Reportf(0, "%s", e.Error())
			}
		}

		return nil
	})

	return nil, err
}

func main() {
	singlechecker.Main(JSONLinter)
}
