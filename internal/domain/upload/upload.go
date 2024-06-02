package upload

import (
	model "github.com/iota-agency/iota-erp/graph/gqlmodels"
	"time"
)

type Upload struct {
	Id        int64
	Name      string
	Path      string
	Mimetype  string
	Size      float64
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

func (u *Upload) ToGraph() *model.Upload {
	return &model.Upload{
		ID:        u.Id,
		Name:      u.Name,
		Path:      u.Path,
		Mimetype:  u.Mimetype,
		Size:      u.Size,
		CreatedAt: *u.CreatedAt,
		UpdatedAt: *u.UpdatedAt,
	}
}
