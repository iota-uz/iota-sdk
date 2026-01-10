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
