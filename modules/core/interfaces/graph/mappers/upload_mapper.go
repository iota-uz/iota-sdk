package mappers

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	model "github.com/iota-uz/iota-sdk/modules/core/interfaces/graph/gqlmodels"
)

func UploadToGraphModel(u upload.Upload) *model.Upload {
	upload := &model.Upload{
		ID:   int64(u.ID()),
		Hash: u.Hash(),
		Name: u.Name(),
		Path: u.Path(),
		Size: u.Size().Bytes(),
		Type: u.Type(),
		URL:  u.URL().String(),
	}

	if mime := u.Mimetype(); mime != nil {
		upload.Mimetype = mime.String()
	}

	return upload
}
