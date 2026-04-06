package spec

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/stretchr/testify/require"
)

func TestLoadRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	_, err := Load([]byte(`{
		"id": "sales-report",
		"title": "Sales report",
		"unknownField": true
	}`))
	require.Error(t, err)
	require.ErrorContains(t, err, "lens.spec.Load")
	require.ErrorContains(t, err, "unknown field")
}

func TestLoadParsesVariableComponentOverride(t *testing.T) {
	t.Parallel()

	doc, err := Load([]byte(`{
		"id": "sales-report",
		"title": "Sales report",
		"variables": [
			{
				"name": "product",
				"label": "Product",
				"kind": "single_select",
				"component": "text_input"
			}
		]
	}`))
	require.NoError(t, err)
	require.Len(t, doc.Variables, 1)
	require.Equal(t, string(lens.VariableComponentTextInput), doc.Variables[0].Component)
}

func TestDocumentValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		doc     Document
		wantErr string
	}{
		{
			name: "rejects blank translated title",
			doc: Document{
				ID:    "sales-report",
				Title: Text{Translations: map[string]string{"en": " ", "ru": "\t"}},
			},
			wantErr: "document title is required",
		},
		{
			name: "rejects invalid body position",
			doc: Document{
				ID:           "sales-report",
				Title:        LiteralText("Sales"),
				BodyPosition: BodyPosition("preprend"),
			},
			wantErr: `unsupported bodyPosition "preprend"`,
		},
		{
			name: "accepts supported body position",
			doc: Document{
				ID:           "sales-report",
				Title:        LiteralText("Sales"),
				BodyPosition: BodyPositionPrepend,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.doc.Validate()
			if testCase.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.EqualError(t, err, testCase.wantErr)
		})
	}
}
