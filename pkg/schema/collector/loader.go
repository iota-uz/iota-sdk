package collector

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/iota-uz/psql-parser/sql/parser"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/sirupsen/logrus"
)

type SchemaLoader interface {
	LoadExistingSchema(ctx context.Context) (*common.Schema, error)
	LoadModuleSchema(ctx context.Context) (*common.Schema, error)
}

type FileLoader struct {
	baseDir  string
	embedFSs []*embed.FS
	logger   logrus.FieldLogger
}

type LoaderConfig struct {
	BaseDir  string
	EmbedFSs []*embed.FS
	Logger   logrus.FieldLogger
}

func NewFileLoader(cfg LoaderConfig) *FileLoader {
	return &FileLoader{
		baseDir:  cfg.BaseDir,
		embedFSs: cfg.EmbedFSs,
		logger:   cfg.Logger,
	}
}

func (l *FileLoader) LoadExistingSchema(ctx context.Context) (*common.Schema, error) {
	l.logger.Info("Loading existing schema files from: ", l.baseDir)

	files, err := os.ReadDir(l.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			l.logger.Info("No existing migrations directory found")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrationFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}

	l.logger.Debugf("Found %d existing migrations", len(migrationFiles))

	sort.Slice(migrationFiles, func(i, j int) bool {
		return l.extractTimestamp(migrationFiles[i]) < l.extractTimestamp(migrationFiles[j])
	})

	schemaState := newSchemaState()

	for _, file := range migrationFiles {
		path := filepath.Join(l.baseDir, file)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", path, err)
		}
		migration, err := migrate.ParseMigration(path, bytes.NewReader(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse migration file %s: %w", path, err)
		}

		stmts, err := parser.Parse(strings.Join(migration.Up, "\n"))
		if err != nil {
			return nil, fmt.Errorf("failed to parse file %s: %w", file, err)
		}

		timestamp := l.extractTimestamp(file)
		schemaState.update(stmts, timestamp, file)
		l.logger.Debugf("Updating schema state from file: %s with timestamp: %d", file, timestamp)
	}

	return schemaState.buildSchema(), nil
}

func (l *FileLoader) LoadModuleSchema(ctx context.Context) (*common.Schema, error) {
	l.logger.Info("Loading module schema files from embed.FS")

	schemaState := newSchemaState()

	// Only process embedded file systems
	if len(l.embedFSs) == 0 {
		l.logger.Warn("No embedded filesystems provided for loading module schema")
		return schemaState.buildSchema(), nil
	}

	l.logger.Infof("Loading module schema from %d embedded filesystems", len(l.embedFSs))

	// Start with a base timestamp value and increment for each module
	// This helps maintain order of processing while allowing subsequent modules to add their schemas
	timestamp := int64(1)

	for i, embedFS := range l.embedFSs {
		// Use incremented timestamp for each filesystem
		currentTimestamp := timestamp + int64(i)

		err := fs.WalkDir(*embedFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() && filepath.Ext(path) == ".sql" {
				content, err := embedFS.ReadFile(path)
				if err != nil {
					l.logger.Warnf("Error reading embed file %s: %v", path, err)
					return nil
				}

				parsed, err := parser.Parse(string(content))
				if err != nil {
					l.logger.Warnf("Error parsing embedded file %s: %v", path, err)
					return nil
				}

				l.logger.Debugf("Processing file: %s", path)
				schemaState.update(parsed, currentTimestamp, path)
			}
			return nil
		})

		if err != nil {
			l.logger.Warnf("Error walking embedded filesystem: %v", err)
		}
	}

	return schemaState.buildSchema(), nil
}

func (l *FileLoader) extractTimestamp(fileName string) int64 {
	ts := strings.TrimSuffix(strings.TrimPrefix(fileName, "changes-"), ".sql")
	timestamp, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return 0
	}
	return timestamp
}
