package validate_test

import (
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/go-i18n/v2/i18n"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2/validate"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestValidatorFieldMapLocalizesNestedFieldReferences(t *testing.T) {
	input := struct {
		Email string `validate:"required"`
	}{}
	source := validator.New().Struct(input)
	err := validate.From(source, func(field string) string { return "Account." + field })
	bundle := i18n.NewBundle(language.English)
	require.NoError(t, bundle.AddMessages(language.English, &i18n.Message{ID: "Account.Email", Other: "Email address"}, &i18n.Message{ID: "ValidationErrors.required", Other: "{{.Field}} is required."}))
	require.Equal(t, map[string]string{"Email": "Email address is required."}, validate.FieldMap(err, i18n.NewLocalizer(bundle, "en")))
	require.Equal(t, map[string]string{"Email": "Check the supplied information."}, validate.FieldMap(err, nil))
	var preserved validator.ValidationErrors
	require.ErrorAs(t, err, &preserved)
	require.Equal(t, source, preserved)
}
func TestValidatorNilAndUnknownErrors(t *testing.T) {
	require.NoError(t, validate.From(nil, nil))
	err := validate.From(errors.New("secret validation text"), nil)
	require.True(t, serrors.HasCode(err, serrors.Invalid))
	require.Empty(t, validate.FieldMap(err, nil))
	require.Empty(t, serrors.FieldsOf(err))
}
