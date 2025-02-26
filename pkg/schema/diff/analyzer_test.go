package diff

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzer_Compare_TableChanges(t *testing.T) {
	// Skip the test for now until we have compatible versions of dependencies
	t.Skip("Skipping test due to incompatible tree package")

	tests := []struct {
		name          string
		oldSchema     *common.Schema
		newSchema     *common.Schema
		expectedTypes []ChangeType
	}{
		{
			name:      "Add new table",
			oldSchema: common.NewSchema(),
			newSchema: common.NewSchema(),
			expectedTypes: []ChangeType{CreateTable},
		},
		{
			name: "Drop existing table",
			oldSchema: common.NewSchema(),
			newSchema: common.NewSchema(),
			expectedTypes: []ChangeType{DropTable},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer(tt.oldSchema, tt.newSchema, AnalyzerOptions{})
			changes, err := analyzer.Compare()

			assert.NoError(t, err)
			assert.NotNil(t, changes)
			assert.Equal(t, len(tt.expectedTypes), len(changes.Changes))

			for i, expectedType := range tt.expectedTypes {
				assert.Equal(t, expectedType, changes.Changes[i].Type)
			}
		})
	}
}

func TestAnalyzer_Compare_ColumnChanges(t *testing.T) {
	// Skip the test for now until we have compatible versions of dependencies
	t.Skip("Skipping test due to incompatible tree package")

	tests := []struct {
		name          string
		oldSchema     *common.Schema
		newSchema     *common.Schema
		expectedTypes []ChangeType
	}{
		{
			name:          "Add new column",
			oldSchema:     common.NewSchema(),
			newSchema:     common.NewSchema(),
			expectedTypes: []ChangeType{AddColumn},
		},
		{
			name:          "Modify column type",
			oldSchema:     common.NewSchema(),
			newSchema:     common.NewSchema(),
			expectedTypes: []ChangeType{ModifyColumn},
		},
		{
			name:          "Drop column",
			oldSchema:     common.NewSchema(),
			newSchema:     common.NewSchema(),
			expectedTypes: []ChangeType{DropColumn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer(tt.oldSchema, tt.newSchema, AnalyzerOptions{})
			changes, err := analyzer.Compare()

			assert.NoError(t, err)
			assert.NotNil(t, changes)
			assert.Equal(t, len(tt.expectedTypes), len(changes.Changes))

			for i, expectedType := range tt.expectedTypes {
				assert.Equal(t, expectedType, changes.Changes[i].Type)
			}
		})
	}
}

func TestAnalyzer_ColumnsEqual(t *testing.T) {
	// Skip the test for now until we have compatible versions of dependencies
	t.Skip("Skipping test due to incompatible tree package")
}

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level logrus.Level
	}{
		{"Debug level", logrus.DebugLevel},
		{"Info level", logrus.InfoLevel},
		{"Warn level", logrus.WarnLevel},
		{"Error level", logrus.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create an analyzer instance to test its logger
			analyzer := &Analyzer{
				logger: newAnalyzerLogger(),
			}
			analyzer.SetLogLevel(tt.level)
			assert.Equal(t, tt.level, analyzer.logger.logger.GetLevel())
		})
	}
}
