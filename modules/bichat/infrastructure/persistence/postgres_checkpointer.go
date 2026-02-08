package persistence

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5"
)

// PostgresCheckpointer implements agents.Checkpointer using PostgreSQL.
// It supports multi-tenant isolation via tenant_id.
type PostgresCheckpointer struct{}

// NewPostgresCheckpointer creates a new PostgreSQL checkpointer.
func NewPostgresCheckpointer() agents.Checkpointer {
	return &PostgresCheckpointer{}
}

const (
	checkpointInsertQuery = `
		INSERT INTO bichat.checkpoints (
			id, thread_id, tenant_id, user_id, agent_name, messages, pending_tools,
			interrupt_type, interrupt_data, session_id, previous_response_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	checkpointSelectQuery = `
		SELECT id, tenant_id, session_id, thread_id, agent_name, messages, pending_tools,
		       interrupt_type, interrupt_data, previous_response_id, created_at
		FROM bichat.checkpoints
		WHERE id = $1 AND tenant_id = $2
	`

	checkpointSelectByThreadQuery = `
		SELECT id, tenant_id, session_id, thread_id, agent_name, messages, pending_tools,
		       interrupt_type, interrupt_data, previous_response_id, created_at
		FROM bichat.checkpoints
		WHERE thread_id = $1 AND tenant_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	checkpointDeleteQuery = `
		DELETE FROM bichat.checkpoints
		WHERE id = $1 AND tenant_id = $2
	`
)

// Save saves a checkpoint to PostgreSQL.
func (p *PostgresCheckpointer) Save(ctx context.Context, checkpoint *agents.Checkpoint) (string, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return "", err
	}

	user, err := composables.UseUser(ctx)
	if err != nil {
		return "", err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return "", err
	}

	// Marshal JSON fields
	messagesJSON, err := json.Marshal(checkpoint.Messages)
	if err != nil {
		return "", err
	}

	pendingToolsJSON, err := json.Marshal(checkpoint.PendingTools)
	if err != nil {
		return "", err
	}

	_, err = tx.Exec(ctx, checkpointInsertQuery,
		checkpoint.ID,
		checkpoint.ThreadID,
		tenantID,
		user.ID(),
		checkpoint.AgentName,
		messagesJSON,
		pendingToolsJSON,
		checkpoint.InterruptType,
		checkpoint.InterruptData,
		checkpoint.SessionID,
		checkpoint.PreviousResponseID,
		checkpoint.CreatedAt,
	)
	if err != nil {
		return "", err
	}

	return checkpoint.ID, nil
}

// Load retrieves a checkpoint by ID from PostgreSQL.
func (p *PostgresCheckpointer) Load(ctx context.Context, id string) (*agents.Checkpoint, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var checkpoint agents.Checkpoint
	var messagesJSON, pendingToolsJSON []byte
	var interruptData *[]byte
	var previousResponseID *string

	err = tx.QueryRow(ctx, checkpointSelectQuery, id, tenantID).Scan(
		&checkpoint.ID,
		&checkpoint.TenantID,
		&checkpoint.SessionID,
		&checkpoint.ThreadID,
		&checkpoint.AgentName,
		&messagesJSON,
		&pendingToolsJSON,
		&checkpoint.InterruptType,
		&interruptData,
		&previousResponseID,
		&checkpoint.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, agents.ErrCheckpointNotFound
		}
		return nil, err
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(messagesJSON, &checkpoint.Messages); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(pendingToolsJSON, &checkpoint.PendingTools); err != nil {
		return nil, err
	}

	if interruptData != nil {
		checkpoint.InterruptData = *interruptData
	}
	checkpoint.PreviousResponseID = previousResponseID

	return &checkpoint, nil
}

// LoadByThreadID retrieves the latest checkpoint for a thread.
func (p *PostgresCheckpointer) LoadByThreadID(ctx context.Context, threadID string) (*agents.Checkpoint, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var checkpoint agents.Checkpoint
	var messagesJSON, pendingToolsJSON []byte
	var interruptData *[]byte
	var previousResponseID *string

	err = tx.QueryRow(ctx, checkpointSelectByThreadQuery, threadID, tenantID).Scan(
		&checkpoint.ID,
		&checkpoint.TenantID,
		&checkpoint.SessionID,
		&checkpoint.ThreadID,
		&checkpoint.AgentName,
		&messagesJSON,
		&pendingToolsJSON,
		&checkpoint.InterruptType,
		&interruptData,
		&previousResponseID,
		&checkpoint.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, agents.ErrCheckpointNotFound
		}
		return nil, err
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(messagesJSON, &checkpoint.Messages); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(pendingToolsJSON, &checkpoint.PendingTools); err != nil {
		return nil, err
	}

	if interruptData != nil {
		checkpoint.InterruptData = *interruptData
	}
	checkpoint.PreviousResponseID = previousResponseID

	return &checkpoint, nil
}

// Delete removes a checkpoint by ID.
func (p *PostgresCheckpointer) Delete(ctx context.Context, id string) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	result, err := tx.Exec(ctx, checkpointDeleteQuery, id, tenantID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return agents.ErrCheckpointNotFound
	}

	return nil
}

// LoadAndDelete atomically retrieves and removes a checkpoint by ID.
func (p *PostgresCheckpointer) LoadAndDelete(ctx context.Context, id string) (*agents.Checkpoint, error) {
	// Use transaction to ensure atomicity
	var checkpoint *agents.Checkpoint
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		checkpoint, err = p.Load(txCtx, id)
		if err != nil {
			return err
		}

		return p.Delete(txCtx, id)
	})

	if err != nil {
		return nil, err
	}

	return checkpoint, nil
}
