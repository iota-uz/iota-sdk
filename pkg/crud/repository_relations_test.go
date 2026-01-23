package crud

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

		columns := BuildRelationSelectColumns("", nil)

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

		columns := BuildRelationSelectColumns("main", relations)

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

		columns := BuildRelationSelectColumns("main", relations)

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

		columns := BuildRelationSelectColumns("main", relations)

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

		columns := BuildRelationSelectColumns("main", relations)

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

	columns := BuildRelationSelectColumns("main", relations)

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

func TestBuildRelationSelectColumns_ThreeLevelDeepNestedPrefix(t *testing.T) {
	t.Parallel()

	// Testing: Vehicle -> Person -> District -> Region
	// This matches the real-world case of loading vehicle with owner's district's region
	regionSchema := createTestRelationSchema("regions", []string{"id", "name"})
	districtSchema := createTestRelationSchema("districts", []string{"id", "name", "region_id"})
	personSchema := createTestRelationSchema("persons", []string{"id", "first_name", "district_id"})

	relations := []RelationDescriptor{
		&Relation[any]{Alias: "p", LocalKey: "owner_person_id", Schema: personSchema, EntityField: "owner_person_entity"},
		&Relation[any]{Alias: "d", LocalKey: "district_id", Schema: districtSchema, Through: "p", EntityField: "district_entity"},
		&Relation[any]{Alias: "dr", LocalKey: "region_id", Schema: regionSchema, Through: "d", EntityField: "region_entity"}, // 3 levels deep
	}

	columns := BuildRelationSelectColumns("main", relations)

	// The region columns should have the FULL ancestor chain prefix: p__d__dr
	// Not just the immediate through: d__dr
	expected := []string{
		// Person (top level)
		"p.id AS p__id",
		"p.first_name AS p__first_name",
		"p.district_id AS p__district_id",
		// District (nested under person)
		"d.id AS p__d__id",
		"d.name AS p__d__name",
		"d.region_id AS p__d__region_id",
		// Region (nested under district, which is under person)
		"dr.id AS p__d__dr__id",
		"dr.name AS p__d__dr__name",
	}

	if len(columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d:\n  got: %v\n  want: %v", len(expected), len(columns), columns, expected)
	}

	for i, col := range columns {
		if col != expected[i] {
			t.Errorf("column[%d] = %q, want %q", i, col, expected[i])
		}
	}
}

func TestBuildRelationSelectColumns_HandlesHasManyAsSubquery(t *testing.T) {
	t.Parallel()

	vtSchema := createTestRelationSchema("vehicle_types", []string{"id", "name"})
	docsSchema := createTestRelationSchema("person_documents", []string{"id", "seria"})

	relations := []RelationDescriptor{
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "vt",
			LocalKey:    "vehicle_type_id",
			Schema:      vtSchema,
			EntityField: "vt_entity",
		},
		&Relation[any]{
			Type:        HasMany,
			Alias:       "docs",
			LocalKey:    "id",
			RemoteKey:   "person_id",
			Schema:      docsSchema,
			EntityField: "docs_entity",
		},
	}

	columns := BuildRelationSelectColumns("main", relations)

	// Should have vt columns (2) + docs subquery (1)
	require.Len(t, columns, 3)
	assert.Contains(t, columns, "vt.id AS vt__id")
	assert.Contains(t, columns, "vt.name AS vt__name")

	// HasMany should be included as JSON subquery, not as regular columns
	var foundDocsSubquery bool
	for _, col := range columns {
		if strings.Contains(col, "docs__json") && strings.Contains(col, "JSON_AGG") {
			foundDocsSubquery = true
			// Should correlate with parent using main.id = docs.person_id
			assert.Contains(t, col, "main.id")
			assert.Contains(t, col, "docs.person_id")
		}
	}
	assert.True(t, foundDocsSubquery, "HasMany should generate JSON subquery")
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

