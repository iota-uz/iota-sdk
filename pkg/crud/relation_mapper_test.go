package crud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to extract "name" field value from FieldValues
func extractName(fvs []FieldValue) string {
	for _, fv := range fvs {
		if fv.Field().Name() == "name" {
			if v := fv.Value(); v != nil {
				return v.(string)
			}
		}
	}
	return ""
}

func TestRelationMapper_ToEntity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fields      []FieldValue
		relations   []RelationDescriptor
		mapOwnFn    func([]FieldValue) string
		expected    string
		wantErr     bool
		errContains string
	}{
		{
			name: "Basic_MapsParentAndChildCorrectly",
			fields: []FieldValue{
				NewStringField("id").Value("parent-uuid"),
				NewStringField("name").Value("parent-name"),
				NewStringField("vt__id").Value("child-uuid"),
				NewStringField("vt__name").Value("child-name"),
			},
			relations: []RelationDescriptor{
				&Relation[any]{
					Alias:  "vt",
					Mapper: NewRelationMapper[string](nil, nil, extractName),
					SetOnParent: func(parent, child any) any {
						return parent.(string) + "+" + child.(string)
					},
				},
			},
			mapOwnFn: extractName,
			expected: "parent-name+child-name",
		},
		{
			name: "ThreeLevels_CascadesNestedRelationsCorrectly",
			fields: []FieldValue{
				NewStringField("id").Value("vehicle-uuid"),
				NewStringField("name").Value("vehicle"),
				NewStringField("vt__id").Value("vt-uuid"),
				NewStringField("vt__name").Value("vtype"),
				NewStringField("vt__vg__id").Value("vg-uuid"),
				NewStringField("vt__vg__name").Value("vgroup"),
			},
			relations: []RelationDescriptor{
				&Relation[any]{
					Alias: "vt",
					Mapper: NewRelationMapper[string](
						nil,
						[]RelationDescriptor{
							&Relation[any]{
								Alias:  "vg",
								Mapper: NewRelationMapper[string](nil, nil, extractName),
								SetOnParent: func(parent, child any) any {
									return parent.(string) + ">" + child.(string)
								},
							},
						},
						extractName,
					),
					SetOnParent: func(parent, child any) any {
						return parent.(string) + ">" + child.(string)
					},
				},
			},
			mapOwnFn: extractName,
			expected: "vehicle>vtype>vgroup",
		},
		{
			name: "NullChild_SkipsSetOnParent",
			fields: []FieldValue{
				NewStringField("id").Value("parent-uuid"),
				NewStringField("name").Value("parent-name"),
				NewStringField("vt__id").Value(nil),
				NewStringField("vt__name").Value(nil),
			},
			relations: []RelationDescriptor{
				&Relation[any]{
					Alias:  "vt",
					Mapper: NewRelationMapper[string](nil, nil, extractName),
					SetOnParent: func(parent, child any) any {
						return parent.(string) + "+" + child.(string)
					},
				},
			},
			mapOwnFn: extractName,
			expected: "parent-name",
		},
		{
			name: "NoRelations_MapsParentOnly",
			fields: []FieldValue{
				NewStringField("id").Value("uuid"),
				NewStringField("name").Value("entity"),
			},
			relations: nil,
			mapOwnFn:  extractName,
			expected:  "entity",
		},
		{
			name: "MissingSetOnParent_SkipsRelation",
			fields: []FieldValue{
				NewStringField("id").Value("parent-uuid"),
				NewStringField("name").Value("parent-name"),
				NewStringField("vt__id").Value("child-uuid"),
				NewStringField("vt__name").Value("child-name"),
			},
			relations: []RelationDescriptor{
				&Relation[any]{
					Alias:       "vt",
					Mapper:      NewRelationMapper[string](nil, nil, extractName),
					SetOnParent: nil, // Missing!
				},
			},
			mapOwnFn: extractName,
			expected: "parent-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			mapper := NewRelationMapper[string](nil, tt.relations, tt.mapOwnFn)

			result, err := mapper.ToEntity(ctx, tt.fields)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRelationMapper_ToEntities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rows     [][]FieldValue
		expected []string
		wantErr  bool
	}{
		{
			name: "MultipleRows_MapsAllCorrectly",
			rows: [][]FieldValue{
				{
					NewStringField("id").Value("1"),
					NewStringField("name").Value("first"),
				},
				{
					NewStringField("id").Value("2"),
					NewStringField("name").Value("second"),
				},
			},
			expected: []string{"first", "second"},
		},
		{
			name:     "EmptyRows_ReturnsEmptySlice",
			rows:     [][]FieldValue{},
			expected: []string{},
		},
		{
			name: "SingleRow_MapsCorrectly",
			rows: [][]FieldValue{
				{
					NewStringField("id").Value("1"),
					NewStringField("name").Value("only"),
				},
			},
			expected: []string{"only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			mapper := NewRelationMapper[string](nil, nil, extractName)

			results, err := mapper.ToEntities(ctx, tt.rows...)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, results, len(tt.expected))

			for i, expected := range tt.expected {
				assert.Equal(t, expected, results[i], "results[%d]", i)
			}
		})
	}
}

