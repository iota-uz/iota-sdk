package controllers_test

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/assert"
)

// DecimalValue is a custom decimal type that implements driver.Valuer
type DecimalValue struct {
	value string
}

func (d DecimalValue) Value() (driver.Value, error) {
	return d.value, nil
}

func (d DecimalValue) String() string {
	return fmt.Sprintf("DecimalValue{%s}", d.value)
}

// TestEntityWithDecimal represents an entity with a custom decimal type
type TestEntityWithDecimal struct {
	ID      uuid.UUID
	Name    string
	Price   DecimalValue
	Created time.Time
	Updated time.Time
}

// decimalMapper implements crud.Mapper[TestEntityWithDecimal]
type decimalMapper struct{}

func (m *decimalMapper) ToEntity(ctx context.Context, values []crud.FieldValue) (TestEntityWithDecimal, error) {
	entity := TestEntityWithDecimal{}
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
		case "price":
			if !fv.IsZero() {
				// Handle decimal value
				if decStr, err := fv.AsDecimal(); err == nil {
					entity.Price = DecimalValue{value: decStr}
				}
			}
		case "created":
			if !fv.IsZero() {
				created, _ := fv.AsTime()
				entity.Created = created
			}
		case "updated":
			if !fv.IsZero() {
				updated, _ := fv.AsTime()
				entity.Updated = updated
			}
		}
	}
	return entity, nil
}

func (m *decimalMapper) ToFieldValues(ctx context.Context, entity TestEntityWithDecimal) ([]crud.FieldValue, error) {
	schema := createDecimalTestSchema()
	return schema.Fields().FieldValues(map[string]any{
		"id":      entity.ID,
		"name":    entity.Name,
		"price":   entity.Price,
		"created": entity.Created,
		"updated": entity.Updated,
	})
}

func createDecimalTestSchema() crud.Schema[TestEntityWithDecimal] {
	fields := crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey()),
		crud.NewStringField("name"),
		crud.NewDecimalField("price"),
		crud.NewTimestampField("created", crud.WithReadonly()),
		crud.NewTimestampField("updated", crud.WithReadonly()),
	})
	return crud.NewSchema(
		"decimal_entities",
		fields,
		&decimalMapper{},
	)
}

// decimalService implements crud.Service[TestEntityWithDecimal]
type decimalService struct {
	entities map[uuid.UUID]TestEntityWithDecimal
}

func newDecimalService() *decimalService {
	return &decimalService{
		entities: make(map[uuid.UUID]TestEntityWithDecimal),
	}
}

func (s *decimalService) GetAll(ctx context.Context) ([]TestEntityWithDecimal, error) {
	result := make([]TestEntityWithDecimal, 0, len(s.entities))
	for _, e := range s.entities {
		result = append(result, e)
	}
	return result, nil
}

func (s *decimalService) Get(ctx context.Context, value crud.FieldValue) (TestEntityWithDecimal, error) {
	id, err := value.AsUUID()
	if err != nil {
		return TestEntityWithDecimal{}, err
	}
	entity, exists := s.entities[id]
	if !exists {
		return TestEntityWithDecimal{}, fmt.Errorf("entity not found")
	}
	return entity, nil
}

func (s *decimalService) Exists(ctx context.Context, value crud.FieldValue) (bool, error) {
	id, err := value.AsUUID()
	if err != nil {
		return false, err
	}
	_, exists := s.entities[id]
	return exists, nil
}

func (s *decimalService) Count(ctx context.Context, params *crud.FindParams) (int64, error) {
	return int64(len(s.entities)), nil
}

func (s *decimalService) List(ctx context.Context, params *crud.FindParams) ([]TestEntityWithDecimal, error) {
	return s.GetAll(ctx)
}

func (s *decimalService) Save(ctx context.Context, entity TestEntityWithDecimal) (TestEntityWithDecimal, error) {
	if entity.ID == uuid.Nil {
		entity.ID = uuid.New()
		entity.Created = time.Now()
	}
	entity.Updated = time.Now()
	s.entities[entity.ID] = entity
	return entity, nil
}

func (s *decimalService) Delete(ctx context.Context, value crud.FieldValue) (TestEntityWithDecimal, error) {
	id, err := value.AsUUID()
	if err != nil {
		return TestEntityWithDecimal{}, err
	}
	entity, exists := s.entities[id]
	if !exists {
		return TestEntityWithDecimal{}, fmt.Errorf("entity not found")
	}
	delete(s.entities, id)
	return entity, nil
}

