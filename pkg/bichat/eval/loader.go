package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func LoadSuite(path string) (TestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TestSuite{}, fmt.Errorf("failed to read test suite file: %w", err)
	}

	var suite TestSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		return TestSuite{}, fmt.Errorf("failed to parse suite JSON: %w", err)
	}

	if err := ValidateSuite(suite); err != nil {
		return TestSuite{}, err
	}

	return suite, nil
}

func LoadSuiteFromDir(dir string) (TestSuite, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return TestSuite{}, fmt.Errorf("failed to read directory: %w", err)
	}

	var all []TestCase
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		suite, err := LoadSuite(filePath)
		if err != nil {
			return TestSuite{}, fmt.Errorf("failed to load %s: %w", filePath, err)
		}
		all = append(all, suite.Tests...)
	}

	suite := TestSuite{Tests: all}
	if err := ValidateSuite(suite); err != nil {
		return TestSuite{}, err
	}
	return suite, nil
}

func LoadTestCases(path string) ([]TestCase, error) {
	suite, err := LoadSuite(path)
	if err != nil {
		return nil, err
	}
	return suite.Tests, nil
}

func LoadTestCasesFromDir(dir string) ([]TestCase, error) {
	suite, err := LoadSuiteFromDir(dir)
	if err != nil {
		return nil, err
	}
	return suite.Tests, nil
}

func FilterByTag(cases []TestCase, tag string) []TestCase {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return cases
	}

	filtered := make([]TestCase, 0, len(cases))
	for _, tc := range cases {
		for _, t := range tc.Tags {
			if strings.EqualFold(strings.TrimSpace(t), tag) {
				filtered = append(filtered, tc)
				break
			}
		}
	}
	return filtered
}

func FilterByCategory(cases []TestCase, category string) []TestCase {
	category = strings.TrimSpace(category)
	if category == "" {
		return cases
	}

	filtered := make([]TestCase, 0, len(cases))
	for _, tc := range cases {
		if strings.EqualFold(strings.TrimSpace(tc.Category), category) {
			filtered = append(filtered, tc)
		}
	}
	return filtered
}

func ValidateSuite(suite TestSuite) error {
	if len(suite.Tests) == 0 {
		return fmt.Errorf("suite has no tests")
	}

	ids := make(map[string]struct{}, len(suite.Tests))
	for i := range suite.Tests {
		tc := suite.Tests[i]
		if strings.TrimSpace(tc.ID) == "" {
			return fmt.Errorf("test[%d]: id is required", i)
		}
		if _, exists := ids[tc.ID]; exists {
			return fmt.Errorf("test[%d]: duplicate id %q", i, tc.ID)
		}
		ids[tc.ID] = struct{}{}
		if strings.TrimSpace(tc.DatasetID) == "" {
			return fmt.Errorf("test[%d]: dataset_id is required", i)
		}
		if len(tc.Turns) == 0 {
			return fmt.Errorf("test[%d]: at least one turn is required", i)
		}
		for j := range tc.Turns {
			if strings.TrimSpace(tc.Turns[j].Prompt) == "" {
				return fmt.Errorf("test[%d].turns[%d]: prompt is required", i, j)
			}
		}
	}

	return nil
}

func SortCasesByID(cases []TestCase) {
	sort.Slice(cases, func(i, j int) bool {
		return cases[i].ID < cases[j].ID
	})
}