func TestRelationMapper_ToEntity_HasManyJSON(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Track children added via SetOnParent
	var addedChildren []map[string]any

	// Create a simple mapper that extracts name from fields
	baseMapper := &mockBaseMapper{
		mapFn: func(fvs []FieldValue) string {
			for _, fv := range fvs {
				if fv.Field().Name() == "name" {
					v, _ := fv.AsString()
					return v
				}
			}
			return ""
		},
	}

	parentMapper := NewRelationMapper[string](
		baseMapper,
		[]RelationDescriptor{
			&Relation[any]{
				Type:  HasMany,
				Alias: "docs",
				SetOnParent: func(parent, child any) any {
					if c, ok := child.(map[string]any); ok {
						addedChildren = append(addedChildren, c)
					}
					return parent
				},
			},
		},
		func(fvs []FieldValue) string {
			for _, fv := range fvs {
				if fv.Field().Name() == "name" {
					v, _ := fv.AsString()
					return v
				}
			}
			return ""
		},
	)

	// Simulate query result with HasMany JSON field
	allFields := []FieldValue{
		NewStringField("name").Value("parent"),
		NewStringField("docs__json").Value(`[{"id":"c1","name":"child1"},{"id":"c2","name":"child2"}]`),
	}

	result, err := parentMapper.ToEntity(ctx, allFields)
	require.NoError(t, err)
	assert.Equal(t, "parent", result)

	// Should have called SetOnParent twice (once per child)
	require.Len(t, addedChildren, 2)
	assert.Equal(t, "c1", addedChildren[0]["id"])
	assert.Equal(t, "child1", addedChildren[0]["name"])
	assert.Equal(t, "c2", addedChildren[1]["id"])
	assert.Equal(t, "child2", addedChildren[1]["name"])
}

type mockBaseMapper struct {
	mapFn func([]FieldValue) string
}

func (m *mockBaseMapper) ToEntities(ctx context.Context, values ...[]FieldValue) ([]string, error) {
	result := make([]string, len(values))
	for i, fvs := range values {
		result[i] = m.mapFn(fvs)
	}
	return result, nil
}

func (m *mockBaseMapper) ToFieldValuesList(ctx context.Context, entities ...string) ([][]FieldValue, error) {
	return nil, nil
}

