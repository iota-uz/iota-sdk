package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsert(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
		fields    []string
		returning []string
		want      string
	}{
		{
			name:      "basic insert",
			tableName: "users",
			fields:    []string{"name", "email", "password"},
			returning: []string{"id", "created_at"},
			want:      "INSERT INTO users (name, email, password) VALUES ($1, $2, $3) RETURNING id, created_at",
		},
		{
			name:      "single field",
			tableName: "status",
			fields:    []string{"value"},
			returning: []string{"id"},
			want:      "INSERT INTO status (value) VALUES ($1) RETURNING id",
		},
		{
			name:      "with schema",
			tableName: "public.products",
			fields:    []string{"name", "price", "category_id"},
			returning: []string{"id", "created_at", "updated_at"},
			want:      "INSERT INTO public.products (name, price, category_id) VALUES ($1, $2, $3) RETURNING id, created_at, updated_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Insert(tt.tableName, tt.fields, tt.returning...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
		fields    []string
		where     []string
		want      string
	}{
		{
			name:      "basic update",
			tableName: "users",
			fields:    []string{"name", "email"},
			where:     []string{"id = $3"},
			want:      "UPDATE users SET name = $1, email = $2 WHERE id = $3",
		},
		{
			name:      "single field",
			tableName: "status",
			fields:    []string{"value"},
			where:     []string{"id = $2"},
			want:      "UPDATE status SET value = $1 WHERE id = $2",
		},
		{
			name:      "multiple conditions",
			tableName: "products",
			fields:    []string{"name", "price", "updated_at"},
			where:     []string{"id = $4", "category_id = $5"},
			want:      "UPDATE products SET name = $1, price = $2, updated_at = $3 WHERE id = $4 AND category_id = $5",
		},
		{
			name:      "with schema",
			tableName: "public.orders",
			fields:    []string{"status", "updated_at"},
			where:     []string{"id = $3"},
			want:      "UPDATE public.orders SET status = $1, updated_at = $2 WHERE id = $3",
		},
		{
			name:      "no conditions",
			tableName: "settings",
			fields:    []string{"value", "updated_at"},
			where:     []string{},
			want:      "UPDATE settings SET value = $1, updated_at = $2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Update(tt.tableName, tt.fields, tt.where...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBatchInsertQueryN(t *testing.T) {
	tests := []struct {
		name      string
		baseQuery string
		rows      [][]interface{}
		wantQuery string
		wantArgs  []interface{}
	}{
		{
			name:      "empty rows",
			baseQuery: "INSERT INTO users (name, email) VALUES",
			rows:      [][]interface{}{},
			wantQuery: "INSERT INTO users (name, email) VALUES",
			wantArgs:  nil,
		},
		{
			name:      "single row",
			baseQuery: "INSERT INTO users (name, email) VALUES",
			rows: [][]interface{}{
				{"John", "john@example.com"},
			},
			wantQuery: "INSERT INTO users (name, email) VALUES ($1,$2)",
			wantArgs:  []interface{}{"John", "john@example.com"},
		},
		{
			name:      "multiple rows",
			baseQuery: "INSERT INTO users (name, email) VALUES",
			rows: [][]interface{}{
				{"John", "john@example.com"},
				{"Jane", "jane@example.com"},
				{"Bob", "bob@example.com"},
			},
			wantQuery: "INSERT INTO users (name, email) VALUES ($1,$2),($3,$4),($5,$6)",
			wantArgs:  []interface{}{"John", "john@example.com", "Jane", "jane@example.com", "Bob", "bob@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, gotArgs := BatchInsertQueryN(tt.baseQuery, tt.rows)
			assert.Equal(t, tt.wantQuery, gotQuery)
			assert.Equal(t, tt.wantArgs, gotArgs)
		})
	}
}

func TestJoinClause(t *testing.T) {
	tests := []struct {
		name     string
		joinType string
		table    string
		alias    string
		leftCol  string
		rightCol string
		expected string
	}{
		{
			name:     "inner join with alias",
			joinType: "INNER JOIN",
			table:    "roles",
			alias:    "r",
			leftCol:  "users.role_id",
			rightCol: "r.id",
			expected: "INNER JOIN roles r ON users.role_id = r.id",
		},
		{
			name:     "left join without alias",
			joinType: "LEFT JOIN",
			table:    "roles",
			alias:    "",
			leftCol:  "users.role_id",
			rightCol: "roles.id",
			expected: "LEFT JOIN roles ON users.role_id = roles.id",
		},
		{
			name:     "right join with alias",
			joinType: "RIGHT JOIN",
			table:    "departments",
			alias:    "d",
			leftCol:  "users.dept_id",
			rightCol: "d.id",
			expected: "RIGHT JOIN departments d ON users.dept_id = d.id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinClause(tt.joinType, tt.table, tt.alias, tt.leftCol, tt.rightCol)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinInner(t *testing.T) {
	result := JoinInner("roles", "r", "users.role_id", "r.id")
	expected := "INNER JOIN roles r ON users.role_id = r.id"
	assert.Equal(t, expected, result)
}

func TestJoinLeft(t *testing.T) {
	result := JoinLeft("roles", "r", "users.role_id", "r.id")
	expected := "LEFT JOIN roles r ON users.role_id = r.id"
	assert.Equal(t, expected, result)
}

func TestJoinRight(t *testing.T) {
	result := JoinRight("departments", "d", "users.dept_id", "d.id")
	expected := "RIGHT JOIN departments d ON users.dept_id = d.id"
	assert.Equal(t, expected, result)
}

func TestMultipleJoins(t *testing.T) {
	// Test combining multiple joins with the Join utility
	baseQuery := "SELECT * FROM users"
	join1 := JoinInner("roles", "r", "users.role_id", "r.id")
	join2 := JoinLeft("departments", "d", "users.dept_id", "d.id")

	query := Join(baseQuery, join1, join2)
	expected := "SELECT * FROM users INNER JOIN roles r ON users.role_id = r.id LEFT JOIN departments d ON users.dept_id = d.id"

	assert.Equal(t, expected, query)
}
