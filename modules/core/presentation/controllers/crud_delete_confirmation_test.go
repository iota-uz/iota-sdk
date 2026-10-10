package controllers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
)

// The edit form's Delete button dispatches open-delete-user-confirmation; the
// form must carry the dialog that listens for it, and confirming must send the
// DELETE itself -- #delete-form is nested in #save-form, so the parser drops
// it and submitting it does nothing.
func TestCrudController_EditFormDeleteHasItsConfirmation(t *testing.T) {
	suite := newCoreCrudSuiteBuilder(t).Build().AsUser(itf.User())
	service := newTestService()
	entity := TestEntity{ID: uuid.New(), Name: "Doomed", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	service.entities[entity.ID] = entity
	suite.Register(controllers.NewCrudController[TestEntity]("/test", createTestBuilder(service)))

	body := suite.GET("/test/" + entity.ID.String() + "/edit").Expect(t).Status(http.StatusOK).Body()

	assert.Contains(t, body, "$dispatch(&#39;open-delete-user-confirmation&#39;)",
		"the Delete button no longer opens the confirmation")
	assert.Contains(t, body, "@open-delete-user-confirmation.window",
		"no dialog listens for the Delete button's event")
	assert.Contains(t, body, `htmx.ajax(&#34;DELETE&#34;, &#34;/test/`+entity.ID.String()+`&#34;`),
		"confirming does not send the DELETE for this record"
}

// The DELETE the dialog sends removes the record and redirects to the list.
func TestCrudController_ConfirmedDeleteRemovesTheRecord(t *testing.T) {
	suite := newCoreCrudSuiteBuilder(t).Build().AsUser(itf.User())
	service := newTestService()
	entity := TestEntity{ID: uuid.New(), Name: "Doomed", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	service.entities[entity.ID] = entity
	suite.Register(controllers.NewCrudController[TestEntity]("/test", createTestBuilder(service)))

	resp := suite.DELETE("/test/" + entity.ID.String()).HTMX().Expect(t).Status(http.StatusOK)

	assert.Equal(t, "/test", resp.Header("HX-Redirect"))
	assert.Empty(t, service.entities, "the record was not deleted")
}