func TestParseHasManyJSON(t *testing.T) {
	t.Parallel()

	type Item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	tests := []struct {
		name        string
		input       any
		expected    []Item
		expectNil   bool
		expectEmpty bool
		wantErr     bool
	}{
		{
			name:  "ValidJSONArray_ParsesCorrectly",
			input: []byte(`[{"id":"1","name":"first"},{"id":"2","name":"second"}]`),
			expected: []Item{
				{ID: "1", Name: "first"},
				{ID: "2", Name: "second"},
			},
		},
		{
			name:      "NilInput_ReturnsNil",
			input:     nil,
			expectNil: true,
		},
		{
			name:      "NullJSON_ReturnsNil",
			input:     []byte("null"),
			expectNil: true,
		},
		{
			name:        "EmptyArray_ReturnsEmptySlice",
			input:       []byte("[]"),
			expectEmpty: true,
		},
		{
			name:     "StringInput_ParsesFromDatabase",
			input:    `[{"id":"test","name":"testname"}]`,
			expected: []Item{{ID: "test", Name: "testname"}},
		},
		{
			name:      "EmptyString_ReturnsNil",
			input:     "",
			expectNil: true,
		},
		{
			name:      "EmptyBytes_ReturnsNil",
			input:     []byte{},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			items, err := parseHasManyJSON[Item](tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.expectNil {
				assert.Nil(t, items)
				return
			}

			if tt.expectEmpty {
				require.NotNil(t, items)
				assert.Empty(t, items)
				return
			}

			require.Len(t, items, len(tt.expected))
			for i, expected := range tt.expected {
				assert.Equal(t, expected.ID, items[i].ID, "items[%d].ID", i)
				assert.Equal(t, expected.Name, items[i].Name, "items[%d].Name", i)
			}
		})
	}
}

// TestRelationMapper_NullChildWithHasManyDefaults tests the scenario where:
//   - A parent has a BelongsTo relation to a child
//   - The child has a HasMany relation
//   - When the parent's foreign key is NULL (LEFT JOIN returns no match),
//     the child's own fields are NULL but the HasMany JSON field has "[]" (not NULL due to COALESCE)
//
// This verifies that the null check only looks at own fields, not nested HasMany defaults.
func TestRelationMapper_NullChildWithHasManyDefaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	childSetOnParentCalled := false

	// Child mapper (Person) that would panic if called with nil data
	childMapper := NewRelationMapper[string](
		nil,
		[]RelationDescriptor{
			&Relation[any]{
				Type:  HasMany,
				Alias: "docs",
				SetOnParent: func(parent, child any) any {
					return parent
				},
			},
		},
		func(fvs []FieldValue) string {
			// Extract "id" to create entity - this would panic if called with nil
			for _, fv := range fvs {
				if fv.Field().Name() == "id" {
					if v := fv.Value(); v != nil {
						return v.(string)
					}
					// If id is nil, this means null relation - should not happen with fix
					panic("child mapper called with nil id - relation should have been skipped")
				}
			}
			return ""
		},
	)

	// Parent mapper (Vehicle) with BelongsTo relation to Person
	parentMapper := NewRelationMapper[string](
		nil,
		[]RelationDescriptor{
			&Relation[any]{
				Alias:  "p",
				Mapper: childMapper,
				SetOnParent: func(parent, child any) any {
					childSetOnParentCalled = true
					return parent.(string) + "+" + child.(string)
				},
			},
		},
		func(fvs []FieldValue) string {
			for _, fv := range fvs {
				if fv.Field().Name() == "id" {
					if v := fv.Value(); v != nil {
						return v.(string)
					}
				}
			}
			return ""
		},
	)

	// Simulate LEFT JOIN result where owner_person_id is NULL:
	// - Vehicle's own fields are present
	// - Person's own fields (p__id, p__first_name) are NULL from LEFT JOIN
	// - Person's HasMany JSON (p__docs__json) is "[]" because of COALESCE in subquery
	allFields := []FieldValue{
		NewStringField("id").Value("vehicle-123"),
		NewStringField("plate_number").Value("01A123BC"),
		NewStringField("p__id").Value(nil),          // Person's id is NULL
		NewStringField("p__first_name").Value(nil),  // Person's first_name is NULL
		NewStringField("p__docs__json").Value("[]"), // HasMany has non-null default!
	}

	result, err := parentMapper.ToEntity(ctx, allFields)
	require.NoError(t, err)
	assert.Equal(t, "vehicle-123", result)

	// Child SetOnParent should NOT have been called because person is null
	assert.False(t, childSetOnParentCalled, "child SetOnParent was called, but should have been skipped for null relation")
}

