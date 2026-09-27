package crud

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type descriptorEntity struct{}

func TestProjectSchema_ExposesOnlySemanticAllowList(t *testing.T) {
	t.Parallel()

	schema := NewSchema[descriptorEntity]("policies", NewFields([]Field{
		NewUUIDField("id", WithKey(), WithReadonly(), WithAttrs(map[string]any{"unsafe": func() {}})),
		NewStringField("number", WithSearchable(), WithSortable(), WithLocalizationKey("Policies.Fields.Number")),
		NewStringField("secret", WithHidden()),
		NewStringField("status", WithRenderer("status-chip")),
	}), struct{ Mapper[descriptorEntity] }{})
	descriptor, err := ProjectSchema(schema,
		WithDescriptorActions(ActionDescriptor{ID: "list"}, ActionDescriptor{ID: "delete", Mutation: true, Destructive: true}),
		WithDescriptorEscapeHatches("status-chip", "status-chip"),
	)
	require.NoError(t, err)
	require.Equal(t, "policies", descriptor.ID())
	require.Equal(t, []string{"id", "number", "status"}, descriptor.View().ListFields())
	require.Equal(t, []string{"number", "status"}, descriptor.View().FormFields())
	require.Equal(t, []string{"status-chip"}, descriptor.EscapeHatchIDs())

	payload, err := json.Marshal(descriptor)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id":"policies","translationKey":"policies",
		"fields":[
			{"id":"id","translationKey":"policies.Fields.id","type":"uuid","key":true,"readonly":true},
			{"id":"number","translationKey":"Policies.Fields.Number","type":"string","searchable":true,"sortable":true},
			{"id":"secret","translationKey":"policies.Fields.secret","type":"string","hidden":true},
			{"id":"status","translationKey":"policies.Fields.status","type":"string","rendererId":"status-chip"}
		],
		"views":{"list":["id","number","status"],"details":["id","number","status"],"form":["number","status"]},
		"actions":[{"id":"list"},{"id":"delete","mutation":true,"destructive":true}],
		"escapeHatchIds":["status-chip"]
	}`, string(payload))
	require.NotContains(t, string(payload), "unsafe")
}

func TestResourceDescriptor_AccessorsAreDefensive(t *testing.T) {
	t.Parallel()

	schema := NewSchema[descriptorEntity]("items", NewFields([]Field{NewUUIDField("id", WithKey())}), struct{ Mapper[descriptorEntity] }{})
	descriptor, err := ProjectSchema(schema)
	require.NoError(t, err)
	fields := descriptor.Fields()
	fields[0].ID = "changed"
	view := descriptor.View().ListFields()
	view[0] = "changed"
	require.Equal(t, "id", descriptor.Fields()[0].ID)
	require.Equal(t, "id", descriptor.View().ListFields()[0])
}
