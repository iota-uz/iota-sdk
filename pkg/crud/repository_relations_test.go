package crud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPrefixedFields(t *testing.T) {
	t.Parallel()

	t.Run("extracts fields with matching prefix", func(t *testing.T) {
		t.Parallel()

		idField := NewIntField("vt__id")
		nameField := NewStringField("vt__name")
		otherField := NewIntField("id")

		fvs := []FieldValue{
			idField.Value(5),
			nameField.Value("Sedan"),
			otherField.Value(1),
		}

		result := ExtractPrefixedFields(fvs, "vt")

		require.Len(t, result, 2)

		// Check that prefix is stripped
		names := make(map[string]any)
		for _, fv := range result {
			names[fv.Field().Name()] = fv.Value()
		}

		assert.Equal(t, 5, names["id"])
		assert.Equal(t, "Sedan", names["name"])
	})

	t.Run("returns empty slice when no matches", func(t *testing.T) {
		t.Parallel()

		idField := NewIntField("id")
		nameField := NewStringField("name")

		fvs := []FieldValue{
			idField.Value(1),
			nameField.Value("Car"),
		}

		result := ExtractPrefixedFields(fvs, "vt")

		assert.Empty(t, result)
	})

	t.Run("handles empty input", func(t *testing.T) {
		t.Parallel()

		result := ExtractPrefixedFields(nil, "vt")

		assert.Empty(t, result)
	})

	t.Run("handles nested prefixes correctly", func(t *testing.T) {
		t.Parallel()

		// vt__id should match "vt" but not "v"
		// vt__vg__id should match "vt__vg" but not "vt" alone
		vtIdField := NewIntField("vt__id")
		vtVgIdField := NewIntField("vt__vg__id")

		fvs := []FieldValue{
			vtIdField.Value(5),
			vtVgIdField.Value(10),
		}

		// Extract with "vt" prefix - should only get vt__id
		vtResult := ExtractPrefixedFields(fvs, "vt")
		require.Len(t, vtResult, 1)
		assert.Equal(t, "id", vtResult[0].Field().Name())
		assert.Equal(t, 5, vtResult[0].Value())

		// Extract with "vt__vg" prefix - should get vt__vg__id
		vgResult := ExtractPrefixedFields(fvs, "vt__vg")
		require.Len(t, vgResult, 1)
		assert.Equal(t, "id", vgResult[0].Field().Name())
		assert.Equal(t, 10, vgResult[0].Value())
	})

	t.Run("preserves field type and value", func(t *testing.T) {
		t.Parallel()

		boolField := NewBoolField("vt__active")
		fvs := []FieldValue{
			boolField.Value(true),
		}

		result := ExtractPrefixedFields(fvs, "vt")

		require.Len(t, result, 1)
		assert.Equal(t, "active", result[0].Field().Name())
		assert.Equal(t, true, result[0].Value())
		assert.False(t, result[0].IsZero())
	})
}

func TestExtractNonPrefixedFields(t *testing.T) {
	t.Parallel()

	t.Run("returns fields without double underscore", func(t *testing.T) {
		t.Parallel()

		idField := NewIntField("id")
		nameField := NewStringField("name")
		vtIdField := NewIntField("vt__id")
		vtNameField := NewStringField("vt__name")

		fvs := []FieldValue{
			idField.Value(1),
			nameField.Value("Car"),
			vtIdField.Value(5),
			vtNameField.Value("Sedan"),
		}

		result := ExtractNonPrefixedFields(fvs)

		require.Len(t, result, 2)

		names := make(map[string]any)
		for _, fv := range result {
			names[fv.Field().Name()] = fv.Value()
		}

		assert.Equal(t, 1, names["id"])
		assert.Equal(t, "Car", names["name"])
	})

	t.Run("handles all prefixed fields", func(t *testing.T) {
		t.Parallel()

		vtIdField := NewIntField("vt__id")
		vtNameField := NewStringField("vt__name")

		fvs := []FieldValue{
			vtIdField.Value(5),
			vtNameField.Value("Sedan"),
		}

		result := ExtractNonPrefixedFields(fvs)

		assert.Empty(t, result)
	})

	t.Run("handles empty input", func(t *testing.T) {
		t.Parallel()

		result := ExtractNonPrefixedFields(nil)

		assert.Empty(t, result)
	})

	t.Run("fields with single underscore are not prefixed", func(t *testing.T) {
		t.Parallel()

		// vehicle_type_id has underscores but no double underscore
		vehicleTypeIdField := NewIntField("vehicle_type_id")
		createdAtField := NewTimestampField("created_at")

		fvs := []FieldValue{
			vehicleTypeIdField.Value(5),
			createdAtField.Value(nil),
		}

		result := ExtractNonPrefixedFields(fvs)

		require.Len(t, result, 2)
		assert.Equal(t, "vehicle_type_id", result[0].Field().Name())
		assert.Equal(t, "created_at", result[1].Field().Name())
	})
}

