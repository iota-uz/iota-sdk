package upload

import (
	"time"

	"github.com/gabriel-vasile/mimetype"
)

type Upload struct {
	ID        uint
	Hash      string
	Path      string
	Size      int
	Mimetype  mimetype.MIME
	CreatedAt time.Time
	UpdatedAt time.Time
}