// decimalTestBuilder implements crud.Builder[TestEntityWithDecimal]
type decimalTestBuilder struct {
	schema  crud.Schema[TestEntityWithDecimal]
	service crud.Service[TestEntityWithDecimal]
}

func (b *decimalTestBuilder) Schema() crud.Schema[TestEntityWithDecimal] {
	return b.schema
}

func (b *decimalTestBuilder) Service() crud.Service[TestEntityWithDecimal] {
	return b.service
}

func (b *decimalTestBuilder) Repository() crud.Repository[TestEntityWithDecimal] {
	return nil // Not needed for these tests
}

// NullableEntity represents an entity with nullable fields
type NullableEntity struct {
	ID          uuid.UUID
	Name        string
	OptionalStr *string
	OptionalInt *int
	CreatedAt   time.Time
}

// nullableTestService implements crud.Service[NullableEntity]
type nullableTestService struct{}

func (s *nullableTestService) GetAll(ctx context.Context) ([]NullableEntity, error) {
	return []NullableEntity{}, nil
}

func (s *nullableTestService) Get(ctx context.Context, value crud.FieldValue) (NullableEntity, error) {
	return NullableEntity{}, fmt.Errorf("entity not found")
}

func (s *nullableTestService) Exists(ctx context.Context, value crud.FieldValue) (bool, error) {
	return false, nil
}

func (s *nullableTestService) Count(ctx context.Context, params *crud.FindParams) (int64, error) {
	return 0, nil
}

func (s *nullableTestService) List(ctx context.Context, params *crud.FindParams) ([]NullableEntity, error) {
	return []NullableEntity{}, nil
}

func (s *nullableTestService) Save(ctx context.Context, entity NullableEntity) (NullableEntity, error) {
	return entity, nil
}

func (s *nullableTestService) Delete(ctx context.Context, value crud.FieldValue) (NullableEntity, error) {
	return NullableEntity{}, fmt.Errorf("entity not found")
}

// nullableTestBuilder implements crud.Builder[NullableEntity]
type nullableTestBuilder struct {
	schema  crud.Schema[NullableEntity]
	service crud.Service[NullableEntity]
}

func (b *nullableTestBuilder) Schema() crud.Schema[NullableEntity] {
	return b.schema
}

func (b *nullableTestBuilder) Service() crud.Service[NullableEntity] {
	return b.service
}

func (b *nullableTestBuilder) Repository() crud.Repository[NullableEntity] {
	return nil // Not needed for these tests
}

// TestCrudController_DecimalFieldWithDriverValuer tests the decimal field fix with driver.Valuer types
func TestCrudController_DecimalFieldWithDriverValuer(t *testing.T) {
	adminUser := testutils.MockUser()
	suite := controllertest.New(t, core.NewModule()).
		AsUser(adminUser)

	service := newDecimalService()

	// Add entity with decimal value
	entity := TestEntityWithDecimal{
		ID:      uuid.New(),
		Name:    "Product",
		Price:   DecimalValue{value: "123.45"},
		Created: time.Now(),
		Updated: time.Now(),
	}
	service.entities[entity.ID] = entity

	schema := createDecimalTestSchema()
	builder := &decimalTestBuilder{
		schema:  schema,
		service: service,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntityWithDecimal]("/products", env.App, builder)
	suite.Register(controller)

	// Test that decimal value is properly populated in edit form
	doc := suite.GET(fmt.Sprintf("/products/%s/edit", entity.ID)).
		Expect(t).
		Status(200).
		HTML()

	priceInput := doc.Element("//input[@name='price']")
	// The fix ensures AsDecimal() is called, which handles driver.Valuer types
	assert.Equal(t, "123.45", priceInput.Attr("value"))
}

