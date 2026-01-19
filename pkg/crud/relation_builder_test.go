package crud

import (
	"context"
	"testing"
)

func TestRelationBuilder_BelongsTo(t *testing.T) {
	mockSchema := newMockSchemaForBuilder("test_types")

	relations := NewRelationBuilder().
		BelongsTo("vt", mockSchema).
		LocalKey("type_id").
		EntityField("type_entity").
		Build()

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	rel := relations[0]
	if rel.GetType() != BelongsTo {
		t.Errorf("expected BelongsTo, got %v", rel.GetType())
	}
	if rel.GetAlias() != "vt" {
		t.Errorf("expected alias 'vt', got %q", rel.GetAlias())
	}
	if rel.GetLocalKey() != "type_id" {
		t.Errorf("expected local key 'type_id', got %q", rel.GetLocalKey())
	}
	if rel.GetEntityField() != "type_entity" {
		t.Errorf("expected entity field 'type_entity', got %q", rel.GetEntityField())
	}
	if rel.GetRemoteKey() != "id" {
		t.Errorf("expected default remote key 'id', got %q", rel.GetRemoteKey())
	}
	if rel.GetJoinType() != JoinTypeLeft {
		t.Errorf("expected default JoinTypeLeft, got %v", rel.GetJoinType())
	}
}

func TestRelationBuilder_NestedRelation(t *testing.T) {
	mockTypeSchema := newMockSchemaForBuilder("types")
	mockGroupSchema := newMockSchemaForBuilder("groups")

	relations := NewRelationBuilder().
		BelongsTo("vt", mockTypeSchema).
		LocalKey("type_id").
		EntityField("type_entity").
		BelongsTo("vg", mockGroupSchema).
		LocalKey("group_id").
		Through("vt").
		EntityField("group_entity").
		Build()

	if len(relations) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(relations))
	}

	rel := relations[1]
	if rel.GetThrough() != "vt" {
		t.Errorf("expected Through 'vt', got %q", rel.GetThrough())
	}
}

func TestRelationBuilder_InnerJoin(t *testing.T) {
	mockSchema := newMockSchemaForBuilder("required_types")

	relations := NewRelationBuilder().
		BelongsTo("rt", mockSchema).
		LocalKey("required_type_id").
		EntityField("required_type_entity").
		InnerJoin().
		Build()

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	if relations[0].GetJoinType() != JoinTypeInner {
		t.Errorf("expected JoinTypeInner, got %v", relations[0].GetJoinType())
	}
}

func TestRelationBuilder_CustomRemoteKey(t *testing.T) {
	mockSchema := newMockSchemaForBuilder("items")

	relations := NewRelationBuilder().
		BelongsTo("i", mockSchema).
		LocalKey("item_code").
		RemoteKey("code").
		EntityField("item_entity").
		Build()

	if relations[0].GetRemoteKey() != "code" {
		t.Errorf("expected remote key 'code', got %q", relations[0].GetRemoteKey())
	}
}

func TestRelationBuilder_HasMany(t *testing.T) {
	mockSchema := newMockSchemaForBuilder("documents")

	relations := NewRelationBuilder().
		HasMany("docs", mockSchema).
		LocalKey("id").
		RemoteKey("person_id").
		EntityField("documents_entity").
		Build()

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	rel := relations[0]
	if rel.GetType() != HasMany {
		t.Errorf("expected HasMany, got %v", rel.GetType())
	}
}

func TestRelationBuilder_SetOnParent(t *testing.T) {
	setOnParent := func(parent, child any) any { return parent }

	relations := NewRelationBuilder().
		BelongsTo("vt", nil).
		LocalKey("vehicle_type_id").
		EntityField("vehicle_type_entity").
		SetOnParent(setOnParent).
		Build()

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	r := relations[0]
	if r.GetSetOnParent() == nil {
		t.Error("SetOnParent should not be nil")
	}
}

// Mock schema for testing - implements Schema[any]
type mockSchemaForBuilder struct {
	name   string
	fields Fields
}

func newMockSchemaForBuilder(name string) Schema[any] {
	return &mockSchemaForBuilder{
		name:   name,
		fields: NewFields([]Field{NewIntField("id", WithKey())}),
	}
}

func (m *mockSchemaForBuilder) Name() string                   { return m.name }
func (m *mockSchemaForBuilder) Fields() Fields                 { return m.fields }
func (m *mockSchemaForBuilder) Mapper() FlatMapper[any]        { return nil }
func (m *mockSchemaForBuilder) Validators() []Validator[any]   { return nil }
func (m *mockSchemaForBuilder) Hooks() Hooks[any]              { return nil }
func (m *mockSchemaForBuilder) ToEntities(ctx context.Context, values ...[]FieldValue) ([]any, error) {
	return nil, nil
}
func (m *mockSchemaForBuilder) ToFieldValuesList(ctx context.Context, entities ...any) ([][]FieldValue, error) {
	return nil, nil
}
