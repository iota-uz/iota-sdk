package collector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/schema/diff"
	"github.com/iota-uz/iota-sdk/pkg/schema/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "default configuration",
			config: Config{
				ModulesPath:    "test_modules",
				MigrationsPath: "test_migrations",
				SQLDialect:     "postgres",
				LogLevel:       logrus.InfoLevel,
			},
		},
		{
			name: "with custom logger",
			config: Config{
				ModulesPath:    "test_modules",
				MigrationsPath: "test_migrations",
				SQLDialect:     "postgres",
				Logger:         logrus.New(),
				LogLevel:       logrus.DebugLevel,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := New(tt.config)
			assert.NotNil(t, collector)
			assert.Equal(t, tt.config.ModulesPath, collector.modulesDir)
			assert.Equal(t, tt.config.MigrationsPath, collector.baseDir)
			assert.NotNil(t, collector.parser)
			assert.NotNil(t, collector.dialect)
			assert.NotNil(t, collector.logger)
			assert.NotNil(t, collector.migrations)
		})
	}
}

func TestCollector_CollectMigrations(t *testing.T) {
	// Create temporary test directories
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	modulesDir := filepath.Join(tmpDir, "modules")

	err := os.MkdirAll(migrationsDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(modulesDir, 0755)
	require.NoError(t, err)

	// Create test migration files
	migrationSQL := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL
	);`
	err = os.WriteFile(filepath.Join(migrationsDir, "changes-1000000000.sql"), []byte(migrationSQL), 0644)
	require.NoError(t, err)

	// Create test module schema
	moduleSQL := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	err = os.WriteFile(filepath.Join(modulesDir, "schema.sql"), []byte(moduleSQL), 0644)
	require.NoError(t, err)

	collector := New(Config{
		ModulesPath:    modulesDir,
		MigrationsPath: migrationsDir,
		SQLDialect:     "postgres",
		LogLevel:       logrus.DebugLevel,
	})

	changes, err := collector.CollectMigrations(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, changes)
	assert.Greater(t, len(changes.Changes), 0)

	// Verify that the changes include adding the created_at column
	foundCreatedAt := false
	for _, change := range changes.Changes {
		if change.Type == diff.AddColumn && change.ObjectName == "created_at" {
			foundCreatedAt = true
			break
		}
	}
	assert.True(t, foundCreatedAt, "Expected to find ADD COLUMN created_at change")
}

func TestCollector_StoreMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	err := os.MkdirAll(migrationsDir, 0755)
	require.NoError(t, err)

	collector := New(Config{
		ModulesPath:    "test_modules",
		MigrationsPath: migrationsDir,
		SQLDialect:     "postgres",
		LogLevel:       logrus.DebugLevel,
	})

	// Create a test change set
	changes := &diff.ChangeSet{
		Changes: []*diff.Change{
			{
				Type:       diff.AddColumn,
				ObjectName: "created_at",
				ParentName: "users",
				Reversible: true,
				Object: &types.Node{
					Type: types.NodeColumn,
					Name: "created_at",
					Metadata: map[string]interface{}{
						"type":        "timestamp",
						"definition":  "created_at timestamp DEFAULT CURRENT_TIMESTAMP",
						"constraints": "DEFAULT CURRENT_TIMESTAMP",
					},
				},
			},
		},
	}

	err = collector.StoreMigrations(changes)
	require.NoError(t, err)

	// Verify that migration files were created
	files, err := os.ReadDir(migrationsDir)
	require.NoError(t, err)
	assert.Greater(t, len(files), 0)

	// Verify that both up and down migration files were created
	hasUp := false
	hasDown := false
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".sql" {
			if filepath.Ext(strings.TrimSuffix(file.Name(), ".sql")) == ".down" {
				hasDown = true
			} else {
				hasUp = true
			}
		}
	}
	assert.True(t, hasUp, "Expected to find up migration file")
	assert.True(t, hasDown, "Expected to find down migration file")
}

func TestCollector_LoadExistingSchema(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	err := os.MkdirAll(migrationsDir, 0755)
	require.NoError(t, err)

	// Create test migration files
	migrations := []struct {
		filename string
		content  string
	}{
		{
			filename: "changes-1000000000.sql",
			content: `CREATE TABLE users (
				id SERIAL PRIMARY KEY,
				name VARCHAR(255) NOT NULL
			);`,
		},
		{
			filename: "changes-1000000001.sql",
			content: `ALTER TABLE users
				ADD COLUMN email VARCHAR(255) UNIQUE NOT NULL;`,
		},
	}

	for _, m := range migrations {
		err = os.WriteFile(filepath.Join(migrationsDir, m.filename), []byte(m.content), 0644)
		require.NoError(t, err)
	}

	collector := New(Config{
		ModulesPath:    "test_modules",
		MigrationsPath: migrationsDir,
		SQLDialect:     "postgres",
		LogLevel:       logrus.DebugLevel,
	})

	tree, err := collector.loadExistingSchema()
	require.NoError(t, err)
	assert.NotNil(t, tree)

	// Verify the schema tree
	assert.Equal(t, 1, len(tree.Root.Children))
	usersTable := tree.Root.Children[0]
	assert.Equal(t, "users", usersTable.Name)
	assert.Equal(t, types.NodeTable, usersTable.Type)
	assert.Equal(t, 3, len(usersTable.Children)) // id, name, email columns

	// Verify column details
	columns := make(map[string]*types.Node)
	for _, col := range usersTable.Children {
		columns[col.Name] = col
	}

	assert.Contains(t, columns, "id")
	assert.Contains(t, columns, "name")
	assert.Contains(t, columns, "email")

	assert.Equal(t, "SERIAL", columns["id"].Metadata["type"])
	assert.Equal(t, "VARCHAR", columns["name"].Metadata["type"])
	assert.Equal(t, "VARCHAR", columns["email"].Metadata["type"])
}

func TestCollector_LoadModuleSchema(t *testing.T) {
	tmpDir := t.TempDir()
	modulesDir := filepath.Join(tmpDir, "modules")
	err := os.MkdirAll(modulesDir, 0755)
	require.NoError(t, err)

	// Create test module schema files
	moduleSchemas := []struct {
		path    string
		content string
	}{
		{
			path: filepath.Join(modulesDir, "users", "schema.sql"),
			content: `CREATE TABLE users (
				id SERIAL PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				email VARCHAR(255) UNIQUE NOT NULL
			);`,
		},
		{
			path: filepath.Join(modulesDir, "posts", "schema.sql"),
			content: `CREATE TABLE posts (
				id SERIAL PRIMARY KEY,
				title VARCHAR(255) NOT NULL,
				content TEXT,
				user_id INTEGER REFERENCES users(id)
			);`,
		},
	}

	for _, schema := range moduleSchemas {
		err = os.MkdirAll(filepath.Dir(schema.path), 0755)
		require.NoError(t, err)
		err = os.WriteFile(schema.path, []byte(schema.content), 0644)
		require.NoError(t, err)
	}

	collector := New(Config{
		ModulesPath:    modulesDir,
		MigrationsPath: "test_migrations",
		SQLDialect:     "postgres",
		LogLevel:       logrus.DebugLevel,
	})

	tree, err := collector.loadModuleSchema()
	require.NoError(t, err)
	assert.NotNil(t, tree)

	// Verify the schema tree
	assert.Equal(t, 2, len(tree.Root.Children))

	// Create a map of tables for easier testing
	tables := make(map[string]*types.Node)
	for _, table := range tree.Root.Children {
		tables[table.Name] = table
	}

	// Verify users table
	assert.Contains(t, tables, "users")
	usersTable := tables["users"]
	assert.Equal(t, 3, len(usersTable.Children))

	// Verify posts table
	assert.Contains(t, tables, "posts")
	postsTable := tables["posts"]
	assert.Equal(t, 4, len(postsTable.Children))

	// Verify foreign key relationship
	var userIdColumn *types.Node
	for _, col := range postsTable.Children {
		if col.Name == "user_id" {
			userIdColumn = col
			break
		}
	}
	assert.NotNil(t, userIdColumn)
	assert.Contains(t, userIdColumn.Metadata["constraints"], "REFERENCES users(id)")
}
