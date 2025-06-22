package controllers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/assert"
)

// TestEntity represents a test entity for CRUD operations
type TestEntity struct {
	ID          uuid.UUID
	Name        string
	Description string
	Amount      float64
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// testService implements crud.Service[TestEntity] for testing
type testService struct {
	entities map[uuid.UUID]TestEntity
	calls    map[string]int
}

func newTestService() *testService {
	return &testService{
		entities: make(map[uuid.UUID]TestEntity),
		calls:    make(map[string]int),
	}
}

func (s *testService) GetAll(ctx context.Context) ([]TestEntity, error) {
	s.calls["GetAll"]++
	result := make([]TestEntity, 0, len(s.entities))
	for _, e := range s.entities {
		result = append(result, e)
	}
	return result, nil
}

func (s *testService) Get(ctx context.Context, value crud.FieldValue) (TestEntity, error) {
	s.calls["Get"]++
	id, err := value.AsUUID()
	if err != nil {
		return TestEntity{}, err
	}
	entity, exists := s.entities[id]
	if !exists {
		return TestEntity{}, fmt.Errorf("entity not found")
	}
	return entity, nil
}

func (s *testService) Exists(ctx context.Context, value crud.FieldValue) (bool, error) {
	s.calls["Exists"]++
	id, err := value.AsUUID()
	if err != nil {
		return false, err
	}
	_, exists := s.entities[id]
	return exists, nil
}

func (s *testService) Count(ctx context.Context, params *crud.FindParams) (int64, error) {
	s.calls["Count"]++
	count := 0
	for _, entity := range s.entities {
		if params.Query != "" && !strings.Contains(strings.ToLower(entity.Name), strings.ToLower(params.Query)) {
			continue
		}
		count++
	}
	return int64(count), nil
}

func (s *testService) List(ctx context.Context, params *crud.FindParams) ([]TestEntity, error) {
	s.calls["List"]++
	result := make([]TestEntity, 0)
	for _, entity := range s.entities {
		if params.Query != "" && !strings.Contains(strings.ToLower(entity.Name), strings.ToLower(params.Query)) {
			continue
		}
		result = append(result, entity)
	}

	// Apply pagination
	start := params.Offset
	end := params.Offset + params.Limit
	if end > len(result) {
		end = len(result)
	}
	if start < len(result) {
		return result[start:end], nil
	}
	return []TestEntity{}, nil
}

func (s *testService) Save(ctx context.Context, entity TestEntity) (TestEntity, error) {
	s.calls["Save"]++
	if entity.ID == uuid.Nil {
		entity.ID = uuid.New()
		entity.CreatedAt = time.Now()
	}
	entity.UpdatedAt = time.Now()
	s.entities[entity.ID] = entity
	return entity, nil
}

func (s *testService) Delete(ctx context.Context, value crud.FieldValue) (TestEntity, error) {
	s.calls["Delete"]++
	id, err := value.AsUUID()
	if err != nil {
		return TestEntity{}, err
	}
	entity, exists := s.entities[id]
	if !exists {
		return TestEntity{}, fmt.Errorf("entity not found")
	}
	delete(s.entities, id)
	return entity, nil
}

// testMapper implements crud.Mapper[TestEntity]
type testMapper struct{}

func (m *testMapper) ToEntity(ctx context.Context, values []crud.FieldValue) (TestEntity, error) {
	entity := TestEntity{}
	for _, fv := range values {
		switch fv.Field().Name() {
		case "id":
			if !fv.IsZero() {
				id, _ := fv.AsUUID()
				entity.ID = id
			}
		case "name":
			if !fv.IsZero() {
				name, _ := fv.AsString()
				entity.Name = name
			}
		case "description":
			if !fv.IsZero() {
				desc, _ := fv.AsString()
				entity.Description = desc
			}
		case "amount":
			if !fv.IsZero() {
				amount, _ := fv.AsFloat64()
				entity.Amount = amount
			}
		case "is_active":
			if !fv.IsZero() {
				active, _ := fv.AsBool()
				entity.IsActive = active
			}
		case "created_at":
			if !fv.IsZero() {
				created, _ := fv.AsTime()
				entity.CreatedAt = created
			}
		case "updated_at":
			if !fv.IsZero() {
				updated, _ := fv.AsTime()
				entity.UpdatedAt = updated
			}
		}
	}
	return entity, nil
}

func (m *testMapper) ToFieldValues(ctx context.Context, entity TestEntity) ([]crud.FieldValue, error) {
	schema := createTestSchema()
	return schema.Fields().FieldValues(map[string]any{
		"id":          entity.ID,
		"name":        entity.Name,
		"description": entity.Description,
		"amount":      entity.Amount,
		"is_active":   entity.IsActive,
		"created_at":  entity.CreatedAt,
		"updated_at":  entity.UpdatedAt,
	})
}

func createTestSchema() crud.Schema[TestEntity] {
	fields := crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey()),
		crud.NewStringField("name", crud.WithSearchable()),
		crud.NewStringField("description"),
		crud.NewFloatField("amount"),
		crud.NewBoolField("is_active"),
		crud.NewTimestampField("created_at", crud.WithReadonly()),
		crud.NewTimestampField("updated_at", crud.WithReadonly()),
	})
	return crud.NewSchema(
		"test_entities",
		fields,
		&testMapper{},
	)
}

