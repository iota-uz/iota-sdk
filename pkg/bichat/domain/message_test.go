package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

func TestMessage_Creation(t *testing.T) {
	t.Parallel()

	t.Run("basic creation with defaults", func(t *testing.T) {
		msg := types.UserMessage("")

		if msg.ID() == uuid.Nil {
			t.Error("Expected non-nil UUID")
		}
		if len(msg.ToolCalls()) != 0 {
			t.Error("Expected empty ToolCalls slice")
		}
		if len(msg.Attachments()) != 0 {
			t.Error("Expected empty Attachments slice")
		}
		if len(msg.Citations()) != 0 {
			t.Error("Expected empty Citations slice")
		}
		if msg.CreatedAt().IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
	})

	t.Run("creation with options", func(t *testing.T) {
		sessionID := uuid.New()
		content := "Hello, world!"

		msg := types.UserMessage(
			content,
			types.WithSessionID(sessionID),
		)

		if msg.SessionID() != sessionID {
			t.Errorf("Expected SessionID %s, got %s", sessionID, msg.SessionID())
		}
		if msg.Role() != types.RoleUser {
			t.Errorf("Expected Role %s, got %s", types.RoleUser, msg.Role())
		}
		if msg.Content() != content {
			t.Errorf("Expected Content '%s', got '%s'", content, msg.Content())
		}
	})

	t.Run("creation with tool calls", func(t *testing.T) {
		toolCalls := []types.ToolCall{
			{ID: "call_1", Name: "tool1", Arguments: `{"arg":"value"}`},
			{ID: "call_2", Name: "tool2", Arguments: `{}`},
		}

		msg := types.UserMessage("", types.WithToolCalls(toolCalls...))

		if len(msg.ToolCalls()) != 2 {
			t.Errorf("Expected 2 tool calls, got %d", len(msg.ToolCalls()))
		}
		if msg.ToolCalls()[0].ID != "call_1" {
			t.Error("ToolCall ID not set correctly")
		}
	})

	t.Run("creation with tool call ID", func(t *testing.T) {
		toolCallID := "call_123"
		msg := types.ToolResponse(toolCallID, "result")

		if msg.ToolCallID() == nil {
			t.Fatal("Expected ToolCallID to be set")
		}
		if *msg.ToolCallID() != toolCallID {
			t.Errorf("Expected ToolCallID '%s', got '%s'", toolCallID, *msg.ToolCallID())
		}
	})

	t.Run("creation with custom ID", func(t *testing.T) {
		customID := uuid.New()
		msg := types.UserMessage("", types.WithMessageID(customID))

		if msg.ID() != customID {
			t.Errorf("Expected ID %s, got %s", customID, msg.ID())
		}
	})
}

func TestMessageRole_Values(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role     types.Role
		expected string
	}{
		{types.RoleUser, "user"},
		{types.RoleAssistant, "assistant"},
		{types.RoleTool, "tool"},
		{types.RoleSystem, "system"},
	}

	for _, tt := range tests {
		if tt.role.String() != tt.expected {
			t.Errorf("Expected role string '%s', got '%s'", tt.expected, tt.role.String())
		}
	}
}

func TestMessageRole_IsUser(t *testing.T) {
	t.Parallel()

	if !types.RoleUser.IsUser() {
		t.Error("Expected RoleUser.IsUser() to be true")
	}
	if types.RoleAssistant.IsUser() {
		t.Error("Expected RoleAssistant.IsUser() to be false")
	}
	if types.RoleTool.IsUser() {
		t.Error("Expected RoleTool.IsUser() to be false")
	}
	if types.RoleSystem.IsUser() {
		t.Error("Expected RoleSystem.IsUser() to be false")
	}
}

func TestMessageRole_IsAssistant(t *testing.T) {
	t.Parallel()

	if !types.RoleAssistant.IsAssistant() {
		t.Error("Expected RoleAssistant.IsAssistant() to be true")
	}
	if types.RoleUser.IsAssistant() {
		t.Error("Expected RoleUser.IsAssistant() to be false")
	}
	if types.RoleTool.IsAssistant() {
		t.Error("Expected RoleTool.IsAssistant() to be false")
	}
	if types.RoleSystem.IsAssistant() {
		t.Error("Expected RoleSystem.IsAssistant() to be false")
	}
}

func TestMessageRole_IsTool(t *testing.T) {
	t.Parallel()

	if !types.RoleTool.IsTool() {
		t.Error("Expected RoleTool.IsTool() to be true")
	}
	if types.RoleUser.IsTool() {
		t.Error("Expected RoleUser.IsTool() to be false")
	}
	if types.RoleAssistant.IsTool() {
		t.Error("Expected RoleAssistant.IsTool() to be false")
	}
	if types.RoleSystem.IsTool() {
		t.Error("Expected RoleSystem.IsTool() to be false")
	}
}

func TestMessageRole_IsSystem(t *testing.T) {
	t.Parallel()

	if !types.RoleSystem.IsSystem() {
		t.Error("Expected RoleSystem.IsSystem() to be true")
	}
	if types.RoleUser.IsSystem() {
		t.Error("Expected RoleUser.IsSystem() to be false")
	}
	if types.RoleAssistant.IsSystem() {
		t.Error("Expected RoleAssistant.IsSystem() to be false")
	}
	if types.RoleTool.IsSystem() {
		t.Error("Expected RoleTool.IsSystem() to be false")
	}
}

