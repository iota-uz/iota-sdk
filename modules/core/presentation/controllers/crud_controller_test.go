package controllers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/itf"
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
	mu       sync.RWMutex
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls["GetAll"]++
	result := make([]TestEntity, 0, len(s.entities))
	for _, e := range s.entities {
		result = append(result, e)
	}
	return result, nil
}

func (s *testService) Get(ctx context.Context, value crud.FieldValue) (TestEntity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls["Exists"]++
	id, err := value.AsUUID()
	if err != nil {
		return false, err
	}
	_, exists := s.entities[id]
	return exists, nil
}

func (s *testService) Count(ctx context.Context, params *crud.FindParams) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
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
type testMapper struct {
	fields crud.Fields
}

func (m *testMapper) ToEntities(_ context.Context, values ...[]crud.FieldValue) ([]TestEntity, error) {
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
			case "name":
				name, err := fv.AsString()
				if err != nil {
					return result, err
				}
				entity.Name = name
			case "description":
				desc, err := fv.AsString()
				if err != nil {
					return result, err
				}
				entity.Description = desc
			case "amount":
				amount, err := fv.AsFloat64()
				if err != nil {
					return result, err
				}
				entity.Amount = amount
			case "is_active":
				active, err := fv.AsBool()
				if err != nil {
					return result, err
				}
				entity.IsActive = active
			case "created_at":
				created, err := fv.AsTime()
				if err != nil {
					return result, err
				}
				entity.CreatedAt = created
			case "updated_at":
				updated, err := fv.AsTime()
				if err != nil {
					return result, err
				}
				entity.UpdatedAt = updated
			}
		}
		result[i] = entity
	}

	return result, nil
}

func (m *testMapper) ToFieldValuesList(_ context.Context, entities ...TestEntity) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))

	for i, entity := range entities {
		fvs, err := m.fields.FieldValues(map[string]any{
			"id":          entity.ID,
			"name":        entity.Name,
			"description": entity.Description,
			"amount":      entity.Amount,
			"is_active":   entity.IsActive,
			"created_at":  entity.CreatedAt,
			"updated_at":  entity.UpdatedAt,
		})
		if err != nil {
			return nil, err
		}
		result[i] = fvs
	}

	return result, nil
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
		&testMapper{
			fields: fields,
		},
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

	suite := itf.HTTP(t, core.NewModule()).
		AsUser(testUser)

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
	suite.Register(controller)

	// Test
	doc := suite.GET("/test").Expect(t).Status(http.StatusOK).HTML()

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

	assert.Contains(t, headerText, "name")
	assert.Contains(t, headerText, "description")
	assert.Contains(t, headerText, "amount")
	assert.Contains(t, headerText, "is_active")

	// Check table rows (excluding hidden rows like preloaders)
	rows := doc.Elements("//tbody/tr[not(contains(@class, 'hidden'))]")

	assert.Len(t, rows, 5)
}

func TestCrudController_List_HTMX(t *testing.T) {
	t.Skip("TODO: Fix HTMX list test")
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Test HTMX request
	doc := suite.GET("/test").HTMX().Expect(t).Status(http.StatusOK).HTML()

	// Should only return table body for HTMX requests
	assert.Empty(t, doc.Elements("//html"))
	assert.NotEmpty(t, doc.Elements("//tr"))
}

func TestCrudController_List_Search(t *testing.T) {
	t.Skip("TODO: Fix search test")
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

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
	suite.Register(controller)

	// Test search
	doc := suite.GET("/test?q=app").Expect(t).Status(http.StatusOK).HTML()

	// Check only Apple appears in results
	rows := doc.Elements("//tbody/tr")
	assert.Len(t, rows, 1)
	appleElement := doc.Element("//tbody/tr[contains(., 'Apple')]")
	appleElement.Exists()
}

func TestCrudController_List_Pagination(t *testing.T) {
	t.Skip("TODO: Fix pagination test")
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

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
	suite.Register(controller)

	// Test first page
	doc := suite.GET("/test?page=1").Expect(t).Status(http.StatusOK).HTML()
	rows := doc.Elements("//tbody/tr")
	assert.Len(t, rows, 20) // Default page size

	// Test second page
	doc2 := suite.GET("/test?page=2").Expect(t).Status(http.StatusOK).HTML()
	rows2 := doc2.Elements("//tbody/tr")
	assert.Len(t, rows2, 5) // Remaining entities
}

func TestCrudController_GetNew(t *testing.T) {
	t.Skip("TODO: Fix GetNew form rendering test")
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Test
	doc := suite.GET("/test/new").Expect(t).Status(http.StatusOK).HTML()

	// Check form fields
	doc.Element("//input[@name='name']").Exists()
	doc.Element("//input[@name='description']").Exists()
	doc.Element("//input[@name='amount']").Exists()
	doc.Element("//input[@name='is_active']").Exists()

	// Readonly fields should not be in form
	doc.Element("//input[@name='created_at']").NotExists()
	doc.Element("//input[@name='updated_at']").NotExists()
}

