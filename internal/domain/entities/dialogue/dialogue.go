package dialogue

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"github.com/sashabaranov/go-openai"
	"time"
)

type Messages []openai.ChatCompletionMessage

// Scan scan value into Jsonb, implements sql.Scanner interface
func (j *Messages) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	result := Messages{}
	err := json.Unmarshal(bytes, &result)
	*j = result
	return err
}

// Value return json value, implement driver.Valuer interface
func (j Messages) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return json.Marshal(j)
}

type Dialogue struct {
	Id        int64
	UserID    int64
	Label     string
	Messages  Messages `gorm:"type:jsonb"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (d *Dialogue) AddMessage(msg openai.ChatCompletionMessage) {
	d.Messages = append(d.Messages, msg)
}

func (d *Dialogue) ToGraph() (*model.Dialogue, error) {
	var messages []*model.Message
	for _, m := range d.Messages {
		if m.Role == openai.ChatMessageRoleSystem {
			continue
		}
		var toolCalls []*model.ToolCall
		for _, tc := range m.ToolCalls {
			toolCalls = append(toolCalls, &model.ToolCall{
				ID:    tc.ID,
				Index: *(tc.Index),
				Type:  string(tc.Type),
			})
		}
		messages = append(messages, &model.Message{
			Role:      m.Role,
			Content:   m.Content,
			ToolCalls: toolCalls,
		})
	}
	return &model.Dialogue{
		ID:        d.Id,
		UserID:    d.UserID,
		Label:     d.Label,
		Messages:  messages,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}, nil
}
