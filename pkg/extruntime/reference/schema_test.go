package reference_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
)

func compile(t *testing.T, doc any) *jsonschema.Schema {
	t.Helper()
	return compileFormat(t, doc, true)
}

// compileFormat compiles doc, asserting "format" (as strict validators do) or
// treating it as an annotation (the 2020-12 default).
func compileFormat(t *testing.T, doc any, assertFormat bool) *jsonschema.Schema {
	t.Helper()
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	if assertFormat {
		c.AssertFormat()
	}
	require.NoError(t, c.AddResource("mem://schema.json", parsed))
	sch, err := c.Compile("mem://schema.json")
	require.NoError(t, err, "schema must compile: %s", raw)
	return sch
}

func validates(t *testing.T, sch *jsonschema.Schema, raw string) error {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(raw))
	require.NoError(t, err)
	return sch.Validate(inst)
}

func prop(name string, t manifest.Type) manifest.Property {
	return manifest.Property{Name: name, Type: t}
}

// every kind, with the modifiers that change the wire rules
func richType() manifest.Type {
	return manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
		prop("name", manifest.Type{Kind: manifest.KindString, MaxLength: 10}),
		prop("count", manifest.Type{Kind: manifest.KindInt}),
		prop("big", manifest.Type{Kind: manifest.KindInt64}),
		prop("price", manifest.Type{Kind: manifest.KindDecimal, Precision: 8, Scale: 2}),
		prop("price0", manifest.Type{Kind: manifest.KindDecimal}),
		prop("price1", manifest.Type{Kind: manifest.KindDecimal, Precision: 1, Scale: 3}),
		prop("active", manifest.Type{Kind: manifest.KindBool}),
		prop("on", manifest.Type{Kind: manifest.KindDate}),
		prop("at", manifest.Type{Kind: manifest.KindDateTime}),
		prop("id", manifest.Type{Kind: manifest.KindUUID}),
		prop("status", manifest.Type{Kind: manifest.KindEnum, Values: []string{"open", "closed"}}),
		prop("meta", manifest.Type{Kind: manifest.KindJSON}),
		prop("owner", manifest.Type{Kind: manifest.KindRef, Entity: "core.user"}),
		prop("anyref", manifest.Type{Kind: manifest.KindRef}),
		prop("tags", manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindString}}),
		prop("note", manifest.Type{Kind: manifest.KindString, Optional: true}),
		prop("nick", manifest.Type{Kind: manifest.KindString, Nullable: true}),
		prop("kind", manifest.Type{Kind: manifest.KindEnum, Values: []string{"a"}, Nullable: true}),
		prop("limit", manifest.Type{Kind: manifest.KindInt, Optional: true, Nullable: true}),
		prop("inner", manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
			prop("v", manifest.Type{Kind: manifest.KindInt}),
			prop("w", manifest.Type{Kind: manifest.KindBool, Optional: true}),
		}}),
		prop("rows", manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
			prop("owner", manifest.Type{Kind: manifest.KindRef, Entity: "core.user", Nullable: true}),
		}}}),
	}}
}

func TestExamplePassesServerValidationAndSchema(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		typ  manifest.Type
	}{
		{"rich object", richType()},
		{"string", manifest.Type{Kind: manifest.KindString, MaxLength: 2}},
		{"decimal with precision under scale", manifest.Type{Kind: manifest.KindDecimal, Precision: 1, Scale: 4}},
		{"nullable enum", manifest.Type{Kind: manifest.KindEnum, Values: []string{"x", "y"}, Nullable: true}},
		{"array of arrays", manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindInt64}}}},
		{"json", manifest.Type{Kind: manifest.KindJSON}},
		{"ref to anything", manifest.Type{Kind: manifest.KindRef}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ex := reference.Example(tc.typ)
			require.NoError(t, manifest.ValidateValue(tc.typ, ex), "example %s", ex)
			sch := compile(t, reference.RootSchema(tc.typ))
			require.NoError(t, validates(t, sch, string(ex)), "example %s", ex)
		})
	}
}

