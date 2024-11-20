package viewmodels

import "time"

type Upload struct {
	ID        string
	URL       string
	Name      string
	Type      string
	Size      int
	CreatedAt time.Time
	UpdatedAt time.Time
}
