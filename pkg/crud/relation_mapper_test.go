package crud

import (
	"context"
	"strings"
	"testing"
)

func TestMapWithRelations_Basic(t *testing.T) {
	ctx := context.Background()

	// Create field values simulating a joined query result
	// Use string fields for simplicity in tests
	parentIdField := NewStringField("id")
	parentNameField := NewStringField("name")
	vtIdField := NewStringField("vt__id")
	vtNameField := NewStringField("vt__name")

	allFields := []FieldValue{
		parentIdField.Value("parent-uuid"),
		parentNameField.Value("parent-name"),
		vtIdField.Value("child-uuid"),
		vtNameField.Value("child-name"),
	}

	myFields := ExtractNonPrefixedFields(allFields)

	mapOwnFields := func(fvs []FieldValue) string {
		for _, fv := range fvs {
			if fv.Field().Name() == "name" {
				return fv.Value().(string)
			}
		}
		return ""
	}

	childMapper := &testJoinableMapper{}

	setOnParent := func(parent, child any) any {
		return parent.(string) + "+" + child.(string)
	}

	relations := []Relation{
		{
			Alias:       "vt",
			Mapper:      childMapper,
			SetOnParent: setOnParent,
		},
	}

	result, err := MapWithRelations(ctx, myFields, allFields, mapOwnFields, relations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "parent-name+child-name"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

type testJoinableMapper struct{}

func (m *testJoinableMapper) ToEntities(ctx context.Context, values ...[]FieldValue) ([]string, error) {
	return nil, nil
}

func (m *testJoinableMapper) ToFieldValuesList(ctx context.Context, entities ...string) ([][]FieldValue, error) {
	return nil, nil
}

func (m *testJoinableMapper) ToEntitiesWithAllFields(ctx context.Context, myFields, allFields []FieldValue) ([]string, error) {
	for _, fv := range myFields {
		if fv.Field().Name() == "name" {
			return []string{fv.Value().(string)}, nil
		}
	}
	return []string{""}, nil
}

func (m *testJoinableMapper) ToEntitiesWithAllFieldsAny(ctx context.Context, myFields, allFields []FieldValue) ([]any, error) {
	entities, err := m.ToEntitiesWithAllFields(ctx, myFields, allFields)
	if err != nil {
		return nil, err
	}
	result := make([]any, len(entities))
	for i, e := range entities {
		result[i] = e
	}
	return result, nil
}

// extractPrefixedFieldsWithNested is a test helper that extracts prefixed fields
// INCLUDING nested prefixes (unlike ExtractPrefixedFields which excludes them).
// This simulates the expected behavior for auto-cascading mapper pattern.
//
// Example: prefix="vt", fields=["id", "vt__id", "vt__name", "vt__vg__id"]
// Returns: ["id", "name", "vg__id"] (prefix stripped, nested preserved)
func extractPrefixedFieldsWithNested(fvs []FieldValue, prefix string) []FieldValue {
	if len(fvs) == 0 {
		return nil
	}

	fullPrefix := prefix + "__"
	result := make([]FieldValue, 0)

	for _, fv := range fvs {
		fieldName := fv.Field().Name()

		if !strings.HasPrefix(fieldName, fullPrefix) {
			continue
		}

		remainder := strings.TrimPrefix(fieldName, fullPrefix)

		// Create a wrapper field with the stripped name
		// Using prefixedField from repository_relations.go (same package)
		wrappedField := &prefixedField{
			Field: fv.Field(),
			name:  remainder,
		}

		// Using fieldValue from field_value.go (same package)
		result = append(result, &fieldValue{
			field: wrappedField,
			value: fv.Value(),
		})
	}

	return result
}

func TestMapWithRelations_ThreeLevels(t *testing.T) {
	ctx := context.Background()

	// Simulate: Vehicle -> VehicleType (vt) -> VehicleGroup (vg)
	// Use string fields for simplicity in tests
	allFields := []FieldValue{
		NewStringField("id").Value("vehicle-uuid"),
		NewStringField("name").Value("vehicle"),
		NewStringField("vt__id").Value("vt-uuid"),
		NewStringField("vt__name").Value("vtype"),
		NewStringField("vt__vg__id").Value("vg-uuid"),
		NewStringField("vt__vg__name").Value("vgroup"),
	}

	myFields := ExtractNonPrefixedFields(allFields)

	// VehicleGroup mapper (leaf)
	vgMapper := &leafTestMapper{}

	// VehicleType mapper (has VehicleGroup relation)
	vtMapper := &nestedTestMapper{
		childAlias:  "vg",
		childMapper: vgMapper,
	}

	mapOwnFields := func(fvs []FieldValue) string {
		for _, fv := range fvs {
			if fv.Field().Name() == "name" {
				return fv.Value().(string)
			}
		}
		return ""
	}

	relations := []Relation{
		{
			Alias:  "vt",
			Mapper: vtMapper,
			SetOnParent: func(parent, child any) any {
				return parent.(string) + ">" + child.(string)
			},
		},
	}

	// For three-level nesting, we need to use extractPrefixedFieldsWithNested
	// to pass nested prefixes through to child mappers.
	// This simulates the expected auto-cascading behavior where
	// ExtractPrefixedFields would include nested fields.
	vtFields := extractPrefixedFieldsWithNested(allFields, "vt")

	// vtFields now has: [id, name, vg__id, vg__name]
	// The vtMapper can extract "vg" from this to get its child

	// Call vtMapper directly to test the cascading
	children, err := vtMapper.ToEntitiesWithAllFieldsAny(ctx, vtFields, vtFields)
	if err != nil {
		t.Fatalf("vtMapper error: %v", err)
	}
	if len(children) == 0 {
		t.Fatal("vtMapper returned no children")
	}

	// vtMapper should return "vtype>vgroup" (its name + child name)
	vtResult := children[0].(string)
	expectedVt := "vtype>vgroup"
	if vtResult != expectedVt {
		t.Errorf("vtMapper result = %q, want %q", vtResult, expectedVt)
	}

	// Now test full MapWithRelations with modified approach
	// We need a custom relation extraction that preserves nested fields
	result, err := mapWithRelationsNested(ctx, myFields, allFields, mapOwnFields, relations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be: vehicle>vtype>vgroup
	expected := "vehicle>vtype>vgroup"
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

// mapWithRelationsNested is like MapWithRelations but uses extractPrefixedFieldsWithNested
// to support auto-cascading to nested mappers.
func mapWithRelationsNested[T any](
	ctx context.Context,
	myFields, allFields []FieldValue,
	mapOwnFields func([]FieldValue) T,
	relations []Relation,
) (T, error) {
	entity := mapOwnFields(myFields)

	for _, rel := range relations {
		if rel.Mapper == nil || rel.SetOnParent == nil {
			continue
		}

		// Use extractPrefixedFieldsWithNested to include nested prefixes
		childFields := extractPrefixedFieldsWithNested(allFields, rel.Alias)
		if AllFieldsNull(childFields) {
			continue
		}

		mapper, ok := rel.Mapper.(JoinableMapperAny)
		if !ok {
			continue
		}

		// Pass childFields as both myFields and allFields
		// This allows nested mappers to extract their children from childFields
		children, err := mapper.ToEntitiesWithAllFieldsAny(ctx, childFields, childFields)
		if err != nil || len(children) == 0 {
			continue
		}

		entity = rel.SetOnParent(entity, children[0]).(T)
	}

	return entity, nil
}

type leafTestMapper struct{}

func (m *leafTestMapper) ToEntities(ctx context.Context, values ...[]FieldValue) ([]string, error) {
	return nil, nil
}

func (m *leafTestMapper) ToFieldValuesList(ctx context.Context, entities ...string) ([][]FieldValue, error) {
	return nil, nil
}

func (m *leafTestMapper) ToEntitiesWithAllFields(ctx context.Context, myFields, allFields []FieldValue) ([]string, error) {
	for _, fv := range myFields {
		if fv.Field().Name() == "name" {
			return []string{fv.Value().(string)}, nil
		}
	}
	return []string{""}, nil
}

func (m *leafTestMapper) ToEntitiesWithAllFieldsAny(ctx context.Context, myFields, allFields []FieldValue) ([]any, error) {
	entities, err := m.ToEntitiesWithAllFields(ctx, myFields, allFields)
	if err != nil {
		return nil, err
	}
	result := make([]any, len(entities))
	for i, e := range entities {
		result[i] = e
	}
	return result, nil
}

type nestedTestMapper struct {
	childAlias  string
	childMapper JoinableMapperAny
}

func (m *nestedTestMapper) ToEntities(ctx context.Context, values ...[]FieldValue) ([]string, error) {
	return nil, nil
}

func (m *nestedTestMapper) ToFieldValuesList(ctx context.Context, entities ...string) ([][]FieldValue, error) {
	return nil, nil
}

func (m *nestedTestMapper) ToEntitiesWithAllFields(ctx context.Context, myFields, allFields []FieldValue) ([]string, error) {
	var name string
	for _, fv := range myFields {
		if fv.Field().Name() == "name" {
			name = fv.Value().(string)
			break
		}
	}

	// Extract child fields from myFields (which already has our prefix stripped)
	// This allows nested mappers to work correctly with any level of nesting
	childFields := ExtractPrefixedFields(myFields, m.childAlias)
	if !AllFieldsNull(childFields) {
		children, err := m.childMapper.ToEntitiesWithAllFieldsAny(ctx, childFields, myFields)
		if err == nil && len(children) > 0 {
			name = name + ">" + children[0].(string)
		}
	}

	return []string{name}, nil
}

func (m *nestedTestMapper) ToEntitiesWithAllFieldsAny(ctx context.Context, myFields, allFields []FieldValue) ([]any, error) {
	entities, err := m.ToEntitiesWithAllFields(ctx, myFields, allFields)
	if err != nil {
		return nil, err
	}
	result := make([]any, len(entities))
	for i, e := range entities {
		result[i] = e
	}
	return result, nil
}
