package agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5"
)

// Checkpoint represents a saved state for Human-in-the-Loop (HITL) support.
// It captures the conversation state when agent execution is paused for user input.
type Checkpoint struct {
	ID            string           `json:"id"`
	ThreadID      string           `json:"thread_id"`
	AgentName     string           `json:"agent_name"`
	Messages      []types.Message  `json:"messages"`
	PendingTools  []types.ToolCall `json:"pending_tools"`
	InterruptType string           `json:"interrupt_type"`
	InterruptData json.RawMessage  `json:"interrupt_data,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
}

// NewCheckpoint creates a new checkpoint with the given parameters.
func NewCheckpoint(threadID, agentName string, messages []types.Message, opts ...CheckpointOption) *Checkpoint {
	cp := &Checkpoint{
		ID:           uuid.New().String(),
		ThreadID:     threadID,
		AgentName:    agentName,
		Messages:     messages,
		PendingTools: []types.ToolCall{},
		CreatedAt:    time.Now(),
	}

	for _, opt := range opts {
		opt(cp)
	}

	return cp
}

// CheckpointOption is a functional option for creating checkpoints.
type CheckpointOption func(*Checkpoint)

// WithCheckpointID sets the checkpoint ID.
func WithCheckpointID(id string) CheckpointOption {
	return func(cp *Checkpoint) {
		cp.ID = id
	}
}

// WithPendingTools sets the pending tool calls.
func WithPendingTools(tools []types.ToolCall) CheckpointOption {
	return func(cp *Checkpoint) {
		cp.PendingTools = tools
	}
}

// WithInterruptType sets the interrupt type (e.g., "ask_user_question", "approval_required").
func WithInterruptType(interruptType string) CheckpointOption {
	return func(cp *Checkpoint) {
		cp.InterruptType = interruptType
	}
}

// WithInterruptData sets the interrupt data as JSON.
func WithInterruptData(data json.RawMessage) CheckpointOption {
	return func(cp *Checkpoint) {
		cp.InterruptData = data
	}
}

// WithCreatedAt sets the created at timestamp.
func WithCreatedAt(createdAt time.Time) CheckpointOption {
	return func(cp *Checkpoint) {
		cp.CreatedAt = createdAt
	}
}

// Checkpointer defines the interface for checkpoint persistence.
// Implementations can use in-memory storage, PostgreSQL, or other backends.
type Checkpointer interface {
	// Save saves a checkpoint and returns its ID.
	Save(ctx context.Context, checkpoint *Checkpoint) (string, error)

	// Load retrieves a checkpoint by ID.
	Load(ctx context.Context, id string) (*Checkpoint, error)

	// LoadByThreadID retrieves the latest checkpoint for a thread.
	LoadByThreadID(ctx context.Context, threadID string) (*Checkpoint, error)

	// Delete removes a checkpoint by ID.
	Delete(ctx context.Context, id string) error

	// LoadAndDelete atomically retrieves and removes a checkpoint by ID.
	// This is useful for resuming execution and preventing duplicate processing.
	LoadAndDelete(ctx context.Context, id string) (*Checkpoint, error)
}

// InMemoryCheckpointer implements Checkpointer using in-memory storage.
// This is useful for testing and non-persistent workflows.
type InMemoryCheckpointer struct {
	mu          sync.RWMutex
	checkpoints map[string]*Checkpoint
	threadIndex map[string]string // threadID -> latest checkpointID
}

// NewInMemoryCheckpointer creates a new in-memory checkpointer.
func NewInMemoryCheckpointer() *InMemoryCheckpointer {
	return &InMemoryCheckpointer{
		checkpoints: make(map[string]*Checkpoint),
		threadIndex: make(map[string]string),
	}
}

// Save saves a checkpoint to memory.
func (c *InMemoryCheckpointer) Save(ctx context.Context, checkpoint *Checkpoint) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clone to prevent external modifications
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return "", err
	}

	var clone Checkpoint
	if err := json.Unmarshal(data, &clone); err != nil {
		return "", err
	}

	c.checkpoints[checkpoint.ID] = &clone
	c.threadIndex[checkpoint.ThreadID] = checkpoint.ID

	return checkpoint.ID, nil
}

// Load retrieves a checkpoint by ID.
func (c *InMemoryCheckpointer) Load(ctx context.Context, id string) (*Checkpoint, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	checkpoint, exists := c.checkpoints[id]
	if !exists {
		return nil, ErrCheckpointNotFound
	}

	// Clone to prevent external modifications
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, err
	}

	var clone Checkpoint
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}

	return &clone, nil
}

// LoadByThreadID retrieves the latest checkpoint for a thread.
func (c *InMemoryCheckpointer) LoadByThreadID(ctx context.Context, threadID string) (*Checkpoint, error) {
	c.mu.RLock()
	checkpointID, exists := c.threadIndex[threadID]
	c.mu.RUnlock()

	if !exists {
		return nil, ErrCheckpointNotFound
	}

	return c.Load(ctx, checkpointID)
}

// Delete removes a checkpoint by ID.
func (c *InMemoryCheckpointer) Delete(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	checkpoint, exists := c.checkpoints[id]
	if !exists {
		return ErrCheckpointNotFound
	}

	delete(c.checkpoints, id)

	// Clean up thread index if this is the latest checkpoint
	if c.threadIndex[checkpoint.ThreadID] == id {
		delete(c.threadIndex, checkpoint.ThreadID)
	}

	return nil
}

// LoadAndDelete atomically retrieves and removes a checkpoint by ID.
func (c *InMemoryCheckpointer) LoadAndDelete(ctx context.Context, id string) (*Checkpoint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	checkpoint, exists := c.checkpoints[id]
	if !exists {
		return nil, ErrCheckpointNotFound
	}

	// Clone before deletion
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, err
	}

	var clone Checkpoint
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}

	// Remove from storage
	delete(c.checkpoints, id)
	if c.threadIndex[checkpoint.ThreadID] == id {
		delete(c.threadIndex, checkpoint.ThreadID)
	}

	return &clone, nil
}

// PostgresCheckpointer implements Checkpointer using PostgreSQL.
// It supports multi-tenant isolation via tenant_id.
type PostgresCheckpointer struct{}

// NewPostgresCheckpointer creates a new PostgreSQL checkpointer.
func NewPostgresCheckpointer() *PostgresCheckpointer {
	return &PostgresCheckpointer{}
}

const (
	checkpointInsertQuery = `
		INSERT INTO bichat_checkpoints (
			id, tenant_id, thread_id, agent_name, messages, pending_tools,
			interrupt_type, interrupt_data, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	checkpointSelectQuery = `
		SELECT id, tenant_id, thread_id, agent_name, messages, pending_tools,
		       interrupt_type, interrupt_data, created_at
		FROM bichat_checkpoints
		WHERE id = $1 AND tenant_id = $2
	`

	checkpointSelectByThreadQuery = `
		SELECT id, tenant_id, thread_id, agent_name, messages, pending_tools,
		       interrupt_type, interrupt_data, created_at
		FROM bichat_checkpoints
		WHERE thread_id = $1 AND tenant_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	checkpointDeleteQuery = `
		DELETE FROM bichat_checkpoints
		WHERE id = $1 AND tenant_id = $2
	`
)

// Note: ErrCheckpointNotFound is defined in errors.go to avoid redeclaration

// Save saves a checkpoint to PostgreSQL.
func (p *PostgresCheckpointer) Save(ctx context.Context, checkpoint *Checkpoint) (string, error) {
	tenantID, err := composables.UseTenantID(ctx)
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
		tenantID,
		checkpoint.ThreadID,
		checkpoint.AgentName,
		messagesJSON,
		pendingToolsJSON,
		checkpoint.InterruptType,
		checkpoint.InterruptData,
		checkpoint.CreatedAt,
	)
	if err != nil {
		return "", err
	}

	return checkpoint.ID, nil
}

// Load retrieves a checkpoint by ID from PostgreSQL.
func (p *PostgresCheckpointer) Load(ctx context.Context, id string) (*Checkpoint, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var checkpoint Checkpoint
	var messagesJSON, pendingToolsJSON []byte
	var interruptData *[]byte

	err = tx.QueryRow(ctx, checkpointSelectQuery, id, tenantID).Scan(
		&checkpoint.ID,
		&tenantID, // Scan but don't use (already have it)
		&checkpoint.ThreadID,
		&checkpoint.AgentName,
		&messagesJSON,
		&pendingToolsJSON,
		&checkpoint.InterruptType,
		&interruptData,
		&checkpoint.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows || err == pgx.ErrNoRows {
			return nil, ErrCheckpointNotFound
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

	return &checkpoint, nil
}

// LoadByThreadID retrieves the latest checkpoint for a thread.
func (p *PostgresCheckpointer) LoadByThreadID(ctx context.Context, threadID string) (*Checkpoint, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var checkpoint Checkpoint
	var messagesJSON, pendingToolsJSON []byte
	var interruptData *[]byte

	err = tx.QueryRow(ctx, checkpointSelectByThreadQuery, threadID, tenantID).Scan(
		&checkpoint.ID,
		&tenantID,
		&checkpoint.ThreadID,
		&checkpoint.AgentName,
		&messagesJSON,
		&pendingToolsJSON,
		&checkpoint.InterruptType,
		&interruptData,
		&checkpoint.CreatedAt,
	)
	if err != nil {
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
		return ErrCheckpointNotFound
	}

	return nil
}

// LoadAndDelete atomically retrieves and removes a checkpoint by ID.
func (p *PostgresCheckpointer) LoadAndDelete(ctx context.Context, id string) (*Checkpoint, error) {
	// Use transaction to ensure atomicity
	var checkpoint *Checkpoint
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
