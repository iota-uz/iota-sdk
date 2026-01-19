package crud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelationBuilder_BelongsTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		alias       string
		schemaName  string
		localKey    string
		entityField string
		remoteKey   string
		joinType    JoinType
	}{
		{
			name:        "basic BelongsTo with defaults",
			alias:       "vt",
			schemaName:  "test_types",
			localKey:    "type_id",
			entityField: "type_entity",
			remoteKey:   "id",
			joinType:    JoinTypeLeft,
		},
		{
			name:        "BelongsTo with different alias",
			alias:       "cat",
			schemaName:  "categories",
			localKey:    "category_id",
			entityField: "category_entity",
			remoteKey:   "id",
			joinType:    JoinTypeLeft,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockSchema := newMockSchemaForBuilder(tt.schemaName)

			relations := NewRelationBuilder().
				BelongsTo(tt.alias, mockSchema).
				LocalKey(tt.localKey).
				EntityField(tt.entityField).
				Build()

			require.Len(t, relations, 1)

			rel := relations[0]
			assert.Equal(t, BelongsTo, rel.GetType())
			assert.Equal(t, tt.alias, rel.GetAlias())
			assert.Equal(t, tt.localKey, rel.GetLocalKey())
			assert.Equal(t, tt.entityField, rel.GetEntityField())
			assert.Equal(t, tt.remoteKey, rel.GetRemoteKey())
			assert.Equal(t, tt.joinType, rel.GetJoinType())
		})
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

func (m *mockSchemaForBuilder) Name() string                 { return m.name }
func (m *mockSchemaForBuilder) Fields() Fields               { return m.fields }
func (m *mockSchemaForBuilder) Mapper() FlatMapper[any]      { return nil }
func (m *mockSchemaForBuilder) Validators() []Validator[any] { return nil }
func (m *mockSchemaForBuilder) Hooks() Hooks[any]            { return nil }
func (m *mockSchemaForBuilder) ToEntities(ctx context.Context, values ...[]FieldValue) ([]any, error) {
	return nil, nil
}
func (m *mockSchemaForBuilder) ToFieldValuesList(ctx context.Context, entities ...any) ([][]FieldValue, error) {
	return nil, nil
}
