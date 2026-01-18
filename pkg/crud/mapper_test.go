package crud

import (
	"context"
	"testing"
)

type mockJoinableMapper struct{}

func (m *mockJoinableMapper) ToEntities(ctx context.Context, values ...[]FieldValue) ([]string, error) {
	return []string{"entity"}, nil
}

func (m *mockJoinableMapper) ToFieldValuesList(ctx context.Context, entities ...string) ([][]FieldValue, error) {
	return nil, nil
}

func (m *mockJoinableMapper) ToEntitiesWithAllFields(ctx context.Context, myFields, allFields []FieldValue) ([]string, error) {
	return []string{"mapped_entity"}, nil
}

func TestJoinableMapper_Interface(t *testing.T) {
	var mapper JoinableMapper[string] = &mockJoinableMapper{}

	entities, err := mapper.ToEntitiesWithAllFields(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 || entities[0] != "mapped_entity" {
		t.Errorf("unexpected result: %v", entities)
	}
}
