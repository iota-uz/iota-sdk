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

		// vt__id should match "vt"
		// vt__vg__id should ALSO match "vt" (preserving nested prefix for cascading)
		vtIdField := NewIntField("vt__id")
		vtVgIdField := NewIntField("vt__vg__id")

		fvs := []FieldValue{
			vtIdField.Value(5),
			vtVgIdField.Value(10),
		}

		// Extract with "vt" prefix - gets both vt__id and vt__vg__id (nested preserved)
		vtResult := ExtractPrefixedFields(fvs, "vt")
		require.Len(t, vtResult, 2)

		// Build map for easier assertion
		resultMap := make(map[string]any)
		for _, fv := range vtResult {
			resultMap[fv.Field().Name()] = fv.Value()
		}
		assert.Equal(t, 5, resultMap["id"])
		assert.Equal(t, 10, resultMap["vg__id"]) // nested prefix preserved

		// Child mapper can extract "vg" from vtResult to get its fields
		vgResult := ExtractPrefixedFields(vtResult, "vg")
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

func TestExtractPrefixedFields_NestedPrefix(t *testing.T) {
	t.Parallel()

	// Create fields simulating a query result with nested prefixes
	// vt__id, vt__name, vt__vg__id, vt__vg__name
	vtIdField := NewIntField("vt__id")
	vtNameField := NewStringField("vt__name")
	vtVgIdField := NewIntField("vt__vg__id")
	vtVgNameField := NewStringField("vt__vg__name")

	fvs := []FieldValue{
		vtIdField.Value(1),
		vtNameField.Value("type1"),
		vtVgIdField.Value(2),
		vtVgNameField.Value("group1"),
	}

	// Extract vt__ fields - should get ALL vt__* fields including nested ones
	// This enables auto-cascading: nested prefixes are preserved for child mappers
	vtFields := ExtractPrefixedFields(fvs, "vt")

	// Should have 4 fields: id, name, vg__id, vg__name (nested preserved)
	if len(vtFields) != 4 {
		t.Fatalf("expected 4 vt fields, got %d", len(vtFields))
	}

	// Build a map of expected field names
	expectedNames := map[string]bool{
		"id":       false,
		"name":     false,
		"vg__id":   false,
		"vg__name": false,
	}

	for _, fv := range vtFields {
		name := fv.Field().Name()
		if _, ok := expectedNames[name]; !ok {
			t.Errorf("unexpected field name: %s", name)
		}
		expectedNames[name] = true
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("missing expected field: %s", name)
		}
	}

	// Now extract vg__ fields from vtFields (simulating child mapper)
	// This is how auto-cascading works: VehicleType mapper extracts vg from its fields
	vgFields := ExtractPrefixedFields(vtFields, "vg")

	if len(vgFields) != 2 {
		t.Fatalf("expected 2 vg fields, got %d", len(vgFields))
	}

	for _, fv := range vgFields {
		name := fv.Field().Name()
		if name != "id" && name != "name" {
			t.Errorf("unexpected vg field name: %s", name)
		}
	}
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

// testRelationMapper is a minimal mapper for testing that implements FlatMapper[any].
type testRelationMapper struct{}

func (m *testRelationMapper) ToEntities(_ context.Context, values ...[]FieldValue) ([]any, error) {
	return nil, nil
}

func (m *testRelationMapper) ToFieldValuesList(_ context.Context, entities ...any) ([][]FieldValue, error) {
	return nil, nil
}

func (m *testRelationMapper) ToEntity(_ context.Context, _ []FieldValue) (any, error) {
	var zero any
	return zero, nil
}