func TestMessageRole_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role     types.Role
		expected bool
	}{
		{types.RoleUser, true},
		{types.RoleAssistant, true},
		{types.RoleTool, true},
		{types.RoleSystem, true},
		{types.Role("invalid"), false},
		{types.Role(""), false},
	}

	for _, tt := range tests {
		if tt.role.Valid() != tt.expected {
			t.Errorf("Valid() for '%s': expected %v, got %v", tt.role, tt.expected, tt.role.Valid())
		}
	}
}

func TestMessage_HasToolCalls(t *testing.T) {
	t.Parallel()

	msgWithToolCalls := types.UserMessage(
		"",
		types.WithToolCalls(types.ToolCall{
			ID: "1", Name: "tool1", Arguments: "{}",
		}),
	)
	msgWithoutToolCalls := types.UserMessage("")

	if !msgWithToolCalls.HasToolCalls() {
		t.Error("Expected message with tool calls to return true")
	}
	if msgWithoutToolCalls.HasToolCalls() {
		t.Error("Expected message without tool calls to return false")
	}
}

func TestMessage_HasAttachments(t *testing.T) {
	t.Parallel()

	msgWithAttachments := types.UserMessage(
		"",
		types.WithAttachments(types.Attachment{}),
	)
	msgWithoutAttachments := types.UserMessage("")

	if !msgWithAttachments.HasAttachments() {
		t.Error("Expected message with attachments to return true")
	}
	if msgWithoutAttachments.HasAttachments() {
		t.Error("Expected message without attachments to return false")
	}
}

func TestMessage_HasCitations(t *testing.T) {
	t.Parallel()

	msgWithCitations := types.UserMessage(
		"",
		types.WithCitations(types.Citation{}),
	)
	msgWithoutCitations := types.UserMessage("")

	if !msgWithCitations.HasCitations() {
		t.Error("Expected message with citations to return true")
	}
	if msgWithoutCitations.HasCitations() {
		t.Error("Expected message without citations to return false")
	}
}

func TestMessage_IsToolMessage(t *testing.T) {
	t.Parallel()

	toolCallID := "call_123"
	toolMessage := types.ToolResponse(toolCallID, "result")
	regularMessage := types.UserMessage("")

	if !toolMessage.IsToolMessage() {
		t.Error("Expected message with ToolCallID to return true for IsToolMessage()")
	}
	if regularMessage.IsToolMessage() {
		t.Error("Expected message without ToolCallID to return false for IsToolMessage()")
	}
}

func TestToolCall_Basic(t *testing.T) {
	t.Parallel()

	toolCall := types.ToolCall{
		ID:        "call_abc123",
		Name:      "get_weather",
		Arguments: `{"location":"San Francisco","unit":"celsius"}`,
	}

	if toolCall.ID != "call_abc123" {
		t.Errorf("Expected ID 'call_abc123', got '%s'", toolCall.ID)
	}
	if toolCall.Name != "get_weather" {
		t.Errorf("Expected Name 'get_weather', got '%s'", toolCall.Name)
	}
	if toolCall.Arguments != `{"location":"San Francisco","unit":"celsius"}` {
		t.Errorf("Unexpected Arguments: %s", toolCall.Arguments)
	}
}

func TestMessage_MultipleOptions(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	content := "Test content"
	toolCalls := []types.ToolCall{
		{ID: "1", Name: "tool", Arguments: "{}"},
	}
	attachments := []types.Attachment{{}}
	citations := []types.Citation{{}}

	msg := types.AssistantMessage(
		content,
		types.WithSessionID(sessionID),
		types.WithToolCalls(toolCalls...),
		types.WithAttachments(attachments...),
		types.WithCitations(citations...),
	)

	// Verify all options were applied
	if msg.SessionID() != sessionID {
		t.Error("SessionID not set correctly")
	}
	if msg.Role() != types.RoleAssistant {
		t.Error("Role not set correctly")
	}
	if msg.Content() != content {
		t.Error("Content not set correctly")
	}
	if len(msg.ToolCalls()) != 1 {
		t.Error("ToolCalls not set correctly")
	}
	if len(msg.Attachments()) != 1 {
		t.Error("Attachments not set correctly")
	}
	if len(msg.Citations()) != 1 {
		t.Error("Citations not set correctly")
	}
}

func TestMessage_EmptyContent(t *testing.T) {
	t.Parallel()

	// Message with empty content is valid (e.g., tool response)
	msg := types.ToolResponse("call_1", "")

	if msg.Content() != "" {
		t.Error("Expected empty content")
	}
	if msg.Role() != types.RoleTool {
		t.Error("Expected Tool role")
	}
}

func TestMessage_WithCreatedAt(t *testing.T) {
	t.Parallel()

	// Custom timestamp
	customTime := types.UserMessage("").CreatedAt().Add(-1000)

	msg := types.UserMessage(
		"",
		types.WithCreatedAt(customTime),
	)

	if !msg.CreatedAt().Equal(customTime) {
		t.Errorf("Expected CreatedAt %v, got %v", customTime, msg.CreatedAt())
	}
}