func TestAllFieldsNull(t *testing.T) {
	t.Parallel()

	t.Run("returns true when all values are nil", func(t *testing.T) {
		t.Parallel()

		idField := NewIntField("id")
		nameField := NewStringField("name")

		fvs := []FieldValue{
			idField.Value(nil),
			nameField.Value(nil),
		}

		assert.True(t, AllFieldsNull(fvs))
	})

	t.Run("returns true when all values are zero", func(t *testing.T) {
		t.Parallel()

		idField := NewIntField("id")
		nameField := NewStringField("name")
		activeField := NewBoolField("active")

		fvs := []FieldValue{
			idField.Value(0),
			nameField.Value(""),
			activeField.Value(false),
		}

		assert.True(t, AllFieldsNull(fvs))
	})

	t.Run("returns false when any value is non-nil and non-zero", func(t *testing.T) {
		t.Parallel()

		idField := NewIntField("id")
		nameField := NewStringField("name")

		fvs := []FieldValue{
			idField.Value(nil),
			nameField.Value("Sedan"),
		}

		assert.False(t, AllFieldsNull(fvs))
	})

	t.Run("returns true for empty slice", func(t *testing.T) {
		t.Parallel()

		assert.True(t, AllFieldsNull(nil))
		assert.True(t, AllFieldsNull([]FieldValue{}))
	})

	t.Run("returns false for non-zero int", func(t *testing.T) {
		t.Parallel()

		idField := NewIntField("id")
		fvs := []FieldValue{
			idField.Value(1),
		}

		assert.False(t, AllFieldsNull(fvs))
	})

	t.Run("returns false for non-empty string", func(t *testing.T) {
		t.Parallel()

		nameField := NewStringField("name")
		fvs := []FieldValue{
			nameField.Value("test"),
		}

		assert.False(t, AllFieldsNull(fvs))
	})

	t.Run("returns false for true bool", func(t *testing.T) {
		t.Parallel()

		activeField := NewBoolField("active")
		fvs := []FieldValue{
			activeField.Value(true),
		}

		assert.False(t, AllFieldsNull(fvs))
	})
}

// Helper to create test schemas for relation tests
func createTestRelationSchema(tableName string, fieldNames []string) Schema[any] {
	fieldList := make([]Field, len(fieldNames))
	for i, name := range fieldNames {
		if i == 0 {
			fieldList[i] = NewIntField(name, WithKey())
		} else {
			fieldList[i] = NewStringField(name)
		}
	}
	fields := NewFields(fieldList)

	return NewSchema(
		tableName,
		fields,
		&testRelationMapper{},
	)
}

// testRelationMapper is a minimal mapper for testing
type testRelationMapper struct{}

func (m *testRelationMapper) ToEntities(_ context.Context, values ...[]FieldValue) ([]any, error) {
	return nil, nil
}

func (m *testRelationMapper) ToFieldValuesList(_ context.Context, entities ...any) ([][]FieldValue, error) {
	return nil, nil
}

