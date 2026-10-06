package reporting

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/reporting"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

var ErrReportMissingMeasure = errors.New("registered report measure contains missing values; no complete report generated")

// ReportCatalogTool keeps discovery bounded; definitions are retrieved on demand
// instead of expanding hundreds of report tools in the model's context.
type ReportCatalogTool struct{ catalog *reporting.Catalog }

func NewReportCatalogTool(c *reporting.Catalog) *ReportCatalogTool {
	return &ReportCatalogTool{catalog: c}
}
func (t *ReportCatalogTool) Name() string { return "discover_report_definitions" }
func (t *ReportCatalogTool) Description() string {
	return "Discover server-defined report datasets, measures, dimensions, population, grain, period basis and limitations. Search is AND across words with bounded pagination; use a short term or empty search if no matches. Get an exact dataset_id before planning. No SQL is returned. An absent methodology must not be invented."
}
func (t *ReportCatalogTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"dataset_id": map[string]any{"type": "string"}, "search": map[string]any{"type": "string"},
		"offset": map[string]any{"type": "integer", "minimum": 0}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 20},
	}}
}
func (t *ReportCatalogTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}
func (t *ReportCatalogTool) CallStructured(_ context.Context, input string) (*types.ToolResult, error) {
	const op = "ReportCatalogTool.Call"
	p, err := agents.ParseToolInput[struct {
		Dataset string `json:"dataset_id"`
		Search  string `json:"search"`
		Offset  int    `json:"offset"`
		Limit   int    `json:"limit"`
	}](input)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if p.Dataset != "" {
		d, err := t.catalog.Dataset(p.Dataset)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		return &types.ToolResult{CodecID: types.CodecJSON, Payload: types.JSONPayload{Output: map[string]any{"definition": d}}}, nil
	}
	if p.Limit == 0 {
		p.Limit = 5
	}
	definitions, total, err := t.catalog.Search(p.Search, p.Offset, p.Limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	// Search hits are summaries. Fetching one definition reveals its fields.
	hits := []map[string]any{}
	for _, d := range definitions {
		hits = append(hits, map[string]any{"id": d.ID, "version": d.Version, "description": d.Description, "grain": d.Grain})
	}
	return &types.ToolResult{CodecID: types.CodecJSON, Payload: types.JSONPayload{Output: map[string]any{"datasets": hits, "total": total, "offset": p.Offset, "has_more": p.Offset+len(hits) < total}}}, nil
}

type SemanticReportTool struct {
	catalog      *reporting.Catalog
	executor     bichatsql.QueryExecutor
	dir, baseURL string
}

func NewSemanticReportTool(c *reporting.Catalog, e bichatsql.QueryExecutor, dir, baseURL string) *SemanticReportTool {
	return &SemanticReportTool{catalog: c, executor: NewReportExecutor(e), dir: dir, baseURL: baseURL}
}
func (t *SemanticReportTool) Name() string { return "execute_report_plan" }
func (t *SemanticReportTool) Description() string {
	return "Compile and execute a declarative report using a discovered dataset/version. Select registered measures and dimensions, detail or aggregate, optional typed dimension filters and ordering. Date buckets only in aggregate mode. Currency dimensions are mandatory and automatically included. No arbitrary SQL, joins, custom formulas or LIMIT. Creates a complete typed Excel with recorded plan, definitions and exact additive totals from the same result, plus a bounded preview. Refuses unsupported semantics or incomplete/unsafe exports."
}
func (t *SemanticReportTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"dataset": map[string]any{"type": "string"}, "version": map[string]any{"type": "string"},
		"mode":       map[string]any{"type": "string", "enum": []string{"detail", "aggregate"}},
		"period":     map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"start": map[string]any{"type": "string"}, "end_exclusive": map[string]any{"type": "string"}}, "required": []string{"start", "end_exclusive"}},
		"measures":   map[string]any{"type": "array", "minItems": 1, "maxItems": 16, "items": map[string]any{"type": "string"}},
		"dimensions": map[string]any{"type": "array", "maxItems": 32, "items": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"name": map[string]any{"type": "string"}, "bucket": map[string]any{"type": "string", "enum": []string{"day", "week", "month", "quarter", "year"}}}, "required": []string{"name"}}},
		"filters":    map[string]any{"type": "array", "maxItems": 32, "items": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"field": map[string]any{"type": "string"}, "operator": map[string]any{"type": "string", "enum": []string{"eq", "ne", "in", "gt", "gte", "lt", "lte", "is_null", "not_null"}}, "values": map[string]any{"type": "array", "maxItems": 100, "items": map[string]any{"type": []string{"string", "number"}}}}, "required": []string{"field", "operator"}}},
		"order":      map[string]any{"type": "array", "maxItems": 16, "items": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"field": map[string]any{"type": "string"}, "descending": map[string]any{"type": "boolean"}}, "required": []string{"field"}}},
	}, "required": []string{"dataset", "version", "mode", "period", "measures", "dimensions"}}
}
func (t *SemanticReportTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}
func (t *SemanticReportTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	const op = "SemanticReportTool.Call"
	p, err := reporting.ParsePlan(input)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	compiled, err := t.catalog.Compile(p)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	// A single SQL statement provides one PostgreSQL statement snapshot for all
	// measures and detail. Export and control totals never re-query the source.
	r, err := t.executor.ExecuteQuery(ctx, compiled.SQL, compiled.Args, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	r, populationCount, err := checkedSemanticResult(compiled, r)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	totals, err := semanticTotals(compiled, r)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	manifest := map[string]any{"plan": compiled.Plan, "plan_sha256": compiled.Fingerprint, "definition": compiled.Definition, "complete": true, "row_count": len(r.Rows), "totals_by_measure_and_unit": totals, "generated_at": time.Now().UTC().Format(time.RFC3339Nano), "assurance": "registered_definition; not financial statement approval", "snapshot": "single SQL statement; workbook preserves executed result"}
	manifest["population_record_count"] = populationCount
	request, err := json.Marshal(map[string]any{"sql": compiled.SQL, "filename": p.Dataset, "description": compiled.Definition.Description})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	out, err := NewReportExportTool(&reportResultExecutor{result: r}, t.dir, t.baseURL).CallStructured(ctx, string(request))
	if err != nil {
		return out, fmt.Errorf("%s: %w", op, err)
	}
	payload, ok := out.Payload.(types.JSONPayload)
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, ErrReportShape)
	}
	encoded, err := json.Marshal(payload.Output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	var response map[string]any
	if err = json.Unmarshal(encoded, &response); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	filename, ok := response["filename"].(string)
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, ErrReportShape)
	}
	path := filepath.Join(t.dir, filename)
	if err = addReportManifest(path, manifest); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(content))
	response["report_contract"], response["sha256"], response["file_size_kb"] = manifest, digest, len(content)/1024
	previewCount := len(r.Rows)
	if previewCount > 200 {
		previewCount = 200
	}
	response["columns"], response["preview_rows"], response["preview_only"] = r.Columns, r.Rows[:previewCount], previewCount < len(r.Rows)
	out.Payload = types.JSONPayload{Output: response}
	for i := range out.Artifacts {
		out.Artifacts[i].Metadata["report_contract"] = manifest
		out.Artifacts[i].Metadata["basis"] = "registered_definition"
		out.Artifacts[i].Metadata["sha256"] = digest
		out.Artifacts[i].Metadata["file_size_kb"] = len(content) / 1024
		out.Artifacts[i].SizeBytes = int64(len(content))
	}
	return out, nil
}