func (m *testRelationMapper) ToFieldValues(_ context.Context, _ any) ([]FieldValue, error) {
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
		relations := []RelationDescriptor{
			&Relation[any]{
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

		relations := []RelationDescriptor{
			&Relation[any]{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			&Relation[any]{
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

	t.Run("handles nested relations with Through using scoped prefixes", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name", "group_id"})
		vgSchema := createTestRelationSchema("insurance.vehicle_groups", []string{"id", "name"})

		relations := []RelationDescriptor{
			&Relation[any]{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			&Relation[any]{
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
		// vt columns use simple prefix (top-level)
		assert.Contains(t, columns, "vt.id AS vt__id")
		assert.Contains(t, columns, "vt.name AS vt__name")
		assert.Contains(t, columns, "vt.group_id AS vt__group_id")
		// vg columns use scoped prefix: vt__vg (nested through vt)
		assert.Contains(t, columns, "vg.id AS vt__vg__id")
		assert.Contains(t, columns, "vg.name AS vt__vg__name")
	})

	t.Run("skips relations with nil schema", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name"})

		relations := []RelationDescriptor{
			&Relation[any]{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			&Relation[any]{
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
		relations := []RelationDescriptor{
			&Relation[any]{
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

		relations := []RelationDescriptor{
			&Relation[any]{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			&Relation[any]{
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

		relations := []RelationDescriptor{
			&Relation[any]{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			&Relation[any]{
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

	t.Run("skips relations with no table source", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name"})

		relations := []RelationDescriptor{
			&Relation[any]{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      vtSchema,
				EntityField: "vehicle_type_entity",
			},
			&Relation[any]{
				Type:        BelongsTo,
				Alias:       "invalid",
				LocalKey:    "invalid_id",
				RemoteKey:   "id",
				JoinType:    JoinTypeLeft,
				Schema:      nil, // no schema
				Manual:      nil, // no manual config - TableName() returns ""
				EntityField: "invalid_entity",
			},
		}

		clauses := BuildRelationJoinClauses("insurance.vehicles", relations)

		// Only vt clause should be present (invalid has no table source)
		require.Len(t, clauses, 1)
		assert.Equal(t, "vt", clauses[0].TableAlias)
	})

	t.Run("defaults RemoteKey to id when empty", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name"})
		relations := []RelationDescriptor{
			&Relation[any]{
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

// testSchemaWithRelations wraps a base Schema[any] and adds RelationsProvider.
type testSchemaWithRelations struct {
	Schema[any]
	relations []RelationDescriptor
}

func (s *testSchemaWithRelations) Relations() []RelationDescriptor { return s.relations }

// createTestSchemaWithRelations creates a Schema[any] that also implements RelationsProvider.
func createTestSchemaWithRelations(tableName string, fieldNames []string, relations []RelationDescriptor) Schema[any] {
	base := createTestRelationSchema(tableName, fieldNames)
	return &testSchemaWithRelations{Schema: base, relations: relations}
}

// mutableSchemaWithRelations is a mutable schema for testing cyclic references.
// Implements Schema[any] and RelationsProvider.
type mutableSchemaWithRelations struct {
	name      string
	relations []RelationDescriptor
}

func (s *mutableSchemaWithRelations) Name() string                    { return s.name }
func (s *mutableSchemaWithRelations) Fields() Fields                  { return NewFields(nil) }
func (s *mutableSchemaWithRelations) Relations() []RelationDescriptor { return s.relations }
func (s *mutableSchemaWithRelations) Mapper() FlatMapper[any]         { return &testRelationMapper{} }
func (s *mutableSchemaWithRelations) Validators() []Validator[any]    { return nil }
func (s *mutableSchemaWithRelations) Hooks() Hooks[any]               { return &testHooks{} }

// testHooks implements Hooks[any] for testing.
type testHooks struct{}

func (h *testHooks) OnCreate() Hook[any] { return nil }
func (h *testHooks) OnUpdate() Hook[any] { return nil }
func (h *testHooks) OnDelete() Hook[any] { return nil }

func TestBuildRelationSelectColumns_NestedPrefix(t *testing.T) {
	t.Parallel()

	vtSchema := createTestRelationSchema("vehicle_types", []string{"id", "name"})
	vgSchema := createTestRelationSchema("vehicle_groups", []string{"id", "name"})

	relations := []RelationDescriptor{
		&Relation[any]{Alias: "vt", LocalKey: "vehicle_type_id", Schema: vtSchema, EntityField: "vt_entity"},
		&Relation[any]{Alias: "vg", LocalKey: "group_id", Schema: vgSchema, Through: "vt", EntityField: "vg_entity"}, // nested
	}

	columns := BuildRelationSelectColumns(relations)

	expected := []string{
		"vt.id AS vt__id",
		"vt.name AS vt__name",
		"vg.id AS vt__vg__id",
		"vg.name AS vt__vg__name",
	}

	if len(columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d: %v", len(expected), len(columns), columns)
	}

	for i, col := range columns {
		if col != expected[i] {
			t.Errorf("column[%d] = %q, want %q", i, col, expected[i])
		}
	}
}

func TestBuildRelationsRecursive(t *testing.T) {
	t.Parallel()

	t.Run("discovers nested relations from schema tree", func(t *testing.T) {
		t.Parallel()

		// Create schemas that implement RelationsProvider
		// Child schema (VehicleGroup) - no nested relations
		childSchema := createTestSchemaWithRelations("vehicle_groups", []string{"id", "name"}, nil)

		// Parent schema (VehicleType) - has VehicleGroup relation
		parentSchema := createTestSchemaWithRelations("vehicle_types", []string{"id", "name", "group_id"}, []RelationDescriptor{
			&Relation[any]{
				Alias:       "vg",
				LocalKey:    "group_id",
				Schema:      childSchema,
				EntityField: "vg_entity",
			},
		})

		// Root relations (Vehicle has VehicleType)
		rootRelations := []RelationDescriptor{
			&Relation[any]{
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				Schema:      parentSchema,
				EntityField: "vt_entity",
			},
		}

		// Discover all relations recursively
		allRelations := BuildRelationsRecursive(rootRelations)

		// Should have 2 relations: vt and vg (discovered through vt)
		if len(allRelations) != 2 {
			t.Fatalf("expected 2 relations, got %d", len(allRelations))
		}

		// First should be vt (no Through)
		if allRelations[0].GetAlias() != "vt" {
			t.Errorf("first relation alias = %q, want 'vt'", allRelations[0].GetAlias())
		}
		if allRelations[0].GetThrough() != "" {
			t.Errorf("first relation Through = %q, want ''", allRelations[0].GetThrough())
		}

		// Second should be vg (Through = "vt")
		if allRelations[1].GetAlias() != "vg" {
			t.Errorf("second relation alias = %q, want 'vg'", allRelations[1].GetAlias())
		}
		if allRelations[1].GetThrough() != "vt" {
			t.Errorf("second relation Through = %q, want 'vt'", allRelations[1].GetThrough())
		}
	})

	t.Run("handles empty relations", func(t *testing.T) {
		t.Parallel()

		result := BuildRelationsRecursive(nil)
		if result != nil {
			t.Errorf("expected nil for empty input, got %v", result)
		}

		result = BuildRelationsRecursive([]RelationDescriptor{})
		if result != nil {
			t.Errorf("expected nil for empty slice, got %v", result)
		}
	})

	t.Run("handles relations without RelationsProvider", func(t *testing.T) {
		t.Parallel()

		// Use a schema that does NOT implement RelationsProvider
		simpleSchema := createTestRelationSchema("simple_table", []string{"id", "name"})

		relations := []RelationDescriptor{
			&Relation[any]{
				Alias:       "st",
				LocalKey:    "simple_id",
				Schema:      simpleSchema,
				EntityField: "st_entity",
			},
		}

		allRelations := BuildRelationsRecursive(relations)

		// Should have only 1 relation since simpleSchema doesn't implement RelationsProvider
		if len(allRelations) != 1 {
			t.Fatalf("expected 1 relation, got %d", len(allRelations))
		}
		if allRelations[0].GetAlias() != "st" {
			t.Errorf("relation alias = %q, want 'st'", allRelations[0].GetAlias())
		}
	})

	t.Run("handles nil schema in relation", func(t *testing.T) {
		t.Parallel()

		relations := []RelationDescriptor{
			&Relation[any]{
				Alias:    "invalid",
				LocalKey: "invalid_id",
				Schema:   nil,
				Manual:   &ManualRelation{Table: "dummy", Columns: []string{"id"}},
			},
		}

		allRelations := BuildRelationsRecursive(relations)

		// Should still include the relation, just no nested discovery
		if len(allRelations) != 1 {
			t.Fatalf("expected 1 relation, got %d", len(allRelations))
		}
	})

	t.Run("prevents infinite loops in cyclic schema references", func(t *testing.T) {
		t.Parallel()

		// Create schemas with a cycle: A -> B -> A (same alias "a")
		// This tests that we don't infinitely loop when discovering relations
		// Using mutable wrapper to create cycle after initial creation
		schemaA := &mutableSchemaWithRelations{
			name:      "table_a",
			relations: nil, // Will be set after schemaB is created
		}

		schemaB := &mutableSchemaWithRelations{
			name: "table_b",
			relations: []RelationDescriptor{
				&Relation[any]{
					Alias:       "a", // Same alias as root - creates a potential infinite loop
					LocalKey:    "a_id",
					Schema:      schemaA,
					EntityField: "a_entity",
				},
			},
		}

		// Complete the cycle
		schemaA.relations = []RelationDescriptor{
			&Relation[any]{
				Alias:       "b",
				LocalKey:    "b_id",
				Schema:      schemaB,
				EntityField: "b_entity",
			},
		}

		rootRelations := []RelationDescriptor{
			&Relation[any]{
				Alias:       "a",
				LocalKey:    "a_id",
				Schema:      schemaA,
				EntityField: "a_entity",
			},
		}

		// Should not infinite loop - visited tracking prevents cycles
		// The function completes without hanging is the main test
		allRelations := BuildRelationsRecursive(rootRelations)

		// Relations discovered: a (root), b (through a), a (through b) - 3 total
		// Each path is unique: ".a", "a.b", "b.a"
		// Without cycle detection on the same schema pointer, we'd loop forever
		// With path-based detection, we get finite results
		if len(allRelations) < 1 {
			t.Fatalf("expected at least 1 relation, got %d", len(allRelations))
		}

		// Verify we got the root relation
		if allRelations[0].GetAlias() != "a" {
			t.Errorf("first relation alias = %q, want 'a'", allRelations[0].GetAlias())
		}
	})

	t.Run("discovers three-level deep relations", func(t *testing.T) {
		t.Parallel()

		// manufacturer -> vehicle_groups -> vehicle_types -> vehicles
		manufacturerSchema := createTestSchemaWithRelations("manufacturers", []string{"id", "name"}, nil) // leaf

		groupSchema := createTestSchemaWithRelations("vehicle_groups", []string{"id", "name", "manufacturer_id"}, []RelationDescriptor{
			&Relation[any]{
				Alias:       "mfr",
				LocalKey:    "manufacturer_id",
				Schema:      manufacturerSchema,
				EntityField: "mfr_entity",
			},
		})

		typeSchema := createTestSchemaWithRelations("vehicle_types", []string{"id", "name", "group_id"}, []RelationDescriptor{
			&Relation[any]{
				Alias:       "vg",
				LocalKey:    "group_id",
				Schema:      groupSchema,
				EntityField: "vg_entity",
			},
		})

		rootRelations := []RelationDescriptor{
			&Relation[any]{
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				Schema:      typeSchema,
				EntityField: "vt_entity",
			},
		}

		allRelations := BuildRelationsRecursive(rootRelations)

		// Should have 3 relations: vt, vg (through vt), mfr (through vg)
		if len(allRelations) != 3 {
			t.Fatalf("expected 3 relations, got %d", len(allRelations))
		}

		// Verify order and Through fields
		expected := []struct {
			alias   string
			through string
		}{
			{"vt", ""},
			{"vg", "vt"},
			{"mfr", "vg"},
		}

		for i, exp := range expected {
			if allRelations[i].GetAlias() != exp.alias {
				t.Errorf("relation[%d] alias = %q, want %q", i, allRelations[i].GetAlias(), exp.alias)
			}
			if allRelations[i].GetThrough() != exp.through {
				t.Errorf("relation[%d] Through = %q, want %q", i, allRelations[i].GetThrough(), exp.through)
			}
		}
	})
}

func TestBuildRelationJoinClauses_SkipsHasMany(t *testing.T) {
	t.Parallel()

	vtSchema := createTestRelationSchema("insurance.vehicle_types", []string{"id", "name"})
	docsSchema := createTestRelationSchema("insurance.person_documents", []string{"id", "seria"})

	relations := []RelationDescriptor{
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "vt",
			LocalKey:    "vehicle_type_id",
			RemoteKey:   "id",
			JoinType:    JoinTypeLeft,
			Schema:      vtSchema,
			EntityField: "vehicle_type_entity",
		},
		&Relation[any]{
			Type:        HasMany,
			Alias:       "docs",
			LocalKey:    "id",
			RemoteKey:   "person_id",
			JoinType:    JoinTypeLeft,
			Schema:      docsSchema,
			EntityField: "documents_entity",
		},
	}

	clauses := BuildRelationJoinClauses("insurance.persons", relations)

	// Should only have vt join, not docs (HasMany)
	require.Len(t, clauses, 1)
	assert.Equal(t, "vt", clauses[0].TableAlias)
}

func TestBuildHasManySubqueries_Simple(t *testing.T) {
	t.Parallel()

	docsSchema := createTestRelationSchema("insurance.person_documents", []string{"id", "seria", "number"})

	relations := []RelationDescriptor{
		&Relation[any]{
			Type:        HasMany,
			Alias:       "docs",
			LocalKey:    "id",
			RemoteKey:   "person_id",
			Schema:      docsSchema,
			EntityField: "documents_entity",
		},
	}

	subqueries := BuildHasManySubqueries("insurance.persons", "p", relations)

	require.Len(t, subqueries, 1)
	// Should contain JSON_AGG with json_build_object
	assert.Contains(t, subqueries[0], "JSON_AGG")
	assert.Contains(t, subqueries[0], "json_build_object")
	assert.Contains(t, subqueries[0], "'id', docs.id")
	assert.Contains(t, subqueries[0], "'seria', docs.seria")
	assert.Contains(t, subqueries[0], "'number', docs.number")
	assert.Contains(t, subqueries[0], "WHERE docs.person_id = p.id")
	assert.Contains(t, subqueries[0], "AS docs__json")
}

func TestTopologicalSortRelations(t *testing.T) {
	t.Parallel()

	t.Run("sorts relations with dependencies after their dependencies", func(t *testing.T) {
		t.Parallel()

		// VehicleGroup depends on VehicleType (through="vt")
		// VehicleType has no dependencies
		vtSchema := createTestRelationSchema("vehicle_types", []string{"id", "name"})
		vgSchema := createTestRelationSchema("vehicle_groups", []string{"id", "name"})
		relations := []RelationDescriptor{
			&Relation[any]{
				Alias:       "vg",
				LocalKey:    "group_id",
				EntityField: "vehicle_group_entity",
				Through:     "vt", // depends on vt
				Schema:      vgSchema,
			},
			&Relation[any]{
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				EntityField: "vehicle_type_entity",
				Through:     "", // no dependency
				Schema:      vtSchema,
			},
		}

		sorted := TopologicalSortRelations(relations)

		require.Len(t, sorted, 2)
		// vt should come first (no dependency)
		assert.Equal(t, "vt", sorted[0].GetAlias())
		// vg should come second (depends on vt)
		assert.Equal(t, "vg", sorted[1].GetAlias())
	})

	t.Run("handles multiple independent relations", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("vehicle_types", []string{"id", "name"})
		ownerSchema := createTestRelationSchema("persons", []string{"id", "name"})
		relations := []RelationDescriptor{
			&Relation[any]{
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				EntityField: "vehicle_type_entity",
				Through:     "",
				Schema:      vtSchema,
			},
			&Relation[any]{
				Alias:       "owner",
				LocalKey:    "owner_id",
				EntityField: "owner_entity",
				Through:     "",
				Schema:      ownerSchema,
			},
		}

		sorted := TopologicalSortRelations(relations)

		require.Len(t, sorted, 2)
		// Both have no dependencies, order preserved
		assert.Equal(t, "vt", sorted[0].GetAlias())
		assert.Equal(t, "owner", sorted[1].GetAlias())
	})

	t.Run("handles chain of dependencies", func(t *testing.T) {
		t.Parallel()

		// Chain: Manufacturer -> VehicleGroup -> VehicleType
		// manufacturer depends on vg, vg depends on vt, vt has no dependency
		mfrSchema := createTestRelationSchema("manufacturers", []string{"id", "name"})
		vgSchema := createTestRelationSchema("vehicle_groups", []string{"id", "name"})
		vtSchema := createTestRelationSchema("vehicle_types", []string{"id", "name"})
		relations := []RelationDescriptor{
			&Relation[any]{
				Alias:       "mfr",
				LocalKey:    "manufacturer_id",
				EntityField: "manufacturer_entity",
				Through:     "vt__vg", // depends on vg (which is accessed via vt__vg)
				Schema:      mfrSchema,
			},
			&Relation[any]{
				Alias:       "vt__vg",
				LocalKey:    "group_id",
				EntityField: "vehicle_group_entity",
				Through:     "vt", // depends on vt
				Schema:      vgSchema,
			},
			&Relation[any]{
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				EntityField: "vehicle_type_entity",
				Through:     "", // no dependency
				Schema:      vtSchema,
			},
		}

		sorted := TopologicalSortRelations(relations)

		require.Len(t, sorted, 3)
		// vt first (no dependency)
		assert.Equal(t, "vt", sorted[0].GetAlias())
		// vg second (depends on vt)
		assert.Equal(t, "vt__vg", sorted[1].GetAlias())
		// mfr third (depends on vg)
		assert.Equal(t, "mfr", sorted[2].GetAlias())
	})

	t.Run("handles empty input", func(t *testing.T) {
		t.Parallel()

		sorted := TopologicalSortRelations(nil)
		assert.Empty(t, sorted)

		sorted = TopologicalSortRelations([]RelationDescriptor{})
		assert.Empty(t, sorted)
	})

	t.Run("handles single relation", func(t *testing.T) {
		t.Parallel()

		vtSchema := createTestRelationSchema("vehicle_types", []string{"id", "name"})
		relations := []RelationDescriptor{
			&Relation[any]{
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				EntityField: "vehicle_type_entity",
				Through:     "",
				Schema:      vtSchema,
			},
		}

		sorted := TopologicalSortRelations(relations)

		require.Len(t, sorted, 1)
		assert.Equal(t, "vt", sorted[0].GetAlias())
	})
}
