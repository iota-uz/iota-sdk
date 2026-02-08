package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var (
	queryBlockRegex = regexp.MustCompile(`(?ms)--\s*<query name>\s*(.*?)\s*</query name>\s*--\s*<query description>\s*(.*?)\s*--\s*</query description>\s*--\s*<query>\s*(.*?)\s*--\s*</query>`)
	tableRefRegex   = regexp.MustCompile(`(?i)\b(?:from|join)\s+([a-zA-Z0-9_."\.]+)`)
)

type tenantValidatedQueryResetter interface {
	DeleteByTenant(ctx context.Context, tenantID uuid.UUID) error
}

type staticDocumentSource struct {
	documents []kb.Document
}

func (s staticDocumentSource) List(ctx context.Context) ([]kb.Document, error) {
	return s.documents, nil
}

var errWatchNotSupported = errors.New("watch not supported")

func (s staticDocumentSource) Watch(ctx context.Context) (<-chan kb.DocumentChange, error) {
	return nil, errWatchNotSupported
}

type queryPattern struct {
	Name        string
	Description string
	SQL         string
	TablesUsed  []string
}

type tableKnowledgeFile struct {
	TableName        string            `json:"table_name"`
	TableDescription string            `json:"table_description"`
	UseCases         []string          `json:"use_cases,omitempty"`
	DataQualityNotes []string          `json:"data_quality_notes,omitempty"`
	ColumnNotes      map[string]string `json:"column_notes,omitempty"`
	TableColumns     []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"table_columns,omitempty"`
	Metrics []struct {
		Name        string `json:"name"`
		Definition  string `json:"definition"`
		Formula     string `json:"formula"`
		Calculation string `json:"calculation"`
	} `json:"metrics,omitempty"`
}

// KnowledgeBootstrapConfig configures static knowledge loading.
type KnowledgeBootstrapConfig struct {
	ValidatedQueryStore learning.ValidatedQueryStore
	KBIndexer           kb.KBIndexer
	MetadataOutputDir   string
	Now                 func() time.Time
}

// KnowledgeBootstrapRequest controls knowledge bootstrap behavior.
type KnowledgeBootstrapRequest struct {
	TenantID     uuid.UUID
	KnowledgeDir string
	Rebuild      bool
}

// KnowledgeBootstrapResult summarizes loaded artifacts.
type KnowledgeBootstrapResult struct {
	TableFilesLoaded       int
	BusinessFilesLoaded    int
	QueryPatternsLoaded    int
	ValidatedQueriesSaved  int
	KnowledgeDocsIndexed   int
	MetadataFilesGenerated int
}

// KnowledgeBootstrapService loads static BI knowledge artifacts.
type KnowledgeBootstrapService struct {
	validatedQueryStore learning.ValidatedQueryStore
	kbIndexer           kb.KBIndexer
	metadataOutputDir   string
	now                 func() time.Time
}

func NewKnowledgeBootstrapService(cfg KnowledgeBootstrapConfig) *KnowledgeBootstrapService {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &KnowledgeBootstrapService{
		validatedQueryStore: cfg.ValidatedQueryStore,
		kbIndexer:           cfg.KBIndexer,
		metadataOutputDir:   strings.TrimSpace(cfg.MetadataOutputDir),
		now:                 now,
	}
}

// Load loads tables/queries/business artifacts with idempotent upsert semantics.
func (s *KnowledgeBootstrapService) Load(ctx context.Context, req KnowledgeBootstrapRequest) (*KnowledgeBootstrapResult, error) {
	const op serrors.Op = "KnowledgeBootstrapService.Load"

	knowledgeDir := strings.TrimSpace(req.KnowledgeDir)
	if knowledgeDir == "" {
		return nil, serrors.E(op, serrors.KindValidation, "knowledge directory is required")
	}

	info, err := os.Stat(knowledgeDir)
	if err != nil {
		return nil, serrors.E(op, err, "failed to read knowledge directory")
	}
	if !info.IsDir() {
		return nil, serrors.E(op, serrors.KindValidation, "knowledge path must be a directory")
	}

	if s.validatedQueryStore != nil && req.TenantID == uuid.Nil {
		return nil, serrors.E(op, serrors.KindValidation, "tenant_id is required when validated query store is configured")
	}

	if req.Rebuild && s.validatedQueryStore != nil {
		resetter, ok := s.validatedQueryStore.(tenantValidatedQueryResetter)
		if !ok {
			return nil, serrors.E(op, serrors.KindValidation, "validated query store does not support rebuild")
		}
		if err := resetter.DeleteByTenant(ctx, req.TenantID); err != nil {
			return nil, serrors.E(op, err, "failed to reset validated query library")
		}
	}

	result := &KnowledgeBootstrapResult{}
	tablesDir := filepath.Join(knowledgeDir, "tables")
	tableFiles, err := knowledgeFiles(tablesDir, ".json")
	if err != nil {
		return nil, serrors.E(op, err, "failed to list tables knowledge files")
	}
	docs := make([]kb.Document, 0, len(tableFiles))
	metadata := make([]schema.TableMetadata, 0, len(tableFiles))
	for _, filePath := range tableFiles {
		raw, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return nil, serrors.E(op, readErr, "failed to read table knowledge file")
		}
		tableMeta, parseErr := parseTableMetadata(raw)
		if parseErr != nil {
			return nil, serrors.E(op, parseErr, "failed to parse table knowledge file")
		}
		metadata = append(metadata, tableMeta)
		docs = append(docs, kb.Document{
			ID:        "knowledge:tables:" + filepath.Base(filePath),
			Title:     tableMeta.TableName,
			Content:   string(raw),
			Path:      filePath,
			Type:      kb.DocumentTypeJSON,
			Metadata:  map[string]string{"source": "tables", "table_name": tableMeta.TableName},
			UpdatedAt: s.now(),
		})
	}
	result.TableFilesLoaded = len(tableFiles)

	businessDir := filepath.Join(knowledgeDir, "business")
	businessFiles, err := knowledgeFiles(businessDir, ".json")
	if err != nil {
		return nil, serrors.E(op, err, "failed to list business knowledge files")
	}
	for _, filePath := range businessFiles {
		raw, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return nil, serrors.E(op, readErr, "failed to read business knowledge file")
		}
		docs = append(docs, kb.Document{
			ID:        "knowledge:business:" + filepath.Base(filePath),
			Title:     strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)),
			Content:   string(raw),
			Path:      filePath,
			Type:      kb.DocumentTypeJSON,
			Metadata:  map[string]string{"source": "business"},
			UpdatedAt: s.now(),
		})
	}
	result.BusinessFilesLoaded = len(businessFiles)

	queriesDir := filepath.Join(knowledgeDir, "queries")
	queryFiles, err := knowledgeFiles(queriesDir, ".sql")
	if err != nil {
		return nil, serrors.E(op, err, "failed to list query knowledge files")
	}
	for _, filePath := range queryFiles {
		raw, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return nil, serrors.E(op, readErr, "failed to read query knowledge file")
		}
		patterns := parseQueryPatterns(string(raw), strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)))
		result.QueryPatternsLoaded += len(patterns)

		for idx, pattern := range patterns {
			queryID := fmt.Sprintf("knowledge:queries:%s:%d", filepath.Base(filePath), idx)
			docs = append(docs, kb.Document{
				ID:        queryID,
				Title:     pattern.Name,
				Content:   pattern.SQL,
				Path:      filePath,
				Type:      kb.DocumentTypeCode,
				Metadata:  map[string]string{"source": "queries"},
				UpdatedAt: s.now(),
			})

			if s.validatedQueryStore != nil {
				question := pattern.Name
				if pattern.Description != "" {
					question = pattern.Description
				}
				summary := pattern.Description
				if summary == "" {
					summary = pattern.Name
				}
				saveErr := s.validatedQueryStore.Save(ctx, learning.ValidatedQuery{
					TenantID:         req.TenantID,
					Question:         question,
					SQL:              pattern.SQL,
					Summary:          summary,
					TablesUsed:       pattern.TablesUsed,
					DataQualityNotes: []string{},
					CreatedAt:        s.now(),
				})
				if saveErr != nil {
					return nil, serrors.E(op, saveErr, "failed to save validated query pattern")
				}
				result.ValidatedQueriesSaved++
			}
		}
	}

	if s.metadataOutputDir != "" {
		written, syncErr := syncMetadataFiles(s.metadataOutputDir, metadata, req.Rebuild)
		if syncErr != nil {
			return nil, serrors.E(op, syncErr, "failed to sync schema metadata files")
		}
		result.MetadataFilesGenerated = written
	}

	if s.kbIndexer != nil {
		if req.Rebuild {
			if err := s.kbIndexer.Rebuild(ctx, staticDocumentSource{documents: docs}); err != nil {
				return nil, serrors.E(op, err, "failed to rebuild knowledge index")
			}
		} else if len(docs) > 0 {
			if err := s.kbIndexer.IndexDocuments(ctx, docs); err != nil {
				return nil, serrors.E(op, err, "failed to index knowledge documents")
			}
		}
		result.KnowledgeDocsIndexed = len(docs)
	}

	return result, nil
}

func knowledgeFiles(dirPath string, extension string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != extension {
			continue
		}
		files = append(files, filepath.Join(dirPath, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func parseTableMetadata(raw []byte) (schema.TableMetadata, error) {
	var file tableKnowledgeFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return schema.TableMetadata{}, err
	}

	columnNotes := map[string]string{}
	for key, value := range file.ColumnNotes {
		columnNotes[key] = value
	}
	for _, col := range file.TableColumns {
		if col.Name == "" || col.Description == "" {
			continue
		}
		if _, exists := columnNotes[col.Name]; !exists {
			columnNotes[col.Name] = col.Description
		}
	}

	metrics := make([]schema.MetricDef, 0, len(file.Metrics))
	for _, metric := range file.Metrics {
		formula := metric.Formula
		if formula == "" {
			formula = metric.Calculation
		}
		metrics = append(metrics, schema.MetricDef{
			Name:       metric.Name,
			Formula:    formula,
			Definition: metric.Definition,
		})
	}

	return schema.TableMetadata{
		TableName:        file.TableName,
		TableDescription: file.TableDescription,
		UseCases:         file.UseCases,
		DataQualityNotes: file.DataQualityNotes,
		ColumnNotes:      columnNotes,
		Metrics:          metrics,
	}, nil
}

func syncMetadataFiles(outputDir string, metadata []schema.TableMetadata, rebuild bool) (int, error) {
	if outputDir == "" {
		return 0, nil
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return 0, err
	}

	if rebuild {
		files, err := knowledgeFiles(outputDir, ".json")
		if err != nil {
			return 0, err
		}
		for _, filePath := range files {
			if err := os.Remove(filePath); err != nil {
				return 0, err
			}
		}
	}

	written := 0
	for _, table := range metadata {
		if table.TableName == "" {
			continue
		}
		payload, err := json.MarshalIndent(table, "", "  ")
		if err != nil {
			return 0, err
		}
		filePath := filepath.Join(outputDir, table.TableName+".json")
		if err := os.WriteFile(filePath, payload, 0644); err != nil {
			return 0, err
		}
		written++
	}
	return written, nil
}

func parseQueryPatterns(raw string, fallbackName string) []queryPattern {
	matches := queryBlockRegex.FindAllStringSubmatch(raw, -1)
	patterns := make([]queryPattern, 0, len(matches))
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		name := strings.TrimSpace(match[1])
		description := strings.TrimSpace(match[2])
		sqlText := strings.TrimSpace(match[3])
		if name == "" || sqlText == "" {
			continue
		}
		patterns = append(patterns, queryPattern{
			Name:        name,
			Description: description,
			SQL:         sqlText,
			TablesUsed:  extractTables(sqlText),
		})
	}

	if len(patterns) > 0 {
		return patterns
	}

	sqlText := strings.TrimSpace(raw)
	if sqlText == "" {
		return []queryPattern{}
	}
	return []queryPattern{
		{
			Name:        fallbackName,
			Description: fallbackName,
			SQL:         sqlText,
			TablesUsed:  extractTables(sqlText),
		},
	}
}

func extractTables(sqlText string) []string {
	matches := tableRefRegex.FindAllStringSubmatch(sqlText, -1)
	if len(matches) == 0 {
		return []string{}
	}

	seen := map[string]struct{}{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		name = strings.Trim(name, `"`)
		name = strings.TrimSuffix(name, ",")
		if name == "" {
			continue
		}
		seen[strings.ToLower(name)] = struct{}{}
	}

	tables := make([]string, 0, len(seen))
	for name := range seen {
		tables = append(tables, name)
	}
	sort.Strings(tables)
	return tables
}