func createTestBuilder(service *testService) crud.Builder[TestEntity] {
	schema := createTestSchema()
	return &testBuilder{
		schema:  schema,
		service: service,
	}
}

// testBuilder implements crud.Builder[TestEntity] for tests
type testBuilder struct {
	schema  crud.Schema[TestEntity]
	service crud.Service[TestEntity]
}

func (b *testBuilder) Schema() crud.Schema[TestEntity] {
	return b.schema
}

func (b *testBuilder) Service() crud.Service[TestEntity] {
	return b.service
}

func (b *testBuilder) Repository() crud.Repository[TestEntity] {
	return nil // Not needed for these tests
}

func TestCrudController_List_Success(t *testing.T) {
	// Setup
	// Create a test user
	testUser := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@example.com"),
		user.UILanguageEN,
	)

	suite := controllertest.New().
		WithModules().
		WithUser(t, testUser).
		Build(t)

	service := newTestService()

	// Add test data
	for i := 0; i < 5; i++ {
		entity := TestEntity{
			ID:          uuid.New(),
			Name:        fmt.Sprintf("Test Entity %d", i),
			Description: fmt.Sprintf("Description %d", i),
			Amount:      float64(i * 100),
			IsActive:    i%2 == 0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		service.entities[entity.ID] = entity
	}

	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test
	doc := suite.GET("/test").Expect().Status(t, http.StatusOK).HTML(t)

	// Assertions
	assert.Equal(t, 1, service.calls["List"])
	assert.Equal(t, 1, service.calls["Count"])

	// Check table headers
	headerElements := doc.Elements("//thead/tr/th")
	assert.GreaterOrEqual(t, len(headerElements), 4)

	// Extract header text
	headerTexts := make([]string, len(headerElements))
	for i := range headerElements {
		headerTexts[i] = doc.Element("//thead/tr/th[" + fmt.Sprintf("%d", i+1) + "]").Text()
	}
	headerText := strings.Join(headerTexts, " ")

	assert.Contains(t, headerText, "Name")
	assert.Contains(t, headerText, "Description")
	assert.Contains(t, headerText, "Amount")
	assert.Contains(t, headerText, "Is Active")

	// Check table rows
	rows := doc.Elements("//tbody/tr")
	assert.Equal(t, 5, len(rows))
}

func TestCrudController_List_HTMX(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test HTMX request
	doc := suite.GET("/test").HTMX().Expect().Status(t, http.StatusOK).HTML(t)

	// Should only return table body for HTMX requests
	assert.Equal(t, 0, len(doc.Elements("//html")))
	assert.Greater(t, len(doc.Elements("//tr")), 0)
}

func TestCrudController_List_Search(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()

	// Add test data
	entity1 := TestEntity{ID: uuid.New(), Name: "Apple", Description: "Fruit"}
	entity2 := TestEntity{ID: uuid.New(), Name: "Banana", Description: "Fruit"}
	entity3 := TestEntity{ID: uuid.New(), Name: "Carrot", Description: "Vegetable"}
	service.entities[entity1.ID] = entity1
	service.entities[entity2.ID] = entity2
	service.entities[entity3.ID] = entity3

	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test search
	doc := suite.GET("/test?q=app").Expect().Status(t, http.StatusOK).HTML(t)

	// Check only Apple appears in results
	rows := doc.Elements("//tbody/tr")
	assert.Equal(t, 1, len(rows))
	appleElement := doc.Element("//tbody/tr[contains(., 'Apple')]")
	appleElement.Exists(t)
}

