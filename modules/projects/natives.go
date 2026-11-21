package projects

import (
	"github.com/iota-uz/iota-sdk/modules/projects/permissions"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// AcceptanceEntity describes the projects.acceptance entity (table
// project_acceptance_documents).
func AcceptanceEntity() data.NativeEntity {
	return data.NativeEntity{
		ID: services.AcceptanceResource, Table: "project_acceptance_documents", KeyColumn: "id", KeyType: data.KeyUUID, TenantColumn: "tenant_id",
		ReadPermission: permissions.ProjectRead.Name(),
		Label: manifest.Label{
			"en": "Acceptance document", "ru": "Акт приёмки", "uz": "Qabul hujjati", "uz-Cyrl": "Қабул ҳужжати",
		},
		Fields: []data.NativeField{
			{ID: "projectId", Column: "project_id", Type: manifest.Type{Kind: manifest.KindUUID}},
			{ID: "kind", Column: "kind", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 32}},
			{ID: "number", Column: "number", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 64}},
			{ID: "date", Column: "document_date", Type: manifest.Type{Kind: manifest.KindDate}},
			{ID: "signedOn", Column: "signed_on", Type: manifest.Type{Kind: manifest.KindDate, Optional: true, Nullable: true}},
			{ID: "amount", Column: "amount", Type: manifest.Type{Kind: manifest.KindInt64}},
			{ID: "currency", Column: "currency_id", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 3}},
			{ID: "status", Column: "status", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 16}},
			{ID: "createdAt", Column: "created_at", Type: manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
			{ID: "updatedAt", Column: "updated_at", Type: manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
		},
	}
}

// Natives is the projects module's native registration: the
// projects.acceptance entity and its signed event. Applications compose it into
// their native.Set (modules.Natives).
func Natives(s *native.Set) error {
	return s.Add(append([]native.Item{native.Entity(AcceptanceEntity())}, services.Events()...)...)
}
