package upload

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"

	"context"

	"github.com/gabriel-vasile/mimetype"
	"github.com/iota-uz/go-i18n/v2/i18n"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/geopoint"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type GeoPoint struct {
	Lat float64
	Lng float64
}

// CreateDTO carries data for upload creation.
// UploadsPath, Domain, and Scheme are populated by the service layer from
// uploadsconfig.Config and httpconfig.Config respectively. When zero, sane
// defaults are applied ("static", "localhost", "http").
type CreateDTO struct {
	File        io.ReadSeeker `validate:"required"`
	Name        string        `validate:"required"`
	Size        int           `validate:"required"`
	Slug        string        `validate:"omitempty,alphanum"`
	GeoPoint    *GeoPoint
	UploadsPath string // populated by service from uploadsconfig.Config.Path
	Domain      string // populated by service from httpconfig.Config.Domain
	Scheme      string // populated by service from httpconfig.Config.Scheme()
}

func (d *CreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(d)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ValidationErrors.%s", err.Tag()),
			TemplateData: map[string]string{
				"Field": err.Field(),
			},
		})
	}

	return errorMessages, len(errorMessages) == 0
}

func (d *CreateDTO) ToEntity() (Upload, []byte, error) {
	uploadsPath := d.UploadsPath
	if uploadsPath == "" {
		uploadsPath = "static"
	}
	domain := d.Domain
	if domain == "" {
		domain = "localhost"
	}
	scheme := d.Scheme
	if scheme == "" {
		scheme = "http"
	}

	raw, err := io.ReadAll(d.File)
	if err != nil {
		return nil, nil, err
	}
	mdsum := md5.Sum(raw)
	hash := hex.EncodeToString(mdsum[:])
	ext := filepath.Ext(d.Name)
	if d.Slug == "" {
		d.Slug = hash
	}
	u := New(
		hash,
		filepath.Join(uploadsPath, d.Slug+ext),
		d.Name,
		d.Slug,
		d.Size,
		mimetype.Detect(raw),
		WithUploadsPath(uploadsPath),
		WithDomain(domain),
		WithScheme(scheme),
	)
	if d.GeoPoint != nil {
		u.SetGeoPoint(geopoint.New(d.GeoPoint.Lat, d.GeoPoint.Lng))
	}
	return u, raw, nil
}