func TestCrudController_List_Pagination(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()

	// Add 25 test entities
	for i := 0; i < 25; i++ {
		entity := TestEntity{
			ID:   uuid.New(),
			Name: fmt.Sprintf("Entity %02d", i),
		}
		service.entities[entity.ID] = entity
	}

	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test first page
	doc := suite.GET("/test?page=1").Expect().Status(t, http.StatusOK).HTML(t)
	rows := doc.Elements("//tbody/tr")
	assert.Equal(t, 20, len(rows)) // Default page size

	// Test second page
	doc2 := suite.GET("/test?page=2").Expect().Status(t, http.StatusOK).HTML(t)
	rows2 := doc2.Elements("//tbody/tr")
	assert.Equal(t, 5, len(rows2)) // Remaining entities
}

func TestCrudController_GetNew(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test
	doc := suite.GET("/test/new").Expect().Status(t, http.StatusOK).HTML(t)

	// Check form fields
	doc.Element("//input[@name='name']").Exists(t)
	doc.Element("//input[@name='description']").Exists(t)
	doc.Element("//input[@name='amount']").Exists(t)
	doc.Element("//input[@name='is_active']").Exists(t)

	// Readonly fields should not be in form
	doc.Element("//input[@name='created_at']").NotExists(t)
	doc.Element("//input[@name='updated_at']").NotExists(t)
}

func TestCrudController_Create_Success(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test
	formData := url.Values{
		"name":        {"New Entity"},
		"description": {"Test Description"},
		"amount":      {"123.45"},
		"is_active":   {"true"},
	}

	resp := suite.POST("/test").
		WithForm(formData).
		Expect().
		Status(t, http.StatusSeeOther)

	// Check redirect
	location := resp.Header("Location")
	assert.Contains(t, location, "/test")

	// Verify entity was created
	assert.Equal(t, 1, service.calls["Save"])
	assert.Equal(t, 1, len(service.entities))

	// Check created entity
	var created TestEntity
	for _, e := range service.entities {
		created = e
		break
	}
	assert.Equal(t, "New Entity", created.Name)
	assert.Equal(t, "Test Description", created.Description)
	assert.Equal(t, 123.45, created.Amount)
	assert.Equal(t, true, created.IsActive)
}

func TestCrudController_Create_ValidationError(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()

	// Create schema with validation
	fieldsWithValidation := crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey()),
		crud.NewStringField("name", crud.WithSearchable(), crud.WithRules(crud.RequiredRule())),
		crud.NewStringField("description"),
		crud.NewFloatField("amount"),
		crud.NewBoolField("is_active"),
		crud.NewTimestampField("created_at", crud.WithReadonly()),
		crud.NewTimestampField("updated_at", crud.WithReadonly()),
	})
	schemaWithValidation := crud.NewSchema(
		"test_entities",
		fieldsWithValidation,
		&testMapper{},
	)

	builder := &testBuilder{
		schema:  schemaWithValidation,
		service: service,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test with empty name
	formData := url.Values{
		"name":        {""},
		"description": {"Test"},
		"amount":      {"100"},
	}

	resp := suite.POST("/test").
		WithForm(formData).
		Expect().
		Status(t, http.StatusUnprocessableEntity)

	// Should not redirect on validation error
	assert.Empty(t, resp.Header("Location"))
}

func TestCrudController_GetEdit_Success(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()

	// Add test entity
	entity := TestEntity{
		ID:          uuid.New(),
		Name:        "Test Entity",
		Description: "Description",
		Amount:      99.99,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	service.entities[entity.ID] = entity

	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test
	doc := suite.GET(fmt.Sprintf("/test/%s/edit", entity.ID)).
		Expect().
		Status(t, http.StatusOK).
		HTML(t)

	// Check form is populated
	nameInput := doc.Element("//input[@name='name']")
	assert.Equal(t, "Test Entity", nameInput.Attr("value"))

	descInput := doc.Element("//input[@name='description']")
	assert.Equal(t, "Description", descInput.Attr("value"))

	amountInput := doc.Element("//input[@name='amount']")
	assert.Equal(t, "99.99", amountInput.Attr("value"))

	// Check checkbox
	activeInput := doc.Element("//input[@name='is_active']")
	assert.Equal(t, "checked", activeInput.Attr("checked"))
}

func TestCrudController_GetEdit_NotFound(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test with non-existent ID
	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("/test/%s/edit", nonExistentID)).
		Expect().
		Status(t, http.StatusNotFound)
}

