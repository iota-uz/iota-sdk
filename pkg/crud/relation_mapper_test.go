package crud

import (
	"context"
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
