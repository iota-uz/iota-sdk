package persistence

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence/models"
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

	m, err := models.CheckpointModelFromDomain(checkpoint, user.ID())
	if err != nil {
		return "", err
	}
	m.TenantID = tenantID

	_, err = tx.Exec(ctx, checkpointInsertQuery,
		m.ID,
		m.ThreadID,
		m.TenantID,
		m.UserID,
		m.AgentName,
		m.Messages,
		m.PendingTools,
		m.InterruptType,
		m.InterruptData,
		m.SessionID,
		m.PreviousResponseID,
		m.CreatedAt,
	)
	if err != nil {
		return "", err
	}

	return checkpoint.ID, nil
}

// scanCheckpoint scans a row into a CheckpointModel.
func scanCheckpoint(row pgx.Row) (*models.CheckpointModel, error) {
	var m models.CheckpointModel
	err := row.Scan(
		&m.ID,
		&m.TenantID,
		&m.SessionID,
		&m.ThreadID,
		&m.AgentName,
		&m.Messages,
		&m.PendingTools,
		&m.InterruptType,
		&m.InterruptData,
		&m.PreviousResponseID,
		&m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, agents.ErrCheckpointNotFound
		}
		return nil, err
	}
	return &m, nil
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

	m, err := scanCheckpoint(tx.QueryRow(ctx, checkpointSelectQuery, id, tenantID))
	if err != nil {
		return nil, err
	}
	return m.ToDomain()
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

	m, err := scanCheckpoint(tx.QueryRow(ctx, checkpointSelectByThreadQuery, threadID, tenantID))
	if err != nil {
		return nil, err
	}
	return m.ToDomain()
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
