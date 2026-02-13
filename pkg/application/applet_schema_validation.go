package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type appletSchemaArtifact struct {
	Version int `json:"version"`
	Tables  map[string]struct {
		Required []string `json:"required"`
	} `json:"tables"`
}

func validateAppletSchemaArtifact(ctx context.Context, pool *pgxpool.Pool, appletName string) error {
	if pool == nil || strings.TrimSpace(appletName) == "" {
		return nil
	}
	artifactPath := resolveAppletSchemaArtifactPath(appletName)
	payload, err := os.ReadFile(artifactPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read schema artifact for %s: %w", appletName, err)
	}

	var artifact appletSchemaArtifact
	if err := json.Unmarshal(payload, &artifact); err != nil {
		return fmt.Errorf("decode schema artifact for %s: %w", appletName, err)
	}
	if len(artifact.Tables) == 0 {
		return nil
	}

	rows, err := pool.Query(ctx, `
SELECT tenant_id, table_name, document_id, value
FROM applets.documents
WHERE applet_id = $1
`, appletName)
	if err != nil {
		return fmt.Errorf("query documents for schema validation (%s): %w", appletName, err)
	}
	defer rows.Close()

	violations := make([]string, 0)
	for rows.Next() {
		var (
			tenantID   string
			tableName  string
			documentID string
			rawValue   []byte
		)
		if err := rows.Scan(&tenantID, &tableName, &documentID, &rawValue); err != nil {
			return fmt.Errorf("scan schema validation row (%s): %w", appletName, err)
		}
		tableDef, ok := artifact.Tables[tableName]
		if !ok || len(tableDef.Required) == 0 {
			continue
		}
		var value map[string]any
		if err := json.Unmarshal(rawValue, &value); err != nil {
			violations = append(violations, fmt.Sprintf("tenant=%s table=%s id=%s invalid_json", tenantID, tableName, documentID))
			continue
		}
		for _, field := range tableDef.Required {
			field = strings.TrimSpace(field)
			if field == "" {
				continue
			}
			if _, exists := value[field]; !exists {
				violations = append(violations, fmt.Sprintf("tenant=%s table=%s id=%s missing=%s", tenantID, tableName, documentID, field))
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate schema validation rows (%s): %w", appletName, err)
	}
	if len(violations) == 0 {
		return nil
	}
	const maxViolations = 25
	if len(violations) > maxViolations {
		violations = violations[:maxViolations]
	}
	return fmt.Errorf("schema validation failed for %s: %s", appletName, strings.Join(violations, "; "))
}

func resolveAppletSchemaArtifactPath(appletName string) string {
	_, currentFile, _, ok := goruntime.Caller(0)
	if !ok {
		return filepath.Join("modules", appletName, "runtime", "schema.artifact.json")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	return filepath.Join(root, "modules", appletName, "runtime", "schema.artifact.json")
}
