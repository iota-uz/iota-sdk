package spotlight

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxProcessor interface {
	PollAndProcess(ctx context.Context) error
}

type PostgresOutboxProcessor struct {
	pool   *pgxpool.Pool
	engine IndexEngine
	limit  int
}

func NewPostgresOutboxProcessor(pool *pgxpool.Pool, engine IndexEngine) *PostgresOutboxProcessor {
	return &PostgresOutboxProcessor{
		pool:   pool,
		engine: engine,
		limit:  200,
	}
}

type outboxRow struct {
	ID         int64
	TenantID   uuid.UUID
	Provider   string
	EventType  string
	DocumentID string
	Payload    []byte
}

func (p *PostgresOutboxProcessor) PollAndProcess(ctx context.Context) error {
	const op serrors.Op = "spotlight.PostgresOutboxProcessor.PollAndProcess"

	if p == nil || p.pool == nil || p.engine == nil {
		return nil
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return serrors.E(op, err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// spotlight_outbox is a shared system queue; tenant isolation is enforced per row during processing using tenant_id.
	rows, err := tx.Query(ctx, `
SELECT id, tenant_id, provider, event_type, document_id, payload
FROM spotlight_outbox
WHERE processed_at IS NULL
ORDER BY created_at
FOR UPDATE SKIP LOCKED
LIMIT $1
`, p.limit)
	if err != nil {
		return serrors.E(op, err)
	}
	defer rows.Close()

	batch := make([]outboxRow, 0, p.limit)
	for rows.Next() {
		var row outboxRow
		if scanErr := rows.Scan(
			&row.ID,
			&row.TenantID,
			&row.Provider,
			&row.EventType,
			&row.DocumentID,
			&row.Payload,
		); scanErr != nil {
			return serrors.E(op, scanErr)
		}
		batch = append(batch, row)
	}
	if err := rows.Err(); err != nil {
		return serrors.E(op, err)
	}
	if len(batch) == 0 {
		if err := tx.Commit(ctx); err != nil {
			return serrors.E(op, err)
		}
		return nil
	}

	processedIDs := make([]int64, 0, len(batch))
	for _, row := range batch {
		if err := p.processEvent(ctx, row); err != nil {
			return serrors.E(op, fmt.Errorf("process outbox id=%d: %w", row.ID, err))
		}
		processedIDs = append(processedIDs, row.ID)
	}

	if _, err := tx.Exec(ctx, `
UPDATE spotlight_outbox
SET processed_at = NOW()
WHERE id = ANY($1::bigint[])
`, processedIDs); err != nil {
		return serrors.E(op, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (p *PostgresOutboxProcessor) processEvent(ctx context.Context, row outboxRow) error {
	const op serrors.Op = "spotlight.PostgresOutboxProcessor.processEvent"

	eventType := strings.ToLower(strings.TrimSpace(row.EventType))
	switch eventType {
	case "delete":
		if row.DocumentID == "" {
			return nil
		}
		return p.engine.Delete(ctx, []DocumentRef{{
			TenantID: row.TenantID,
			ID:       row.DocumentID,
		}})
	case "create", "update", "":
		doc, err := parseOutboxDocument(row)
		if err != nil {
			return serrors.E(op, err)
		}
		if doc == nil {
			return nil
		}
		return p.engine.Upsert(ctx, []SearchDocument{*doc})
	default:
		return nil
	}
}

func parseOutboxDocument(row outboxRow) (*SearchDocument, error) {
	const op serrors.Op = "spotlight.parseOutboxDocument"

	payload := strings.TrimSpace(string(row.Payload))
	if payload == "" || payload == "{}" || payload == "null" {
		return nil, nil
	}

	var event DocumentEvent
	eventParseErr := json.Unmarshal(row.Payload, &event)
	if eventParseErr == nil && event.Document != nil {
		doc := *event.Document
		normalizeOutboxDocument(&doc, row)
		return &doc, nil
	}

	var doc SearchDocument
	if err := json.Unmarshal(row.Payload, &doc); err != nil {
		return nil, serrors.E(op, fmt.Errorf(
			"unable to parse spotlight outbox payload as document event or document: event_parse_err=%v document_parse_err=%w",
			eventParseErr,
			err,
		))
	}
	normalizeOutboxDocument(&doc, row)
	return &doc, nil
}

func normalizeOutboxDocument(doc *SearchDocument, row outboxRow) {
	if doc == nil {
		return
	}
	if doc.TenantID == uuid.Nil {
		doc.TenantID = row.TenantID
	}
	if doc.Provider == "" {
		doc.Provider = row.Provider
	}
	if doc.ID == "" {
		doc.ID = row.DocumentID
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = time.Now().UTC()
	}
	if doc.Access.Visibility == "" {
		doc.Access.Visibility = VisibilityRestricted
	}
}
