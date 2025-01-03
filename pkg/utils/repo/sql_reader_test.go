package repo_test

import (
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
	"reflect"
	"testing"
)

func TestParseSQLQueries(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        map[string]string
		expectError bool
	}{
		{
			name: "Valid SQL with multiple queries",
			input: `
				-- name:select 
				SELECT wp.id, wp.title FROM warehouse_positions wp;

				-- name:count 
				SELECT COUNT(*) AS count FROM warehouse_positions;`,
			want: map[string]string{
				"select": "SELECT wp.id, wp.title FROM warehouse_positions wp",
				"count":  "SELECT COUNT(*) AS count FROM warehouse_positions",
			},
			expectError: false,
		},
		{
			name: "Empty input",
			input: `
			`,
			want:        map[string]string{},
			expectError: false,
		},
		{
			name:        "Input with missing query name",
			input:       `SELECT * FROM warehouse_positions;`,
			want:        map[string]string{},
			expectError: false,
		},
		{
			name: "Malformed SQL with invalid format",
			input: `
			-- name: select
			SELECT wp.id, wp.title FROM warehouse_positions wp;
			-- name: count
			SELECT COUNT(*) AS count FROM warehouse_positions;`,
			want: map[string]string{
				"select": "SELECT wp.id, wp.title FROM warehouse_positions wp",
				"count":  "SELECT COUNT(*) AS count FROM warehouse_positions",
			},
			expectError: false,
		},
		{
			name: "No queries defined",
			input: `
-- Some comment here
-- Another comment
			`,
			want:        map[string]string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.ParseSQLQueries(tt.input)

			if (err != nil) != tt.expectError {
				t.Errorf("parseSQLQueries() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("have %v, want %v", got, tt.want)
			}
		})
	}
}
