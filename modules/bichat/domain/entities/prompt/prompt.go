package prompt

import "time"

type Prompt struct {
	ID          string
	Title       string
	Description string
	Prompt      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
