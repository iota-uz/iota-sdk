package spotlight

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresPGTextSearchEngine struct {
	pool *pgxpool.Pool
}

func NewPostgresPGTextSearchEngine(pool *pgxpool.Pool) *PostgresPGTextSearchEngine {
	return &PostgresPGTextSearchEngine{pool: pool}
}

func (e *PostgresPGTextSearchEngine) Upsert(ctx context.Context, docs []SearchDocument) error {
	if len(docs) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, doc := range docs {
		meta, err := json.Marshal(doc.Metadata)
		if err != nil {
			return err
		}
		accessPolicy, err := json.Marshal(doc.Access)
		if err != nil {
			return err
		}
		batch.Queue(`
INSERT INTO spotlight_documents(
    id, tenant_id, provider, entity_type, title, body, url, language, metadata, access_policy, updated_at, embedding
) VALUES (
    $1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11,$12::vector
)
ON CONFLICT (tenant_id, id) DO UPDATE SET
    provider = EXCLUDED.provider,
    entity_type = EXCLUDED.entity_type,
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    url = EXCLUDED.url,
    language = EXCLUDED.language,
    metadata = EXCLUDED.metadata,
    access_policy = EXCLUDED.access_policy,
    updated_at = EXCLUDED.updated_at,
    embedding = EXCLUDED.embedding
`,
			doc.ID,
			doc.TenantID,
			doc.Provider,
			doc.EntityType,
			doc.Title,
			doc.Body,
			doc.URL,
			doc.Language,
			string(meta),
			string(accessPolicy),
			doc.UpdatedAt,
			toVectorLiteral(doc.Embedding),
		)
	}
	res := e.pool.SendBatch(ctx, batch)
	defer res.Close()

	for range docs {
		if _, err := res.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (e *PostgresPGTextSearchEngine) Delete(ctx context.Context, refs []DocumentRef) error {
	if len(refs) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, ref := range refs {
		batch.Queue(`DELETE FROM spotlight_documents WHERE tenant_id = $1 AND id = $2`, ref.TenantID, ref.ID)
	}
	res := e.pool.SendBatch(ctx, batch)
	defer res.Close()
	for range refs {
		if _, err := res.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (e *PostgresPGTextSearchEngine) Search(ctx context.Context, req SearchRequest) ([]SearchHit, error) {
	if req.TenantID == uuid.Nil {
		return nil, nil
	}
	limit := req.normalizedTopK()

	rows, err := e.pool.Query(ctx, `
WITH lexical AS (
    SELECT
        id,
        tenant_id,
        provider,
        entity_type,
        title,
        body,
        url,
        language,
        metadata,
        access_policy,
        updated_at,
        embedding,
        1.0 / (1.0 + bm25_score(sd.id)) AS lexical_score
    FROM spotlight_documents sd
    WHERE sd.tenant_id = $1
      AND ($2 = '' OR sd.id @@@ $2)
    ORDER BY lexical_score DESC
    LIMIT $3
), vector_ranked AS (
    SELECT
        id,
        CASE WHEN $4::vector IS NULL OR embedding IS NULL THEN 0.0
             ELSE 1.0 - (embedding <=> $4::vector)
        END AS vector_score
    FROM lexical
)
SELECT
    l.id,
    l.tenant_id,
    l.provider,
    l.entity_type,
    l.title,
    l.body,
    l.url,
    l.language,
    l.metadata,
    l.access_policy,
    l.updated_at,
    l.lexical_score,
    v.vector_score,
    (l.lexical_score * 0.75 + v.vector_score * 0.25) AS final_score
FROM lexical l
JOIN vector_ranked v ON v.id = l.id
ORDER BY final_score DESC
LIMIT $3
`, req.TenantID, strings.TrimSpace(req.Query), limit, toVectorLiteral(req.QueryEmbedding))
	if err != nil {
		return e.searchFallback(ctx, req, limit)
	}
	defer rows.Close()

	out := make([]SearchHit, 0, limit)
	for rows.Next() {
		var doc SearchDocument
		var metadataRaw []byte
		var accessPolicyRaw []byte
		var lexicalScore float64
		var vectorScore float64
		var finalScore float64
		if err := rows.Scan(
			&doc.ID,
			&doc.TenantID,
			&doc.Provider,
			&doc.EntityType,
			&doc.Title,
			&doc.Body,
			&doc.URL,
			&doc.Language,
			&metadataRaw,
			&accessPolicyRaw,
			&doc.UpdatedAt,
			&lexicalScore,
			&vectorScore,
			&finalScore,
		); err != nil {
			return nil, err
		}
		if len(metadataRaw) > 0 {
			doc.Metadata = map[string]string{}
			_ = json.Unmarshal(metadataRaw, &doc.Metadata)
		}
		if len(accessPolicyRaw) > 0 {
			_ = json.Unmarshal(accessPolicyRaw, &doc.Access)
		}
		out = append(out, SearchHit{
			Document:     doc,
			LexicalScore: lexicalScore,
			VectorScore:  vectorScore,
			FinalScore:   finalScore,
			WhyMatched:   "lexical+vector",
		})
	}
	return out, rows.Err()
}

func (e *PostgresPGTextSearchEngine) searchFallback(ctx context.Context, req SearchRequest, limit int) ([]SearchHit, error) {
	rows, err := e.pool.Query(ctx, `
SELECT
    id,
    tenant_id,
    provider,
    entity_type,
    title,
    body,
    url,
    language,
    metadata,
    access_policy,
    updated_at,
    ts_rank(to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(body,'')), plainto_tsquery('simple', $2)) AS lexical_score
FROM spotlight_documents
WHERE tenant_id = $1
  AND ($2 = '' OR to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(body,'')) @@ plainto_tsquery('simple', $2))
ORDER BY lexical_score DESC, updated_at DESC
LIMIT $3
`, req.TenantID, strings.TrimSpace(req.Query), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SearchHit, 0, limit)
	for rows.Next() {
		var doc SearchDocument
		var metadataRaw []byte
		var accessPolicyRaw []byte
		var lexicalScore float64
		if err := rows.Scan(
			&doc.ID,
			&doc.TenantID,
			&doc.Provider,
			&doc.EntityType,
			&doc.Title,
			&doc.Body,
			&doc.URL,
			&doc.Language,
			&metadataRaw,
			&accessPolicyRaw,
			&doc.UpdatedAt,
			&lexicalScore,
		); err != nil {
			return nil, err
		}
		if len(metadataRaw) > 0 {
			doc.Metadata = map[string]string{}
			_ = json.Unmarshal(metadataRaw, &doc.Metadata)
		}
		if len(accessPolicyRaw) > 0 {
			_ = json.Unmarshal(accessPolicyRaw, &doc.Access)
		}
		out = append(out, SearchHit{
			Document:     doc,
			LexicalScore: lexicalScore,
			FinalScore:   lexicalScore,
			WhyMatched:   "lexical-fallback",
		})
	}
	return out, rows.Err()
}

func (e *PostgresPGTextSearchEngine) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var n int
	if err := e.pool.QueryRow(ctx, `SELECT 1`).Scan(&n); err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("unexpected health check value: %d", n)
	}
	return nil
}

func toVectorLiteral(v []float32) any {
	if len(v) == 0 {
		return nil
	}
	parts := make([]string, 0, len(v))
	for _, n := range v {
		parts = append(parts, fmt.Sprintf("%f", n))
	}
	return "[" + strings.Join(parts, ",") + "]"
}