func TestCrudController_Create_Success(t *testing.T) {
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Test
	formData := url.Values{
		"name":        {"New Entity"},
		"description": {"Test Description"},
		"amount":      {"123.45"},
		"is_active":   {"true"},
	}

	resp := suite.POST("/test").
		Form(formData).
		Expect(t).
		Status(http.StatusSeeOther)

	// Check redirect
	location := resp.Header("Location")
	assert.Contains(t, location, "/test")

	// Verify entity was created
	assert.Equal(t, 1, service.calls["Save"])
	assert.Len(t, service.entities, 1)

	// Check created entity
	var created TestEntity
	for _, e := range service.entities {
		created = e
		break
	}
	assert.Equal(t, "New Entity", created.Name)
	assert.Equal(t, "Test Description", created.Description)
	assert.InDelta(t, 123.45, created.Amount, 0.01)
	assert.True(t, created.IsActive)
}

func TestCrudController_Create_ValidationError(t *testing.T) {
	t.Skip("TODO: Fix validation error test")
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

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
	suite.Register(controller)

	// Test with empty name
	formData := url.Values{
		"name":        {""},
		"description": {"Test"},
		"amount":      {"100"},
	}

	resp := suite.POST("/test").
		Form(formData).
		Expect(t).
		Status(http.StatusUnprocessableEntity)

	// Should not redirect on validation error
	assert.Empty(t, resp.Header("Location"))
}

func TestCrudController_GetEdit_Success(t *testing.T) {
	t.Skip("TODO: Fix GetEdit form rendering test")
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

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
	suite.Register(controller)

	// Test
	doc := suite.GET(fmt.Sprintf("/test/%s/edit", entity.ID)).
		Expect(t).
		Status(http.StatusOK).
		HTML()

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
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Test with non-existent ID
	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("/test/%s/edit", nonExistentID)).
		Expect(t).
		Status(http.StatusNotFound)
}

func TestCrudController_Update_Success(t *testing.T) {
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

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
	suite.Register(controller)

	// Test update
	formData := url.Values{
		"name":        {"Updated Name"},
		"description": {"Updated Description"},
		"amount":      {"200.50"},
		"is_active":   {"true"},
	}

	resp := suite.POST(fmt.Sprintf("/test/%s", entity.ID)).
		Form(formData).
		Expect(t).
		Status(http.StatusSeeOther)

	// Check redirect
	location := resp.Header("Location")
	assert.Contains(t, location, "/test")

	// Verify entity was updated
	updated := service.entities[entity.ID]
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, "Updated Description", updated.Description)
	assert.InDelta(t, 200.50, updated.Amount, 0.01)
	assert.True(t, updated.IsActive)

	// TODO: Readonly field preservation during updates
	// Currently, readonly fields are not preserved during updates because
	// they aren't included in form data and the mapper creates a new entity
	// from form fields only. This should be enhanced to:
	// 1. Fetch existing entity first
	// 2. Apply form updates to existing entity
	// 3. Preserve readonly fields
	//
	// For now, we test that the entity was updated correctly for non-readonly fields
	// Note: The timestamps will be zero because they weren't in the form data
}

func TestCrudController_Delete_Success(t *testing.T) {
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

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
	suite.Register(controller)

	// Test delete (non-HTMX request defaults to 303 redirect)
	resp := suite.DELETE(fmt.Sprintf("/test/%s", entity.ID)).
		Expect(t).
		Status(http.StatusSeeOther)

	// For non-HTMX delete, should return 303 redirect
	location := resp.Header("Location")
	assert.Contains(t, location, "/test")

	// Verify entity was deleted
	assert.Equal(t, 1, service.calls["Delete"])
	assert.Empty(t, service.entities)
}

func TestCrudController_Delete_NotFound(t *testing.T) {
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Test delete non-existent (service error returns 500)
	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("/test/%s", nonExistentID)).
		Expect(t).
		Status(http.StatusInternalServerError)
}

func TestCrudController_InvalidUUID(t *testing.T) {
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Test various endpoints with invalid UUID
	suite.GET("/test/invalid-uuid/edit").Expect(t).Status(http.StatusBadRequest)
	suite.POST("/test/invalid-uuid").Form(url.Values{}).Expect(t).Status(http.StatusBadRequest)
	suite.DELETE("/test/invalid-uuid").Expect(t).Status(http.StatusBadRequest)
}