func TestBuildHasManySubqueries_EmptyParameters(t *testing.T) {
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

	t.Run("empty mainTable returns nil", func(t *testing.T) {
		t.Parallel()

		result := BuildHasManySubqueries("", "p", relations)
		assert.Nil(t, result)
	})

	t.Run("empty mainAlias returns nil", func(t *testing.T) {
		t.Parallel()

		result := BuildHasManySubqueries("insurance.persons", "", relations)
		assert.Nil(t, result)
	})

	t.Run("both empty returns nil", func(t *testing.T) {
		t.Parallel()

		result := BuildHasManySubqueries("", "", relations)
		assert.Nil(t, result)
	})
}

func TestBuildHasManySubqueries_MixedRelationTypes(t *testing.T) {
	t.Parallel()

	docsSchema := createTestRelationSchema("insurance.person_documents", []string{"id", "seria", "number"})
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
			Type:        HasMany,
			Alias:       "docs",
			LocalKey:    "id",
			RemoteKey:   "person_id",
			Schema:      docsSchema,
			EntityField: "documents_entity",
		},
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "owner",
			LocalKey:    "owner_id",
			RemoteKey:   "id",
			JoinType:    JoinTypeLeft,
			Schema:      vtSchema,
			EntityField: "owner_entity",
		},
	}

	subqueries := BuildHasManySubqueries("insurance.persons", "p", relations)

	// Should only include HasMany relation (docs), not BelongsTo relations (vt, owner)
	require.Len(t, subqueries, 1)
	assert.Contains(t, subqueries[0], "docs__json")
	assert.NotContains(t, subqueries[0], "vt")
	assert.NotContains(t, subqueries[0], "owner")
}

func TestBuildHasManySubqueries_WithNestedBelongsTo(t *testing.T) {
	t.Parallel()

	// Document has DocumentAuthority (BelongsTo)
	authoritySchema := createTestRelationSchema("insurance.document_authorities", []string{"id", "name"})
	docsSchema := createTestSchemaWithRelations("insurance.person_documents", []string{"id", "seria", "authority_id"}, []RelationDescriptor{
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "da",
			LocalKey:    "authority_id",
			RemoteKey:   "id",
			Schema:      authoritySchema,
			EntityField: "authority_entity",
		},
	})

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
	// Should contain nested json_build_object for authority
	assert.Contains(t, subqueries[0], "'da', json_build_object('id', docs_da.id, 'name', docs_da.name)")
	// Should contain LEFT JOIN for nested BelongsTo
	assert.Contains(t, subqueries[0], "LEFT JOIN insurance.document_authorities docs_da ON docs.authority_id = docs_da.id")
}

func TestBuildHasManySubqueries_NestedHasMany(t *testing.T) {
	t.Parallel()

	// Authority has Regions (HasMany)
	regionsSchema := createTestRelationSchema("insurance.authority_regions", []string{"id", "name"})
	authoritySchema := createTestSchemaWithRelations("insurance.document_authorities", []string{"id", "name"}, []RelationDescriptor{
		&Relation[any]{
			Type:        HasMany,
			Alias:       "regions",
			LocalKey:    "id",
			RemoteKey:   "authority_id",
			Schema:      regionsSchema,
			EntityField: "regions_entity",
		},
	})

	// Document has Authority (BelongsTo)
	docsSchema := createTestSchemaWithRelations("insurance.person_documents", []string{"id", "seria"}, []RelationDescriptor{
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "da",
			LocalKey:    "authority_id",
			RemoteKey:   "id",
			Schema:      authoritySchema,
			EntityField: "authority_entity",
		},
	})

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
	// Should contain nested subquery for regions inside authority
	assert.Contains(t, subqueries[0], "'regions', (SELECT COALESCE(JSON_AGG")
	assert.Contains(t, subqueries[0], "FROM insurance.authority_regions")
	assert.Contains(t, subqueries[0], "WHERE docs_da_regions.authority_id = docs_da.id")
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