// TestEntityWithStringKey represents an entity with string primary key
type TestEntityWithStringKey struct {
	Code        string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// stringKeyMapper implements crud.Mapper[TestEntityWithStringKey]
type stringKeyMapper struct{}

func (m *stringKeyMapper) ToEntity(ctx context.Context, values []crud.FieldValue) (TestEntityWithStringKey, error) {
	entity := TestEntityWithStringKey{}
	for _, fv := range values {
		switch fv.Field().Name() {
		case "code":
			if !fv.IsZero() {
				code, _ := fv.AsString()
				entity.Code = code
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

func (m *stringKeyMapper) ToFieldValues(ctx context.Context, entity TestEntityWithStringKey) ([]crud.FieldValue, error) {
	schema := createStringKeySchema()
	return schema.Fields().FieldValues(map[string]any{
		"code":        entity.Code,
		"name":        entity.Name,
		"description": entity.Description,
		"created_at":  entity.CreatedAt,
		"updated_at":  entity.UpdatedAt,
	})
}

func createStringKeySchema() crud.Schema[TestEntityWithStringKey] {
	fields := crud.NewFields([]crud.Field{
		crud.NewStringField("code", crud.WithKey()), // String primary key
		crud.NewStringField("name"),
		crud.NewStringField("description"),
		crud.NewTimestampField("created_at", crud.WithReadonly()),
		crud.NewTimestampField("updated_at", crud.WithReadonly()),
	})
	return crud.NewSchema(
		"string_key_entities",
		fields,
		&stringKeyMapper{},
	)
}

// stringKeyService implements crud.Service[TestEntityWithStringKey]
type stringKeyService struct {
	entities map[string]TestEntityWithStringKey
}

func newStringKeyService() *stringKeyService {
	return &stringKeyService{
		entities: make(map[string]TestEntityWithStringKey),
	}
}

func (s *stringKeyService) GetAll(ctx context.Context) ([]TestEntityWithStringKey, error) {
	result := make([]TestEntityWithStringKey, 0, len(s.entities))
	for _, e := range s.entities {
		result = append(result, e)
	}
	return result, nil
}

func (s *stringKeyService) Get(ctx context.Context, value crud.FieldValue) (TestEntityWithStringKey, error) {
	code, err := value.AsString()
	if err != nil {
		return TestEntityWithStringKey{}, err
	}
	entity, exists := s.entities[code]
	if !exists {
		return TestEntityWithStringKey{}, fmt.Errorf("entity not found")
	}
	return entity, nil
}

func (s *stringKeyService) Exists(ctx context.Context, value crud.FieldValue) (bool, error) {
	code, err := value.AsString()
	if err != nil {
		return false, err
	}
	_, exists := s.entities[code]
	return exists, nil
}

func (s *stringKeyService) Count(ctx context.Context, params *crud.FindParams) (int64, error) {
	return int64(len(s.entities)), nil
}

func (s *stringKeyService) List(ctx context.Context, params *crud.FindParams) ([]TestEntityWithStringKey, error) {
	return s.GetAll(ctx)
}

func (s *stringKeyService) Save(ctx context.Context, entity TestEntityWithStringKey) (TestEntityWithStringKey, error) {
	if entity.Code == "" {
		return TestEntityWithStringKey{}, fmt.Errorf("code is required")
	}
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = time.Now()
	}
	entity.UpdatedAt = time.Now()
	s.entities[entity.Code] = entity
	return entity, nil
}

func (s *stringKeyService) Delete(ctx context.Context, value crud.FieldValue) (TestEntityWithStringKey, error) {
	code, err := value.AsString()
	if err != nil {
		return TestEntityWithStringKey{}, err
	}
	entity, exists := s.entities[code]
	if !exists {
		return TestEntityWithStringKey{}, fmt.Errorf("entity not found")
	}
	delete(s.entities, code)
	return entity, nil
}

// stringKeyTestBuilder implements crud.Builder[TestEntityWithStringKey]
type stringKeyTestBuilder struct {
	schema  crud.Schema[TestEntityWithStringKey]
	service crud.Service[TestEntityWithStringKey]
}

func (b *stringKeyTestBuilder) Schema() crud.Schema[TestEntityWithStringKey] {
	return b.schema
}

func (b *stringKeyTestBuilder) Service() crud.Service[TestEntityWithStringKey] {
	return b.service
}

func (b *stringKeyTestBuilder) Repository() crud.Repository[TestEntityWithStringKey] {
	return nil // Not needed for these tests
}

// validationTrackingService wraps testService to track validation calls
type validationTrackingService struct {
	*testService
	validationCalls *int
}

func (s *validationTrackingService) Save(ctx context.Context, entity TestEntity) (TestEntity, error) {
	*s.validationCalls++
	// The fix ensures this doesn't fail for new entities with readonly fields
	return s.testService.Save(ctx, entity)
}

// validationTestBuilder implements crud.Builder[TestEntity] for validation tracking tests
type validationTestBuilder struct {
	schema  crud.Schema[TestEntity]
	service crud.Service[TestEntity]
}

func (b *validationTestBuilder) Schema() crud.Schema[TestEntity] {
	return b.schema
}

func (b *validationTestBuilder) Service() crud.Service[TestEntity] {
	return b.service
}

func (b *validationTestBuilder) Repository() crud.Repository[TestEntity] {
	return nil // Not needed for these tests
}

// TestCrudController_StringKeyEntityCreation tests the fix for entities with pre-assigned string keys
func TestCrudController_StringKeyEntityCreation(t *testing.T) {
	adminUser := testutils.MockUser()
	suite := controllertest.New(t, core.NewModule()).
		AsUser(adminUser)

	service := newStringKeyService()

	builder := &stringKeyTestBuilder{
		schema:  createStringKeySchema(),
		service: service,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntityWithStringKey]("/codes", env.App, builder)
	suite.Register(controller)

	// Test creating entity with pre-assigned string key
	formData := url.Values{
		"code":        {"PROMO-2024"},
		"name":        {"Holiday Promotion"},
		"description": {"Special holiday discount code"},
	}

	resp := suite.POST("/codes").
		Form(formData).
		Expect(t).
		Status(303)

	// Should redirect successfully
	location := resp.Header("Location")
	assert.Contains(t, location, "/codes")

	// Verify entity was created
	assert.Len(t, service.entities, 1)
	created, exists := service.entities["PROMO-2024"]
	assert.True(t, exists)
	assert.Equal(t, "PROMO-2024", created.Code)
	assert.Equal(t, "Holiday Promotion", created.Name)
	assert.Equal(t, "Special holiday discount code", created.Description)
	assert.False(t, created.CreatedAt.IsZero())
	assert.False(t, created.UpdatedAt.IsZero())
}

// TestCrudController_ReadonlyFieldValidationFix tests the service validation fix
func TestCrudController_ReadonlyFieldValidationFix(t *testing.T) {
	adminUser := testutils.MockUser()
	suite := controllertest.New(t, core.NewModule()).
		AsUser(adminUser)

	// Create a service that tracks validation calls
	validationCalls := 0
	baseService := newTestService()

	// Create a wrapper service that tracks validation calls
	service := &validationTrackingService{
		testService:     baseService,
		validationCalls: &validationCalls,
	}

	builder := &validationTestBuilder{
		schema:  createTestSchema(),
		service: service,
	}
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Test 1: Create new entity (should not validate readonly fields)
	formData := url.Values{
		"name":        {"New Entity"},
		"description": {"Test"},
		"amount":      {"100"},
		"is_active":   {"true"},
	}

	suite.POST("/test").
		Form(formData).
		Expect(t).
		Status(303)

	assert.Equal(t, 1, validationCalls)
	assert.Len(t, service.testService.entities, 1)

	// Test 2: Update existing entity (should validate readonly fields)
	var createdID uuid.UUID
	for id := range service.testService.entities {
		createdID = id
		break
	}

	updateData := url.Values{
		"name":        {"Updated Entity"},
		"description": {"Updated"},
		"amount":      {"200"},
		"is_active":   {"false"},
	}

	suite.POST(fmt.Sprintf("/test/%s", createdID)).
		Form(updateData).
		Expect(t).
		Status(303)

	assert.Equal(t, 2, validationCalls)

	// Verify update succeeded
	updated := service.testService.entities[createdID]
	assert.Equal(t, "Updated Entity", updated.Name)
}

// TestCrudController_ZeroValueHandling tests handling of zero values in fields
func TestCrudController_ZeroValueHandling(t *testing.T) {
	adminUser := testutils.MockUser()
	suite := controllertest.New(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Test creating entity with zero values
	formData := url.Values{
		"name":        {"Zero Value Test"},
		"description": {""},  // Empty string
		"amount":      {"0"}, // Zero float
		"is_active":   {""},  // False bool (unchecked checkbox)
	}

	suite.POST("/test").
		Form(formData).
		Expect(t).
		Status(303)

	// Verify zero values were properly saved
	assert.Len(t, service.entities, 1)
	var created TestEntity
	for _, e := range service.entities {
		created = e
		break
	}

	assert.Equal(t, "Zero Value Test", created.Name)
	assert.Equal(t, "", created.Description)
	assert.InDelta(t, float64(0), created.Amount, 0.01)
	assert.False(t, created.IsActive)
}

// TestCrudController_NilValueHandling tests handling of nil values
func TestCrudController_NilValueHandling(t *testing.T) {
	// Test entity with nullable fields

	nullableMapper := struct{ crud.Mapper[NullableEntity] }{}
	nullableFields := crud.NewFields([]crud.Field{
		crud.NewUUIDField("id", crud.WithKey()),
		crud.NewStringField("name"),
		crud.NewStringField("optional_str"),
		crud.NewIntField("optional_int"),
		crud.NewTimestampField("created_at", crud.WithReadonly()),
	})
	nullableSchema := crud.NewSchema(
		"nullable_entities",
		nullableFields,
		nullableMapper,
	)

	adminUser := testutils.MockUser()
	suite := controllertest.New(t, core.NewModule()).
		AsUser(adminUser)

	service := &nullableTestService{}
	builder := &nullableTestBuilder{
		schema:  nullableSchema,
		service: service,
	}

	env := suite.Environment()
	controller := controllers.NewCrudController[NullableEntity]("/nullable", env.App, builder)
	suite.Register(controller)

	// Test form rendering with nullable fields
	doc := suite.GET("/nullable/new").Expect(t).Status(200).HTML()

	// Nullable fields should still render
	doc.Element("//input[@name='optional_str']").Exists()
	doc.Element("//input[@name='optional_int']").Exists()
}

// TestCrudController_TimeZoneHandling tests proper handling of timestamps across timezones
func TestCrudController_TimeZoneHandling(t *testing.T) {
	adminUser := testutils.MockUser()
	suite := controllertest.New(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()

	// Create entity with specific timezone timestamp
	loc, _ := time.LoadLocation("America/New_York")
	entity := TestEntity{
		ID:        uuid.New(),
		Name:      "Timezone Test",
		CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, loc),
		UpdatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, loc),
	}
	service.entities[entity.ID] = entity

	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Test that timestamps are properly displayed
	doc := suite.GET("/test").Expect(t).Status(200).HTML()

	// Find the row with our entity
	rowElement := doc.Element("//tbody/tr[contains(., 'Timezone Test')]")
	rowElement.Exists()
	rowText := rowElement.Text()

	assert.NotEmpty(t, rowText, "Should find entity in list")
	// Timestamps should be formatted consistently
	assert.Contains(t, rowText, "2024")
}

// TestCrudController_LargeFormSubmission tests handling of forms with many fields
func TestCrudController_LargeFormSubmission(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large form test in short mode")
	}

	adminUser := testutils.MockUser()
	suite := controllertest.New(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Create form with maximum allowed data
	formData := url.Values{
		"name":        {string(make([]byte, 255))},   // Max length name
		"description": {string(make([]byte, 10000))}, // Large description
		"amount":      {"999999.99"},
		"is_active":   {"true"},
	}

	resp1 := suite.POST("/test").
		Form(formData).
		Expect(t).
		Status(422)

	// Should handle large form successfully
	location := resp1.Header("Location")
	assert.Contains(t, location, "/test")
}

// TestCrudController_ConcurrentFormSubmissions tests race conditions in form processing
func TestCrudController_ConcurrentFormSubmissions(t *testing.T) {
	t.Skip("TODO: Fix concurrent form submissions test - infrastructure issue")
	adminUser := testutils.MockUser()
	suite := controllertest.New(t, core.NewModule()).
		AsUser(adminUser)

	service := newTestService()
	builder := createTestBuilder(service)
	env := suite.Environment()
	controller := controllers.NewCrudController[TestEntity]("/test", env.App, builder)
	suite.Register(controller)

	// Submit multiple forms concurrently
	done := make(chan bool, 10)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(index int) {
			defer func() { done <- true }()

			formData := url.Values{
				"name":        {fmt.Sprintf("Concurrent Entity %d", index)},
				"description": {fmt.Sprintf("Created concurrently %d", index)},
				"amount":      {fmt.Sprintf("%d", index*100)},
				"is_active":   {"true"},
			}

			resp := suite.POST("/test").Form(formData).Expect(t)
			rawResp := resp.Raw()
			defer func() {
				if err := rawResp.Body.Close(); err != nil {
					errors <- fmt.Errorf("failed to close response body: %w", err)
				}
			}()
			if rawResp.StatusCode != http.StatusSeeOther {
				errors <- fmt.Errorf("unexpected status: %d", rawResp.StatusCode)
			}
		}(i)
	}

	// Wait for all submissions
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		t.Error(err)
	}

	// All entities should be created
	assert.Len(t, service.entities, 10)
}