// SUM ignores NULL, so output shape/precision alone cannot establish complete
// aggregation. Check null counts and population count from the same statement.
func checkedSemanticResult(c *reporting.Compiled, r *bichatsql.QueryResult) (*bichatsql.QueryResult, string, error) {
	if r == nil || r.Truncated || r.RowCount != len(r.Rows) {
		return nil, "", ErrIncompleteReport
	}
	visible := len(c.Columns)
	fields := append(append([]reporting.Field(nil), c.Columns...), c.Diagnostics...)
	if len(r.Columns) != len(fields) || len(r.ColumnTypes) != len(fields) {
		return nil, "", ErrReportShape
	}
	for i, f := range fields {
		if r.Columns[i] != f.Name || r.ColumnTypes[i] != f.Type {
			return nil, "", ErrReportShape
		}
	}
	population := decimal.Zero
	out := *r
	out.Columns = r.Columns[:visible]
	out.ColumnTypes = r.ColumnTypes[:visible]
	out.Rows = make([][]any, len(r.Rows))
	for i, row := range r.Rows {
		if len(row) != len(fields) {
			return nil, "", ErrReportShape
		}
		for j, field := range c.Diagnostics {
			count, err := decimal.NewFromString(fmt.Sprint(row[visible+j]))
			if err != nil || count.IsNegative() || !count.Equal(count.Truncate(0)) {
				return nil, "", ErrReportShape
			}
			if field.Name == "_report_population_count" {
				population = population.Add(count)
			} else if !count.IsZero() {
				return nil, "", ErrReportMissingMeasure
			}
		}
		out.Rows[i] = append([]any(nil), row[:visible]...)
	}
	if c.Plan.Mode == "detail" {
		population = decimal.NewFromInt(int64(len(r.Rows)))
	}
	return &out, population.String(), nil
}

