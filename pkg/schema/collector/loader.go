package collector

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/auxten/postgresql-parser/pkg/sql/parser"
	"github.com/iota-uz/iota-sdk/pkg/schema/common"
	"github.com/sirupsen/logrus"
)

type SchemaLoader interface {
	LoadExistingSchema(ctx context.Context) (*common.Schema, error)
	LoadModuleSchema(ctx context.Context) (*common.Schema, error)
}

type FileLoader struct {
	baseDir    string
	embedFSs   []*embed.FS
	logger     logrus.FieldLogger
}

type LoaderConfig struct {
	BaseDir    string
	EmbedFSs   []*embed.FS
	Logger     logrus.FieldLogger
}

func NewFileLoader(cfg LoaderConfig) *FileLoader {
	return &FileLoader{
		baseDir:    cfg.BaseDir,
		embedFSs:   cfg.EmbedFSs,
		logger:     cfg.Logger,
	}
}

func (l *FileLoader) LoadExistingSchema(ctx context.Context) (*common.Schema, error) {
	l.logger.Info("Loading existing schema files from: ", l.baseDir)

	files, err := l.readMigrationFiles()
	if err != nil {
		return nil, err
	}

	schemaState := newSchemaState()

	for _, file := range files {
		if err := l.processMigrationFile(ctx, file, schemaState); err != nil {
			return nil, err
		}
	}

	return schemaState.buildSchema(), nil
}

func (l *FileLoader) LoadModuleSchema(ctx context.Context) (*common.Schema, error) {
	l.logger.Info("Loading module schema files from embed.FS")

	schemaState := newSchemaState()

	// Only process embedded file systems
	if len(l.embedFSs) > 0 {
		for _, embedFS := range l.embedFSs {
			err := fs.WalkDir(*embedFS, ".", func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				
				if !d.IsDir() && strings.HasSuffix(path, ".sql") && strings.Contains(path, "schema") {
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
					
					schemaState.update(parsed, 0, path)
				}
				return nil
			})
			
			if err != nil {
				l.logger.Warnf("Error walking embedded filesystem: %v", err)
			}
		}
	} else {
		l.logger.Warn("No embedded filesystems provided for loading module schema")
	}

	return schemaState.buildSchema(), nil
}

// Internal helper methods

func (l *FileLoader) readMigrationFiles() ([]string, error) {
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
		if l.isValidMigrationFile(file) {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}

	sort.Slice(migrationFiles, func(i, j int) bool {
		return l.extractTimestamp(migrationFiles[i]) < l.extractTimestamp(migrationFiles[j])
	})

	return migrationFiles, nil
}

func (l *FileLoader) processMigrationFile(ctx context.Context, fileName string, state *schemaState) error {
	path := filepath.Join(l.baseDir, fileName)
	content, err := l.readFile(path)
	if err != nil {
		return err
	}

	stmts, err := parser.Parse(content)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", fileName, err)
	}

	timestamp := l.extractTimestamp(fileName)
	state.update(stmts, timestamp, fileName)

	return nil
}

// This function is no longer needed since we only load from embed.FS
// and the loading logic is inside LoadModuleSchema method

func (l *FileLoader) readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return string(content), nil
}

func (l *FileLoader) isValidMigrationFile(file os.DirEntry) bool {
	return !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") &&
		strings.HasPrefix(file.Name(), "changes-")
}

// This function is no longer needed since we only load from embed.FS

func (l *FileLoader) extractTimestamp(fileName string) int64 {
	ts := strings.TrimSuffix(strings.TrimPrefix(fileName, "changes-"), ".sql")
	timestamp, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return 0
	}
	return timestamp
}
