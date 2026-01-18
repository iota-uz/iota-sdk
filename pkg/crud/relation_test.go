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
	tests := []struct {
		name    string
		rel     Relation
		wantErr bool
	}{
		{
			name: "valid BelongsTo",
			rel: Relation{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				EntityField: "vehicle_type_entity",
			},
			wantErr: false,
		},
		{
			name: "missing alias",
			rel: Relation{
				Type:        BelongsTo,
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				EntityField: "vehicle_type_entity",
			},
			wantErr: true,
		},
		{
			name: "missing local key",
			rel: Relation{
				Type:        BelongsTo,
				Alias:       "vt",
				RemoteKey:   "id",
				EntityField: "vehicle_type_entity",
			},
			wantErr: true,
		},
		{
			name: "missing entity field",
			rel: Relation{
				Type:      BelongsTo,
				Alias:     "vt",
				LocalKey:  "vehicle_type_id",
				RemoteKey: "id",
			},
			wantErr: true,
		},
		{
			name: "valid nested relation with Through",
			rel: Relation{
				Type:        BelongsTo,
				Alias:       "vg",
				LocalKey:    "group_id",
				RemoteKey:   "id",
				EntityField: "vehicle_group_entity",
				Through:     "vt",
			},
			wantErr: false,
		},
		{
			name: "valid HasMany",
			rel: Relation{
				Type:        HasMany,
				Alias:       "docs",
				LocalKey:    "id",
				RemoteKey:   "person_id",
				EntityField: "documents",
			},
			wantErr: false,
		},
		{
			name: "defaults RemoteKey to id",
			rel: Relation{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				EntityField: "vehicle_type_entity",
			},
			wantErr: false,
		},
		{
			name: "defaults JoinType to LEFT",
			rel: Relation{
				Type:        BelongsTo,
				Alias:       "vt",
				LocalKey:    "vehicle_type_id",
				RemoteKey:   "id",
				EntityField: "vehicle_type_entity",
			},
			wantErr: false,
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
	rel := Relation{
		Type:        BelongsTo,
		Alias:       "vt",
		LocalKey:    "vehicle_type_id",
		EntityField: "vehicle_type_entity",
	}

	err := rel.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	// Check defaults were applied
	if rel.RemoteKey != "id" {
		t.Errorf("RemoteKey = %q, want %q", rel.RemoteKey, "id")
	}
	if rel.JoinType != JoinTypeLeft {
		t.Errorf("JoinType = %v, want %v", rel.JoinType, JoinTypeLeft)
	}
}

func TestRelation_TableName(t *testing.T) {
	tests := []struct {
		name     string
		schema   any
		expected string
	}{
		{
			name:     "schema with Name method",
			schema:   mockSchema{tableName: "vehicle_types"},
			expected: "vehicle_types",
		},
		{
			name:     "nil schema",
			schema:   nil,
			expected: "",
		},
		{
			name:     "schema without Name method",
			schema:   struct{}{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel := Relation{Schema: tt.schema}
			if got := rel.TableName(); got != tt.expected {
				t.Errorf("TableName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// mockSchema implements interface{ Name() string } for testing
type mockSchema struct {
	tableName string
}

func (m mockSchema) Name() string {
	return m.tableName
}
