package controllers_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/stretchr/testify/assert"
)

func TestCrudController_TestDataStructures(t *testing.T) {
	// Test that our test data structures are properly defined

	// Create test entity
	entity := TestEntity{
		ID:          uuid.New(),
		Name:        "Test",
		Description: "Description",
		Amount:      123.45,
		IsActive:    true,
	}

	assert.NotEqual(t, uuid.Nil, entity.ID)
	assert.Equal(t, "Test", entity.Name)
	assert.Equal(t, "Description", entity.Description)
	assert.Equal(t, 123.45, entity.Amount)
	assert.True(t, entity.IsActive)
}

func TestCrudController_TestMapper(t *testing.T) {
	// Test that the mapper works correctly
	mapper := &testMapper{}

	// Test ToFieldValues
	entity := TestEntity{
		ID:          uuid.New(),
		Name:        "Test Entity",
		Description: "Test Description",
		Amount:      99.99,
		IsActive:    true,
	}

	fieldValues, err := mapper.ToFieldValues(nil, entity)
	assert.NoError(t, err)
	assert.NotEmpty(t, fieldValues)

	// Test ToEntity
	entityFromValues, err := mapper.ToEntity(nil, fieldValues)
	assert.NoError(t, err)
	assert.Equal(t, entity.ID, entityFromValues.ID)
	assert.Equal(t, entity.Name, entityFromValues.Name)
	assert.Equal(t, entity.Description, entityFromValues.Description)
	assert.Equal(t, entity.Amount, entityFromValues.Amount)
	assert.Equal(t, entity.IsActive, entityFromValues.IsActive)
}

func TestCrudController_TestService(t *testing.T) {
	// Test that the test service works correctly
	service := newTestService()

	// Test Save
	entity := TestEntity{
		Name:        "Test Entity",
		Description: "Test Description",
		Amount:      100.0,
		IsActive:    true,
	}

	saved, err := service.Save(nil, entity)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, saved.ID)
	assert.Equal(t, entity.Name, saved.Name)

	// Test Get
	idField := crud.NewUUIDField("id")
	idValue := idField.Value(saved.ID)
	retrieved, err := service.Get(nil, idValue)
	assert.NoError(t, err)
	assert.Equal(t, saved.ID, retrieved.ID)
	assert.Equal(t, saved.Name, retrieved.Name)

	// Test GetAll
	entities, err := service.GetAll(nil)
	assert.NoError(t, err)
	assert.Len(t, entities, 1)
	assert.Equal(t, saved.ID, entities[0].ID)

	// Test Count
	count, err := service.Count(nil, &crud.FindParams{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Test List
	list, err := service.List(nil, &crud.FindParams{Limit: 10})
	assert.NoError(t, err)
	assert.Len(t, list, 1)

	// Test Delete
	deleted, err := service.Delete(nil, idValue)
	assert.NoError(t, err)
	assert.Equal(t, saved.ID, deleted.ID)

	// Verify deletion
	_, err = service.Get(nil, idValue)
	assert.Error(t, err)
}

func TestCrudController_SchemaCreation(t *testing.T) {
	// Test that the schema is created correctly
	schema := createTestSchema()

	assert.Equal(t, "test_entities", schema.Name())

	fields := schema.Fields()
	assert.NotNil(t, fields)

	// Check that required fields exist
	idField, err := fields.Field("id")
	assert.NoError(t, err)
	assert.NotNil(t, idField)
	assert.True(t, idField.Key())

	nameField, err := fields.Field("name")
	assert.NoError(t, err)
	assert.NotNil(t, nameField)
	assert.True(t, nameField.Searchable())

	createdAtField, err := fields.Field("created_at")
	assert.NoError(t, err)
	assert.NotNil(t, createdAtField)
	assert.True(t, createdAtField.Readonly())

	updatedAtField, err := fields.Field("updated_at")
	assert.NoError(t, err)
	assert.NotNil(t, updatedAtField)
	assert.True(t, updatedAtField.Readonly())
}

func TestCrudController_BuilderInterface(t *testing.T) {
	// Test that the builder implements the interface correctly
	service := newTestService()
	builder := createTestBuilder(service)

	assert.NotNil(t, builder.Schema())
	assert.NotNil(t, builder.Service())
	assert.Nil(t, builder.Repository()) // Repository not needed for tests

	// Test schema name
	assert.Equal(t, "test_entities", builder.Schema().Name())

	// Test service methods are accessible
	entities, err := builder.Service().GetAll(nil)
	assert.NoError(t, err)
	assert.Empty(t, entities)
}

func TestCrudController_FieldValueConversions(t *testing.T) {
	// Test field value conversions work correctly
	schema := createTestSchema()
	fields := schema.Fields()

	// Test UUID field
	uuidField, err := fields.Field("id")
	assert.NoError(t, err)
	testUUID := uuid.New()
	uuidValue := uuidField.Value(testUUID)
	convertedUUID, err := uuidValue.AsUUID()
	assert.NoError(t, err)
	assert.Equal(t, testUUID, convertedUUID)

	// Test string field
	stringField, err := fields.Field("name")
	assert.NoError(t, err)
	stringValue := stringField.Value("test string")
	convertedString, err := stringValue.AsString()
	assert.NoError(t, err)
	assert.Equal(t, "test string", convertedString)

	// Test float field
	floatField, err := fields.Field("amount")
	assert.NoError(t, err)
	floatValue := floatField.Value(123.45)
	convertedFloat, err := floatValue.AsFloat64()
	assert.NoError(t, err)
	assert.Equal(t, 123.45, convertedFloat)

	// Test bool field
	boolField, err := fields.Field("is_active")
	assert.NoError(t, err)
	boolValue := boolField.Value(true)
	convertedBool, err := boolValue.AsBool()
	assert.NoError(t, err)
	assert.True(t, convertedBool)
}
