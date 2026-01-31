package agents

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckpoint_JSONSerialization(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		*types.UserMessage("Hello"),
		*types.AssistantMessage("Hi there!", types.WithToolCalls(types.ToolCall{
			ID: "call_1", Name: "search", Arguments: `{"query": "test"}`,
		})),
	}

	interruptData := json.RawMessage(`{"question": "Proceed?", "options": ["yes", "no"]}`)

	checkpoint := NewCheckpoint(
		"thread-123",
		"test-agent",
		messages,
		WithPendingTools([]types.ToolCall{
			{ID: "call_2", Name: "execute", Arguments: `{"command": "test"}`},
		}),
		WithInterruptType("ask_user_question"),
		WithInterruptData(interruptData),
	)

	// Marshal to JSON
	data, err := json.Marshal(checkpoint)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Unmarshal from JSON
	var decoded Checkpoint
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, checkpoint.ID, decoded.ID)
	assert.Equal(t, checkpoint.ThreadID, decoded.ThreadID)
	assert.Equal(t, checkpoint.AgentName, decoded.AgentName)
	assert.Equal(t, "ask_user_question", decoded.InterruptType)
	assert.JSONEq(t, string(interruptData), string(decoded.InterruptData))

	// Verify messages
	require.Len(t, decoded.Messages, 2)
	assert.Equal(t, types.RoleUser, decoded.Messages[0].Role)
	assert.Equal(t, "Hello", decoded.Messages[0].Content)
	assert.Equal(t, types.RoleAssistant, decoded.Messages[1].Role)
	assert.Equal(t, "Hi there!", decoded.Messages[1].Content)
	require.Len(t, decoded.Messages[1].ToolCalls, 1)
	assert.Equal(t, "call_1", decoded.Messages[1].ToolCalls[0].ID)

	// Verify pending tools
	require.Len(t, decoded.PendingTools, 1)
	assert.Equal(t, "call_2", decoded.PendingTools[0].ID)
	assert.Equal(t, "execute", decoded.PendingTools[0].Name)
}

func TestInMemoryCheckpointer_CRUD(t *testing.T) {
	t.Parallel()

	checkpointer := NewInMemoryCheckpointer()
	ctx := context.Background()

	messages := []types.Message{
		*types.UserMessage("Test message"),
	}

	t.Run("Save and Load", func(t *testing.T) {
		checkpoint := NewCheckpoint("thread-1", "agent-1", messages)

		// Save
		id, err := checkpointer.Save(ctx, checkpoint)
		require.NoError(t, err)
		assert.Equal(t, checkpoint.ID, id)

		// Load
		loaded, err := checkpointer.Load(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, checkpoint.ID, loaded.ID)
		assert.Equal(t, checkpoint.ThreadID, loaded.ThreadID)
		assert.Equal(t, checkpoint.AgentName, loaded.AgentName)
		assert.Len(t, loaded.Messages, 1)
		assert.Equal(t, "Test message", loaded.Messages[0].Content)
	})

	t.Run("LoadByThreadID", func(t *testing.T) {
		checkpoint1 := NewCheckpoint("thread-2", "agent-1", messages)
		checkpoint2 := NewCheckpoint("thread-2", "agent-1", messages)

		// Save both (checkpoint2 is later)
		_, err := checkpointer.Save(ctx, checkpoint1)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond) // Ensure different timestamps

		_, err = checkpointer.Save(ctx, checkpoint2)
		require.NoError(t, err)

		// LoadByThreadID should return the latest (checkpoint2)
		loaded, err := checkpointer.LoadByThreadID(ctx, "thread-2")
		require.NoError(t, err)
		assert.Equal(t, checkpoint2.ID, loaded.ID)
	})

	t.Run("Delete", func(t *testing.T) {
		checkpoint := NewCheckpoint("thread-3", "agent-1", messages)

		id, err := checkpointer.Save(ctx, checkpoint)
		require.NoError(t, err)

		// Delete
		err = checkpointer.Delete(ctx, id)
		require.NoError(t, err)

		// Load should fail
		_, err = checkpointer.Load(ctx, id)
		assert.Error(t, err)
	})

	t.Run("LoadAndDelete", func(t *testing.T) {
		checkpoint := NewCheckpoint("thread-4", "agent-1", messages)

		id, err := checkpointer.Save(ctx, checkpoint)
		require.NoError(t, err)

		// LoadAndDelete
		loaded, err := checkpointer.LoadAndDelete(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, checkpoint.ID, loaded.ID)

		// Load should fail after deletion
		_, err = checkpointer.Load(ctx, id)
		assert.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := checkpointer.Load(ctx, "non-existent")
		assert.Error(t, err)

		_, err = checkpointer.LoadByThreadID(ctx, "non-existent-thread")
		assert.Error(t, err)

		err = checkpointer.Delete(ctx, "non-existent")
		assert.Error(t, err)

		_, err = checkpointer.LoadAndDelete(ctx, "non-existent")
		assert.Error(t, err)
	})
}

