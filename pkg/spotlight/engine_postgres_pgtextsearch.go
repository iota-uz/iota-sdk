package spotlight

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type PostgresPGTextSearchEngine struct {
	pool   *pgxpool.Pool
	logger *logrus.Logger
}

func NewPostgresPGTextSearchEngine(pool *pgxpool.Pool) *PostgresPGTextSearchEngine {
	return &PostgresPGTextSearchEngine{
		pool:   pool,
		logger: logrus.StandardLogger(),
	}
}

func (e *PostgresPGTextSearchEngine) Upsert(ctx context.Context, docs []SearchDocument) error {
	const op serrors.Op = "spotlight.PostgresPGTextSearchEngine.Upsert"

	if len(docs) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, doc := range docs {
		meta, err := json.Marshal(doc.Metadata)
		if err != nil {
			return serrors.E(op, err)
		}
		accessPolicy, err := json.Marshal(doc.Access)
		if err != nil {
			return serrors.E(op, err)
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
			return serrors.E(op, err)
		}
	}
	return nil
}

func (e *PostgresPGTextSearchEngine) Delete(ctx context.Context, refs []DocumentRef) error {
	const op serrors.Op = "spotlight.PostgresPGTextSearchEngine.Delete"

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
			return serrors.E(op, err)
		}
	}
	return nil
}

func (e *PostgresPGTextSearchEngine) Search(ctx context.Context, req SearchRequest) ([]SearchHit, error) {
	if req.TenantID == uuid.Nil {
		return nil, nil
	}
	limit := req.normalizedTopK()
	query := strings.TrimSpace(req.Query)
	userID := strings.TrimSpace(req.UserID)
	roles := dedupeAndSort(req.Roles)
	permissions := dedupeAndSort(req.Permissions)
	if len(req.QueryEmbedding) == 0 {
		return e.searchLexicalOnly(ctx, req.TenantID, query, userID, roles, permissions, limit)
	}
	return e.searchHybrid(ctx, req.TenantID, query, req.QueryEmbedding, userID, roles, permissions, limit)
}

