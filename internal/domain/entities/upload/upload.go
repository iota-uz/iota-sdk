package upload

import (
	"time"

	"github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
)

type Upload struct {
	Id        int64
	Name      string
	Path      string
	Mimetype  string
	Size      int64
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

func (u *Upload) ToGraph() *model.Media {
	return &model.Media{
		ID:        u.Id,
		Name:      u.Name,
		Path:      u.Path,
		Mimetype:  u.Mimetype,
		Size:      u.Size,
		CreatedAt: *u.CreatedAt,
		UpdatedAt: *u.UpdatedAt,
	}
}
