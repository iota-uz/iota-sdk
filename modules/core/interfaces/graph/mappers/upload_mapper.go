package mappers

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	model "github.com/iota-uz/iota-sdk/modules/core/interfaces/graph/gqlmodels"
)

func UploadToGraphModel(u upload.Upload) *model.Upload {
	return &model.Upload{
		ID:       int64(u.ID()),
		Hash:     u.Hash(),
		Name:     u.Name(),
		Path:     u.Path(),
		Size:     u.Size().Bytes(),
		Mimetype: u.Mimetype().String(),
		Type:     u.Type(),
		URL:      u.URL().String(),
	}
}