func TestBuildRelations_ComplexScenario(t *testing.T) {
	t.Parallel()

	// === BelongsTo schemas (will create JOINs) ===
	genderSchema := createTestRelationSchema("insurance.genders", []string{"id", "name"})
	countrySchema := createTestRelationSchema("insurance.countries", []string{"id", "name", "code"})
	regionSchema := createTestRelationSchema("insurance.regions", []string{"id", "name"})

	// === HasMany #1: Documents with nested BelongsTo (DocumentAuthority) ===
	authoritySchema := createTestRelationSchema("insurance.document_authorities", []string{"id", "name", "code"})
	documentsSchema := createTestSchemaWithRelations(
		"insurance.person_documents",
		[]string{"id", "seria", "number", "authority_id"},
		[]RelationDescriptor{
			&Relation[any]{
				Type:        BelongsTo,
				Alias:       "da",
				LocalKey:    "authority_id",
				RemoteKey:   "id",
				Schema:      authoritySchema,
				EntityField: "authority_entity",
			},
		},
	)

	// === HasMany #2: PINFLs (simple, no nested relations) ===
	pinflsSchema := createTestRelationSchema("insurance.person_pinfls", []string{"id", "value", "status"})

	// === All relations ===
	relations := []RelationDescriptor{
		// 3 BelongsTo relations
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "g",
			LocalKey:    "gender_id",
			RemoteKey:   "id",
			JoinType:    JoinTypeLeft,
			Schema:      genderSchema,
			EntityField: "gender_entity",
		},
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "c",
			LocalKey:    "country_id",
			RemoteKey:   "id",
			JoinType:    JoinTypeLeft,
			Schema:      countrySchema,
			EntityField: "country_entity",
		},
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "r",
			LocalKey:    "region_id",
			RemoteKey:   "id",
			JoinType:    JoinTypeLeft,
			Schema:      regionSchema,
			EntityField: "region_entity",
		},
		// 2 HasMany relations
		&Relation[any]{
			Type:        HasMany,
			Alias:       "docs",
			LocalKey:    "id",
			RemoteKey:   "person_id",
			Schema:      documentsSchema,
			EntityField: "documents_entity",
		},
		&Relation[any]{
			Type:        HasMany,
			Alias:       "pinfls",
			LocalKey:    "id",
			RemoteKey:   "person_id",
			Schema:      pinflsSchema,
			EntityField: "pinfls_entity",
		},
	}

	// === Test BuildRelationJoinClauses ===
	t.Run("JoinClauses", func(t *testing.T) {
		t.Parallel()

		joinClauses := BuildRelationJoinClauses("insurance.persons", relations)

		// Should only have 3 JOINs (BelongsTo), not HasMany
		require.Len(t, joinClauses, 3, "expected 3 JOIN clauses for BelongsTo relations")

		// Verify each BelongsTo generates a JOIN
		aliases := make([]string, len(joinClauses))
		for i, jc := range joinClauses {
			aliases[i] = jc.TableAlias
		}
		assert.Contains(t, aliases, "g", "gender join missing")
		assert.Contains(t, aliases, "c", "country join missing")
		assert.Contains(t, aliases, "r", "region join missing")

		// Verify HasMany aliases are NOT in joins
		assert.NotContains(t, aliases, "docs", "docs should not be in JOINs")
		assert.NotContains(t, aliases, "pinfls", "pinfls should not be in JOINs")
	})

	// === Test BuildRelationSelectColumns ===
	t.Run("SelectColumns", func(t *testing.T) {
		t.Parallel()

		selectCols := BuildRelationSelectColumns("p", relations)

		// Should have columns for BelongsTo: gender(2) + country(3) + region(2) = 7
		// Plus HasMany subqueries: docs(1) + pinfls(1) = 2
		// Total: 9
		require.Len(t, selectCols, 9, "expected 9 SELECT columns (7 BelongsTo + 2 HasMany subqueries)")

		// Verify BelongsTo columns exist
		assert.Contains(t, selectCols, "g.id AS g__id")
		assert.Contains(t, selectCols, "g.name AS g__name")
		assert.Contains(t, selectCols, "c.id AS c__id")
		assert.Contains(t, selectCols, "c.name AS c__name")
		assert.Contains(t, selectCols, "c.code AS c__code")
		assert.Contains(t, selectCols, "r.id AS r__id")
		assert.Contains(t, selectCols, "r.name AS r__name")

		// Verify HasMany subqueries are present
		var foundDocsSubquery, foundPinflsSubquery bool
		for _, col := range selectCols {
			if strings.Contains(col, "docs__json") && strings.Contains(col, "JSON_AGG") {
				foundDocsSubquery = true
				assert.Contains(t, col, "p.id", "docs subquery should correlate with parent alias p")
			}
			if strings.Contains(col, "pinfls__json") && strings.Contains(col, "JSON_AGG") {
				foundPinflsSubquery = true
				assert.Contains(t, col, "p.id", "pinfls subquery should correlate with parent alias p")
			}
		}
		assert.True(t, foundDocsSubquery, "docs HasMany subquery should be present")
		assert.True(t, foundPinflsSubquery, "pinfls HasMany subquery should be present")
	})

	// === Test BuildHasManySubqueries ===
	t.Run("HasManySubqueries", func(t *testing.T) {
		t.Parallel()

		subqueries := BuildHasManySubqueries("insurance.persons", "p", relations)

		// Should have 2 subqueries (one per HasMany)
		require.Len(t, subqueries, 2, "expected 2 subqueries for HasMany relations")

		// Find docs and pinfls subqueries
		var docsSubquery, pinflsSubquery string
		for _, sq := range subqueries {
			if strings.Contains(sq, "docs__json") {
				docsSubquery = sq
			}
			if strings.Contains(sq, "pinfls__json") {
				pinflsSubquery = sq
			}
		}

		require.NotEmpty(t, docsSubquery, "docs subquery not found")
		require.NotEmpty(t, pinflsSubquery, "pinfls subquery not found")

		// === Verify docs subquery (has nested BelongsTo) ===
		// Should have JSON_AGG
		assert.Contains(t, docsSubquery, "JSON_AGG")
		assert.Contains(t, docsSubquery, "json_build_object")

		// Should have own fields
		assert.Contains(t, docsSubquery, "'id', docs.id")
		assert.Contains(t, docsSubquery, "'seria', docs.seria")
		assert.Contains(t, docsSubquery, "'number', docs.number")

		// Should have nested BelongsTo (DocumentAuthority) as json_build_object
		assert.Contains(t, docsSubquery, "'da', json_build_object('id', docs_da.id, 'name', docs_da.name, 'code', docs_da.code)")

		// Should have LEFT JOIN inside subquery for nested BelongsTo
		assert.Contains(t, docsSubquery, "LEFT JOIN insurance.document_authorities docs_da ON docs.authority_id = docs_da.id")

		// Should have WHERE clause linking to parent
		assert.Contains(t, docsSubquery, "WHERE docs.person_id = p.id")

		// === Verify pinfls subquery (simple, no nested) ===
		assert.Contains(t, pinflsSubquery, "JSON_AGG")
		assert.Contains(t, pinflsSubquery, "json_build_object")
		assert.Contains(t, pinflsSubquery, "'id', pinfls.id")
		assert.Contains(t, pinflsSubquery, "'value', pinfls.value")
		assert.Contains(t, pinflsSubquery, "'status', pinfls.status")
		assert.Contains(t, pinflsSubquery, "WHERE pinfls.person_id = p.id")

		// pinfls should NOT have any JOINs inside
		assert.NotContains(t, pinflsSubquery, "LEFT JOIN")
	})
}

