package data

import (
	"fmt"
	"sort"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// NativeField exposes one column of a native table under a stable field ID.
type NativeField struct {
	ID     string
	Column string
	Type   manifest.Type
}

// NativeEntity describes a core/Granite table exposed read-only to packages.
// Writes go through published operations, never through generic CRUD.
type NativeEntity struct {
	ID string
	// Schema defaults to "public".
	Schema       string
	Table        string
	KeyColumn    string
	KeyType      KeyType
	TenantColumn string
	Fields       []NativeField
	// ReadPermission is required to read any row; empty means every
	// authenticated subject.
	ReadPermission string
	// FieldPermissions lists, per field ID, the permissions that must all be
	// held to read the field.
	FieldPermissions map[string][]string
	Label            manifest.Label
}

func (n NativeEntity) schema() string {
	if n.Schema == "" {
		return "public"
	}
	return n.Schema
}

// Resource renders the descriptor as a catalog resource (kind entity), so a
// catalog can publish it in the developer kit.
func (n NativeEntity) Resource() manifest.Resource {
	r := manifest.Resource{ID: n.ID, Kind: manifest.ResEntity, Label: n.Label, KeyType: n.KeyType.Wire()}
	for _, f := range n.Fields {
		r.Fields = append(r.Fields, manifest.Field{ID: f.ID, Type: f.Type})
	}
	acc := &manifest.Access{Read: []manifest.ReadRule{{Permission: n.ReadPermission, All: true}}}
	if len(n.FieldPermissions) > 0 {
		acc.Fields = n.FieldPermissions
	}
	r.Access = acc
	return r
}

func (n NativeEntity) validate() error {
	for name, v := range map[string]string{"schema": n.schema(), "table": n.Table, "key column": n.KeyColumn, "tenant column": n.TenantColumn} {
		if !identRe.MatchString(v) {
			return fmt.Errorf("native %s: invalid %s %q", n.ID, name, v)
		}
	}
	if !n.KeyType.valid() {
		return fmt.Errorf("native %s: invalid key type %q", n.ID, n.KeyType)
	}
	seen := map[string]bool{}
	for _, f := range n.Fields {
		if !fieldKeyRe.MatchString(f.ID) || f.ID == "key" {
			return fmt.Errorf("native %s: invalid field id %q", n.ID, f.ID)
		}
		if seen[f.ID] {
			return fmt.Errorf("native %s: field %q declared twice", n.ID, f.ID)
		}
		seen[f.ID] = true
		if !identRe.MatchString(f.Column) {
			return fmt.Errorf("native %s: invalid column %q", n.ID, f.Column)
		}
		switch f.Type.Kind {
		case manifest.KindObject, manifest.KindArray:
			return fmt.Errorf("native %s: field %q has unsupported type %s", n.ID, f.ID, f.Type.Kind)
		case manifest.KindString, manifest.KindInt, manifest.KindInt64, manifest.KindDecimal,
			manifest.KindBool, manifest.KindDate, manifest.KindDateTime, manifest.KindUUID,
			manifest.KindEnum, manifest.KindJSON, manifest.KindRef:
			// Scalar kinds are all supported.
		}
	}
	for id := range n.FieldPermissions {
		if !seen[id] {
			return fmt.Errorf("native %s: field permission for unknown field %q", n.ID, id)
		}
	}
	return nil
}

// NativeRegistry holds the native entity descriptors.
type NativeRegistry struct {
	mu sync.RWMutex
	m  map[string]NativeEntity
}

func NewNativeRegistry(entities ...NativeEntity) *NativeRegistry {
	r := &NativeRegistry{m: map[string]NativeEntity{}}
	for _, e := range entities {
		if err := r.Register(e); err != nil {
			panic(err)
		}
	}
	return r
}

// Register adds a descriptor; IDs are unique.
func (r *NativeRegistry) Register(e NativeEntity) error {
	if err := e.validate(); err != nil {
		return fail("extruntime.data.NativeRegistry.Register", ErrIncompatibleSchema, "%v", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.m[e.ID]; dup {
		return fail("extruntime.data.NativeRegistry.Register", ErrIncompatibleSchema, "native entity %s registered twice", e.ID)
	}
	r.m[e.ID] = e
	return nil
}

func (r *NativeRegistry) Get(id string) (NativeEntity, bool) {
	if r == nil {
		return NativeEntity{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.m[id]
	return e, ok
}

// List returns the descriptors sorted by ID.
func (r *NativeRegistry) List() []NativeEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]NativeEntity, 0, len(r.m))
	for _, e := range r.m {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Native entity IDs shipped by the SDK.
const (
	NativeUser   = "core.user"
	NativeClient = "crm.client"
	// NativeClientList is the public ID of the CRM clients list, which
	// packages extend with columns.
	NativeClientList = "crm.lists.clients"
)

func str(maxLen int) manifest.Type {
	return manifest.Type{Kind: manifest.KindString, MaxLength: maxLen}
}
func optStr(maxLen int) manifest.Type {
	return manifest.Type{Kind: manifest.KindString, MaxLength: maxLen, Optional: true, Nullable: true}
}

// CoreUser describes modules/core `users` (key: bigint).
func CoreUser() NativeEntity {
	return NativeEntity{
		ID: NativeUser, Table: "users", KeyColumn: "id", KeyType: KeyInt64, TenantColumn: "tenant_id",
		ReadPermission: "User.Read",
		Label:          manifest.Label{"en": "User", "ru": "Пользователь", "uz": "Foydalanuvchi"},
		Fields: []NativeField{
			{"firstName", "first_name", str(255)},
			{"lastName", "last_name", str(255)},
			{"middleName", "middle_name", optStr(255)},
			{"email", "email", str(255)},
			{"phone", "phone", optStr(255)},
			{"uiLanguage", "ui_language", str(16)},
			{"createdAt", "created_at", manifest.Type{Kind: manifest.KindDateTime}},
			{"updatedAt", "updated_at", manifest.Type{Kind: manifest.KindDateTime}},
		},
	}
}

// CRMClient describes modules/crm `clients` (key: bigint). Personal
// identification (pin, passport) is deliberately not exposed.
func CRMClient() NativeEntity {
	return NativeEntity{
		ID: NativeClient, Table: "clients", KeyColumn: "id", KeyType: KeyInt64, TenantColumn: "tenant_id",
		ReadPermission: "Client.Read",
		Label:          manifest.Label{"en": "Client", "ru": "Клиент", "uz": "Mijoz"},
		Fields: []NativeField{
			{"firstName", "first_name", str(255)},
			{"lastName", "last_name", optStr(255)},
			{"middleName", "middle_name", optStr(255)},
			{"phone", "phone_number", optStr(255)},
			{"address", "address", manifest.Type{Kind: manifest.KindString, Optional: true, Nullable: true}},
			{"email", "email", optStr(255)},
			{"dateOfBirth", "date_of_birth", manifest.Type{Kind: manifest.KindDate, Optional: true, Nullable: true}},
			{"gender", "gender", optStr(15)},
			{"comments", "comments", manifest.Type{Kind: manifest.KindString, Optional: true, Nullable: true}},
			{"createdAt", "created_at", manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
			{"updatedAt", "updated_at", manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
		},
	}
}

// DefaultNatives registers the SDK's native descriptors.
func DefaultNatives() *NativeRegistry {
	return NewNativeRegistry(CoreUser(), CRMClient())
}
