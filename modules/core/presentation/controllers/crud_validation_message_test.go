package controllers_test

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/assert"
)

// refusingBuilder is the test schema with a validator that refuses an amount
// over 100 with err.
func refusingBuilder(service *testService, err error) *testBuilder {
	base := createTestSchema()
	schema := crud.NewSchema("test_entities", base.Fields(), &testMapper{fields: base.Fields()},
		crud.WithValidator[TestEntity](func(e TestEntity) error {
			if e.Amount > 100 {
				return err
			}
			return nil
		}))
	return &testBuilder{schema: schema, service: service}
}

// A save the schema's validator refuses comes back with the validator's public
// message on the form. The form used to say only "Field validation failed",
// whatever it was handed.
func TestCrudController_RefusedSaveShowsThePublicReason(t *testing.T) {
	suite := newCoreCrudSuiteBuilder(t).Build().AsUser(itf.User())
	service := newTestService()
	refusal := serrors.NewInvalid("amount over limit").
		WithPublic(serrors.Message{Text: "Amount must not exceed 100."})
	suite.Register(controllers.NewCrudController[TestEntity]("/test", refusingBuilder(service, refusal)))

	body := suite.POST("/test").HTMX().
		Form(url.Values{"name": {"Too much"}, "description": {"x"}, "amount": {"150"}}).
		Expect(t).Status(http.StatusOK).Body()

	assert.Contains(t, body, `data-testid="field-error"`, "the error block is missing")
	assert.Contains(t, body, "Amount must not exceed 100.",
		"the refused save does not say why it was refused")
	assert.NotContains(t, body, "Field validation failed",
		"the form still shows only the generic text")
	assert.Empty(t, service.entities, "a refused entity was saved")
}

// A validator error with no public message is not leaked: the form shows the
// generic public text serrors gives it, not the internal string.
func TestCrudController_RefusedSaveDoesNotLeakAnInternalReason(t *testing.T) {
	suite := newCoreCrudSuiteBuilder(t).Build().AsUser(itf.User())
	service := newTestService()
	suite.Register(controllers.NewCrudController[TestEntity]("/test",
		refusingBuilder(service, errors.New("internal: limit table missing"))))

	body := suite.POST("/test").HTMX().
		Form(url.Values{"name": {"Too much"}, "description": {"x"}, "amount": {"150"}}).
		Expect(t).Status(http.StatusOK).Body()

	assert.Contains(t, body, `data-testid="field-error"`, "the error block is missing")
	assert.NotContains(t, body, "limit table missing", "an internal error reached the form")
	assert.Empty(t, service.entities, "a refused entity was saved")
}
