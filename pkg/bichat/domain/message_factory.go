// Package domain provides this package.
package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

const UserMessageMaxContentLen = 20_000

var (
	ErrInvalidMessage          = errors.New("invalid message")
	ErrInvalidMessageContent   = errors.New("invalid message content")
	ErrDuplicateToolCallID     = errors.New("duplicate tool call id")
	ErrInvalidAssistantMessage = errors.New("invalid assistant message")
)

type UserMessageSpec struct {
	SessionID    uuid.UUID
	AuthorUserID *int64
	Content      string
	Attachments  []Attachment
	CreatedAt    time.Time
}

type AssistantMessageSpec struct {
	SessionID    uuid.UUID
	Content      string
	ToolCalls    []types.ToolCall
	DebugTrace   *types.DebugTrace
	QuestionData *types.QuestionData
	CreatedAt    time.Time
}

func NewUserMessage(spec UserMessageSpec) (types.Message, error) {
	if spec.SessionID == uuid.Nil {
		return nil, ErrInvalidMessage
	}
	content := strings.TrimSpace(spec.Content)
	if content == "" && len(spec.Attachments) == 0 {
		return nil, ErrInvalidMessageContent
	}
	if len([]rune(content)) > UserMessageMaxContentLen {
		return nil, ErrInvalidMessageContent
	}

	opts := []types.MessageOption{types.WithSessionID(spec.SessionID)}
	if spec.AuthorUserID != nil && *spec.AuthorUserID > 0 {
		opts = append(opts, types.WithAuthorUserID(*spec.AuthorUserID))
	}
	if !spec.CreatedAt.IsZero() {
		opts = append(opts, types.WithCreatedAt(spec.CreatedAt))
	}

	return types.UserMessage(content, opts...), nil
}

func NewAssistantMessage(spec AssistantMessageSpec) (types.Message, error) {
	if spec.SessionID == uuid.Nil {
		return nil, ErrInvalidAssistantMessage
	}
	if spec.QuestionData != nil && !spec.QuestionData.IsPending() {
		return nil, ErrInvalidAssistantMessage
	}

	seenToolIDs := make(map[string]struct{}, len(spec.ToolCalls))
	for _, call := range spec.ToolCalls {
		toolID := strings.TrimSpace(call.ID)
		if toolID == "" {
			continue
		}
		if _, exists := seenToolIDs[toolID]; exists {
			return nil, ErrDuplicateToolCallID
		}
		seenToolIDs[toolID] = struct{}{}
	}

	opts := []types.MessageOption{types.WithSessionID(spec.SessionID)}
	if len(spec.ToolCalls) > 0 {
		opts = append(opts, types.WithToolCalls(spec.ToolCalls...))
	}
	if spec.DebugTrace != nil {
		opts = append(opts, types.WithDebugTrace(spec.DebugTrace))
	}
	if spec.QuestionData != nil {
		opts = append(opts, types.WithQuestionData(spec.QuestionData))
	}
	if !spec.CreatedAt.IsZero() {
		opts = append(opts, types.WithCreatedAt(spec.CreatedAt))
	}

	return types.AssistantMessage(strings.TrimSpace(spec.Content), opts...), nil
}
