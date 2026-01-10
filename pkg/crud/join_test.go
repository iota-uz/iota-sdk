package crud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoinType_String(t *testing.T) {
	tests := []struct {
		name     string
		joinType JoinType
		expected string
	}{
		{"inner join", JoinTypeInner, "INNER JOIN"},
		{"left join", JoinTypeLeft, "LEFT JOIN"},
		{"right join", JoinTypeRight, "RIGHT JOIN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.joinType.String())
		})
	}
}

func TestJoinClause_Validate(t *testing.T) {
	tests := []struct {
		name      string
		clause    JoinClause
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid inner join",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				TableAlias:  "r",
				LeftColumn:  "users.role_id",
				RightColumn: "r.id",
			},
			expectErr: false,
		},
		{
			name: "missing table",
			clause: JoinClause{
				Type:        JoinTypeLeft,
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "join table cannot be empty",
		},
		{
			name: "missing left column",
			clause: JoinClause{
				Type:        JoinTypeLeft,
				Table:       "roles",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "join left column cannot be empty",
		},
		{
			name: "missing right column",
			clause: JoinClause{
				Type:       JoinTypeRight,
				Table:      "roles",
				LeftColumn: "users.role_id",
			},
			expectErr: true,
			errMsg:    "join right column cannot be empty",
		},
		{
			name: "invalid table alias format - starts with number",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				TableAlias:  "1role",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "invalid table alias specification",
		},
		{
			name: "invalid table alias format - special characters",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				TableAlias:  "r@le",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "invalid table alias specification",
		},
		{
			name: "invalid table alias format - contains space",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				TableAlias:  "r ole",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "invalid table alias specification",
		},
		{
			name: "valid clause without table alias",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.clause.Validate()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestJoinClause_Validate_SQLInjectionPrevention(t *testing.T) {
	tests := []struct {
		name      string
		clause    JoinClause
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid clause",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				TableAlias:  "r",
				LeftColumn:  "users.role_id",
				RightColumn: "r.id",
			},
			expectErr: false,
		},
		{
			name: "SQL injection in table name",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles; DROP TABLE users--",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "SQL injection in left column",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				LeftColumn:  "users.id UNION SELECT password",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "SQL injection in right column",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id; DELETE FROM users--",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "SQL injection in table alias",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				TableAlias:  "r; DROP TABLE users--",
				LeftColumn:  "users.role_id",
				RightColumn: "r.id",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "invalid table name format",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "123invalid",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "invalid table specification",
		},
		{
			name: "invalid column format",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				LeftColumn:  "users.role_id",
				RightColumn: "COUNT(roles.id)",
			},
			expectErr: true,
			errMsg:    "invalid right column specification",
		},
		{
			name: "comment injection in table",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles--",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "comment injection in left column",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				LeftColumn:  "users.role_id--",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "block comment injection",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id /* DROP TABLE users */",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "union injection in column",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id UNION ALL SELECT password FROM admin",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "insert injection in table",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles; INSERT INTO admin VALUES(1)",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "update injection in alias",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				TableAlias:  "r; UPDATE users SET role='admin'",
				LeftColumn:  "users.role_id",
				RightColumn: "r.id",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "create injection",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles; CREATE TABLE malicious",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "alter injection",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id; ALTER TABLE users",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "exec injection",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				TableAlias:  "r; EXEC sp_executesql",
				LeftColumn:  "users.role_id",
				RightColumn: "r.id",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
		{
			name: "execute injection",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles; EXECUTE malicious_proc",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expectErr: true,
			errMsg:    "dangerous SQL keyword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.clause.Validate()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestJoinClause_ToSQL(t *testing.T) {
	tests := []struct {
		name     string
		clause   JoinClause
		expected string
	}{
		{
			name: "inner join with alias",
			clause: JoinClause{
				Type:        JoinTypeInner,
				Table:       "roles",
				TableAlias:  "r",
				LeftColumn:  "users.role_id",
				RightColumn: "r.id",
			},
			expected: "INNER JOIN roles r ON users.role_id = r.id",
		},
		{
			name: "left join without alias",
			clause: JoinClause{
				Type:        JoinTypeLeft,
				Table:       "roles",
				LeftColumn:  "users.role_id",
				RightColumn: "roles.id",
			},
			expected: "LEFT JOIN roles ON users.role_id = roles.id",
		},
		{
			name: "right join with alias",
			clause: JoinClause{
				Type:        JoinTypeRight,
				Table:       "departments",
				TableAlias:  "d",
				LeftColumn:  "users.dept_id",
				RightColumn: "d.id",
			},
			expected: "RIGHT JOIN departments d ON users.dept_id = d.id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.clause.ToSQL()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinOptions_Validate(t *testing.T) {
	tests := []struct {
		name      string
		options   *JoinOptions
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid options",
			options: &JoinOptions{
				Joins: []JoinClause{
					{
						Type:        JoinTypeInner,
						Table:       "roles",
						LeftColumn:  "users.role_id",
						RightColumn: "roles.id",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid join clause",
			options: &JoinOptions{
				Joins: []JoinClause{
					{
						Type:        JoinTypeLeft,
						Table:       "", // Invalid: empty table
						LeftColumn:  "users.role_id",
						RightColumn: "roles.id",
					},
				},
			},
			expectErr: true,
			errMsg:    "join clause 0",
		},
		{
			name: "multiple clauses with one invalid",
			options: &JoinOptions{
				Joins: []JoinClause{
					{
						Type:        JoinTypeInner,
						Table:       "roles",
						LeftColumn:  "users.role_id",
						RightColumn: "roles.id",
					},
					{
						Type:        JoinTypeLeft,
						Table:       "departments",
						LeftColumn:  "", // Invalid: empty left column
						RightColumn: "departments.id",
					},
				},
			},
			expectErr: true,
			errMsg:    "join clause 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestJoinOptions_Validate_SelectColumns(t *testing.T) {
	tests := []struct {
		name          string
		selectColumns []string
		expectErr     bool
		errMsg        string
	}{
		{
			name:          "valid columns",
			selectColumns: []string{"users.*", "roles.name AS role_name", "departments.name"},
			expectErr:     false,
		},
		{
			name:          "valid wildcard",
			selectColumns: []string{"*"},
			expectErr:     false,
		},
		{
			name:          "valid simple columns",
			selectColumns: []string{"id", "name", "email"},
			expectErr:     false,
		},
		{
			name:          "valid table.column",
			selectColumns: []string{"users.id", "users.email", "roles.name"},
			expectErr:     false,
		},
		{
			name:          "valid with aliases",
			selectColumns: []string{"users.id AS user_id", "roles.name as role_name"},
			expectErr:     false,
		},
		{
			name:          "valid table wildcard",
			selectColumns: []string{"users.*", "roles.*"},
			expectErr:     false,
		},
		{
			name:          "SQL injection - UNION",
			selectColumns: []string{"users.id, (SELECT password FROM admin UNION SELECT 1)"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - comment",
			selectColumns: []string{"users.id -- DROP TABLE users"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - semicolon",
			selectColumns: []string{"users.*; DROP TABLE users; --"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - block comment start",
			selectColumns: []string{"users.id /* comment */"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - block comment end",
			selectColumns: []string{"users.id */ DROP TABLE users /*"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - SELECT keyword",
			selectColumns: []string{"users.id, (SELECT 1)"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - INSERT keyword",
			selectColumns: []string{"users.id; INSERT INTO admin VALUES(1)"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - UPDATE keyword",
			selectColumns: []string{"users.id; UPDATE users SET role='admin'"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - DELETE keyword",
			selectColumns: []string{"users.id; DELETE FROM users"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - DROP keyword",
			selectColumns: []string{"users.id; DROP TABLE users"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - CREATE keyword",
			selectColumns: []string{"users.id; CREATE TABLE malicious"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - ALTER keyword",
			selectColumns: []string{"users.id; ALTER TABLE users"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - EXEC keyword",
			selectColumns: []string{"users.id; EXEC sp_executesql"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "SQL injection - EXECUTE keyword",
			selectColumns: []string{"users.id; EXECUTE sp_executesql"},
			expectErr:     true,
			errMsg:        "dangerous SQL keyword",
		},
		{
			name:          "invalid syntax - parentheses",
			selectColumns: []string{"COUNT(users.id)"},
			expectErr:     true,
			errMsg:        "invalid column specification",
		},
		{
			name:          "invalid syntax - special chars",
			selectColumns: []string{"users.id@admin"},
			expectErr:     true,
			errMsg:        "invalid column specification",
		},
		{
			name:          "empty column",
			selectColumns: []string{"users.id", "", "roles.name"},
			expectErr:     true,
			errMsg:        "empty column specification",
		},
		{
			name:          "whitespace only column",
			selectColumns: []string{"users.id", "   ", "roles.name"},
			expectErr:     true,
			errMsg:        "empty column specification",
		},
		{
			name:          "invalid column name - starts with number",
			selectColumns: []string{"123column"},
			expectErr:     true,
			errMsg:        "invalid column specification",
		},
		{
			name:          "invalid column name - special characters",
			selectColumns: []string{"user$name"},
			expectErr:     true,
			errMsg:        "invalid column specification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &JoinOptions{
				Joins: []JoinClause{
					{
						Type:        JoinTypeInner,
						Table:       "roles",
						LeftColumn:  "users.role_id",
						RightColumn: "roles.id",
					},
				},
				SelectColumns: tt.selectColumns,
			}

			err := options.Validate()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestJoinOptions_ToSQL(t *testing.T) {
	tests := []struct {
		name     string
		options  *JoinOptions
		expected []string // Expected join clauses in order
	}{
		{
			name: "single inner join",
			options: &JoinOptions{
				Joins: []JoinClause{
					{
						Type:        JoinTypeInner,
						Table:       "roles",
						TableAlias:  "r",
						LeftColumn:  "users.role_id",
						RightColumn: "r.id",
					},
				},
			},
			expected: []string{"INNER JOIN roles r ON users.role_id = r.id"},
		},
		{
			name: "multiple joins",
			options: &JoinOptions{
				Joins: []JoinClause{
					{
						Type:        JoinTypeLeft,
						Table:       "roles",
						TableAlias:  "r",
						LeftColumn:  "users.role_id",
						RightColumn: "r.id",
					},
					{
						Type:        JoinTypeInner,
						Table:       "departments",
						TableAlias:  "d",
						LeftColumn:  "users.department_id",
						RightColumn: "d.id",
					},
				},
			},
			expected: []string{
				"LEFT JOIN roles r ON users.role_id = r.id",
				"INNER JOIN departments d ON users.department_id = d.id",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clauses := tt.options.ToSQL()
			require.Len(t, clauses, len(tt.expected))
			for i, expected := range tt.expected {
				assert.Equal(t, expected, clauses[i])
			}
		})
	}
}

func TestMergeJoinOptions(t *testing.T) {
	t.Run("both nil returns nil", func(t *testing.T) {
		result := MergeJoinOptions(nil, nil)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("default nil returns request", func(t *testing.T) {
		request := &JoinOptions{
			Joins: []JoinClause{{Table: "test"}},
		}
		result := MergeJoinOptions(nil, request)
		if result != request {
			t.Errorf("expected request, got %v", result)
		}
	})

	t.Run("request nil returns default", func(t *testing.T) {
		defaults := &JoinOptions{
			Joins: []JoinClause{{Table: "test"}},
		}
		result := MergeJoinOptions(defaults, nil)
		if result != defaults {
			t.Errorf("expected defaults, got %v", result)
		}
	})

	t.Run("merges joins from both", func(t *testing.T) {
		defaults := &JoinOptions{
			Joins: []JoinClause{
				{Table: "default_table", LeftColumn: "a.id", RightColumn: "b.id"},
			},
			SelectColumns: []string{"a.*", "b.name"},
		}
		request := &JoinOptions{
			Joins: []JoinClause{
				{Table: "request_table", LeftColumn: "c.id", RightColumn: "d.id"},
			},
		}

		result := MergeJoinOptions(defaults, request)

		if len(result.Joins) != 2 {
			t.Errorf("expected 2 joins, got %d", len(result.Joins))
		}
		if result.Joins[0].Table != "default_table" {
			t.Errorf("expected default join first, got %s", result.Joins[0].Table)
		}
		if result.Joins[1].Table != "request_table" {
			t.Errorf("expected request join second, got %s", result.Joins[1].Table)
		}
		if len(result.SelectColumns) != 2 {
			t.Errorf("expected default SelectColumns when request doesn't specify, got %v", result.SelectColumns)
		}
	})

	t.Run("request SelectColumns override defaults", func(t *testing.T) {
		defaults := &JoinOptions{
			SelectColumns: []string{"a.*", "b.name"},
		}
		request := &JoinOptions{
			SelectColumns: []string{"c.*", "d.name"},
		}

		result := MergeJoinOptions(defaults, request)

		if len(result.SelectColumns) != 2 {
			t.Errorf("expected 2 select columns, got %d", len(result.SelectColumns))
		}
		if result.SelectColumns[0] != "c.*" {
			t.Errorf("expected request SelectColumns to override, got %s", result.SelectColumns[0])
		}
	})
}
