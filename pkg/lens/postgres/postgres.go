// Package postgres provides a PostgreSQL-backed Lens datasource implementation.
package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	leadingSQLKeywordPattern = regexp.MustCompile(`^[\s(]*([A-Z]+)\b`)
	forbiddenSQLPatterns     = []*regexp.Regexp{
		regexp.MustCompile(`\bINSERT\b`),
		regexp.MustCompile(`\bUPDATE\b`),
		regexp.MustCompile(`\bDELETE\b`),
		regexp.MustCompile(`\bMERGE\b`),
		regexp.MustCompile(`\bCREATE\b`),
		regexp.MustCompile(`\bALTER\b`),
		regexp.MustCompile(`\bDROP\b`),
		regexp.MustCompile(`\bTRUNCATE\b`),
		regexp.MustCompile(`\bGRANT\b`),
		regexp.MustCompile(`\bREVOKE\b`),
		regexp.MustCompile(`\bCOPY\b`),
		regexp.MustCompile(`\bCALL\b`),
	}
)

type Config struct {
	ConnectionString string
	MaxConnections   int32
	MinConnections   int32
	QueryTimeout     time.Duration
}

type DataSource struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func New(cfg Config) (*DataSource, error) {
	op := serrors.Op("lens/postgres.New")
	if strings.TrimSpace(cfg.ConnectionString) == "" {
		return nil, serrors.E(op, fmt.Errorf("connection string is required"))
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.ConnectionString)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if cfg.MaxConnections > 0 {
		poolCfg.MaxConns = cfg.MaxConnections
	}
	if cfg.MinConnections > 0 {
		poolCfg.MinConns = cfg.MinConnections
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	timeout := cfg.QueryTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &DataSource{pool: pool, timeout: timeout}, nil
}

func NewFromPool(pool *pgxpool.Pool) *DataSource {
	return &DataSource{pool: pool, timeout: 30 * time.Second}
}

func (d *DataSource) Capabilities() datasource.CapabilitySet {
	return datasource.CapabilitySet{
		datasource.CapabilityParameterizedQueries: true,
		datasource.CapabilityTransactions:         true,
		datasource.CapabilityTimeouts:             true,
	}
}

func (d *DataSource) Run(ctx context.Context, req datasource.QueryRequest) (*frame.FrameSet, error) {
	op := serrors.Op("lens/postgres.Run")
	if err := validateQuery(req.Text); err != nil {
		return nil, serrors.E(op, err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	args := pgx.NamedArgs(req.Params)
	type queryer interface {
		Query(context.Context, string, ...any) (pgx.Rows, error)
	}

	var executor queryer
	if tx, err := composables.UseTx(queryCtx); err == nil {
		executor = tx
	} else {
		executor = d.pool
	}

	rows, err := executor.Query(queryCtx, applyMaxRows(req.Text, req.MaxRows), args)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	descs := rows.FieldDescriptions()
	fields := make([]frame.Field, len(descs))
	for i, desc := range descs {
		fields[i] = frame.Field{
			Name: desc.Name,
			Type: inferType(desc.DataTypeOID),
		}
	}

	fr, err := frame.New(req.Source, fields...)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	for rows.Next() {
		values, valueErr := rows.Values()
		if valueErr != nil {
			return nil, serrors.E(op, valueErr)
		}
		row := make(map[string]any, len(fields))
		for i, field := range fields {
			row[field.Name] = values[i]
		}
		if err := fr.AppendRow(row); err != nil {
			return nil, serrors.E(op, err)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return frame.NewFrameSet(fr)
}

func validateQuery(query string) error {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return fmt.Errorf("query is required")
	}
	sanitized := strings.ToUpper(sanitizeSQL(trimmed))
	matches := leadingSQLKeywordPattern.FindStringSubmatch(sanitized)
	if len(matches) < 2 {
		return fmt.Errorf("query is required")
	}
	switch matches[1] {
	case "SELECT", "WITH":
	default:
		return fmt.Errorf("only SELECT and WITH queries are allowed")
	}
	for _, pattern := range forbiddenSQLPatterns {
		if pattern.MatchString(sanitized) {
			return fmt.Errorf("only read-only SELECT queries are allowed")
		}
	}
	return nil
}

func inferType(oid uint32) frame.FieldType {
	switch oid {
	case 25, 1043, 1042:
		return frame.FieldTypeString
	case 21, 23, 20, 700, 701, 1700: // 1700 = PostgreSQL numeric/decimal
		return frame.FieldTypeNumber
	case 16:
		return frame.FieldTypeBoolean
	case 1114, 1184, 1082, 1083:
		return frame.FieldTypeTime
	default:
		return frame.FieldTypeUnknown
	}
}

func applyMaxRows(query string, maxRows int) string {
	if maxRows <= 0 {
		return query
	}
	trimmed := strings.TrimSpace(query)
	trimmed = strings.TrimSuffix(trimmed, ";")
	return fmt.Sprintf("SELECT * FROM (%s) AS lens_query LIMIT %d", trimmed, maxRows)
}

func sanitizeSQL(query string) string {
	var b strings.Builder
	b.Grow(len(query))
	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(query); i++ {
		ch := query[i]
		next := byte(0)
		if i+1 < len(query) {
			next = query[i+1]
		}
		switch {
		case inLineComment:
			if ch == '\n' {
				inLineComment = false
				b.WriteByte(ch)
			} else {
				b.WriteByte(' ')
			}
		case inBlockComment:
			if ch == '*' && next == '/' {
				inBlockComment = false
				b.WriteString("  ")
				i++
			} else {
				b.WriteByte(' ')
			}
		case inSingleQuote:
			if ch == '\'' {
				if next == '\'' {
					b.WriteString("  ")
					i++
					continue
				}
				inSingleQuote = false
			}
			b.WriteByte(' ')
		case inDoubleQuote:
			if ch == '"' {
				inDoubleQuote = false
			}
			b.WriteByte(' ')
		case ch == '-' && next == '-':
			inLineComment = true
			b.WriteString("  ")
			i++
		case ch == '/' && next == '*':
			inBlockComment = true
			b.WriteString("  ")
			i++
		case ch == '\'':
			inSingleQuote = true
			b.WriteByte(' ')
		case ch == '"':
			inDoubleQuote = true
			b.WriteByte(' ')
		default:
			b.WriteByte(ch)
		}
	}
	return b.String()
}
