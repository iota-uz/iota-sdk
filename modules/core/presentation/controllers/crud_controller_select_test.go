package controllers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/assert"
)

func TestCrudController_SelectFieldLabels(t *testing.T) {
	// Create test user
	testUser := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@example.com"),
		user.UILanguageEN,
	)

	suite := controllertest.New(t, core.NewModule()).
		AsUser(testUser)

	t.Run("displays labels in list view", func(t *testing.T) {
		// Create schema with select fields
		fields := crud.NewFields([]crud.Field{
			crud.NewUUIDField("id", crud.WithKey()),
			crud.NewSelectField("status").
				WithStaticOptions(
					crud.SelectOption{Value: "active", Label: "Active"},
					crud.SelectOption{Value: "inactive", Label: "Inactive"},
					crud.SelectOption{Value: "pending", Label: "Pending"},
				),
			crud.NewSelectField("type").
				AsIntSelect().
				WithStaticOptions(
					crud.SelectOption{Value: 1, Label: "Type One"},
					crud.SelectOption{Value: 2, Label: "Type Two"},
					crud.SelectOption{Value: 3, Label: "Type Three"},
				),
		})

		// Create service with test data
		service := newTestService()

		// Add test entities
		entities := []TestEntity{
			{
				ID:          uuid.New(),
				Name:        "active", // Using name field to store status
				Description: "1",      // Using description field to store type as string
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			{
				ID:          uuid.New(),
				Name:        "inactive",
				Description: "2",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			{
				ID:          uuid.New(),
				Name:        "pending",
				Description: "3",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		}

		for _, entity := range entities {
			service.entities[entity.ID] = entity
		}

		// Create custom mapper for select fields
		selectMapper := &selectTestMapper{
			fields:      fields,
			statusField: fields.Fields()[1].(crud.SelectField),
			typeField:   fields.Fields()[2].(crud.SelectField),
		}

		selectSchema := crud.NewSchema(
			"test_entities",
			fields,
			selectMapper,
		)

		builder := &testBuilder{
			schema:  selectSchema,
			service: service,
		}

		env := suite.Environment()
		controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
		suite.Register(controller)

		// Make request
		resp := suite.GET("/test").Expect(t).Status(http.StatusOK)
		body := resp.Body()

		// Verify labels are displayed instead of raw values
		assert.Contains(t, body, "Active")
		assert.Contains(t, body, "Inactive")
		assert.Contains(t, body, "Pending")
		assert.Contains(t, body, "Type One")
		assert.Contains(t, body, "Type Two")
		assert.Contains(t, body, "Type Three")

		// Also check using HTML parser for more precise verification
		doc := resp.HTML()

		// Check table cells contain labels, not raw values
		cells := doc.Elements("//tbody/tr/td")
		cellTexts := []string{}
		for i := 0; i < len(cells); i++ {
			cellTexts = append(cellTexts, doc.Element(fmt.Sprintf("//tbody/tr/td[%d]", i+1)).Text())
		}

		// Verify labels appear in cells
		bodyText := ""
		for _, text := range cellTexts {
			bodyText += text + " "
		}
		assert.Contains(t, bodyText, "Active")
		assert.Contains(t, bodyText, "Type One")
	})

	t.Run("displays labels in details view", func(t *testing.T) {
		// Create schema with select fields
		fields := crud.NewFields([]crud.Field{
			crud.NewUUIDField("id", crud.WithKey()),
			crud.NewSelectField("status").
				WithStaticOptions(
					crud.SelectOption{Value: "active", Label: "Active Status"},
					crud.SelectOption{Value: "inactive", Label: "Inactive Status"},
				),
			crud.NewSelectField("type").
				AsIntSelect().
				WithStaticOptions(
					crud.SelectOption{Value: 1, Label: "Type Number One"},
					crud.SelectOption{Value: 2, Label: "Type Number Two"},
				),
		})

		// Create service with test data
		service := newTestService()

		// Add test entity
		entity := TestEntity{
			ID:          uuid.New(),
			Name:        "active", // Using name field to store status
			Description: "2",      // Using description field to store type
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		service.entities[entity.ID] = entity

		// Create custom mapper for select fields
		selectMapper := &selectTestMapper{
			fields:      fields,
			statusField: fields.Fields()[1].(crud.SelectField),
			typeField:   fields.Fields()[2].(crud.SelectField),
		}

		selectSchema := crud.NewSchema(
			"test_entities",
			fields,
			selectMapper,
		)

		builder := &testBuilder{
			schema:  selectSchema,
			service: service,
		}

		env := suite.Environment()
		controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
		suite.Register(controller)

		// Make request
		resp := suite.GET(fmt.Sprintf("/test/%s/details", entity.ID)).
			Expect(t).
			Status(http.StatusOK)

		body := resp.Body()

		// Verify labels are displayed
		assert.Contains(t, body, "Active Status")
		assert.Contains(t, body, "Type Number Two")

		// Verify raw values are not displayed as visible text
		assert.NotContains(t, body, ">active<")
		assert.NotContains(t, body, ">2<")
	})

	t.Run("handles dynamic options loader", func(t *testing.T) {
		// Create schema with dynamic select field
		fields := crud.NewFields([]crud.Field{
			crud.NewUUIDField("id", crud.WithKey()),
			crud.NewSelectField("status").
				SetOptionsLoader(func(ctx context.Context) []crud.SelectOption {
					return []crud.SelectOption{
						{Value: "opt1", Label: "Dynamic Option 1"},
						{Value: "opt2", Label: "Dynamic Option 2"},
					}
				}),
		})

		// Create service with test data
		service := newTestService()

		// Add test entity
		entity := TestEntity{
			ID:        uuid.New(),
			Name:      "opt1", // Using name field to store status
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		service.entities[entity.ID] = entity

		// Create custom mapper
		selectMapper := &selectTestMapper{
			fields:      fields,
			statusField: fields.Fields()[1].(crud.SelectField),
		}

		selectSchema := crud.NewSchema(
			"test_entities",
			fields,
			selectMapper,
		)

		builder := &testBuilder{
			schema:  selectSchema,
			service: service,
		}

		env := suite.Environment()
		controller := controllers.NewCrudController[TestEntity]("/dynamic", env.App, builder)
		suite.Register(controller)

		// Make request
		resp := suite.GET("/dynamic").Expect(t).Status(http.StatusOK)
		body := resp.Body()

		// Verify dynamic label is displayed
		assert.Contains(t, body, "Dynamic Option 1")
		assert.NotContains(t, body, ">opt1<")
	})

	t.Run("displays raw value when no matching option", func(t *testing.T) {
		// Create schema with select fields
		fields := crud.NewFields([]crud.Field{
			crud.NewUUIDField("id", crud.WithKey()),
			crud.NewSelectField("status").
				WithStaticOptions(
					crud.SelectOption{Value: "active", Label: "Active"},
					crud.SelectOption{Value: "inactive", Label: "Inactive"},
				),
			crud.NewSelectField("type").
				AsIntSelect().
				WithStaticOptions(
					crud.SelectOption{Value: 1, Label: "Type One"},
					crud.SelectOption{Value: 2, Label: "Type Two"},
				),
		})

		// Create service with test data
		service := newTestService()

		// Add entity with values that don't match any options
		entity := TestEntity{
			ID:          uuid.New(),
			Name:        "unknown_status", // This doesn't match any option
			Description: "99",             // This doesn't match any option
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		service.entities[entity.ID] = entity

		// Create custom mapper
		selectMapper := &selectTestMapper{
			fields:      fields,
			statusField: fields.Fields()[1].(crud.SelectField),
			typeField:   fields.Fields()[2].(crud.SelectField),
		}

		selectSchema := crud.NewSchema(
			"test_entities",
			fields,
			selectMapper,
		)

		builder := &testBuilder{
			schema:  selectSchema,
			service: service,
		}

		env := suite.Environment()
		controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
		suite.Register(controller)

		// Make request
		resp := suite.GET("/test").Expect(t).Status(http.StatusOK)
		body := resp.Body()

		// Verify raw values are displayed as fallback
		assert.Contains(t, body, "unknown_status")
		assert.Contains(t, body, "99")
	})
}

// selectTestMapper is a custom mapper that handles select fields
type selectTestMapper struct {
	fields      crud.Fields
	statusField crud.SelectField
	typeField   crud.SelectField
}

func (m *selectTestMapper) ToEntities(_ context.Context, values ...[]crud.FieldValue) ([]TestEntity, error) {
	result := make([]TestEntity, len(values))

	for i, fvs := range values {
		entity := TestEntity{}
		for _, fv := range fvs {
			switch fv.Field().Name() {
			case "id":
				id, err := fv.AsUUID()
				if err != nil {
					return result, err
				}
				entity.ID = id
			case "status":
				// Store status in Name field
				if str, err := fv.AsString(); err == nil {
					entity.Name = str
				}
			case "type":
				// Store type in Description field as string
				if intVal, err := fv.AsInt(); err == nil {
					entity.Description = fmt.Sprintf("%d", intVal)
				}
			}
		}
		result[i] = entity
	}

	return result, nil
}

func (m *selectTestMapper) ToFieldValuesList(_ context.Context, entities ...TestEntity) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))

	for i, entity := range entities {
		fieldValues := []crud.FieldValue{}

		// ID field
		if idField, err := m.fields.Field("id"); err == nil {
			fieldValues = append(fieldValues, idField.Value(entity.ID))
		}

		// Status field (from Name)
		if m.statusField != nil {
			fieldValues = append(fieldValues, m.statusField.Value(entity.Name))
		}

		// Type field (from Description, converted to int)
		if m.typeField != nil {
			var typeValue int
			_, err := fmt.Sscanf(entity.Description, "%d", &typeValue)
			if err != nil {
				// Default to 0 if parsing fails
				typeValue = 0
			}
			fieldValues = append(fieldValues, m.typeField.Value(typeValue))
		}

		result[i] = fieldValues
	}

	return result, nil
}
