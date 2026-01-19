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
