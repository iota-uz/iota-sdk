package openapi

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const docURL = "mem://openapi.json"

// Validate checks a generated document: it is JSON, declares OpenAPI 3.1, every
// "$ref" resolves inside the document, every operation has a unique
// operationId, every path parameter is declared, and every schema (the
// components and each inline request, response and parameter schema) compiles
// as a JSON Schema 2020-12 schema.
func Validate(raw []byte) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("openapi: not valid JSON: %w", err)
	}
	root, ok := doc.(map[string]any)
	if !ok {
		return errors.New("openapi: the document is not an object")
	}
	if v, _ := root["openapi"].(string); !strings.HasPrefix(v, "3.1") {
		return fmt.Errorf("openapi: version %q, want 3.1.x", v)
	}
	var errs []error
	checkRefs(root, root, "#", &errs)
	errs = append(errs, checkOperations(root)...)

	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	if err := c.AddResource(docURL, doc); err != nil {
		return fmt.Errorf("openapi: %w", err)
	}
	for _, loc := range schemaLocations(root) {
		if _, err := c.Compile(docURL + loc); err != nil {
			errs = append(errs, fmt.Errorf("schema %s: %w", loc, err))
		}
	}
	return errors.Join(errs...)
}

// checkRefs reports every "$ref" that does not resolve to a member of root.
func checkRefs(root map[string]any, v any, at string, errs *[]error) {
	switch x := v.(type) {
	case map[string]any:
		if ref, ok := x["$ref"].(string); ok {
			if !strings.HasPrefix(ref, "#/") || !resolves(root, ref) {
				*errs = append(*errs, fmt.Errorf("%s: $ref %q does not resolve", at, ref))
			}
		}
		for k, child := range x {
			checkRefs(root, child, at+"/"+escapePointer(k), errs)
		}
	case []any:
		for i, child := range x {
			checkRefs(root, child, fmt.Sprintf("%s/%d", at, i), errs)
		}
	}
}

func resolves(root map[string]any, ref string) bool {
	var cur any = root
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		part = strings.NewReplacer("~1", "/", "~0", "~").Replace(part)
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		if cur, ok = m[part]; !ok {
			return false
		}
	}
	return true
}

func escapePointer(s string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
}

func checkOperations(root map[string]any) []error {
	var errs []error
	ids := map[string]string{}
	paths, _ := root["paths"].(map[string]any)
	for _, p := range sortedKeys(paths) {
		item, _ := paths[p].(map[string]any)
		for _, method := range sortedKeys(item) {
			op, _ := item[method].(map[string]any)
			where := strings.ToUpper(method) + " " + p
			if id, ok := op["operationId"].(string); ok {
				if prev, dup := ids[id]; dup {
					errs = append(errs, fmt.Errorf("operationId %q is used by %s and %s", id, prev, where))
				}
				ids[id] = where
			}
			declared := map[string]bool{}
			if params, ok := op["parameters"].([]any); ok {
				for _, raw := range params {
					if pm, ok := raw.(map[string]any); ok && pm["in"] == "path" {
						name, _ := pm["name"].(string)
						declared[name] = true
					}
				}
			}
			for _, name := range pathParams(p) {
				if !declared[name] {
					errs = append(errs, fmt.Errorf("%s: path parameter {%s} is not declared", where, name))
				}
			}
			if len(op) > 0 {
				if _, ok := op["responses"]; !ok {
					errs = append(errs, fmt.Errorf("%s: no responses", where))
				}
			}
		}
	}
	return errs
}

// pathParams lists the {name} placeholders of an OpenAPI path.
func pathParams(path string) []string {
	var out []string
	for {
		i := strings.IndexByte(path, '{')
		if i < 0 {
			return out
		}
		j := strings.IndexByte(path[i:], '}')
		if j < 0 {
			return out
		}
		out = append(out, path[i+1:i+j])
		path = path[i+j+1:]
	}
}

// PathParams is pathParams for the packages that declare operations.
func PathParams(path string) []string { return pathParams(path) }

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// schemaLocations lists the JSON pointers of every schema in the document.
func schemaLocations(root map[string]any) []string {
	var locs []string
	schemaIn := func(base string, v any) {
		if m, ok := v.(map[string]any); ok {
			if s, ok := m["schema"]; ok {
				_ = s
				locs = append(locs, base+"/schema")
			}
		}
	}
	if comps, ok := root["components"].(map[string]any); ok {
		schemas, _ := comps["schemas"].(map[string]any)
		for _, n := range sortedKeys(schemas) {
			locs = append(locs, "#/components/schemas/"+escapePointer(n))
		}
		responses, _ := comps["responses"].(map[string]any)
		for _, n := range sortedKeys(responses) {
			resp, _ := responses[n].(map[string]any)
			content, _ := resp["content"].(map[string]any)
			for _, ct := range sortedKeys(content) {
				schemaIn("#/components/responses/"+escapePointer(n)+"/content/"+escapePointer(ct), content[ct])
			}
		}
	}
	paths, _ := root["paths"].(map[string]any)
	for _, p := range sortedKeys(paths) {
		item, _ := paths[p].(map[string]any)
		for _, method := range sortedKeys(item) {
			op, _ := item[method].(map[string]any)
			base := "#/paths/" + escapePointer(p) + "/" + method
			if params, ok := op["parameters"].([]any); ok {
				for i, raw := range params {
					schemaIn(fmt.Sprintf("%s/parameters/%d", base, i), raw)
				}
			}
			if body, ok := op["requestBody"].(map[string]any); ok {
				content, _ := body["content"].(map[string]any)
				for _, ct := range sortedKeys(content) {
					schemaIn(base+"/requestBody/content/"+escapePointer(ct), content[ct])
				}
			}
			responses, _ := op["responses"].(map[string]any)
			for _, status := range sortedKeys(responses) {
				resp, _ := responses[status].(map[string]any)
				content, _ := resp["content"].(map[string]any)
				for _, ct := range sortedKeys(content) {
					schemaIn(base+"/responses/"+status+"/content/"+escapePointer(ct), content[ct])
				}
			}
		}
	}
	return locs
}
