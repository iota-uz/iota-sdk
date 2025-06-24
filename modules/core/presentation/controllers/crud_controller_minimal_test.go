package controllers_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.InDelta(t, 123.45, entity.Amount, 0.01)
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

	fieldValues, err := mapper.ToFieldValues(context.TODO(), entity)
	require.NoError(t, err)
	assert.NotEmpty(t, fieldValues)

	// Test ToEntity
	entityFromValues, err := mapper.ToEntity(context.TODO(), fieldValues)
	require.NoError(t, err)
	assert.Equal(t, entity.ID, entityFromValues.ID)
	assert.Equal(t, entity.Name, entityFromValues.Name)
	assert.Equal(t, entity.Description, entityFromValues.Description)
	assert.InDelta(t, entity.Amount, entityFromValues.Amount, 0.01)
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

	saved, err := service.Save(context.TODO(), entity)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, saved.ID)
	assert.Equal(t, entity.Name, saved.Name)

	// Test Get
	idField := crud.NewUUIDField("id")
	idValue := idField.Value(saved.ID)
	retrieved, err := service.Get(context.TODO(), idValue)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, retrieved.ID)
	assert.Equal(t, saved.Name, retrieved.Name)

	// Test GetAll
	entities, err := service.GetAll(context.TODO())
	require.NoError(t, err)
	assert.Len(t, entities, 1)
	assert.Equal(t, saved.ID, entities[0].ID)

	// Test Count
	count, err := service.Count(context.TODO(), &crud.FindParams{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Test List
	list, err := service.List(context.TODO(), &crud.FindParams{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Test Delete
	deleted, err := service.Delete(context.TODO(), idValue)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, deleted.ID)

	// Verify deletion
	_, err = service.Get(context.TODO(), idValue)
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
	require.NoError(t, err)
	assert.NotNil(t, idField)
	assert.True(t, idField.Key())

	nameField, err := fields.Field("name")
	require.NoError(t, err)
	assert.NotNil(t, nameField)
	assert.True(t, nameField.Searchable())

	createdAtField, err := fields.Field("created_at")
	require.NoError(t, err)
	assert.NotNil(t, createdAtField)
	assert.True(t, createdAtField.Readonly())

	updatedAtField, err := fields.Field("updated_at")
	require.NoError(t, err)
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
	entities, err := builder.Service().GetAll(context.TODO())
	require.NoError(t, err)
	assert.Empty(t, entities)
}

func TestCrudController_FieldValueConversions(t *testing.T) {
	// Test field value conversions work correctly
	schema := createTestSchema()
	fields := schema.Fields()

	// Test UUID field
	uuidField, err := fields.Field("id")
	require.NoError(t, err)
	testUUID := uuid.New()
	uuidValue := uuidField.Value(testUUID)
	convertedUUID, err := uuidValue.AsUUID()
	require.NoError(t, err)
	assert.Equal(t, testUUID, convertedUUID)

	// Test string field
	stringField, err := fields.Field("name")
	require.NoError(t, err)
	stringValue := stringField.Value("test string")
	convertedString, err := stringValue.AsString()
	require.NoError(t, err)
	assert.Equal(t, "test string", convertedString)

	// Test float field
	floatField, err := fields.Field("amount")
	require.NoError(t, err)
	floatValue := floatField.Value(123.45)
	convertedFloat, err := floatValue.AsFloat64()
	require.NoError(t, err)
	assert.InDelta(t, 123.45, convertedFloat, 0.01)

	// Test bool field
	boolField, err := fields.Field("is_active")
	require.NoError(t, err)
	boolValue := boolField.Value(true)
	convertedBool, err := boolValue.AsBool()
	require.NoError(t, err)
	assert.True(t, convertedBool)
}
