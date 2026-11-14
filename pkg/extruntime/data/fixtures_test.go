package data_test

import (
	"encoding/json"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

const (
	customerID = ns + ".customer"
	policyID   = ns + ".policy"
	extraID    = ns + ".clientExtra"
)

func ty(kind manifest.TypeKind) manifest.Type { return manifest.Type{Kind: kind} }

func opt(t manifest.Type) manifest.Type { t.Optional, t.Nullable = true, true; return t }

func raw(v any) json.RawMessage { return js(v) }

func enumT(vals ...string) manifest.Type {
	return manifest.Type{Kind: manifest.KindEnum, Values: vals}
}

func refT(entity string) manifest.Type { return manifest.Type{Kind: manifest.KindRef, Entity: entity} }

func ptr[T any](v T) *T { return &v }

// insuranceV1 is the reference package: a customer, a policy with every
// storable type, and an overlay on crm.client.
func insuranceV1() *manifest.Manifest {
	return &manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: ns,
		Resources: []manifest.Resource{
			{ID: customerID, Kind: manifest.ResEntity, Fields: []manifest.Field{
				{ID: "name", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 100}},
				{ID: "owner", Type: opt(refT("core.user"))},
			}},
			{ID: policyID, Kind: manifest.ResEntity,
				Fields: []manifest.Field{
					{ID: "number", Type: ty(manifest.KindString), Unique: true},
					{ID: "holder", Type: refT(customerID)},
					{ID: "premium", Type: manifest.Type{Kind: manifest.KindDecimal, Precision: 18, Scale: 2}},
					{ID: "limit", Type: ty(manifest.KindInt64)},
					{ID: "seats", Type: opt(ty(manifest.KindInt))},
					{ID: "expiresOn", Type: ty(manifest.KindDate), Index: true},
					{ID: "issuedAt", Type: opt(ty(manifest.KindDateTime))},
					{ID: "status", Type: enumT("draft", "active", "expired"), Default: raw("draft")},
					{ID: "active", Type: opt(ty(manifest.KindBool))},
					{ID: "notes", Type: opt(ty(manifest.KindString))},
					{ID: "extra", Type: opt(ty(manifest.KindJSON))},
					{ID: "secret", Type: opt(ty(manifest.KindString))},
				},
				Access: &manifest.Access{
					Read: []manifest.ReadRule{
						{Permission: "Policy.ReadAll", All: true},
						{Rows: &manifest.Filter{Fields: map[string]manifest.Condition{
							"holder.owner": {Eq: raw(map[string]string{"$": "user"})},
						}}},
					},
					Fields: map[string][]string{"secret": {"Policy.Secret"}},
				}},
			{ID: extraID, Kind: manifest.ResOverlay, Base: "crm.client", Fields: []manifest.Field{
				{ID: "loyaltyTier", Type: opt(enumT("none", "silver", "gold")), Default: raw("none")},
				{ID: "score", Type: opt(ty(manifest.KindInt))},
				{ID: "vip", Type: opt(ty(manifest.KindString))},
			}, Access: &manifest.Access{Fields: map[string][]string{"vip": {"Extra.Vip"}}}},
		},
	}
}
