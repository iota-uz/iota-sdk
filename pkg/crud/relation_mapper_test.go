package crud

import (
	"context"
	"testing"
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

func TestRelationMapper_Basic(t *testing.T) {
	ctx := context.Background()

	// Create field values simulating a joined query result
	allFields := []FieldValue{
		NewStringField("id").Value("parent-uuid"),
		NewStringField("name").Value("parent-name"),
		NewStringField("vt__id").Value("child-uuid"),
		NewStringField("vt__name").Value("child-name"),
	}

	// Child mapper (leaf - no relations)
	childMapper := NewRelationMapper[string](
		nil, // no inner mapper needed
		nil, // no relations (leaf)
		extractName,
	)

	// Parent mapper with child relation
	parentMapper := NewRelationMapper[string](
		nil,
		[]RelationDescriptor{
			&Relation[any]{
				Alias:  "vt",
				Mapper: childMapper,
				SetOnParent: func(parent, child any) any {
					return parent.(string) + "+" + child.(string)
				},
			},
		},
		extractName,
	)

	result, err := parentMapper.ToEntity(ctx, allFields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "parent-name+child-name"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

func TestRelationMapper_ThreeLevels(t *testing.T) {
	ctx := context.Background()

	// Simulate: Vehicle -> VehicleType (vt) -> VehicleGroup (vg)
	allFields := []FieldValue{
		NewStringField("id").Value("vehicle-uuid"),
		NewStringField("name").Value("vehicle"),
		NewStringField("vt__id").Value("vt-uuid"),
		NewStringField("vt__name").Value("vtype"),
		NewStringField("vt__vg__id").Value("vg-uuid"),
		NewStringField("vt__vg__name").Value("vgroup"),
	}

	// VehicleGroup mapper (leaf - no relations)
	vgMapper := NewRelationMapper[string](nil, nil, extractName)

	// VehicleType mapper (has VehicleGroup relation)
	vtMapper := NewRelationMapper[string](
		nil,
		[]RelationDescriptor{
			&Relation[any]{
				Alias:  "vg",
				Mapper: vgMapper,
				SetOnParent: func(parent, child any) any {
					return parent.(string) + ">" + child.(string)
				},
			},
		},
		extractName,
	)

	// Vehicle mapper (has VehicleType relation - doesn't know about VehicleGroup!)
	vehicleMapper := NewRelationMapper[string](
		nil,
		[]RelationDescriptor{
			&Relation[any]{
				Alias:  "vt",
				Mapper: vtMapper,
				SetOnParent: func(parent, child any) any {
					return parent.(string) + ">" + child.(string)
				},
			},
		},
		extractName,
	)

	result, err := vehicleMapper.ToEntity(ctx, allFields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be: vehicle>vtype>vgroup
	// Vehicle mapper only knows about VehicleType, but VehicleType internally
	// handles VehicleGroup - that's the auto-cascading pattern!
	expected := "vehicle>vtype>vgroup"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

func TestRelationMapper_NullChild(t *testing.T) {
	ctx := context.Background()

	// LEFT JOIN with no match - child fields are all null
	allFields := []FieldValue{
		NewStringField("id").Value("parent-uuid"),
		NewStringField("name").Value("parent-name"),
		NewStringField("vt__id").Value(nil),
		NewStringField("vt__name").Value(nil),
	}

	childMapper := NewRelationMapper[string](nil, nil, extractName)

	parentMapper := NewRelationMapper[string](
		nil,
		[]RelationDescriptor{
			&Relation[any]{
				Alias:  "vt",
				Mapper: childMapper,
				SetOnParent: func(parent, child any) any {
					return parent.(string) + "+" + child.(string)
				},
			},
		},
		extractName,
	)

	result, err := parentMapper.ToEntity(ctx, allFields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Child is null, so SetOnParent should not be called
	expected := "parent-name"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

func TestRelationMapper_ToEntities(t *testing.T) {
	ctx := context.Background()

	// Multiple rows
	row1 := []FieldValue{
		NewStringField("id").Value("1"),
		NewStringField("name").Value("first"),
	}
	row2 := []FieldValue{
		NewStringField("id").Value("2"),
		NewStringField("name").Value("second"),
	}

	mapper := NewRelationMapper[string](nil, nil, extractName)

	results, err := mapper.ToEntities(ctx, row1, row2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0] != "first" {
		t.Errorf("results[0] = %q, want %q", results[0], "first")
	}
	if results[1] != "second" {
		t.Errorf("results[1] = %q, want %q", results[1], "second")
	}
}

func TestRelationMapper_NoRelations(t *testing.T) {
	ctx := context.Background()

	allFields := []FieldValue{
		NewStringField("id").Value("uuid"),
		NewStringField("name").Value("entity"),
	}

	// Mapper with no relations (leaf entity)
	mapper := NewRelationMapper[string](nil, nil, extractName)

	result, err := mapper.ToEntity(ctx, allFields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "entity" {
		t.Errorf("result = %q, want %q", result, "entity")
	}
}

func TestRelationMapper_MissingSetOnParent(t *testing.T) {
	ctx := context.Background()

	allFields := []FieldValue{
		NewStringField("id").Value("parent-uuid"),
		NewStringField("name").Value("parent-name"),
		NewStringField("vt__id").Value("child-uuid"),
		NewStringField("vt__name").Value("child-name"),
	}

	childMapper := NewRelationMapper[string](nil, nil, extractName)

	// Relation without SetOnParent - should be skipped
	parentMapper := NewRelationMapper[string](
		nil,
		[]RelationDescriptor{
			&Relation[any]{
				Alias:       "vt",
				Mapper:      childMapper,
				SetOnParent: nil, // Missing!
			},
		},
		extractName,
	)

	result, err := parentMapper.ToEntity(ctx, allFields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Relation skipped because SetOnParent is nil
	expected := "parent-name"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
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
	// JSON from PostgreSQL comes as []byte, but parseHasManyJSON handles both string and []byte
	allFields := []FieldValue{
		NewStringField("name").Value("parent"),
		NewStringField("docs__json").Value(`[{"id":"c1","name":"child1"},{"id":"c2","name":"child2"}]`),
	}

	result, err := parentMapper.ToEntity(ctx, allFields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "parent" {
		t.Errorf("result = %q, want %q", result, "parent")
	}

	// Should have called SetOnParent twice (once per child)
	if len(addedChildren) != 2 {
		t.Fatalf("expected 2 children, got %d", len(addedChildren))
	}
	if addedChildren[0]["id"] != "c1" {
		t.Errorf("addedChildren[0][\"id\"] = %q, want %q", addedChildren[0]["id"], "c1")
	}
	if addedChildren[0]["name"] != "child1" {
		t.Errorf("addedChildren[0][\"name\"] = %q, want %q", addedChildren[0]["name"], "child1")
	}
	if addedChildren[1]["id"] != "c2" {
		t.Errorf("addedChildren[1][\"id\"] = %q, want %q", addedChildren[1]["id"], "c2")
	}
	if addedChildren[1]["name"] != "child2" {
		t.Errorf("addedChildren[1][\"name\"] = %q, want %q", addedChildren[1]["name"], "child2")
	}
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

	t.Run("parses valid JSON array", func(t *testing.T) {
		t.Parallel()

		type Item struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}

		jsonData := []byte(`[{"id":"1","name":"first"},{"id":"2","name":"second"}]`)

		items, err := parseHasManyJSON[Item](jsonData)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if items[0].ID != "1" {
			t.Errorf("items[0].ID = %q, want %q", items[0].ID, "1")
		}
		if items[0].Name != "first" {
			t.Errorf("items[0].Name = %q, want %q", items[0].Name, "first")
		}
		if items[1].ID != "2" {
			t.Errorf("items[1].ID = %q, want %q", items[1].ID, "2")
		}
		if items[1].Name != "second" {
			t.Errorf("items[1].Name = %q, want %q", items[1].Name, "second")
		}
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		t.Parallel()

		type Item struct{}
		items, err := parseHasManyJSON[Item](nil)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if items != nil {
			t.Errorf("expected nil, got %v", items)
		}
	})

	t.Run("returns nil for null JSON", func(t *testing.T) {
		t.Parallel()

		type Item struct{}
		items, err := parseHasManyJSON[Item]([]byte("null"))

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if items != nil {
			t.Errorf("expected nil, got %v", items)
		}
	})

	t.Run("returns empty slice for empty array", func(t *testing.T) {
		t.Parallel()

		type Item struct{}
		items, err := parseHasManyJSON[Item]([]byte("[]"))

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if items == nil {
			t.Errorf("expected empty slice, got nil")
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("handles string input from database", func(t *testing.T) {
		t.Parallel()

		type Item struct {
			ID string `json:"id"`
		}

		items, err := parseHasManyJSON[Item](`[{"id":"test"}]`)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
		if items[0].ID != "test" {
			t.Errorf("items[0].ID = %q, want %q", items[0].ID, "test")
		}
	})
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parent should be mapped correctly
	if result != "vehicle-123" {
		t.Errorf("result = %q, want %q", result, "vehicle-123")
	}

	// Child SetOnParent should NOT have been called because person is null
	if childSetOnParentCalled {
		t.Error("child SetOnParent was called, but should have been skipped for null relation")
	}
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
					// This is where the real panic happens - parent is nil
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Vehicle should be mapped correctly
	if result != "vehicle:123" {
		t.Errorf("result = %q, want %q", result, "vehicle:123")
	}

	// Person SetOnParent should NOT have been called because person is null
	if personSetOnParentCalled {
		t.Error("person SetOnParent was called, but should have been skipped for null relation")
	}

	// Gender SetOnParent should definitely NOT have been called
	if genderSetOnParentCalled {
		t.Error("gender SetOnParent was called, but should have been skipped when person is null")
	}
}
