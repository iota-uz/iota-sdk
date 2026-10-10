package controllers_test

import (
	"net/http"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
)

// A select built with WithLocalizationKey and then given a value type
// (AsStringSelect, AsUUIDSelect, ...) must still be labelled by that key.
// SetValueType rebuilt the field and dropped the key, so the form fell back to
// the raw field name. The key here is "ConfirmDelete" only because its
// translation is distinctive and appears nowhere else on a new form.
func TestCrudController_SelectKeepsLocalizationKeyAfterValueType(t *testing.T) {
	suite := newCoreCrudSuiteBuilder(t).Build().AsUser(itf.User())

	fields := crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey()),
		crud.NewStringField("name", crud.WithSearchable()),
		crud.NewSelectField("description", crud.WithLocalizationKey("ConfirmDelete")).
			AsStringSelect().
			WithStaticOptions(
				crud.SelectOption{Value: "a", Label: "Option A"},
				crud.SelectOption{Value: "b", Label: "Option B"},
			),
		crud.NewFloatField("amount"),
		crud.NewBoolField("is_active"),
		crud.NewTimestampField("created_at", crud.WithReadonly()),
		crud.NewTimestampField("updated_at", crud.WithReadonly()),
	})
	builder := &testBuilder{
		schema:  crud.NewSchema("test_entities", fields, &testMapper{fields: fields}),
		service: newTestService(),
	}
	suite.Register(controllers.NewCrudController[TestEntity]("/test", builder))

	body := suite.GET("/test/new").Expect(t).Status(http.StatusOK).Body()

	assert.Contains(t, body, "Option A", "the select was not rendered")
	assert.Contains(t, body, "Are you sure you want to delete this item?",
		"the select lost its localization key and is labelled by its field name")
}
