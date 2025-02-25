package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type SchemaLoader interface {
	LoadExistingSchema(ctx context.Context) (*SchemaTree, error)
	LoadModuleSchema(ctx context.Context) (*SchemaTree, error)
}

type Parser interface {
	ParseSQL(sql string) (*SchemaTree, error)
	GetDialect() string
}

type FileLoader struct {
	baseDir    string
	modulesDir string
	parser     Parser
	logger     logrus.FieldLogger
}

type LoaderConfig struct {
	BaseDir    string
	ModulesDir string
	Parser     Parser
	Logger     logrus.FieldLogger
}

func NewFileLoader(cfg LoaderConfig) *FileLoader {
	return &FileLoader{
		baseDir:    cfg.BaseDir,
		modulesDir: cfg.ModulesDir,
		parser:     cfg.Parser,
		logger:     cfg.Logger,
	}
}

func (l *FileLoader) LoadExistingSchema(ctx context.Context) (*SchemaTree, error) {
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

	return schemaState.buildFinalTree(), nil
}

func (l *FileLoader) LoadModuleSchema(ctx context.Context) (*SchemaTree, error) {
	l.logger.Info("Loading module schema files from: ", l.modulesDir)

	schemaState := newSchemaState()

	err := filepath.Walk(l.modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look specifically for SQL files in schema subdirectories
		if info.Mode().IsRegular() && strings.HasSuffix(info.Name(), ".sql") {
			dirPath := filepath.Dir(path)
			if strings.Contains(dirPath, "schema") {
				if err := l.processModuleFile(ctx, path, schemaState); err != nil {
					l.logger.Warnf("Error processing file %s: %v", path, err)
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking module directory: %w", err)
	}

	return schemaState.buildFinalTree(), nil
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

	parsed, err := l.parser.ParseSQL(content)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", fileName, err)
	}

	timestamp := l.extractTimestamp(fileName)
	state.updateFromParsedTree(parsed, timestamp, fileName)

	return nil
}

func (l *FileLoader) processModuleFile(ctx context.Context, path string, state *schemaState) error {
	content, err := l.readFile(path)
	if err != nil {
		return err
	}

	parsed, err := l.parser.ParseSQL(content)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", path, err)
	}

	state.updateFromParsedTree(parsed, 0, path)
	return nil
}

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

func (l *FileLoader) isValidSchemaFile(info os.FileInfo) bool {
	return !info.IsDir() && strings.HasSuffix(info.Name(), ".sql")
}

func (l *FileLoader) extractTimestamp(fileName string) int64 {
	ts := strings.TrimSuffix(strings.TrimPrefix(fileName, "changes-"), ".sql")
	timestamp, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return 0
	}
	return timestamp
}
