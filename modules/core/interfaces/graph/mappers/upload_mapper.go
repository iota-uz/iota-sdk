package mappers

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	model "github.com/iota-uz/iota-sdk/modules/core/interfaces/graph/gqlmodels"
)

func UploadToGraphModel(u upload.Upload) *model.Upload {
	upload := &model.Upload{
		ID:       int64(u.ID()),
		Hash:     u.Hash(),
		Name:     u.Name(),
		Path:     u.Path(),
		Slug:     u.Slug(),
		Size:     u.Size().Bytes(),
		Type:     u.Type(),
		Source:   u.Source(),
		URL:      u.URL().String(),
		GeoPoint: &model.GeoPoint{},
	}

	if point := u.GeoPoint(); point != nil {
		upload.GeoPoint = &model.GeoPoint{
			Lat: point.Lat(),
			Lng: point.Lng(),
		}
	}

	if mime := u.Mimetype(); mime != nil {
		upload.Mimetype = mime.String()
	}

	return upload
}