func TestCrudController_Update_Success(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()

	// Add test entity
	entity := TestEntity{
		ID:          uuid.New(),
		Name:        "Original Name",
		Description: "Original Description",
		Amount:      100.0,
		IsActive:    false,
		CreatedAt:   time.Now().Add(-time.Hour),
		UpdatedAt:   time.Now().Add(-time.Hour),
	}
	service.entities[entity.ID] = entity

	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test update
	formData := url.Values{
		"name":        {"Updated Name"},
		"description": {"Updated Description"},
		"amount":      {"200.50"},
		"is_active":   {"true"},
	}

	resp := suite.POST(fmt.Sprintf("/test/%s", entity.ID)).
		WithForm(formData).
		Expect().
		Status(t, http.StatusSeeOther)

	// Check redirect
	location := resp.Header("Location")
	assert.Contains(t, location, "/test")

	// Verify entity was updated
	updated := service.entities[entity.ID]
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, "Updated Description", updated.Description)
	assert.Equal(t, 200.50, updated.Amount)
	assert.Equal(t, true, updated.IsActive)

	// Timestamps should be preserved (readonly)
	assert.Equal(t, entity.CreatedAt, updated.CreatedAt)
}

func TestCrudController_Delete_Success(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()

	// Add test entity
	entity := TestEntity{
		ID:   uuid.New(),
		Name: "To Delete",
	}
	service.entities[entity.ID] = entity

	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test delete
	resp := suite.DELETE(fmt.Sprintf("/test/%s", entity.ID)).
		Expect().
		Status(t, http.StatusOK)

	// For HTMX delete, should return 200 with HX-Redirect header
	assert.Contains(t, resp.Header("HX-Redirect"), "/test")

	// Verify entity was deleted
	assert.Equal(t, 1, service.calls["Delete"])
	assert.Equal(t, 0, len(service.entities))
}

func TestCrudController_Delete_NotFound(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test delete non-existent
	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("/test/%s", nonExistentID)).
		Expect().
		Status(t, http.StatusNotFound)
}

func TestCrudController_InvalidUUID(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test various endpoints with invalid UUID
	suite.GET("/test/invalid-uuid/edit").Expect().Status(t, http.StatusBadRequest)
	suite.POST("/test/invalid-uuid").WithForm(url.Values{}).Expect().Status(t, http.StatusBadRequest)
	suite.DELETE("/test/invalid-uuid").Expect().Status(t, http.StatusBadRequest)
}

func TestCrudController_WithoutEdit(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder, controllers.WithoutEdit[TestEntity]())
	suite.RegisterController(controller)

	// Add test entity
	entity := TestEntity{ID: uuid.New(), Name: "Test"}
	service.entities[entity.ID] = entity

	// Edit endpoints should return 404
	suite.GET(fmt.Sprintf("/test/%s/edit", entity.ID)).Expect().Status(t, http.StatusNotFound)
	suite.POST(fmt.Sprintf("/test/%s", entity.ID)).WithForm(url.Values{}).Expect().Status(t, http.StatusNotFound)
}

func TestCrudController_WithoutDelete(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder, controllers.WithoutDelete[TestEntity]())
	suite.RegisterController(controller)

	// Add test entity
	entity := TestEntity{ID: uuid.New(), Name: "Test"}
	service.entities[entity.ID] = entity

	// Delete endpoint should return 404
	suite.DELETE(fmt.Sprintf("/test/%s", entity.ID)).Expect().Status(t, http.StatusNotFound)
}

func TestCrudController_WithoutCreate(t *testing.T) {
	// Setup
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder, controllers.WithoutCreate[TestEntity]())
	suite.RegisterController(controller)

	// Create endpoints should return 404
	suite.GET("/test/new").Expect().Status(t, http.StatusNotFound)
	suite.POST("/test").WithForm(url.Values{}).Expect().Status(t, http.StatusNotFound)
}