func TestInMemoryCheckpointer_Concurrent(t *testing.T) {
	t.Parallel()

	checkpointer := NewInMemoryCheckpointer()
	ctx := context.Background()

	const numGoroutines = 10
	const numOpsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			for j := 0; j < numOpsPerGoroutine; j++ {
				threadID := uuid.New().String()
				messages := []types.Message{
					*types.UserMessage("Concurrent test"),
				}

				checkpoint := NewCheckpoint(threadID, "test-agent", messages)

				// Save
				id, err := checkpointer.Save(ctx, checkpoint)
				assert.NoError(t, err)

				// Load
				loaded, err := checkpointer.Load(ctx, id)
				assert.NoError(t, err)
				assert.Equal(t, checkpoint.ID, loaded.ID)

				// Delete
				err = checkpointer.Delete(ctx, id)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestPostgresCheckpointer_CRUD(t *testing.T) {
	t.Parallel()

	// Setup test environment with database
	env := itf.Setup(t)
	ctx := composables.WithTx(env.Ctx, env.Tx)

	checkpointer := NewPostgresCheckpointer()

	messages := []Message{
		NewUserMessage("Hello from DB"),
		NewAssistantMessage("Response", []ToolCall{
			{ID: "call_1", Name: "tool1", Arguments: `{"arg": "value"}`},
		}),
	}

	interruptData := json.RawMessage(`{"question": "Approve?"}`)

	t.Run("Save and Load", func(t *testing.T) {
		checkpoint := NewCheckpoint(
			"thread-db-1",
			"db-agent",
			messages,
			WithInterruptType("approval"),
			WithInterruptData(interruptData),
		)

		// Save
		id, err := checkpointer.Save(ctx, checkpoint)
		require.NoError(t, err)
		assert.Equal(t, checkpoint.ID, id)

		// Load
		loaded, err := checkpointer.Load(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, checkpoint.ID, loaded.ID)
		assert.Equal(t, checkpoint.ThreadID, loaded.ThreadID)
		assert.Equal(t, checkpoint.AgentName, loaded.AgentName)
		assert.Equal(t, "approval", loaded.InterruptType)
		assert.JSONEq(t, string(interruptData), string(loaded.InterruptData))
		assert.Len(t, loaded.Messages, 2)
		assert.Equal(t, "Hello from DB", loaded.Messages[0].Content)
	})

	t.Run("LoadByThreadID", func(t *testing.T) {
		threadID := "thread-db-2"
		checkpoint1 := NewCheckpoint(threadID, "agent", messages)
		checkpoint2 := NewCheckpoint(threadID, "agent", messages)

		// Save both
		_, err := checkpointer.Save(ctx, checkpoint1)
		require.NoError(t, err)

		// Sleep to ensure different timestamps
		time.Sleep(10 * time.Millisecond)

		_, err = checkpointer.Save(ctx, checkpoint2)
		require.NoError(t, err)

		// LoadByThreadID should return the latest
		loaded, err := checkpointer.LoadByThreadID(ctx, threadID)
		require.NoError(t, err)
		assert.Equal(t, checkpoint2.ID, loaded.ID)
	})

	t.Run("Delete", func(t *testing.T) {
		checkpoint := NewCheckpoint("thread-db-3", "agent", messages)

		id, err := checkpointer.Save(ctx, checkpoint)
		require.NoError(t, err)

		// Delete
		err = checkpointer.Delete(ctx, id)
		require.NoError(t, err)

		// Load should fail
		_, err = checkpointer.Load(ctx, id)
		assert.Error(t, err)
	})

	t.Run("LoadAndDelete", func(t *testing.T) {
		checkpoint := NewCheckpoint("thread-db-4", "agent", messages)

		id, err := checkpointer.Save(ctx, checkpoint)
		require.NoError(t, err)

		// LoadAndDelete
		loaded, err := checkpointer.LoadAndDelete(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, checkpoint.ID, loaded.ID)

		// Load should fail after deletion
		_, err = checkpointer.Load(ctx, id)
		assert.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := checkpointer.Load(ctx, "non-existent")
		assert.Error(t, err)

		err = checkpointer.Delete(ctx, "non-existent")
		assert.Error(t, err)
	})
}

func TestPostgresCheckpointer_TenantIsolation(t *testing.T) {
	t.Parallel()

	// Setup test environment
	env := itf.Setup(t)
	ctx := composables.WithTx(env.Ctx, env.Tx)

	checkpointer := NewPostgresCheckpointer()

	messages := []Message{
		NewUserMessage("Tenant test"),
	}

	// Create checkpoint for tenant 1
	checkpoint1 := NewCheckpoint("thread-tenant-1", "agent", messages)
	id1, err := checkpointer.Save(ctx, checkpoint1)
	require.NoError(t, err)

	// Create a second tenant context (simulate different tenant)
	tenant2ID := uuid.New()
	ctx2 := composables.WithTenantID(env.Ctx, tenant2ID)
	ctx2 = composables.WithTx(ctx2, env.Tx)

	// Create checkpoint for tenant 2
	checkpoint2 := NewCheckpoint("thread-tenant-2", "agent", messages)
	id2, err := checkpointer.Save(ctx2, checkpoint2)
	require.NoError(t, err)

	// Tenant 1 should NOT see tenant 2's checkpoint
	_, err = checkpointer.Load(ctx, id2)
	assert.Error(t, err, "Tenant 1 should not access tenant 2's checkpoint")

	// Tenant 2 should NOT see tenant 1's checkpoint
	_, err = checkpointer.Load(ctx2, id1)
	assert.Error(t, err, "Tenant 2 should not access tenant 1's checkpoint")

	// Each tenant should see their own checkpoint
	loaded1, err := checkpointer.Load(ctx, id1)
	require.NoError(t, err)
	assert.Equal(t, checkpoint1.ID, loaded1.ID)

	loaded2, err := checkpointer.Load(ctx2, id2)
	require.NoError(t, err)
	assert.Equal(t, checkpoint2.ID, loaded2.ID)

	// LoadByThreadID should also respect tenant isolation
	_, err = checkpointer.LoadByThreadID(ctx, "thread-tenant-2")
	assert.Error(t, err, "Tenant 1 should not access tenant 2's thread")

	_, err = checkpointer.LoadByThreadID(ctx2, "thread-tenant-1")
	assert.Error(t, err, "Tenant 2 should not access tenant 1's thread")
}