func TestCrudController_WithoutEdit(t *testing.T) {
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder, controllers.WithoutEdit[TestEntity]())
	suite.Register(controller)

	// Add test entity
	entity := TestEntity{ID: uuid.New(), Name: "Test"}
	service.entities[entity.ID] = entity

	// Edit endpoints return 405 when edit is disabled
	suite.GET(fmt.Sprintf("/test/%s/edit", entity.ID)).Expect(t).Status(http.StatusNotFound)
	suite.POST(fmt.Sprintf("/test/%s", entity.ID)).Form(url.Values{}).Expect(t).Status(http.StatusMethodNotAllowed)
}

func TestCrudController_WithoutDelete(t *testing.T) {
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder, controllers.WithoutDelete[TestEntity]())
	suite.Register(controller)

	// Add test entity
	entity := TestEntity{ID: uuid.New(), Name: "Test"}
	service.entities[entity.ID] = entity

	// Delete endpoint should return 405 (Method Not Allowed)
	suite.DELETE(fmt.Sprintf("/test/%s", entity.ID)).Expect(t).Status(http.StatusMethodNotAllowed)
}

func TestCrudController_WithoutCreate(t *testing.T) {
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder, controllers.WithoutCreate[TestEntity]())
	suite.Register(controller)

	// Create endpoints return 405 when create is disabled
	suite.GET("/test/new").Expect(t).Status(http.StatusMethodNotAllowed)
	suite.POST("/test").Form(url.Values{}).Expect(t).Status(http.StatusNotFound)
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
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := &complexTestService{}
	builder := &complexTestBuilder{
		schema:  complexSchema,
		service: service,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[ComplexEntity]("/complex", env.App, builder)
	suite.Register(controller)

	doc := suite.GET("/complex/new").Expect(t).Status(http.StatusOK).HTML()

	// Check each field type renders correctly
	doc.Element("//input[@name='string' and @type='text']").Exists()
	doc.Element("//input[@name='int' and @type='number']").Exists()
	doc.Element("//input[@name='bool' and @type='checkbox']").Exists()
	doc.Element("//input[@name='float' and @type='number']").Exists()
	doc.Element("//input[@name='decimal' and @type='number']").Exists()
	doc.Element("//input[@name='date' and @type='date']").Exists()
	doc.Element("//input[@name='datetime' and @type='datetime-local']").Exists()
	doc.Element("//input[@name='timestamp' and @type='datetime-local']").Exists()
	doc.Element("//input[@name='uuid' and @type='text']").Exists()
}

// testDecimalMapper implements crud.Mapper[TestEntity] for decimal field testing
type testDecimalMapper struct {
	fields crud.Fields
}

func (m *testDecimalMapper) ToEntities(_ context.Context, values ...[]crud.FieldValue) ([]TestEntity, error) {
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
			case "name":
				name, err := fv.AsString()
				if err != nil {
					return result, err
				}
				entity.Name = name
			case "amount":
				decimalStr, err := fv.AsDecimal()
				if err != nil {
					return result, err
				}
				amount, err := strconv.ParseFloat(decimalStr, 64)
				if err != nil {
					return result, err
				}
				entity.Amount = amount
			case "created_at":
				created, err := fv.AsTime()
				if err != nil {
					return result, err
				}
				entity.CreatedAt = created
			case "updated_at":
				updated, err := fv.AsTime()
				if err != nil {
					return result, err
				}
				entity.UpdatedAt = updated
			}
		}
		result[i] = entity
	}

	return result, nil
}

func (m *testDecimalMapper) ToFieldValuesList(_ context.Context, entities ...TestEntity) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))

	for i, entity := range entities {
		fvs, err := m.fields.FieldValues(map[string]any{
			"id":         entity.ID,
			"name":       entity.Name,
			"amount":     fmt.Sprintf("%.2f", entity.Amount),
			"created_at": entity.CreatedAt,
			"updated_at": entity.UpdatedAt,
		})
		if err != nil {
			return nil, err
		}
		result[i] = fvs
	}

	return result, nil
}

func TestCrudController_DecimalFieldHandling(t *testing.T) {
	// This test specifically covers the decimal field fix
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

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
		&testDecimalMapper{
			fields: decimalFields,
		},
	)

	builder := &testBuilder{
		schema:  decimalSchema,
		service: service,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Test that decimal value is properly populated in edit form
	doc := suite.GET(fmt.Sprintf("/test/%s/edit", entity.ID)).
		Expect(t).
		Status(http.StatusOK).
		HTML()

	amountInput := doc.Element("//input[@name='amount']")
	assert.Equal(t, "123.45", amountInput.Attr("value"))
}

