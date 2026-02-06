package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadTestCases loads test cases from a JSON file.
// The file should contain an array of TestCase objects.
func LoadTestCases(path string) ([]TestCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read test cases file: %w", err)
	}

	var cases []TestCase
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("failed to parse test cases JSON: %w", err)
	}

	return cases, nil
}

// LoadTestCasesFromDir loads all .json files from a directory.
// Each file should contain an array of TestCase objects.
// Returns all test cases from all files combined.
func LoadTestCasesFromDir(dir string) ([]TestCase, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var allCases []TestCase

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .json files
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		cases, err := LoadTestCases(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", filePath, err)
		}

		allCases = append(allCases, cases...)
	}

	return allCases, nil
}

// FilterByTag filters test cases by tag.
// Returns only test cases that have the specified tag.
func FilterByTag(cases []TestCase, tag string) []TestCase {
	if tag == "" {
		return cases
	}

	filtered := make([]TestCase, 0)

	for _, tc := range cases {
		for _, t := range tc.Tags {
			if t == tag {
				filtered = append(filtered, tc)
				break
			}
		}
	}

	return filtered
}

// FilterByCategory filters test cases by category.
// Returns only test cases that match the specified category.
func FilterByCategory(cases []TestCase, category string) []TestCase {
	if category == "" {
		return cases
	}

	filtered := make([]TestCase, 0)

	for _, tc := range cases {
		if tc.Category == category {
			filtered = append(filtered, tc)
		}
	}

	return filtered
}
