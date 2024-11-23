package upload

import (
	"time"
)

type Upload struct {
	ID        string
	URL       string
	Name      string
	Type      string
	Size      int
	Mimetype  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
