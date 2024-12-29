package dialogue

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
)

type Messages []openai.ChatCompletionMessage

// Scan scan value into Jsonb, implements sql.Scanner interface.
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

// Value return json value, implement driver.Valuer interface.
func (j Messages) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil //nolint:nilnil
	}
	return json.Marshal(j)
}

type Dialogue struct {
	Id        int64
	UserID    uint
	Label     string
	Messages  Messages `gorm:"type:jsonb"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (d *Dialogue) AddMessage(msg openai.ChatCompletionMessage) {
	d.Messages = append(d.Messages, msg)
}