func TestBuildRelationSelectColumns(t *testing.T) {
	t.Parallel()

	t.Run("returns empty slice for empty relations", func(t *testing.T) {
		t.Parallel()

		columns := BuildRelationSelectColumns(nil)

		assert.Empty(t, columns)
	})

	t.Run("builds prefixed columns for single relation", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name"})
		relations := []Relation{
			{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
		}

		columns := BuildRelationSelectColumns(relations)

		require.Len(t, columns, 2)
		assert.Contains(t, columns, "vt.id AS vt__id")
		assert.Contains(t, columns, "vt.name AS vt__name")
	})

	t.Run("builds prefixed columns for multiple relations", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name"})
		ownerSchema := createTestRelationSchema("insurance.persons", []string{"id", "first_name"})

		relations := []Relation{
			{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			{
				Type:        BelongsTo,
				Alias:       "owner",
				LocalKey:    "owner_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      ownerSchema,
				EntityField: "owner_entity",
			},
		}

		columns := BuildRelationSelectColumns(relations)

		require.Len(t, columns, 4)
		assert.Contains(t, columns, "vt.id AS vt__id")
		assert.Contains(t, columns, "vt.name AS vt__name")
		assert.Contains(t, columns, "owner.id AS owner__id")
		assert.Contains(t, columns, "owner.first_name AS owner__first_name")
	})

	t.Run("handles nested relations with Through", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name", "group_id"})
		vgSchema := createTestRelationSchema("insurance.vehicle_groups", []string{"id", "name"})

		relations := []Relation{
			{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			{
				Type:        BelongsTo,
				Alias:       "vg",
				LocalKey:    "group_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vgSchema,
				EntityField: "vehicle_group_entity",
				Through:     "vt",
			},
		}

		columns := BuildRelationSelectColumns(relations)

		// vt has 3 fields, vg has 2 fields = 5 total
		require.Len(t, columns, 5)
		// vt columns use simple prefix
		assert.Contains(t, columns, "vt.id AS vt__id")
		assert.Contains(t, columns, "vt.name AS vt__name")
		assert.Contains(t, columns, "vt.group_id AS vt__group_id")
		// vg columns use vg prefix (not vt__vg)
		assert.Contains(t, columns, "vg.id AS vg__id")
		assert.Contains(t, columns, "vg.name AS vg__name")
	})

	t.Run("skips relations with nil schema", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name"})

		relations := []Relation{
			{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			{
				Type:        BelongsTo,
				Alias:       "invalid",
				LocalKey:    "invalid_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      nil, // nil schema
				EntityField: "invalid_entity",
			},
		}

		columns := BuildRelationSelectColumns(relations)

		// Only vt columns should be present
		require.Len(t, columns, 2)
		assert.Contains(t, columns, "vt.id AS vt__id")
		assert.Contains(t, columns, "vt.name AS vt__name")
	})
}

func TestBuildRelationJoinClauses(t *testing.T) {
	t.Parallel()

	t.Run("returns empty slice for empty relations", func(t *testing.T) {
		t.Parallel()

		clauses := BuildRelationJoinClauses("insurance.vehicles", nil)

		assert.Empty(t, clauses)
	})

	t.Run("builds join clause for single relation", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name"})
		relations := []Relation{
			{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
		}

		clauses := BuildRelationJoinClauses("insurance.vehicles", relations)

		require.Len(t, clauses, 1)
		assert.Equal(t, "insurance.vehicle_types", clauses[0].Table)
		assert.Equal(t, "vt", clauses[0].TableAlias)
		assert.Equal(t, "insurance.vehicles.vehicle_type_id", clauses[0].LeftColumn)
		assert.Equal(t, "vt.id", clauses[0].RightColumn)
		assert.Equal(t, JoinTypeLeft, clauses[0].Type)
	})

	t.Run("builds join clauses for multiple independent relations", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name"})
		ownerSchema := createTestRelationSchema("insurance.persons", []string{"id", "first_name"})

		relations := []Relation{
			{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			{
				Type:        BelongsTo,
				Alias:       "owner",
				LocalKey:    "owner_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeInner,
				Schema:      ownerSchema,
				EntityField: "owner_entity",
			},
		}

		clauses := BuildRelationJoinClauses("insurance.vehicles", relations)

		require.Len(t, clauses, 2)

		// First join (vt)
		assert.Equal(t, "insurance.vehicle_types", clauses[0].Table)
		assert.Equal(t, "vt", clauses[0].TableAlias)
		assert.Equal(t, "insurance.vehicles.vehicle_type_id", clauses[0].LeftColumn)
		assert.Equal(t, "vt.id", clauses[0].RightColumn)
		assert.Equal(t, JoinTypeLeft, clauses[0].Type)

		// Second join (owner)
		assert.Equal(t, "insurance.persons", clauses[1].Table)
		assert.Equal(t, "owner", clauses[1].TableAlias)
		assert.Equal(t, "insurance.vehicles.owner_id", clauses[1].LeftColumn)
		assert.Equal(t, "owner.id", clauses[1].RightColumn)
		assert.Equal(t, JoinTypeInner, clauses[1].Type)
	})

	t.Run("builds join clauses for nested relations with Through", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name", "group_id"})
		vgSchema := createTestRelationSchema("insurance.vehicle_groups", []string{"id", "name"})

		relations := []Relation{
			{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			{
				Type:        BelongsTo,
				Alias:       "vg",
				LocalKey:    "group_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vgSchema,
				EntityField: "vehicle_group_entity",
				Through:     "vt",
			},
		}

		clauses := BuildRelationJoinClauses("insurance.vehicles", relations)

		require.Len(t, clauses, 2)

		// First join (vt) - from main table
		assert.Equal(t, "insurance.vehicle_types", clauses[0].Table)
		assert.Equal(t, "vt", clauses[0].TableAlias)
		assert.Equal(t, "insurance.vehicles.vehicle_type_id", clauses[0].LeftColumn)
		assert.Equal(t, "vt.id", clauses[0].RightColumn)

		// Second join (vg) - from vt (Through)
		assert.Equal(t, "insurance.vehicle_groups", clauses[1].Table)
		assert.Equal(t, "vg", clauses[1].TableAlias)
		assert.Equal(t, "vt.group_id", clauses[1].LeftColumn) // Uses vt alias, not main table
		assert.Equal(t, "vg.id", clauses[1].RightColumn)
	})

	t.Run("skips relations with nil schema", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name"})

		relations := []Relation{
			{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			{
				Type:        BelongsTo,
				Alias:       "invalid",
				LocalKey:    "invalid_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      nil, // nil schema
				EntityField: "invalid_entity",
			},
		}

		clauses := BuildRelationJoinClauses("insurance.vehicles", relations)

		// Only vt clause should be present
		require.Len(t, clauses, 1)
		assert.Equal(t, "vt", clauses[0].TableAlias)
	})

	t.Run("defaults RemoteKey to id when empty", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name"})
		relations := []Relation{
			{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "", // Empty, should default to "id"
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
		}

		clauses := BuildRelationJoinClauses("insurance.vehicles", relations)

		require.Len(t, clauses, 1)
		assert.Equal(t, "vt.id", clauses[0].RightColumn)
	})
}

