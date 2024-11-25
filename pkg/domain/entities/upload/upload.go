package upload

import (
	"time"

	"github.com/gabriel-vasile/mimetype"
)

type Upload struct {
	ID        uint
	Hash      string
	URL       string
	Name      string
	Size      int
	Mimetype  mimetype.MIME
	CreatedAt time.Time
	UpdatedAt time.Time
}
