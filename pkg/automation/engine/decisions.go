package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opDecisions = serrors.Op("automation.engine.decisions")

// DecisionTTL is how long a cached decision is kept.
const DecisionTTL = 30 * 24 * time.Hour

// DecisionCache keeps the decisions of models (a classifier, an LLM) for dry runs in cached mode: a
// repeated dry run takes the decision from the cache without calling the provider.
type DecisionCache interface {
	// Get is the cached decision of key; ok is false on a miss or an entry older than DecisionTTL.
	Get(ctx context.Context, tenantID uuid.UUID, key string) (json.RawMessage, bool, error)
	Put(ctx context.Context, tenantID uuid.UUID, key string, output json.RawMessage) error
}

// DecisionNode is an optional interface of a catalog.CallNode whose answer is a decision of a model: the
// key of a call is DecisionKey of its prompt, input, model and model params. A dry run in cached mode
// answers it from the DecisionCache when it can.
type DecisionNode interface {
	catalog.CallNode
	DecisionKey(in catalog.CallInput) (key string, ok bool)
}

// DecisionKey is the cache key of a decision: sha256 of the prompt, the input, the model and its params.
func DecisionKey(prompt string, input any, model string, params map[string]any) string {
	body, err := json.Marshal(struct {
		Prompt string         `json:"prompt"`
		Input  any            `json:"input"`
		Model  string         `json:"model"`
		Params map[string]any `json:"params"`
	}{prompt, input, model, params})
	if err != nil {
		body = []byte(prompt + "\x00" + model)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// dbDecisionCache is DecisionCache over automation.decision_cache.
type dbDecisionCache struct{ db *pgxpool.Pool }

func (c *dbDecisionCache) Get(ctx context.Context, tenantID uuid.UUID, key string) (json.RawMessage, bool, error) {
	var out []byte
	err := c.db.QueryRow(ctx, `SELECT output FROM automation.decision_cache WHERE tenant_id = $1 AND key = $2 AND created_at > $3`,
		tenantID, key, time.Now().Add(-DecisionTTL)).Scan(&out)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, serrors.FromDB(opDecisions, err)
	}
	return out, true, nil
}

func (c *dbDecisionCache) Put(ctx context.Context, tenantID uuid.UUID, key string, output json.RawMessage) error {
	_, err := c.db.Exec(ctx, `
		INSERT INTO automation.decision_cache (tenant_id, key, output) VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, key) DO UPDATE SET output = EXCLUDED.output, created_at = now()`, tenantID, key, []byte(output))
	return serrors.FromDB(opDecisions, err)
}