func TestCrudController_ReadonlyFieldExclusion(t *testing.T) {
	// This test covers the readonly field exclusion fix
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

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
		Form(formData).
		Expect(t).
		Status(http.StatusSeeOther)

	// Verify entity was created without readonly field values from form
	assert.Len(t, service.entities, 1)
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
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

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
	suite.Register(controller)

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
		Form(formData).
		Expect(t).
		Status(http.StatusSeeOther)

	// Verify entity was created with the pre-assigned ID
	created, exists := service.entities[preAssignedID]
	assert.True(t, exists)
	assert.Equal(t, preAssignedID, created.ID)
	assert.Equal(t, "Pre-assigned ID Entity", created.Name)
}

func TestCrudController_FormFieldBuilder(t *testing.T) {
	// Test the form field builder functionality
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Test that form fields are properly built
	doc := suite.GET("/test/new").Expect(t).Status(200).HTML()

	// Verify form has the expected fields
	doc.Element("//input[@name='name']").Exists()
	doc.Element("//input[@name='description']").Exists()
	doc.Element("//input[@name='amount']").Exists()
	doc.Element("//input[@name='is_active']").Exists()
}

func TestCrudController_ErrorHandling(t *testing.T) {
	t.Skip("TODO: Error handling test - error message display not yet implemented")

	// TODO: Re-enable when error message display is properly implemented
	// Currently the error message display mechanism is being worked on

	/*
		// Test various error scenarios
		adminUser := itf.User()
		suite := itf.HTTP(t, core.NewModule()).
			AsUser(adminUser)

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
		suite.Register(controller)

		// Test create with service error
		formData := url.Values{
			"name": {"Test"},
		}

		resp := suite.POST("/test").
			Form(formData).
			Expect(t).
			Status(http.StatusInternalServerError)

		// Should show error message
		doc := resp.HTML()
		errorMsg := doc.Element("//*[@data-testid='alert-message']").Text()
		assert.Contains(t, errorMsg, "save failed")
	*/
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
	JSON      string
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

func TestCrudController_JSONField_FormHandling(t *testing.T) {
	// Setup
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule()).
		AsUser(adminUser)

	service := &complexTestService{}

	// Create schema with JSON field
	fields := crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey()),
		crud.NewStringField("string"),
		crud.NewJSONField("json", crud.JSONFieldConfig[interface{}]{}),
	})

	schema := crud.NewSchema[ComplexEntity](
		"complex_entities",
		fields,
		&complexTestMapper{fields: fields},
	)

	builder := &complexTestBuilder{
		schema:  schema,
		service: service,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[ComplexEntity]("/complex", env.App, builder)
	suite.Register(controller)

	// Test GET /new - should render textarea for JSON field
	doc := suite.GET("/complex/new").Expect(t).Status(200).HTML()

	// Check that JSON field is rendered as textarea
	jsonField := doc.Element("//textarea[@name='json']")
	jsonField.Exists()

	// Test POST with valid JSON - should redirect after successful creation
	validJSON := `{"name": "test", "value": 123}`
	formData := url.Values{
		"string": {"test string"},
		"json":   {validJSON},
	}

	suite.POST("/complex").
		Form(formData).
		Expect(t).
		Status(303) // Redirect after successful create

	// Test POST with invalid JSON - should return error
	invalidJSON := `{"name": "test", "value": 123` // Missing closing brace
	formData = url.Values{
		"string": {"test string"},
		"json":   {invalidJSON},
	}

	response := suite.POST("/complex").
		Form(formData).
		Expect(t).
		Status(400)

	// Check that error message is displayed
	body := response.Body()
	assert.Contains(t, body, "Invalid form data")
}

// complexTestMapper implements crud.Mapper[ComplexEntity]
type complexTestMapper struct {
	fields crud.Fields
}

func (m *complexTestMapper) ToEntities(ctx context.Context, values ...[]crud.FieldValue) ([]ComplexEntity, error) {
	entities := make([]ComplexEntity, len(values))
	for i, fvs := range values {
		entity := ComplexEntity{}
		for _, fv := range fvs {
			switch fv.Field().Name() {
			case "id":
				if id, err := fv.AsUUID(); err == nil {
					entity.ID = id
				}
			case "string":
				if str, err := fv.AsString(); err == nil {
					entity.String = str
				}
			case "json":
				if json, err := fv.AsString(); err == nil {
					entity.JSON = json
				}
			}
		}
		entities[i] = entity
	}
	return entities, nil
}

func (m *complexTestMapper) ToFieldValuesList(ctx context.Context, entities ...ComplexEntity) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))
	for i, entity := range entities {
		fvs := make([]crud.FieldValue, 0)

		if field, err := m.fields.Field("id"); err == nil {
			fvs = append(fvs, field.Value(entity.ID))
		}
		if field, err := m.fields.Field("string"); err == nil {
			fvs = append(fvs, field.Value(entity.String))
		}
		if field, err := m.fields.Field("json"); err == nil {
			fvs = append(fvs, field.Value(entity.JSON))
		}

		result[i] = fvs
	}
	return result, nil
}