// TestRelationMapper_NullChildWithNestedBelongsToAndHasMany tests the scenario from the actual panic:
// - Vehicle has BelongsTo Person
// - Person has BelongsTo Gender AND HasMany docs
// - When Vehicle.owner_person_id is NULL:
//   - Person's own fields (p__id, p__first_name) are NULL
//   - Person's Gender fields (p__g__id, p__g__name) are NULL
//   - Person's HasMany JSON (p__docs__json) is "[]" (not NULL due to COALESCE)
//
// Without the fix, Person mapper is still called (because docs__json isn't null),
// mapOwn returns nil Person, then Gender's SetOnParent panics on nil.(Person) assertion.
func TestRelationMapper_NullChildWithNestedBelongsToAndHasMany(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	personSetOnParentCalled := false
	genderSetOnParentCalled := false

	// Gender mapper (leaf level)
	genderMapper := NewRelationMapper[string](
		nil,
		nil,
		func(fvs []FieldValue) string {
			for _, fv := range fvs {
				if fv.Field().Name() == "name" {
					if v := fv.Value(); v != nil {
						return v.(string)
					}
				}
			}
			return ""
		},
	)

	// Person mapper - has Gender (BelongsTo) and docs (HasMany)
	personMapper := NewRelationMapper[string](
		nil,
		[]RelationDescriptor{
			&Relation[any]{
				Alias:  "g",
				Mapper: genderMapper,
				SetOnParent: func(parent, child any) any {
					genderSetOnParentCalled = true
					p := parent.(string)
					if child == nil {
						return p
					}
					return p + "+gender:" + child.(string)
				},
			},
			&Relation[any]{
				Type:  HasMany,
				Alias: "docs",
				SetOnParent: func(parent, child any) any {
					return parent
				},
			},
		},
		func(fvs []FieldValue) string {
			// This mapOwn returns empty string when id is nil
			for _, fv := range fvs {
				if fv.Field().Name() == "id" {
					if v := fv.Value(); v != nil {
						return "person:" + v.(string)
					}
				}
			}
			return "" // Returns empty when person doesn't exist
		},
	)

	// Vehicle mapper - has Person (BelongsTo)
	vehicleMapper := NewRelationMapper[string](
		nil,
		[]RelationDescriptor{
			&Relation[any]{
				Alias:  "p",
				Mapper: personMapper,
				SetOnParent: func(parent, child any) any {
					personSetOnParentCalled = true
					v := parent.(string)
					if child == nil {
						return v
					}
					return v + "+" + child.(string)
				},
			},
		},
		func(fvs []FieldValue) string {
			for _, fv := range fvs {
				if fv.Field().Name() == "id" {
					if v := fv.Value(); v != nil {
						return "vehicle:" + v.(string)
					}
				}
			}
			return ""
		},
	)

	// Simulate LEFT JOIN result where owner_person_id is NULL:
	allFields := []FieldValue{
		NewStringField("id").Value("123"),           // Vehicle ID
		NewStringField("plate_number").Value("ABC"), // Vehicle field
		NewStringField("p__id").Value(nil),          // Person ID is NULL
		NewStringField("p__first_name").Value(nil),  // Person first_name is NULL
		NewStringField("p__g__id").Value(nil),       // Person's Gender ID is NULL
		NewStringField("p__g__name").Value(nil),     // Person's Gender name is NULL
		NewStringField("p__docs__json").Value("[]"), // HasMany has non-null default!
	}

	result, err := vehicleMapper.ToEntity(ctx, allFields)
	require.NoError(t, err)
	assert.Equal(t, "vehicle:123", result)

	// Person SetOnParent should NOT have been called because person is null
	assert.False(t, personSetOnParentCalled, "person SetOnParent was called, but should have been skipped for null relation")

	// Gender SetOnParent should definitely NOT have been called
	assert.False(t, genderSetOnParentCalled, "gender SetOnParent was called, but should have been skipped when person is null")
}
