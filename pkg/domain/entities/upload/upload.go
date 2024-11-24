package upload

import (
	"github.com/gabriel-vasile/mimetype"
	"time"
)

type Upload struct {
	ID        string
	URL       string
	Name      string
	Size      int
	Mimetype  mimetype.MIME
	CreatedAt time.Time
	UpdatedAt time.Time
}
