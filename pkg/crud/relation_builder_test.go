package crud

import (
	"testing"
)

func TestRelationBuilder_BelongsTo(t *testing.T) {
	mockSchema := &mockSchemaForRelation{name: "test_types"}

	relations := NewRelationBuilder().
		BelongsTo("vt", mockSchema).
		LocalKey("type_id").
		EntityField("type_entity").
		Build()

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	rel := relations[0]
	if rel.Type != BelongsTo {
		t.Errorf("expected BelongsTo, got %v", rel.Type)
	}
	if rel.Alias != "vt" {
		t.Errorf("expected alias 'vt', got %q", rel.Alias)
	}
	if rel.LocalKey != "type_id" {
		t.Errorf("expected local key 'type_id', got %q", rel.LocalKey)
	}
	if rel.EntityField != "type_entity" {
		t.Errorf("expected entity field 'type_entity', got %q", rel.EntityField)
	}
	if rel.RemoteKey != "id" {
		t.Errorf("expected default remote key 'id', got %q", rel.RemoteKey)
	}
	if rel.JoinType != JoinTypeLeft {
		t.Errorf("expected default JoinTypeLeft, got %v", rel.JoinType)
	}
}

func TestRelationBuilder_NestedRelation(t *testing.T) {
	mockTypeSchema := &mockSchemaForRelation{name: "types"}
	mockGroupSchema := &mockSchemaForRelation{name: "groups"}

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
	if rel.Through != "vt" {
		t.Errorf("expected Through 'vt', got %q", rel.Through)
	}
}

func TestRelationBuilder_InnerJoin(t *testing.T) {
	mockSchema := &mockSchemaForRelation{name: "required_types"}

	relations := NewRelationBuilder().
		BelongsTo("rt", mockSchema).
		LocalKey("required_type_id").
		EntityField("required_type_entity").
		InnerJoin().
		Build()

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	if relations[0].JoinType != JoinTypeInner {
		t.Errorf("expected JoinTypeInner, got %v", relations[0].JoinType)
	}
}

func TestRelationBuilder_CustomRemoteKey(t *testing.T) {
	mockSchema := &mockSchemaForRelation{name: "items"}

	relations := NewRelationBuilder().
		BelongsTo("i", mockSchema).
		LocalKey("item_code").
		RemoteKey("code").
		EntityField("item_entity").
		Build()

	if relations[0].RemoteKey != "code" {
		t.Errorf("expected remote key 'code', got %q", relations[0].RemoteKey)
	}
}

func TestRelationBuilder_HasMany(t *testing.T) {
	mockSchema := &mockSchemaForRelation{name: "documents"}

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
	if rel.Type != HasMany {
		t.Errorf("expected HasMany, got %v", rel.Type)
	}
}

// Mock schema for testing
type mockSchemaForRelation struct {
	name string
}

func (m *mockSchemaForRelation) Name() string { return m.name }