func (e *PostgresPGTextSearchEngine) searchHybrid(ctx context.Context, tenantID uuid.UUID, query string, embedding []float32, userID string, roles []string, permissions []string, limit int) ([]SearchHit, error) {
	const op serrors.Op = "spotlight.PostgresPGTextSearchEngine.searchHybrid"

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
      AND (
        (sd.access_policy->>'visibility') = 'public'
        OR (
            $7 <> ''
            AND (sd.access_policy->>'visibility') = 'owner'
            AND (sd.access_policy->>'owner_id') = $7
        )
        OR (
            (sd.access_policy->>'visibility') = 'restricted'
            AND (
                ($7 <> '' AND COALESCE(sd.access_policy->'allowed_users', '[]'::jsonb) ? $7)
                OR (COALESCE(cardinality($8::text[]), 0) > 0 AND COALESCE(sd.access_policy->'allowed_roles', '[]'::jsonb) ?| $8::text[])
                OR (COALESCE(cardinality($9::text[]), 0) > 0 AND COALESCE(sd.access_policy->'allowed_permissions', '[]'::jsonb) ?| $9::text[])
            )
        )
      )
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
    (l.lexical_score * $5::float8 + v.vector_score * $6::float8) AS final_score
FROM lexical l
JOIN vector_ranked v ON v.id = l.id
ORDER BY final_score DESC
LIMIT $3
`, tenantID, query, limit, toVectorLiteral(embedding), DefaultLexicalWeight, DefaultVectorWeight, userID, roles, permissions)
	if err != nil {
		e.logger.WithError(err).WithFields(logrus.Fields{
			"tenant_id": tenantID.String(),
			"query":     query,
		}).Warn("spotlight primary search query failed, using fallback")
		return e.searchFallback(ctx, SearchRequest{
			TenantID:    tenantID,
			Query:       query,
			UserID:      userID,
			Roles:       roles,
			Permissions: permissions,
		}, limit)
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
			return nil, serrors.E(op, err)
		}
		if len(metadataRaw) > 0 {
			doc.Metadata = map[string]string{}
			if err := json.Unmarshal(metadataRaw, &doc.Metadata); err != nil {
				return nil, serrors.E(op, err)
			}
		}
		if len(accessPolicyRaw) > 0 {
			if err := json.Unmarshal(accessPolicyRaw, &doc.Access); err != nil {
				return nil, serrors.E(op, err)
			}
		}
		out = append(out, SearchHit{
			Document:     doc,
			LexicalScore: lexicalScore,
			VectorScore:  vectorScore,
			FinalScore:   finalScore,
			WhyMatched:   "lexical+vector",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

func (e *PostgresPGTextSearchEngine) searchLexicalOnly(ctx context.Context, tenantID uuid.UUID, query string, userID string, roles []string, permissions []string, limit int) ([]SearchHit, error) {
	const op serrors.Op = "spotlight.PostgresPGTextSearchEngine.searchLexicalOnly"

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
    1.0 / (1.0 + bm25_score(sd.id)) AS lexical_score
FROM spotlight_documents sd
WHERE sd.tenant_id = $1
  AND ($2 = '' OR sd.id @@@ $2)
  AND (
    (sd.access_policy->>'visibility') = 'public'
    OR (
        $4 <> ''
        AND (sd.access_policy->>'visibility') = 'owner'
        AND (sd.access_policy->>'owner_id') = $4
    )
    OR (
        (sd.access_policy->>'visibility') = 'restricted'
        AND (
            ($4 <> '' AND COALESCE(sd.access_policy->'allowed_users', '[]'::jsonb) ? $4)
            OR (COALESCE(cardinality($5::text[]), 0) > 0 AND COALESCE(sd.access_policy->'allowed_roles', '[]'::jsonb) ?| $5::text[])
            OR (COALESCE(cardinality($6::text[]), 0) > 0 AND COALESCE(sd.access_policy->'allowed_permissions', '[]'::jsonb) ?| $6::text[])
        )
    )
  )
ORDER BY lexical_score DESC, updated_at DESC
LIMIT $3
`, tenantID, query, limit, userID, roles, permissions)
	if err != nil {
		e.logger.WithError(err).WithFields(logrus.Fields{
			"tenant_id": tenantID.String(),
			"query":     query,
		}).Warn("spotlight lexical search query failed, using fallback")
		return e.searchFallback(ctx, SearchRequest{
			TenantID:    tenantID,
			Query:       query,
			UserID:      userID,
			Roles:       roles,
			Permissions: permissions,
		}, limit)
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
			return nil, serrors.E(op, err)
		}
		if len(metadataRaw) > 0 {
			doc.Metadata = map[string]string{}
			if err := json.Unmarshal(metadataRaw, &doc.Metadata); err != nil {
				return nil, serrors.E(op, err)
			}
		}
		if len(accessPolicyRaw) > 0 {
			if err := json.Unmarshal(accessPolicyRaw, &doc.Access); err != nil {
				return nil, serrors.E(op, err)
			}
		}
		out = append(out, SearchHit{
			Document:     doc,
			LexicalScore: lexicalScore,
			FinalScore:   lexicalScore,
			WhyMatched:   "lexical",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

func (e *PostgresPGTextSearchEngine) searchFallback(ctx context.Context, req SearchRequest, limit int) ([]SearchHit, error) {
	const op serrors.Op = "spotlight.PostgresPGTextSearchEngine.searchFallback"

	userID := strings.TrimSpace(req.UserID)
	roles := dedupeAndSort(req.Roles)
	permissions := dedupeAndSort(req.Permissions)

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
    ts_rank(to_tsvector('simple', coalesce(sd.title,'') || ' ' || coalesce(sd.body,'')), plainto_tsquery('simple', $2)) AS lexical_score
	FROM spotlight_documents sd
WHERE sd.tenant_id = $1
  AND ($2 = '' OR to_tsvector('simple', coalesce(sd.title,'') || ' ' || coalesce(sd.body,'')) @@ plainto_tsquery('simple', $2))
  AND (
    (sd.access_policy->>'visibility') = 'public'
    OR (
        $4 <> ''
        AND (sd.access_policy->>'visibility') = 'owner'
        AND (sd.access_policy->>'owner_id') = $4
    )
    OR (
        (sd.access_policy->>'visibility') = 'restricted'
        AND (
            ($4 <> '' AND COALESCE(sd.access_policy->'allowed_users', '[]'::jsonb) ? $4)
            OR (COALESCE(cardinality($5::text[]), 0) > 0 AND COALESCE(sd.access_policy->'allowed_roles', '[]'::jsonb) ?| $5::text[])
            OR (COALESCE(cardinality($6::text[]), 0) > 0 AND COALESCE(sd.access_policy->'allowed_permissions', '[]'::jsonb) ?| $6::text[])
        )
    )
  )
ORDER BY lexical_score DESC, updated_at DESC
LIMIT $3
	`, req.TenantID, strings.TrimSpace(req.Query), limit, userID, roles, permissions)
	if err != nil {
		return nil, serrors.E(op, err)
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
			return nil, serrors.E(op, err)
		}
		if len(metadataRaw) > 0 {
			doc.Metadata = map[string]string{}
			if err := json.Unmarshal(metadataRaw, &doc.Metadata); err != nil {
				return nil, serrors.E(op, err)
			}
		}
		if len(accessPolicyRaw) > 0 {
			if err := json.Unmarshal(accessPolicyRaw, &doc.Access); err != nil {
				return nil, serrors.E(op, err)
			}
		}
		out = append(out, SearchHit{
			Document:     doc,
			LexicalScore: lexicalScore,
			FinalScore:   lexicalScore,
			WhyMatched:   "lexical-fallback",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

func (e *PostgresPGTextSearchEngine) Health(ctx context.Context) error {
	const op serrors.Op = "spotlight.PostgresPGTextSearchEngine.Health"

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var n int
	if err := e.pool.QueryRow(ctx, `SELECT 1`).Scan(&n); err != nil {
		return serrors.E(op, err)
	}
	if n != 1 {
		return serrors.E(op, fmt.Errorf("unexpected health check value: %d", n))
	}
	return nil
}

func toVectorLiteral(v []float32) any {
	if len(v) == 0 {
		return nil
	}
	parts := make([]string, 0, len(v))
	for _, n := range v {
		parts = append(parts, strconv.FormatFloat(float64(n), 'f', -1, 32))
	}
	return "[" + strings.Join(parts, ",") + "]"
}
