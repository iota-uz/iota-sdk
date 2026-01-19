package crud

import (
	"testing"
)

func TestRelationType_String(t *testing.T) {
	tests := []struct {
		rt       RelationType
		expected string
	}{
		{BelongsTo, "belongs_to"},
		{HasMany, "has_many"},
	}

	for _, tt := range tests {
		if got := tt.rt.String(); got != tt.expected {
			t.Errorf("RelationType.String() = %q, want %q", got, tt.expected)
		}
	}
}

func TestRelation_Validate(t *testing.T) {
	mockSchema := newMockSchemaForBuilder("test_table")

	tests := []struct {
		name    string
		rel     *Relation[any]
		wantErr bool
	}{
		{
			name: "valid BelongsTo",
			rel: &Relation[any]{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				EntityField: "vehicle_type_entity",
				Schema:      mockSchema,
			},
			wantErr: false,
		},
		{
			name: "missing alias",
			rel: &Relation[any]{
				Type:        BelongsTo,
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				EntityField: "vehicle_type_entity",
				Schema:      mockSchema,
			},
			wantErr: true,
		},
		{
			name: "missing local key",
			rel: &Relation[any]{
				Type:        BelongsTo,
				Alias:       "vt",
				RemoteKey:   "id",
				EntityField: "vehicle_type_entity",
				Schema:      mockSchema,
			},
			wantErr: true,
		},
		{
			name: "missing entity field for schema relation",
			rel: &Relation[any]{
				Type:      BelongsTo,
				Alias:     "vt",
				LocalKey:  "vehicle_type_id",
				RemoteKey: "id",
				Schema:    mockSchema,
			},
			wantErr: true,
		},
		{
			name: "missing schema and manual",
			rel: &Relation[any]{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				EntityField: "vehicle_type_entity",
			},
			wantErr: true,
		},
		{
			name: "valid manual relation without entity field",
			rel: &Relation[any]{
				Type:     BelongsTo,
				Alias:    "p",
				LocalKey: "person_id",
				Manual:   &ManualRelation{Table: "persons", Columns: []string{"id", "name"}},
			},
			wantErr: false,
		},
		{
			name: "valid nested relation with Through",
			rel: &Relation[any]{
				Type:        BelongsTo,
				Alias:       "vg",
				LocalKey:    "group_id",
				RemoteKey:   "id",
				EntityField: "vehicle_group_entity",
				Through:     "vt",
				Schema:      mockSchema,
			},
			wantErr: false,
		},
		{
			name: "valid HasMany",
			rel: &Relation[any]{
				Type:        HasMany,
				Alias:       "docs",
				LocalKey:    "id",
				RemoteKey:   "person_id",
				EntityField: "documents",
				Schema:      mockSchema,
			},
			wantErr: false,
		},
		{
			name: "defaults RemoteKey to id",
			rel: &Relation[any]{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				EntityField: "vehicle_type_entity",
				Schema:      mockSchema,
			},
			wantErr: false,
		},
		{
			name: "defaults JoinType to LEFT",
			rel: &Relation[any]{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				EntityField: "vehicle_type_entity",
				Schema:      mockSchema,
			},
			wantErr: false,
		},
		{
			name: "manual relation missing table",
			rel: &Relation[any]{
				Type:     BelongsTo,
				Alias:    "p",
				LocalKey: "person_id",
				Manual:   &ManualRelation{Columns: []string{"id"}},
			},
			wantErr: true,
		},
		{
			name: "manual relation missing columns",
			rel: &Relation[any]{
				Type:     BelongsTo,
				Alias:    "p",
				LocalKey: "person_id",
				Manual:   &ManualRelation{Table: "persons"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rel.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Relation.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRelation_Validate_Defaults(t *testing.T) {
	rel := &Relation[any]{
		Type:        BelongsTo,
		Alias:       "vt",
		LocalKey:    "vehicle_type_id",
		EntityField: "vehicle_type_entity",
		Schema:      newMockSchemaForBuilder("vehicle_types"),
	}

	err := rel.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if rel.RemoteKey != "id" {
		t.Errorf("RemoteKey = %q, want %q", rel.RemoteKey, "id")
	}
	if rel.JoinType != JoinTypeLeft {
		t.Errorf("JoinType = %v, want %v", rel.JoinType, JoinTypeLeft)
	}
}

func TestRelation_TableName(t *testing.T) {
	t.Run("schema with Name method", func(t *testing.T) {
		rel := &Relation[any]{Schema: newMockSchemaForBuilder("vehicle_types")}
		if got := rel.TableName(); got != "vehicle_types" {
			t.Errorf("TableName() = %q, want %q", got, "vehicle_types")
		}
	})

	t.Run("nil schema", func(t *testing.T) {
		rel := &Relation[any]{Schema: nil}
		if got := rel.TableName(); got != "" {
			t.Errorf("TableName() = %q, want %q", got, "")
		}
	})

	t.Run("manual relation", func(t *testing.T) {
		rel := &Relation[any]{
			Manual: &ManualRelation{Table: "manual_table"},
		}
		if got := rel.TableName(); got != "manual_table" {
			t.Errorf("TableName() = %q, want %q", got, "manual_table")
		}
	})
}

func TestRelation_WithMapperAndSetOnParent(t *testing.T) {
	setOnParent := func(parent, child any) any {
		return parent
	}

	r := &Relation[any]{
		Alias:       "vt",
		LocalKey:    "vehicle_type_id",
		EntityField: "vehicle_type_entity",
		SetOnParent: setOnParent,
	}

	if r.SetOnParent == nil {
		t.Error("SetOnParent should not be nil")
	}

	result := r.SetOnParent("parent", "child")
	if result != "parent" {
		t.Errorf("SetOnParent returned %v, want 'parent'", result)
	}
}

func TestRelationDescriptor_Interface(t *testing.T) {
	rel := &Relation[any]{
		Type:        BelongsTo,
		Alias:       "vt",
		LocalKey:    "vehicle_type_id",
		RemoteKey:   "id",
		JoinType:    JoinTypeLeft,
		EntityField: "vehicle_type_entity",
		Through:     "parent",
		Manual:      &ManualRelation{Table: "test"},
	}

	var rd RelationDescriptor = rel

	if rd.GetType() != BelongsTo {
		t.Errorf("GetType() = %v, want %v", rd.GetType(), BelongsTo)
	}
	if rd.GetAlias() != "vt" {
		t.Errorf("GetAlias() = %q, want %q", rd.GetAlias(), "vt")
	}
	if rd.GetLocalKey() != "vehicle_type_id" {
		t.Errorf("GetLocalKey() = %q, want %q", rd.GetLocalKey(), "vehicle_type_id")
	}
	if rd.GetRemoteKey() != "id" {
		t.Errorf("GetRemoteKey() = %q, want %q", rd.GetRemoteKey(), "id")
	}
	if rd.GetJoinType() != JoinTypeLeft {
		t.Errorf("GetJoinType() = %v, want %v", rd.GetJoinType(), JoinTypeLeft)
	}
	if rd.GetEntityField() != "vehicle_type_entity" {
		t.Errorf("GetEntityField() = %q, want %q", rd.GetEntityField(), "vehicle_type_entity")
	}
	if rd.GetThrough() != "parent" {
		t.Errorf("GetThrough() = %q, want %q", rd.GetThrough(), "parent")
	}
	if rd.GetManual() == nil {
		t.Error("GetManual() should not be nil")
	}
}