func TestTopologicalSortRelations(t *testing.T) {
	t.Parallel()

	t.Run("sorts relations with dependencies after their dependencies", func(t *testing.T) {
		t.Parallel()

		// VehicleGroup depends on VehicleType (through="vt")
		// VehicleType has no dependencies
		relations := []Relation{
			{
				Alias:       "vg",
				LocalKey:    "group_id",
				EntityField: "vehicle_group_entity",
				Through:     "vt", // depends on vt
			},
			{
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				EntityField: "vehicle_type_entity",
				Through:     "", // no dependency
			},
		}

		sorted := TopologicalSortRelations(relations)

		require.Len(t, sorted, 2)
		// vt should come first (no dependency)
		assert.Equal(t, "vt", sorted[0].Alias)
		// vg should come second (depends on vt)
		assert.Equal(t, "vg", sorted[1].Alias)
	})

	t.Run("handles multiple independent relations", func(t *testing.T) {
		t.Parallel()

		relations := []Relation{
			{
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				EntityField: "vehicle_type_entity",
				Through:     "",
			},
			{
				Alias:       "owner",
				LocalKey:    "owner_id",
				EntityField: "owner_entity",
				Through:     "",
			},
		}

		sorted := TopologicalSortRelations(relations)

		require.Len(t, sorted, 2)
		// Both have no dependencies, order preserved
		assert.Equal(t, "vt", sorted[0].Alias)
		assert.Equal(t, "owner", sorted[1].Alias)
	})

	t.Run("handles chain of dependencies", func(t *testing.T) {
		t.Parallel()

		// Chain: Manufacturer -> VehicleGroup -> VehicleType
		// manufacturer depends on vg, vg depends on vt, vt has no dependency
		relations := []Relation{
			{
				Alias:       "mfr",
				LocalKey:    "manufacturer_id",
				EntityField: "manufacturer_entity",
				Through:     "vt__vg", // depends on vg (which is accessed via vt__vg)
			},
			{
				Alias:       "vt__vg",
				LocalKey:    "group_id",
				EntityField: "vehicle_group_entity",
				Through:     "vt", // depends on vt
			},
			{
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				EntityField: "vehicle_type_entity",
				Through:     "", // no dependency
			},
		}

		sorted := TopologicalSortRelations(relations)

		require.Len(t, sorted, 3)
		// vt first (no dependency)
		assert.Equal(t, "vt", sorted[0].Alias)
		// vg second (depends on vt)
		assert.Equal(t, "vt__vg", sorted[1].Alias)
		// mfr third (depends on vg)
		assert.Equal(t, "mfr", sorted[2].Alias)
	})

	t.Run("handles empty input", func(t *testing.T) {
		t.Parallel()

		sorted := TopologicalSortRelations(nil)
		assert.Empty(t, sorted)

		sorted = TopologicalSortRelations([]Relation{})
		assert.Empty(t, sorted)
	})

	t.Run("handles single relation", func(t *testing.T) {
		t.Parallel()

		relations := []Relation{
			{
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				EntityField: "vehicle_type_entity",
				Through:     "",
			},
		}

		sorted := TopologicalSortRelations(relations)

		require.Len(t, sorted, 1)
		assert.Equal(t, "vt", sorted[0].Alias)
	})
}