func TestExampleWithKeepsRefKeysOfTheirEntity(t *testing.T) {
	t.Parallel()
	typ := manifest.Type{Kind: manifest.KindRef, Entity: "core.user"}
	ex := reference.ExampleWith(typ, func(entity string) string {
		require.Equal(t, "core.user", entity)
		return "42"
	})
	require.JSONEq(t, `{"entity":"core.user","key":"42"}`, string(ex))
	require.NoError(t, manifest.ValidateValue(typ, ex))
}

func TestSchemaAndServerAgreeOnWireRules(t *testing.T) {
	t.Parallel()
	obj := func(ps ...manifest.Property) manifest.Type {
		return manifest.Type{Kind: manifest.KindObject, Properties: ps}
	}
	cases := []struct {
		name  string
		typ   manifest.Type
		value string
		valid bool
	}{
		{"int ok", manifest.Type{Kind: manifest.KindInt}, `9007199254740991`, true},
		{"int above safe range", manifest.Type{Kind: manifest.KindInt}, `9007199254740992`, false},
		{"int as a quoted number", manifest.Type{Kind: manifest.KindInt}, `"1"`, false},
		{"int64 string", manifest.Type{Kind: manifest.KindInt64}, `"-9223372036854775808"`, true},
		{"int64 as number", manifest.Type{Kind: manifest.KindInt64}, `1`, false},
		{"int64 leading zero", manifest.Type{Kind: manifest.KindInt64}, `"01"`, false},
		{"decimal ok", manifest.Type{Kind: manifest.KindDecimal, Precision: 5, Scale: 2}, `"123.45"`, true},
		{"decimal too many fraction digits", manifest.Type{Kind: manifest.KindDecimal, Precision: 5, Scale: 2}, `"1.234"`, false},
		{"decimal small value under a high scale", manifest.Type{Kind: manifest.KindDecimal, Precision: 1, Scale: 3}, `"0.001"`, true},
		{"decimal leading zeros are not digits", manifest.Type{Kind: manifest.KindDecimal, Precision: 2, Scale: 9}, `"0.000000012"`, true},
		{"decimal long zero fraction without a scale", manifest.Type{Kind: manifest.KindDecimal, Precision: 2}, `"0.00000000000000000000005"`, true},
		{"decimal as number", manifest.Type{Kind: manifest.KindDecimal}, `1.5`, false},
		{"decimal exponent", manifest.Type{Kind: manifest.KindDecimal}, `"1e5"`, false},
		{"date ok", manifest.Type{Kind: manifest.KindDate}, `"2024-02-29"`, true},
		{"date with time", manifest.Type{Kind: manifest.KindDate}, `"2024-02-29T00:00:00Z"`, false},
		{"datetime ok", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29T10:00:00.123+05:00"`, true},
		{"datetime without offset", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29T10:00:00"`, false},
		{"datetime lower-case t", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29t10:00:00Z"`, false},
		{"datetime lower-case z", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29T10:00:00z"`, false},
		{"datetime space for T", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29 10:00:00Z"`, false},
		{"datetime offset without a colon", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29T10:00:00+0500"`, false},
		{"datetime fraction without digits", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29T10:00:00.Z"`, false},
		{"datetime nanoseconds", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29T10:00:00.123456789Z"`, true},
		{"datetime more than nanosecond digits", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29T10:00:00.1234567890123Z"`, true},
		{"datetime impossible day", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-30T10:00:00Z"`, false},
		{"array without an item type rejects an empty array", manifest.Type{Kind: manifest.KindArray}, `[]`, false},
		{"array without an item type rejects items", manifest.Type{Kind: manifest.KindArray}, `[1]`, false},
		{"array without an item type rejects null", manifest.Type{Kind: manifest.KindArray}, `null`, false},
		{"nullable array without an item type takes null", manifest.Type{Kind: manifest.KindArray, Nullable: true}, `null`, true},
		{"nullable array without an item type rejects []", manifest.Type{Kind: manifest.KindArray, Nullable: true}, `[]`, false},
		{"int with a fraction written out", manifest.Type{Kind: manifest.KindInt}, `1.5`, false},
		{"uuid ok", manifest.Type{Kind: manifest.KindUUID}, `"123e4567-e89b-12d3-a456-426614174000"`, true},
		{"uuid without hyphens", manifest.Type{Kind: manifest.KindUUID}, `"123e4567e89b12d3a456426614174000"`, false},
		{"enum member", manifest.Type{Kind: manifest.KindEnum, Values: []string{"a", "b"}}, `"b"`, true},
		{"enum stranger", manifest.Type{Kind: manifest.KindEnum, Values: []string{"a", "b"}}, `"c"`, false},
		{"string too long", manifest.Type{Kind: manifest.KindString, MaxLength: 3}, `"abcd"`, false},
		{"string in runes", manifest.Type{Kind: manifest.KindString, MaxLength: 2}, `"яя"`, true},
		{"null for required", manifest.Type{Kind: manifest.KindString}, `null`, false},
		{"null for nullable", manifest.Type{Kind: manifest.KindString, Nullable: true}, `null`, true},
		{"null for nullable enum", manifest.Type{Kind: manifest.KindEnum, Values: []string{"a"}, Nullable: true}, `null`, true},
		{"json null is a missing value", manifest.Type{Kind: manifest.KindJSON}, `null`, false},
		{"json any", manifest.Type{Kind: manifest.KindJSON}, `{"a":[1]}`, true},
		{"ref to the right entity", manifest.Type{Kind: manifest.KindRef, Entity: "core.user"}, `{"entity":"core.user","key":"1"}`, true},
		{"ref to another entity", manifest.Type{Kind: manifest.KindRef, Entity: "core.user"}, `{"entity":"crm.client","key":"1"}`, false},
		{"ref without key", manifest.Type{Kind: manifest.KindRef}, `{"entity":"core.user"}`, false},
		{"ref with empty key", manifest.Type{Kind: manifest.KindRef}, `{"entity":"core.user","key":""}`, false},
		{"array item wrong", manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindBool}}, `[true,1]`, false},
		{"object ok", obj(prop("a", manifest.Type{Kind: manifest.KindInt})), `{"a":1}`, true},
		{"object missing required", obj(prop("a", manifest.Type{Kind: manifest.KindInt})), `{}`, false},
		{"object optional absent", obj(prop("a", manifest.Type{Kind: manifest.KindInt, Optional: true})), `{}`, true},
		{"object optional null", obj(prop("a", manifest.Type{Kind: manifest.KindInt, Optional: true})), `{"a":null}`, false},
		{"object optional nullable null", obj(prop("a", manifest.Type{Kind: manifest.KindInt, Optional: true, Nullable: true})), `{"a":null}`, true},
		{"object unknown property", obj(prop("a", manifest.Type{Kind: manifest.KindInt})), `{"a":1,"b":2}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sch := compile(t, reference.RootSchema(tc.typ))
			serverErr := manifest.ValidateValue(tc.typ, json.RawMessage(tc.value))
			schemaErr := validates(t, sch, tc.value)
			if tc.valid {
				require.NoError(t, serverErr, "server")
				require.NoError(t, schemaErr, "schema")
				return
			}
			require.Error(t, serverErr, "server must reject %s", tc.value)
			require.Error(t, schemaErr, "schema must reject %s", tc.value)
		})
	}
}

// TestSchemaAndServerKnownDivergences pins the places where the schema cannot
// say what the server enforces (or where a strict validator is stricter than
// the server). Each is stated in the schema's description; a divergence that
// closes makes this test fail, so it is removed rather than forgotten.
func TestSchemaAndServerKnownDivergences(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		typ         manifest.Type
		value       string
		format      bool // assert "format" when validating
		serverValid bool
		schemaValid bool
		described   string // the description names the divergence
	}{
		// JSON Schema validates the parsed number: 1.0 and 1e2 are the integers
		// 1 and 100, but the server accepts bare integer literals only (its
		// handlers decode them into integers, which refuse the other spellings).
		{"int written as 1.0", manifest.Type{Kind: manifest.KindInt}, `1.0`, true, false, true, "1.0"},
		{"int written as 1e2", manifest.Type{Kind: manifest.KindInt}, `1e2`, true, false, true, "1e2"},
		// Significant digits ignore leading zeros: no length bound states that,
		// and a pattern for it needs a lookahead.
		{"decimal over its precision", manifest.Type{Kind: manifest.KindDecimal, Precision: 3, Scale: 2}, `"12345.6"`, true, false, true, "3 significant digits"},
		// Go parses these; RFC 3339 "format" validators are stricter.
		{"datetime with a comma fraction", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29T10:00:00,5Z"`, true, true, false, ""},
		{"datetime with a +24:00 offset", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29T10:00:00+24:00"`, true, true, false, ""},
		// Without format assertion the same values pass the pattern, as on the server.
		{"datetime with a comma fraction, pattern only", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29T10:00:00,5Z"`, false, true, true, ""},
		{"datetime with a +24:00 offset, pattern only", manifest.Type{Kind: manifest.KindDateTime}, `"2024-02-29T10:00:00+24:00"`, false, true, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			schema := reference.TypeSchema(tc.typ)
			sch := compileFormat(t, reference.RootSchema(tc.typ), tc.format)
			require.Equal(t, tc.serverValid, manifest.ValidateValue(tc.typ, json.RawMessage(tc.value)) == nil, "server on %s", tc.value)
			require.Equal(t, tc.schemaValid, validates(t, sch, tc.value) == nil, "schema on %s", tc.value)
			if tc.described != "" {
				require.Contains(t, schema["description"], tc.described)
			}
		})
	}
}

func TestDecimalSchemaStatesPrecisionAndScaleWithoutALengthBound(t *testing.T) {
	t.Parallel()
	s := reference.TypeSchema(manifest.Type{Kind: manifest.KindDecimal, Precision: 8, Scale: 2})
	require.NotContains(t, s, "maxLength", "leading zeros are not significant digits, so no length bound is right")
	require.Contains(t, s["pattern"], `{1,2}`, "the scale is enforced")
	require.Contains(t, s["description"], "at most 2 fractional digits")
	require.Contains(t, s["description"], "at most 8 significant digits")
}

func TestNullableUsesATypeUnion(t *testing.T) {
	t.Parallel()
	s := reference.TypeSchema(manifest.Type{Kind: manifest.KindDate, Nullable: true})
	require.Equal(t, []any{"string", "null"}, s["type"])
	s = reference.TypeSchema(manifest.Type{Kind: manifest.KindEnum, Values: []string{"a"}, Nullable: true})
	require.Equal(t, []any{"a", nil}, s["enum"])
}

func TestObjectSchemaIsClosedAndListsRequired(t *testing.T) {
	t.Parallel()
	s := reference.TypeSchema(manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
		prop("a", manifest.Type{Kind: manifest.KindInt}),
		prop("b", manifest.Type{Kind: manifest.KindInt, Optional: true}),
	}})
	require.Equal(t, false, s["additionalProperties"])
	require.Equal(t, []any{"a"}, s["required"])
}

func TestSignature(t *testing.T) {
	t.Parallel()
	require.Equal(t,
		"{ id: uuid, note?: string(255), price: decimal(8,2) | null, tags: string[], who: ref<core.user>, st: enum(a|b) }",
		reference.Signature(manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
			prop("id", manifest.Type{Kind: manifest.KindUUID}),
			prop("note", manifest.Type{Kind: manifest.KindString, MaxLength: 255, Optional: true}),
			prop("price", manifest.Type{Kind: manifest.KindDecimal, Precision: 8, Scale: 2, Nullable: true}),
			prop("tags", manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindString}}),
			prop("who", manifest.Type{Kind: manifest.KindRef, Entity: "core.user"}),
			prop("st", manifest.Type{Kind: manifest.KindEnum, Values: []string{"a", "b"}}),
		}}))
}

// compileAt compiles the schema at pointer inside a whole OpenAPI document.
func compileAt(t *testing.T, doc map[string]any, pointer string) *jsonschema.Schema {
	t.Helper()
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	require.NoError(t, c.AddResource("mem://openapi.json", parsed))
	sch, err := c.Compile("mem://openapi.json" + pointer)
	require.NoError(t, err)
	return sch
}
