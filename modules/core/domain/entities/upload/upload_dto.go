package upload

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"

	"github.com/gabriel-vasile/mimetype"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/net/context"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type CreateDTO struct {
	File io.ReadSeeker `validate:"required"`
	Name string        `validate:"required"`
	Size int           `validate:"required"`
	Type string
}

func (d *CreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := ctx.Value(constants.LocalizerKey).(*i18n.Localizer)
	if !ok {
		panic("localizer not found in context")
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
	conf := configuration.Use()
	bytes, err := io.ReadAll(d.File)
	if err != nil {
		return nil, nil, err
	}
	mdsum := md5.Sum(bytes)
	hash := hex.EncodeToString(mdsum[:])
	ext := filepath.Ext(d.Name)
	return New(
		hash,
		filepath.Join(conf.UploadsPath, hash+ext),
		d.Size,
		mimetype.Detect(bytes),
	), bytes, nil
}
