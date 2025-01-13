package dialogue

import (
	"time"
)

type Messages []ChatCompletionMessage

type Dialogue interface {
	ID() uint
	UserID() uint
	Label() string
	Messages() Messages
	CreatedAt() time.Time
	UpdatedAt() time.Time

<<<<<<< Updated upstream
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
	ID        int64
	UserID    uint
	Label     string
	Messages  Messages `gorm:"type:jsonb"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (d *Dialogue) AddMessage(msg openai.ChatCompletionMessage) {
	d.Messages = append(d.Messages, msg)
=======
	AddMessage(msg ChatCompletionMessage) Dialogue
>>>>>>> Stashed changes
}
