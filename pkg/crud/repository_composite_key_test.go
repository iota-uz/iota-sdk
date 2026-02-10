package crud_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CompositeKeyEntity represents an entity with a composite primary key
type CompositeKeyEntity interface {
	OrgID() int
	UserID() int
	Name() string
	Email() string

	SetOrgID(int) CompositeKeyEntity
	SetUserID(int) CompositeKeyEntity
	SetName(string) CompositeKeyEntity
	SetEmail(string) CompositeKeyEntity
}

type compositeKeyEntity struct {
	orgID  int
	userID int
	name   string
	email  string
}

func (e *compositeKeyEntity) OrgID() int              { return e.orgID }
func (e *compositeKeyEntity) UserID() int             { return e.userID }
func (e *compositeKeyEntity) Name() string            { return e.name }
func (e *compositeKeyEntity) Email() string           { return e.email }
func (e *compositeKeyEntity) SetOrgID(id int) CompositeKeyEntity {
	result := *e
	result.orgID = id
	return &result
}
func (e *compositeKeyEntity) SetUserID(id int) CompositeKeyEntity {
	result := *e
	result.userID = id
	return &result
}
func (e *compositeKeyEntity) SetName(name string) CompositeKeyEntity {
	result := *e
	result.name = name
	return &result
}
func (e *compositeKeyEntity) SetEmail(email string) CompositeKeyEntity {
	result := *e
	result.email = email
	return &result
}

func NewCompositeKeyEntity(orgID, userID int, name, email string) CompositeKeyEntity {
	return &compositeKeyEntity{
		orgID:  orgID,
		userID: userID,
		name:   name,
		email:  email,
	}
}

// Mapper for CompositeKeyEntity
type compositeKeyMapper struct {
	fields crud.Fields
}

func NewCompositeKeyMapper(fields crud.Fields) crud.Mapper[CompositeKeyEntity] {
	return &compositeKeyMapper{fields: fields}
}

func (m *compositeKeyMapper) ToEntities(_ context.Context, values ...[]crud.FieldValue) ([]CompositeKeyEntity, error) {
	result := make([]CompositeKeyEntity, len(values))

	for i, fvs := range values {
		var orgID, userID int
		var name, email string

		for _, v := range fvs {
			switch v.Field().Name() {
			case "organization_id":
				id, err := v.AsInt()
				if err != nil {
					return nil, fmt.Errorf("invalid organization_id field: %w", err)
				}
				orgID = id
			case "user_id":
				id, err := v.AsInt()
				if err != nil {
					return nil, fmt.Errorf("invalid user_id field: %w", err)
				}
				userID = id
			case "name":
				str, err := v.AsString()
				if err != nil {
					return nil, fmt.Errorf("invalid name field: %w", err)
				}
				name = str
			case "email":
				str, err := v.AsString()
				if err != nil {
					return nil, fmt.Errorf("invalid email field: %w", err)
				}
				email = str
			}
		}

		result[i] = NewCompositeKeyEntity(orgID, userID, name, email)
	}

	return result, nil
}

func (m *compositeKeyMapper) ToFieldValuesList(_ context.Context, entities ...CompositeKeyEntity) ([][]crud.FieldValue, error) {
	result := make([][]crud.FieldValue, len(entities))

	for i, entity := range entities {
		fvs, err := m.fields.FieldValues(map[string]any{
			"organization_id": entity.OrgID(),
			"user_id":         entity.UserID(),
			"name":            entity.Name(),
			"email":           entity.Email(),
		})
		if err != nil {
			return nil, err
		}
		result[i] = fvs
	}

	return result, nil
}

func buildCompositeKeySchema() crud.Schema[CompositeKeyEntity] {
	fields := crud.NewFields([]crud.Field{
		crud.NewIntField("organization_id", crud.WithKey()),
		crud.NewIntField("user_id", crud.WithKey()),
		crud.NewStringField("name"),
		crud.NewStringField("email"),
	})
	return crud.NewSchema(
		"composite_key_test",
		fields,
		NewCompositeKeyMapper(fields),
	)
}

type compositeKeyTestFixtures struct {
	ctx       context.Context
	schema    crud.Schema[CompositeKeyEntity]
	publisher eventbus.EventBus
}

