package bichat

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sashabaranov/go-openai"
)

type Messages []*openai.ChatCompletionMessage

func (m *Messages) Scan(val interface{}) error {
	switch v := val.(type) {
	case []byte:
		return json.Unmarshal(v, &m)
	case string:
		return json.Unmarshal([]byte(v), &m)
	default:
		return errors.New(fmt.Sprintf("Unsupported type: %T", v))
	}
}
func (m *Messages) Value() (driver.Value, error) {
	return json.Marshal(m)
}

type Dialogue struct {
	Id       int      `json:"id" db:"id"`
	Label    string   `json:"label" db:"label"`
	Messages Messages `json:"messages" db:"messages"`
}

type Prompt struct {
	Id          string `json:"id" db:"id"`
	Title       string `json:"title" db:"title"`
	Description string `json:"description" db:"description"`
	Prompt      string `json:"prompt" db:"prompt"`
	CreatedAt   string `json:"created_at" db:"created_at"`
}
