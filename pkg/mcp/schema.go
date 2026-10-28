package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	validator "github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var issuePrinter = message.NewPrinter(language.English)

// compiledSchema validates instances against a published schema and reports
// every violation with its JSON pointer, so callers can fix all of them at once.
type compiledSchema struct{ schema *validator.Schema }

func compileSchema(s *jsonschema.Schema) (*compiledSchema, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	doc, err := validator.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	c := validator.NewCompiler()
	c.DefaultDraft(validator.Draft2020)
	if err := c.AddResource("urn:iota:mcp:schema", doc); err != nil {
		return nil, err
	}
	compiled, err := c.Compile("urn:iota:mcp:schema")
	if err != nil {
		return nil, err
	}
	return &compiledSchema{schema: compiled}, nil
}

func (c *compiledSchema) validate(input json.RawMessage) []Issue {
	if len(bytes.TrimSpace(input)) == 0 {
		input = json.RawMessage("{}")
	}
	inst, err := validator.UnmarshalJSON(bytes.NewReader(input))
	if err != nil {
		return []Issue{{Path: "", Code: "json", Message: "input is not valid JSON"}}
	}
	err = c.schema.Validate(inst)
	if err == nil {
		return nil
	}
	var verr *validator.ValidationError
	if !errors.As(err, &verr) {
		return []Issue{{Path: "", Code: "schema", Message: err.Error()}}
	}
	var issues []Issue
	collectIssues(verr, &issues)
	return issues
}

func collectIssues(e *validator.ValidationError, out *[]Issue) {
	if len(e.Causes) > 0 {
		for _, cause := range e.Causes {
			collectIssues(cause, out)
		}
		return
	}
	code := "schema"
	if kw := e.ErrorKind.KeywordPath(); len(kw) > 0 {
		code = kw[len(kw)-1]
	}
	*out = append(*out, Issue{
		Path:    pointer(e.InstanceLocation),
		Code:    code,
		Message: e.ErrorKind.LocalizedString(issuePrinter),
	})
}

func pointer(location []string) string {
	if len(location) == 0 {
		return ""
	}
	escaped := make([]string, len(location))
	for i, tok := range location {
		escaped[i] = strings.ReplaceAll(strings.ReplaceAll(tok, "~", "~0"), "/", "~1")
	}
	return "/" + strings.Join(escaped, "/")
}