func setupCompositeKeyTest(t *testing.T) *compositeKeyTestFixtures {
	t.Helper()
	skipUnlessDB(t)

	dm := itf.NewDatabaseManager(t)
	pool := dm.Pool()

	ctx := composables.WithPool(context.Background(), pool)

	conf := configuration.Use()
	publisher := eventbus.NewEventPublisher(conf.Logger())

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	})

	ctx = composables.WithTx(ctx, tx)

	// Create table with composite primary key
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS composite_key_test (
			organization_id INT NOT NULL,
			user_id INT NOT NULL,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			PRIMARY KEY (organization_id, user_id)
		);`
	_, err = pool.Exec(ctx, createTableSQL)
	require.NoError(t, err)

	schema := buildCompositeKeySchema()

	return &compositeKeyTestFixtures{
		ctx:       ctx,
		schema:    schema,
		publisher: publisher,
	}
}

func TestCompositeKeyRepository(t *testing.T) {
	fixture := setupCompositeKeyTest(t)
	ctx := fixture.ctx
	schema := fixture.schema
	rep := crud.DefaultRepository[CompositeKeyEntity](schema)

	t.Run("Create with composite key", func(t *testing.T) {
		entity := NewCompositeKeyEntity(1, 100, "John Doe", "john@example.com")
		fields, err := schema.Mapper().ToFieldValues(ctx, entity)
		require.NoError(t, err)

		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)
		assert.Equal(t, 1, created.OrgID())
		assert.Equal(t, 100, created.UserID())
		assert.Equal(t, "John Doe", created.Name())
	})

	t.Run("Update with composite key", func(t *testing.T) {
		// Create entity
		entity := NewCompositeKeyEntity(2, 200, "Jane Doe", "jane@example.com")
		fields, err := schema.Mapper().ToFieldValues(ctx, entity)
		require.NoError(t, err)

		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)

		// Update entity
		updated := created.SetName("Jane Smith").SetEmail("jane.smith@example.com")
		updateFields, err := schema.Mapper().ToFieldValues(ctx, updated)
		require.NoError(t, err)

		result, err := rep.Update(ctx, updateFields)
		require.NoError(t, err)
		assert.Equal(t, 2, result.OrgID())
		assert.Equal(t, 200, result.UserID())
		assert.Equal(t, "Jane Smith", result.Name())
		assert.Equal(t, "jane.smith@example.com", result.Email())
	})

	t.Run("Update fails when composite key field is missing", func(t *testing.T) {
		entity := NewCompositeKeyEntity(3, 300, "Bob", "bob@example.com")
		fields, err := schema.Mapper().ToFieldValues(ctx, entity)
		require.NoError(t, err)

		_, err = rep.Create(ctx, fields)
		require.NoError(t, err)

		// Try to update without one of the key fields
		orgIDField, err := schema.Fields().Field("organization_id")
		require.NoError(t, err)
		nameField, err := schema.Fields().Field("name")
		require.NoError(t, err)

		incompleteFields := []crud.FieldValue{
			orgIDField.Value(3),
			// Missing user_id
			nameField.Value("Bob Updated"),
		}

		_, err = rep.Update(ctx, incompleteFields)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing or zero value for primary key field")
	})

	t.Run("KeyFields returns all composite keys", func(t *testing.T) {
		keyFields := schema.Fields().KeyFields()
		assert.Len(t, keyFields, 2)
		assert.Equal(t, "organization_id", keyFields[0].Name())
		assert.Equal(t, "user_id", keyFields[1].Name())
	})

	t.Run("KeyField returns first key for backward compatibility", func(t *testing.T) {
		keyField := schema.Fields().KeyField()
		assert.Equal(t, "organization_id", keyField.Name())
	})

	t.Run("Multiple entities with same partial key", func(t *testing.T) {
		// Create multiple entities with same organization_id but different user_id
		entity1 := NewCompositeKeyEntity(4, 401, "Alice", "alice@example.com")
		fields1, err := schema.Mapper().ToFieldValues(ctx, entity1)
		require.NoError(t, err)
		_, err = rep.Create(ctx, fields1)
		require.NoError(t, err)

		entity2 := NewCompositeKeyEntity(4, 402, "Bob", "bob@example.com")
		fields2, err := schema.Mapper().ToFieldValues(ctx, entity2)
		require.NoError(t, err)
		_, err = rep.Create(ctx, fields2)
		require.NoError(t, err)

		// Both should exist independently
		all, err := rep.GetAll(ctx)
		require.NoError(t, err)

		// Filter for org_id = 4
		var org4Entities []CompositeKeyEntity
		for _, e := range all {
			if e.OrgID() == 4 {
				org4Entities = append(org4Entities, e)
			}
		}
		assert.GreaterOrEqual(t, len(org4Entities), 2)
	})

	t.Run("Get with composite key using WithKeyValues", func(t *testing.T) {
		// Create entity
		entity := NewCompositeKeyEntity(5, 500, "Charlie", "charlie@example.com")
		fields, err := schema.Mapper().ToFieldValues(ctx, entity)
		require.NoError(t, err)

		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)

		// Get by composite key
		orgIDField, err := schema.Fields().Field("organization_id")
		require.NoError(t, err)
		userIDField, err := schema.Fields().Field("user_id")
		require.NoError(t, err)

		retrieved, err := rep.Get(ctx, nil, crud.WithKeyValues(
			orgIDField.Value(5),
			userIDField.Value(500),
		))
		require.NoError(t, err)
		assert.Equal(t, created.OrgID(), retrieved.OrgID())
		assert.Equal(t, created.UserID(), retrieved.UserID())
		assert.Equal(t, "Charlie", retrieved.Name())
	})

	t.Run("Exists with composite key using WithKeyValues", func(t *testing.T) {
		// Create entity
		entity := NewCompositeKeyEntity(6, 600, "Dave", "dave@example.com")
		fields, err := schema.Mapper().ToFieldValues(ctx, entity)
		require.NoError(t, err)

		_, err = rep.Create(ctx, fields)
		require.NoError(t, err)

		// Check existence with composite key
		orgIDField, err := schema.Fields().Field("organization_id")
		require.NoError(t, err)
		userIDField, err := schema.Fields().Field("user_id")
		require.NoError(t, err)

		exists, err := rep.Exists(ctx, nil, crud.WithKeyValues(
			orgIDField.Value(6),
			userIDField.Value(600),
		))
		require.NoError(t, err)
		assert.True(t, exists)

		// Check non-existent composite key
		exists, err = rep.Exists(ctx, nil, crud.WithKeyValues(
			orgIDField.Value(6),
			userIDField.Value(999),
		))
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Delete with composite key using WithKeyValues", func(t *testing.T) {
		// Create entity
		entity := NewCompositeKeyEntity(7, 700, "Eve", "eve@example.com")
		fields, err := schema.Mapper().ToFieldValues(ctx, entity)
		require.NoError(t, err)

		created, err := rep.Create(ctx, fields)
		require.NoError(t, err)

		// Delete by composite key
		orgIDField, err := schema.Fields().Field("organization_id")
		require.NoError(t, err)
		userIDField, err := schema.Fields().Field("user_id")
		require.NoError(t, err)

		deleted, err := rep.Delete(ctx, nil, crud.WithKeyValues(
			orgIDField.Value(7),
			userIDField.Value(700),
		))
		require.NoError(t, err)
		assert.Equal(t, created.OrgID(), deleted.OrgID())
		assert.Equal(t, created.UserID(), deleted.UserID())

		// Verify it's gone
		exists, err := rep.Exists(ctx, nil, crud.WithKeyValues(
			orgIDField.Value(7),
			userIDField.Value(700),
		))
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Get with single key (backward compatibility)", func(t *testing.T) {
		// Create entity
		entity := NewCompositeKeyEntity(8, 800, "Frank", "frank@example.com")
		fields, err := schema.Mapper().ToFieldValues(ctx, entity)
		require.NoError(t, err)

		_, err = rep.Create(ctx, fields)
		require.NoError(t, err)

		// Should still work with single key field (gets first matching row)
		orgIDField, err := schema.Fields().Field("organization_id")
		require.NoError(t, err)

		retrieved, err := rep.Get(ctx, orgIDField.Value(8))
		require.NoError(t, err)
		assert.Equal(t, 8, retrieved.OrgID())
	})
}
