package crud_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/crud"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRelatedEntity is a mock entity for testing EntityField
type testRelatedEntity struct {
	ID   int
	Name string
}

func TestEntityField_Value(t *testing.T) {
	t.Parallel()

	field := crud.NewEntityField[testRelatedEntity]("related_entity")

	assert.Equal(t, "related_entity", field.Name())
	assert.Equal(t, crud.EntityFieldType, field.Type())

	entity := testRelatedEntity{ID: 1, Name: "test"}
	fv := field.Value(entity)

	assert.Equal(t, "related_entity", fv.Field().Name())
	assert.Equal(t, entity, fv.Value())
}

func TestEntityField_Properties(t *testing.T) {
	t.Parallel()

	field := crud.NewEntityField[testRelatedEntity]("related_entity")

	// EntityField should be hidden by default (not exposed in forms)
	assert.True(t, field.Hidden(), "EntityField should be hidden by default")

	// EntityField should be readonly (computed from JOINs)
	assert.True(t, field.Readonly(), "EntityField should be readonly")

	// EntityField should not be searchable
	assert.False(t, field.Searchable(), "EntityField should not be searchable")

	// EntityField should not be sortable
	assert.False(t, field.Sortable(), "EntityField should not be sortable")

	// EntityField should not be a key
	assert.False(t, field.Key(), "EntityField should not be a key")
}

func TestAsEntity_Success(t *testing.T) {
	t.Parallel()

	field := crud.NewEntityField[testRelatedEntity]("related_entity")
	entity := testRelatedEntity{ID: 42, Name: "extracted"}
	fv := field.Value(entity)

	extracted, ok := crud.AsEntity[testRelatedEntity](fv)
	require.True(t, ok, "AsEntity should return true for EntityFieldValue")

	assert.Equal(t, 42, extracted.ID)
	assert.Equal(t, "extracted", extracted.Name)
}

func TestAsEntity_WrongType(t *testing.T) {
	t.Parallel()

	field := crud.NewEntityField[testRelatedEntity]("related_entity")
	entity := testRelatedEntity{ID: 1, Name: "test"}
	fv := field.Value(entity)

	type otherEntity struct{ Value int }
	_, ok := crud.AsEntity[otherEntity](fv)
	assert.False(t, ok, "AsEntity should return false for wrong type")
}

func TestAsEntity_NonEntityFieldValue(t *testing.T) {
	t.Parallel()

	stringField := crud.NewStringField("name")
	fv := stringField.Value("test")

	_, ok := crud.AsEntity[testRelatedEntity](fv)
	assert.False(t, ok, "AsEntity should return false for non-EntityFieldValue")
}

func TestEntityField_IsZero(t *testing.T) {
	t.Parallel()

	t.Run("nil pointer entity is zero", func(t *testing.T) {
		t.Parallel()
		field := crud.NewEntityField[*testRelatedEntity]("related_entity")

		fvNil := field.Value(nil)
		assert.True(t, fvNil.IsZero(), "nil entity should be zero")
	})

	t.Run("non-nil pointer entity is not zero", func(t *testing.T) {
		t.Parallel()
		field := crud.NewEntityField[*testRelatedEntity]("related_entity")

		entity := &testRelatedEntity{ID: 1, Name: "test"}
		fvEntity := field.Value(entity)
		assert.False(t, fvEntity.IsZero(), "non-nil entity should not be zero")
	})

	t.Run("zero value struct is zero", func(t *testing.T) {
		t.Parallel()
		field := crud.NewEntityField[testRelatedEntity]("related_entity")

		fvZero := field.Value(testRelatedEntity{})
		assert.True(t, fvZero.IsZero(), "zero value struct should be zero")
	})

	t.Run("non-zero value struct is not zero", func(t *testing.T) {
		t.Parallel()
		field := crud.NewEntityField[testRelatedEntity]("related_entity")

		entity := testRelatedEntity{ID: 1, Name: "test"}
		fvEntity := field.Value(entity)
		assert.False(t, fvEntity.IsZero(), "non-zero struct should not be zero")
	})
}

func TestEntityField_TypeCasting(t *testing.T) {
	t.Parallel()

	field := crud.NewEntityField[testRelatedEntity]("related_entity")

	// All type casting should fail with ErrFieldTypeMismatch
	_, err := field.AsStringField()
	require.Error(t, err)
	require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

	_, err = field.AsIntField()
	require.Error(t, err)
	require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

	_, err = field.AsBoolField()
	require.Error(t, err)
	require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

	_, err = field.AsFloatField()
	require.Error(t, err)
	require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

	_, err = field.AsDecimalField()
	require.Error(t, err)
	require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

	_, err = field.AsDateField()
	require.Error(t, err)
	require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

	_, err = field.AsTimeField()
	require.Error(t, err)
	require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

	_, err = field.AsDateTimeField()
	require.Error(t, err)
	require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

	_, err = field.AsTimestampField()
	require.Error(t, err)
	require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)

	_, err = field.AsUUIDField()
	require.Error(t, err)
	require.ErrorIs(t, err, crud.ErrFieldTypeMismatch)
}

func TestEntityFieldValue_TypeCasting(t *testing.T) {
	t.Parallel()

	field := crud.NewEntityField[testRelatedEntity]("related_entity")
	entity := testRelatedEntity{ID: 1, Name: "test"}
	fv := field.Value(entity)

	// All FieldValue type casting should fail with ErrFieldTypeMismatch
	_, err := fv.AsString()
	require.Error(t, err)

	_, err = fv.AsInt()
	require.Error(t, err)

	_, err = fv.AsInt32()
	require.Error(t, err)

	_, err = fv.AsInt64()
	require.Error(t, err)

	_, err = fv.AsBool()
	require.Error(t, err)

	_, err = fv.AsFloat32()
	require.Error(t, err)

	_, err = fv.AsFloat64()
	require.Error(t, err)

	_, err = fv.AsDecimal()
	require.Error(t, err)

	_, err = fv.AsTime()
	require.Error(t, err)

	_, err = fv.AsUUID()
	require.Error(t, err)

	_, err = fv.AsJSON()
	require.Error(t, err)
}

func TestEntityField_InitialValue(t *testing.T) {
	t.Parallel()

	field := crud.NewEntityField[testRelatedEntity]("related_entity")

	// InitialValue should return nil for EntityField
	assert.Nil(t, field.InitialValue(nil))
}

func TestEntityField_OtherMethods(t *testing.T) {
	t.Parallel()

	field := crud.NewEntityField[testRelatedEntity]("related_entity")

	// Rules should be empty
	assert.Nil(t, field.Rules())

	// Attrs should be empty
	assert.Nil(t, field.Attrs())

	// RendererType should be empty (no rendering)
	assert.Empty(t, field.RendererType())

	// LocalizationKey should be empty
	assert.Empty(t, field.LocalizationKey())
}