func TestCrudController_FieldTypes(t *testing.T) {
	// Test entity with various field types

	complexMapper := struct{ crud.Mapper[ComplexEntity] }{}
	complexFields := crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey()),
		crud.NewStringField("string"),
		crud.NewIntField("int"),
		crud.NewBoolField("bool"),
		crud.NewFloatField("float"),
		crud.NewDecimalField("decimal"),
		crud.NewDateField("date"),
		crud.NewDateTimeField("datetime"),
		crud.NewTimestampField("timestamp"),
		crud.NewUUIDField("uuid"),
	})
	complexSchema := crud.NewSchema(
		"complex_entities",
		complexFields,
		complexMapper,
	)

	// Test form rendering for each field type
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := &complexTestService{}
	builder := &complexTestBuilder{
		schema:  complexSchema,
		service: service,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[ComplexEntity]("/complex", env.App, builder)
	suite.RegisterController(controller)

	doc := suite.GET("/complex/new").Expect().Status(t, http.StatusOK).HTML(t)

	// Check each field type renders correctly
	doc.Element("//input[@name='string' and @type='text']").Exists(t)
	doc.Element("//input[@name='int' and @type='number']").Exists(t)
	doc.Element("//input[@name='bool' and @type='checkbox']").Exists(t)
	doc.Element("//input[@name='float' and @type='number']").Exists(t)
	doc.Element("//input[@name='decimal' and @type='number']").Exists(t)
	doc.Element("//input[@name='date' and @type='date']").Exists(t)
	doc.Element("//input[@name='datetime' and @type='datetime-local']").Exists(t)
	doc.Element("//input[@name='timestamp' and @type='datetime-local']").Exists(t)
	doc.Element("//input[@name='uuid' and @type='text']").Exists(t)
}

func TestCrudController_DecimalFieldHandling(t *testing.T) {
	// This test specifically covers the decimal field fix
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()

	// Add entity with decimal value
	entity := TestEntity{
		ID:     uuid.New(),
		Name:   "Test",
		Amount: 123.45, // This is treated as a decimal
	}
	service.entities[entity.ID] = entity

	// Create schema with decimal field
	decimalFields := crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey()),
		crud.NewStringField("name"),
		crud.NewDecimalField("amount"), // Changed to decimal type
		crud.NewTimestampField("created_at", crud.WithReadonly()),
		crud.NewTimestampField("updated_at", crud.WithReadonly()),
	})
	decimalSchema := crud.NewSchema(
		"test_entities",
		decimalFields,
		&testMapper{},
	)

	builder := &testBuilder{
		schema:  decimalSchema,
		service: service,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test that decimal value is properly populated in edit form
	doc := suite.GET(fmt.Sprintf("/test/%s/edit", entity.ID)).
		Expect().
		Status(t, http.StatusOK).
		HTML(t)

	amountInput := doc.Element("//input[@name='amount']")
	assert.Equal(t, "123.45", amountInput.Attr("value"))
}

func TestCrudController_ReadonlyFieldExclusion(t *testing.T) {
	// This test covers the readonly field exclusion fix
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test create with readonly fields in form data
	formData := url.Values{
		"name":        {"New Entity"},
		"description": {"Test"},
		"amount":      {"100"},
		"is_active":   {"true"},
		// Include readonly fields that should be ignored
		"created_at": {"2024-01-01T00:00:00Z"},
		"updated_at": {"2024-01-01T00:00:00Z"},
	}

	suite.POST("/test").
		WithForm(formData).
		Expect().
		Status(t, http.StatusSeeOther)

	// Verify entity was created without readonly field values from form
	assert.Equal(t, 1, len(service.entities))
	var created TestEntity
	for _, e := range service.entities {
		created = e
		break
	}

	// Readonly fields should not be set from form data
	assert.NotEqual(t, "2024-01-01T00:00:00Z", created.CreatedAt.Format(time.RFC3339))
	assert.NotEqual(t, "2024-01-01T00:00:00Z", created.UpdatedAt.Format(time.RFC3339))
}

// preAssignedTestService wraps testService to handle pre-assigned IDs
type preAssignedTestService struct {
	*testService
}

func (s *preAssignedTestService) Save(ctx context.Context, entity TestEntity) (TestEntity, error) {
	if entity.ID != uuid.Nil {
		// Entity has pre-assigned ID, should not fail validation
		s.entities[entity.ID] = entity
		return entity, nil
	}
	return s.testService.Save(ctx, entity)
}