func TestBuildRelations_FullSQLQuery(t *testing.T) {
	t.Parallel()

	// === Setup same schemas as complex scenario ===
	genderSchema := createTestRelationSchema("insurance.genders", []string{"id", "name"})
	countrySchema := createTestRelationSchema("insurance.countries", []string{"id", "name", "code"})
	regionSchema := createTestRelationSchema("insurance.regions", []string{"id", "name"})

	authoritySchema := createTestRelationSchema("insurance.document_authorities", []string{"id", "name", "code"})
	documentsSchema := createTestSchemaWithRelations(
		"insurance.person_documents",
		[]string{"id", "seria", "number", "authority_id"},
		[]RelationDescriptor{
			&Relation[any]{
				Type:        BelongsTo,
				Alias:       "da",
				LocalKey:    "authority_id",
				RemoteKey:   "id",
				Schema:      authoritySchema,
				EntityField: "authority_entity",
			},
		},
	)

	pinflsSchema := createTestRelationSchema("insurance.person_pinfls", []string{"id", "value", "status"})

	relations := []RelationDescriptor{
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "g",
			LocalKey:    "gender_id",
			RemoteKey:   "id",
			JoinType:    JoinTypeLeft,
			Schema:      genderSchema,
			EntityField: "gender_entity",
		},
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "c",
			LocalKey:    "country_id",
			RemoteKey:   "id",
			JoinType:    JoinTypeLeft,
			Schema:      countrySchema,
			EntityField: "country_entity",
		},
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "r",
			LocalKey:    "region_id",
			RemoteKey:   "id",
			JoinType:    JoinTypeLeft,
			Schema:      regionSchema,
			EntityField: "region_entity",
		},
		&Relation[any]{
			Type:        HasMany,
			Alias:       "docs",
			LocalKey:    "id",
			RemoteKey:   "person_id",
			Schema:      documentsSchema,
			EntityField: "documents_entity",
		},
		&Relation[any]{
			Type:        HasMany,
			Alias:       "pinfls",
			LocalKey:    "id",
			RemoteKey:   "person_id",
			Schema:      pinflsSchema,
			EntityField: "pinfls_entity",
		},
	}

	mainTable := "insurance.persons"
	mainAlias := "p"

	// === Build all components ===
	// BuildRelationSelectColumns handles both BelongsTo (SELECT columns) and HasMany (JSON subqueries)
	selectCols := BuildRelationSelectColumns(mainAlias, relations)
	joinClauses := BuildRelationJoinClauses(mainTable, relations)

	// === Construct full SELECT clause ===
	allSelectParts := []string{mainTable + ".*"}
	allSelectParts = append(allSelectParts, selectCols...)
	selectClause := strings.Join(allSelectParts, ", ")

	// === Construct full JOIN clause ===
	joinParts := make([]string, 0, len(joinClauses))
	for _, jc := range joinClauses {
		joinParts = append(joinParts, fmt.Sprintf("%s %s %s ON %s = %s",
			jc.Type, jc.Table, jc.TableAlias,
			strings.Replace(jc.LeftColumn, mainTable+".", mainAlias+".", 1), jc.RightColumn))
	}
	joinClause := strings.Join(joinParts, " ")

	// === Construct full SQL ===
	fullSQL := fmt.Sprintf("SELECT %s FROM %s %s %s",
		selectClause, mainTable, mainAlias, joinClause)

	// === Expected SQL (the monstrous query) ===
	expectedSQL := `SELECT insurance.persons.*, ` +
		// BelongsTo SELECT columns
		`g.id AS g__id, g.name AS g__name, ` +
		`c.id AS c__id, c.name AS c__name, c.code AS c__code, ` +
		`r.id AS r__id, r.name AS r__name, ` +
		// HasMany #1: docs with nested BelongsTo
		`(SELECT COALESCE(JSON_AGG(json_build_object(` +
		`'id', docs.id, 'seria', docs.seria, 'number', docs.number, 'authority_id', docs.authority_id, ` +
		`'da', json_build_object('id', docs_da.id, 'name', docs_da.name, 'code', docs_da.code)` +
		`)), '[]'::json) FROM insurance.person_documents docs ` +
		`LEFT JOIN insurance.document_authorities docs_da ON docs.authority_id = docs_da.id ` +
		`WHERE docs.person_id = p.id) AS docs__json, ` +
		// HasMany #2: pinfls (simple)
		`(SELECT COALESCE(JSON_AGG(json_build_object(` +
		`'id', pinfls.id, 'value', pinfls.value, 'status', pinfls.status` +
		`)), '[]'::json) FROM insurance.person_pinfls pinfls ` +
		`WHERE pinfls.person_id = p.id) AS pinfls__json ` +
		// FROM and JOINs
		`FROM insurance.persons p ` +
		`LEFT JOIN insurance.genders g ON p.gender_id = g.id ` +
		`LEFT JOIN insurance.countries c ON p.country_id = c.id ` +
		`LEFT JOIN insurance.regions r ON p.region_id = r.id`

	// === Compare ===
	assert.Equal(t, expectedSQL, fullSQL, "Generated SQL should match expected monstrous query")

	// === Print for visual inspection ===
	t.Logf("\n=== GENERATED SQL ===\n%s\n", fullSQL)
}

