// Package models provides this package.
package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

var (
	ErrNilMessageModel = errors.New("message model is nil")
	ErrNilMessage      = errors.New("message is nil")
)

// MessageModel is the database model for bichat.messages projections.
type MessageModel struct {
	ID               uuid.UUID
	SessionID        uuid.UUID
	Role             types.Role
	Content          string
	AuthorUserID     *int64
	AuthorFirstName  string
	AuthorLastName   string
	ToolCallsJSON    []byte
	ToolCallID       *string
	CitationsJSON    []byte
	DebugTraceJSON   []byte
	QuestionDataJSON []byte
	CreatedAt        time.Time
}