func TestCrudController_PreAssignedKeyHandling(t *testing.T) {
	// Test handling of entities with pre-assigned keys (like string IDs or UUIDs)
	suite := controllertest.New().
		WithModules().
		Build(t)

	baseService := newTestService()

	// Create wrapper service that handles pre-assigned IDs properly
	service := &preAssignedTestService{
		testService: baseService,
	}

	builder := &testBuilder{
		schema:  createTestSchema(),
		service: service,
	}
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test create with pre-assigned UUID
	preAssignedID := uuid.New()
	formData := url.Values{
		"id":          {preAssignedID.String()},
		"name":        {"Pre-assigned ID Entity"},
		"description": {"Test"},
		"amount":      {"100"},
		"is_active":   {"true"},
	}

	suite.POST("/test").
		WithForm(formData).
		Expect().
		Status(t, http.StatusSeeOther)

	// Verify entity was created with the pre-assigned ID
	created, exists := service.entities[preAssignedID]
	assert.True(t, exists)
	assert.Equal(t, preAssignedID, created.ID)
	assert.Equal(t, "Pre-assigned ID Entity", created.Name)
}

func TestCrudController_FormFieldBuilder(t *testing.T) {
	// Test the form field builder functionality
	suite := controllertest.New().
		WithModules().
		Build(t)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test that form fields are properly built
	doc := suite.GET("/test/new").Expect().Status(t, 200).HTML(t)

	// Verify form has the expected fields
	doc.Element("//input[@name='name']").Exists(t)
	doc.Element("//input[@name='description']").Exists(t)
	doc.Element("//input[@name='amount']").Exists(t)
	doc.Element("//input[@name='is_active']").Exists(t)
}

// errorTestService wraps testService to simulate errors
type errorTestService struct {
	*testService
}

func (s *errorTestService) Save(ctx context.Context, entity TestEntity) (TestEntity, error) {
	return TestEntity{}, fmt.Errorf("save failed")
}

func TestCrudController_ErrorHandling(t *testing.T) {
	// Test various error scenarios
	suite := controllertest.New().
		WithModules().
		Build(t)

	baseService := newTestService()

	// Create error service wrapper
	errorService := &errorTestService{
		testService: baseService,
	}

	builder := &testBuilder{
		schema:  createTestSchema(),
		service: errorService,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.RegisterController(controller)

	// Test create with service error
	formData := url.Values{
		"name": {"Test"},
	}

	resp := suite.POST("/test").
		WithForm(formData).
		Expect().
		Status(t, http.StatusInternalServerError)

	// Should show error message
	doc := resp.HTML(t)
	errorMsg := doc.Element("//*[@data-testid='alert-message']").Text()
	assert.Contains(t, errorMsg, "save failed")
}

// ComplexEntity represents an entity with various field types
type ComplexEntity struct {
	ID        uuid.UUID
	String    string
	Int       int
	Bool      bool
	Float     float64
	Decimal   string
	Date      time.Time
	DateTime  time.Time
	Timestamp time.Time
	UUID      uuid.UUID
}

// complexTestService implements crud.Service[ComplexEntity]
type complexTestService struct{}

func (s *complexTestService) GetAll(ctx context.Context) ([]ComplexEntity, error) {
	return []ComplexEntity{}, nil
}

func (s *complexTestService) Get(ctx context.Context, value crud.FieldValue) (ComplexEntity, error) {
	return ComplexEntity{}, fmt.Errorf("entity not found")
}

func (s *complexTestService) Exists(ctx context.Context, value crud.FieldValue) (bool, error) {
	return false, nil
}

func (s *complexTestService) Count(ctx context.Context, params *crud.FindParams) (int64, error) {
	return 0, nil
}

func (s *complexTestService) List(ctx context.Context, params *crud.FindParams) ([]ComplexEntity, error) {
	return []ComplexEntity{}, nil
}

func (s *complexTestService) Save(ctx context.Context, entity ComplexEntity) (ComplexEntity, error) {
	return entity, nil
}

func (s *complexTestService) Delete(ctx context.Context, value crud.FieldValue) (ComplexEntity, error) {
	return ComplexEntity{}, fmt.Errorf("entity not found")
}

// complexTestBuilder implements crud.Builder[ComplexEntity]
type complexTestBuilder struct {
	schema  crud.Schema[ComplexEntity]
	service crud.Service[ComplexEntity]
}

func (b *complexTestBuilder) Schema() crud.Schema[ComplexEntity] {
	return b.schema
}

func (b *complexTestBuilder) Service() crud.Service[ComplexEntity] {
	return b.service
}

func (b *complexTestBuilder) Repository() crud.Repository[ComplexEntity] {
	return nil // Not needed for these tests
}