// newTestPool creates a database pool for integration tests using environment variables.
// Required env vars: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME (optional, defaults to "postgres")
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "postgres"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "postgres"
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		t.Skipf("skipping integration test: failed to parse connection config: %v", err)
	}

	config.MaxConns = 2
	config.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Skipf("skipping integration test: failed to connect to database: %v", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping integration test: failed to ping database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func TestBuildRelations_FullSQLQuery_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create a pool directly (will skip if no database available)
	pool := newTestPool(t)
	ctx := context.Background()

	// === Create test tables ===
	createTablesSQL := `
		CREATE SCHEMA IF NOT EXISTS test_hasmany;

		DROP TABLE IF EXISTS test_hasmany.person_pinfls CASCADE;
		DROP TABLE IF EXISTS test_hasmany.person_documents CASCADE;
		DROP TABLE IF EXISTS test_hasmany.document_authorities CASCADE;
		DROP TABLE IF EXISTS test_hasmany.persons CASCADE;
		DROP TABLE IF EXISTS test_hasmany.regions CASCADE;
		DROP TABLE IF EXISTS test_hasmany.countries CASCADE;
		DROP TABLE IF EXISTS test_hasmany.genders CASCADE;

		CREATE TABLE test_hasmany.genders (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL
		);

		CREATE TABLE test_hasmany.countries (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			code TEXT NOT NULL
		);

		CREATE TABLE test_hasmany.regions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL
		);

		CREATE TABLE test_hasmany.document_authorities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			code TEXT NOT NULL
		);

		CREATE TABLE test_hasmany.persons (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			gender_id UUID REFERENCES test_hasmany.genders(id),
			country_id UUID REFERENCES test_hasmany.countries(id),
			region_id UUID REFERENCES test_hasmany.regions(id)
		);

		CREATE TABLE test_hasmany.person_documents (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			person_id UUID NOT NULL REFERENCES test_hasmany.persons(id),
			seria TEXT NOT NULL,
			number TEXT NOT NULL,
			authority_id UUID REFERENCES test_hasmany.document_authorities(id)
		);

		CREATE TABLE test_hasmany.person_pinfls (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			person_id UUID NOT NULL REFERENCES test_hasmany.persons(id),
			value TEXT NOT NULL,
			status INT NOT NULL DEFAULT 1
		);
	`
	_, err := pool.Exec(ctx, createTablesSQL)
	require.NoError(t, err)

	// Cleanup
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA test_hasmany CASCADE")
	})

	// === Insert test data ===
	insertDataSQL := `
		-- BelongsTo data
		INSERT INTO test_hasmany.genders (id, name) VALUES
			('11111111-1111-1111-1111-111111111111', 'Male');

		INSERT INTO test_hasmany.countries (id, name, code) VALUES
			('22222222-2222-2222-2222-222222222222', 'Uzbekistan', 'UZ');

		INSERT INTO test_hasmany.regions (id, name) VALUES
			('33333333-3333-3333-3333-333333333333', 'Tashkent');

		INSERT INTO test_hasmany.document_authorities (id, name, code) VALUES
			('44444444-4444-4444-4444-444444444444', 'Ministry of Internal Affairs', 'MIA'),
			('55555555-5555-5555-5555-555555555555', 'Tax Authority', 'TAX');

		-- Person
		INSERT INTO test_hasmany.persons (id, first_name, last_name, gender_id, country_id, region_id) VALUES
			('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'John', 'Doe',
			 '11111111-1111-1111-1111-111111111111',
			 '22222222-2222-2222-2222-222222222222',
			 '33333333-3333-3333-3333-333333333333');

		-- HasMany: 3 documents (would cause 3x duplication with JOIN)
		INSERT INTO test_hasmany.person_documents (id, person_id, seria, number, authority_id) VALUES
			('dddddddd-dddd-dddd-dddd-dddddddddd01', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'AA', '1234567', '44444444-4444-4444-4444-444444444444'),
			('dddddddd-dddd-dddd-dddd-dddddddddd02', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'BB', '7654321', '55555555-5555-5555-5555-555555555555'),
			('dddddddd-dddd-dddd-dddd-dddddddddd03', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'CC', '9999999', '44444444-4444-4444-4444-444444444444');

		-- HasMany: 2 pinfls (would cause 2x duplication with JOIN)
		INSERT INTO test_hasmany.person_pinfls (id, person_id, value, status) VALUES
			('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee01', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '12345678901234', 1),
			('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee02', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '98765432109876', 2);
	`
	_, err = pool.Exec(ctx, insertDataSQL)
	require.NoError(t, err)

	// === Build the monstrous query ===
	// Setup schemas (same as unit test but with test_hasmany schema)
	genderSchema := createTestRelationSchema("test_hasmany.genders", []string{"id", "name"})
	countrySchema := createTestRelationSchema("test_hasmany.countries", []string{"id", "name", "code"})
	regionSchema := createTestRelationSchema("test_hasmany.regions", []string{"id", "name"})

	authoritySchema := createTestRelationSchema("test_hasmany.document_authorities", []string{"id", "name", "code"})
	documentsSchema := createTestSchemaWithRelations(
		"test_hasmany.person_documents",
		[]string{"id", "seria", "number", "authority_id"},
		[]RelationDescriptor{
			&Relation[any]{
				Type:        BelongsTo,
				Alias:       "da",
				LocalKey:    "authority_id",
				RemoteKey:   "id",
				Schema:      authoritySchema,
				EntityField: "authority_entity",
			},
		},
	)

	pinflsSchema := createTestRelationSchema("test_hasmany.person_pinfls", []string{"id", "value", "status"})

	relations := []RelationDescriptor{
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "g",
			LocalKey:    "gender_id",
			RemoteKey:   "id",
			JoinType:    JoinTypeLeft,
			Schema:      genderSchema,
			EntityField: "gender_entity",
		},
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "c",
			LocalKey:    "country_id",
			RemoteKey:   "id",
			JoinType:    JoinTypeLeft,
			Schema:      countrySchema,
			EntityField: "country_entity",
		},
		&Relation[any]{
			Type:        BelongsTo,
			Alias:       "r",
			LocalKey:    "region_id",
			RemoteKey:   "id",
			JoinType:    JoinTypeLeft,
			Schema:      regionSchema,
			EntityField: "region_entity",
		},
		&Relation[any]{
			Type:        HasMany,
			Alias:       "docs",
			LocalKey:    "id",
			RemoteKey:   "person_id",
			Schema:      documentsSchema,
			EntityField: "documents_entity",
		},
		&Relation[any]{
			Type:        HasMany,
			Alias:       "pinfls",
			LocalKey:    "id",
			RemoteKey:   "person_id",
			Schema:      pinflsSchema,
			EntityField: "pinfls_entity",
		},
	}

	mainTable := "test_hasmany.persons"
	mainAlias := "p"

	// Build components
	// BuildRelationSelectColumns handles both BelongsTo (SELECT columns) and HasMany (JSON subqueries)
	selectCols := BuildRelationSelectColumns(mainAlias, relations)
	joinClauses := BuildRelationJoinClauses(mainTable, relations)

	// Construct full SELECT clause
	// When using table alias, must use alias.* not schema.table.*
	allSelectParts := []string{mainAlias + ".*"}
	allSelectParts = append(allSelectParts, selectCols...)
	selectClause := strings.Join(allSelectParts, ", ")

	// Construct full JOIN clause
	joinParts := make([]string, 0, len(joinClauses))
	for _, jc := range joinClauses {
		joinParts = append(joinParts, fmt.Sprintf("%s %s %s ON %s = %s",
			jc.Type, jc.Table, jc.TableAlias,
			strings.Replace(jc.LeftColumn, mainTable+".", mainAlias+".", 1), jc.RightColumn))
	}
	joinClause := strings.Join(joinParts, " ")

	// Full SQL
	fullSQL := fmt.Sprintf("SELECT %s FROM %s %s %s",
		selectClause, mainTable, mainAlias, joinClause)

	t.Logf("\n=== EXECUTING SQL ===\n%s\n", fullSQL)

	// === Execute query ===
	rows, err := pool.Query(ctx, fullSQL)
	require.NoError(t, err)
	defer rows.Close()

	// === Verify results ===
	var rowCount int
	var docsData, pinflsData any
	var genderName, countryName, countryCode, regionName string

	for rows.Next() {
		rowCount++

		// Scan all columns - we need to handle the dynamic columns
		values, err := rows.Values()
		require.NoError(t, err)

		t.Logf("Row %d has %d columns", rowCount, len(values))

		// Find the JSON columns by column names
		fieldDescs := rows.FieldDescriptions()
		for i, fd := range fieldDescs {
			colName := fd.Name
			switch colName {
			case "g__name":
				if values[i] != nil {
					genderName = values[i].(string)
				}
			case "c__name":
				if values[i] != nil {
					countryName = values[i].(string)
				}
			case "c__code":
				if values[i] != nil {
					countryCode = values[i].(string)
				}
			case "r__name":
				if values[i] != nil {
					regionName = values[i].(string)
				}
			case "docs__json":
				docsData = values[i]
			case "pinfls__json":
				pinflsData = values[i]
			}
		}
	}
	require.NoError(t, rows.Err())

	// === KEY ASSERTION: Only 1 row returned (no duplication!) ===
	assert.Equal(t, 1, rowCount, "Should return exactly 1 row, not duplicated by HasMany relations")

	// === Verify BelongsTo data ===
	assert.Equal(t, "Male", genderName)
	assert.Equal(t, "Uzbekistan", countryName)
	assert.Equal(t, "UZ", countryCode)
	assert.Equal(t, "Tashkent", regionName)

	// === Verify HasMany JSON: docs (3 documents with nested authority) ===
	// pgx returns JSON_AGG result as []interface{} directly, not as bytes
	docs, ok := docsData.([]interface{})
	require.True(t, ok, "docs__json should be []interface{}, got %T", docsData)
	assert.Len(t, docs, 3, "Should have 3 documents in JSON array")

	// Verify nested BelongsTo (authority) is included
	for _, docRaw := range docs {
		doc, ok := docRaw.(map[string]interface{})
		require.True(t, ok, "Document should be a map")
		da, ok := doc["da"].(map[string]interface{})
		require.True(t, ok, "Document should have nested 'da' (authority), got %T", doc["da"])
		assert.NotEmpty(t, da["name"], "Authority should have name")
		assert.NotEmpty(t, da["code"], "Authority should have code")
	}

	// === Verify HasMany JSON: pinfls (2 pinfls) ===
	pinfls, ok := pinflsData.([]interface{})
	require.True(t, ok, "pinfls__json should be []interface{}, got %T", pinflsData)
	assert.Len(t, pinfls, 2, "Should have 2 pinfls in JSON array")

	t.Logf("\n=== SUCCESS ===")
	t.Logf("Rows returned: %d (would be %d with JOINs)", rowCount, 3*2) // 3 docs * 2 pinfls = 6
	t.Logf("Documents: %d", len(docs))
	t.Logf("PINFLs: %d", len(pinfls))
	t.Logf("Gender: %s", genderName)
	t.Logf("Country: %s (%s)", countryName, countryCode)
	t.Logf("Region: %s", regionName)
}