func semanticTotals(c *reporting.Compiled, r *bichatsql.QueryResult) (map[string]map[string]string, error) {
	if r == nil || r.Truncated || r.RowCount != len(r.Rows) {
		return nil, ErrIncompleteReport
	}
	if len(r.Columns) != len(c.Columns) || len(r.ColumnTypes) != len(c.Columns) {
		return nil, ErrReportShape
	}
	indices := map[string]int{}
	for i, f := range c.Columns {
		if r.Columns[i] != f.Name || r.ColumnTypes[i] != f.Type {
			return nil, ErrReportShape
		}
		indices[f.Name] = i
	}
	result := map[string]map[string]string{}
	for _, m := range c.Measures {
		sums := map[string]decimal.Decimal{}
		for _, row := range r.Rows {
			if len(row) != len(c.Columns) {
				return nil, ErrReportShape
			}
			unit := m.Unit
			if m.CurrencyDimension != "" {
				code, ok := row[indices[m.CurrencyDimension]].(string)
				if !ok || code == "" {
					return nil, ErrReportShape
				}
				unit = code
			}
			value := row[indices[m.Name]]
			if value == nil {
				return nil, ErrReportMissingMeasure
			}
			amount, err := decimal.NewFromString(fmt.Sprint(value))
			if err != nil {
				return nil, ErrReportShape
			}
			sums[unit] = sums[unit].Add(amount)
		}
		result[m.Name] = map[string]string{}
		for unit, sum := range sums {
			result[m.Name][unit] = sum.String()
		}
	}
	return result, nil
}

// Nested metadata and exact decimal totals are JSON text so Excel cannot round
// them. The downloaded file remains self-describing outside the conversation.
func addReportManifest(filename string, manifest map[string]any) (err error) {
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close report manifest workbook: %w", closeErr)
		}
	}()
	if _, err = f.NewSheet("Report contract"); err != nil {
		return err
	}
	keys := make([]string, 0, len(manifest))
	for key := range manifest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		value, err := json.Marshal(manifest[key])
		if err != nil {
			return err
		}
		if err = f.SetSheetRow("Report contract", fmt.Sprintf("A%d", i+1), &[]any{key, string(value)}); err != nil {
			return err
		}
	}
	return f.Save()
}
