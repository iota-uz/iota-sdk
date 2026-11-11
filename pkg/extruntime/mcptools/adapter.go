package mcptools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// actorOf derives the acting subject of a call: the principal's user with the
// effective permissions the authorizer just read from the account, never from
// model arguments. The credential names the MCP client for audit.
func actorOf(ctx context.Context) (spi.Subject, error) {
	p, ok := mcp.PrincipalFrom(ctx)
	if !ok || p.UserID == 0 {
		return spi.Subject{}, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryReauthenticate, "extension management needs a signed-in user")
	}
	sub, err := runtime.SubjectFromContext(ctx)
	if err != nil || sub.UserID != int64(p.UserID) || sub.TenantID != p.TenantID {
		return spi.Subject{}, mcp.Fail(mcp.CodeForbidden, mcp.RecoveryReauthenticate, "the account is not available")
	}
	client := p.ClientID
	if client == "" {
		client = "personal"
	}
	sub.Credential = "mcp:" + client
	return sub, nil
}

// failure maps a service error onto the MCP contract. Management messages are
// written for authors and safe to show; internal errors are not.
func failure(err error) error {
	if err == nil {
		return nil
	}
	var f *mcp.Failure
	if errors.As(err, &f) {
		return f
	}
	out := *mcp.FailureFrom(err)
	if serrors.CodeOf(err) == serrors.Internal {
		return &out
	}
	out.Message = leafMessage(err)
	var de *manage.DiagnosticsError
	var se *data.SchemaError
	switch {
	case errors.As(err, &de):
		out.Issues = append(out.Issues, diagnosticIssues(de.Diagnostics)...)
		out.Recovery = mcp.RecoveryFixInput
	case errors.As(err, &se):
		for _, d := range se.Diagnostics {
			path := d.Resource
			if d.Field != "" {
				path += "." + d.Field
			}
			out.Issues = append(out.Issues, mcp.Issue{Path: path, Code: d.Code, Message: d.Message})
		}
		out.Recovery = mcp.RecoveryFixInput
	}
	return &out
}

func diagnosticIssues(ds []build.Diagnostic) []mcp.Issue {
	out := make([]mcp.Issue, 0, len(ds))
	for _, d := range ds {
		out = append(out, mcp.Issue{Path: diagnosticPath(d), Code: d.Code, Message: d.Message})
	}
	return out
}

// diagnosticPath is "file:line:column", the form editors and agents jump to.
func diagnosticPath(d build.Diagnostic) string {
	switch {
	case d.Line > 0 && d.Column > 0:
		return fmt.Sprintf("%s:%d:%d", d.File, d.Line, d.Column)
	case d.Line > 0:
		return fmt.Sprintf("%s:%d", d.File, d.Line)
	}
	return d.File
}

func leafMessage(err error) string {
	parts := strings.Split(err.Error(), ": ")
	for len(parts) > 1 && !strings.ContainsAny(parts[0], " ") && strings.Contains(parts[0], ".") {
		parts = parts[1:]
	}
	return strings.Join(parts, ": ")
}

// schemaOptions maps the types whose JSON form differs from their Go shape.
func schemaOptions() (*jsonschema.ForOptions, error) {
	anything := &jsonschema.Schema{}
	opts := &jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[uuid.UUID](): {Type: "string", Format: "uuid"},
		// raw JSON documents are described by the resource contract, not here
		reflect.TypeFor[json.RawMessage](): anything,
		// the manifest is a large recursive document; ext_version_get documents it
		reflect.TypeFor[manifest.Manifest](): {Type: "object", Description: "iota.manifest/1 document of the version."},
		reflect.TypeFor[manifest.Type]():     {Type: "object", Description: "Wire type description."},
		reflect.TypeFor[manifest.Resource](): {Type: "object", Description: "Resource contract as declared in the manifest."},
	}}
	// VersionDetail embeds Version and adds the manifest; the inferrer does not
	// flatten embedded structs, so the flat shape is stated here.
	version, err := jsonschema.For[manage.Version](opts)
	if err != nil {
		return nil, err
	}
	detail := version.CloneSchemas()
	detail.Properties["manifest"] = opts.TypeSchemas[reflect.TypeFor[manifest.Manifest]()]
	detail.Required = append(detail.Required, "manifest")
	opts.TypeSchemas[reflect.TypeFor[manage.VersionDetail]()] = detail
	return opts, nil
}

func schemaOf[T any]() (*jsonschema.Schema, error) {
	opts, err := schemaOptions()
	if err != nil {
		return nil, err
	}
	return jsonschema.For[T](opts)
}

// add registers a typed tool whose handler receives the acting subject.
func add[In, Out any](r *registrar, spec mcp.Spec, h func(context.Context, spi.Subject, In) (Out, error)) {
	in, err := schemaOf[In]()
	if err != nil {
		r.err = errors.Join(r.err, fmt.Errorf("input schema of %s: %w", spec.Name, err))
		return
	}
	spec.Input = in
	if reflect.TypeFor[Out]().Kind() != reflect.Interface {
		out, err := schemaOf[Out]()
		if err != nil {
			r.err = errors.Join(r.err, fmt.Errorf("output schema of %s: %w", spec.Name, err))
			return
		}
		spec.Output = out
	}
	r.add(mcp.New(spec, func(ctx context.Context, in In) (Out, error) {
		var zero Out
		actor, err := actorOf(ctx)
		if err != nil {
			return zero, err
		}
		out, err := h(ctx, actor, in)
		if err != nil {
			return zero, failure(err)
		}
		return out, nil
	}))
}

func parseUUID(field, v string) (uuid.UUID, error) {
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "invalid input",
			mcp.Issue{Path: field, Code: "uuid", Message: field + " must be a UUID"})
	}
	return id, nil
}

// ack is the result of actions that return nothing.
type ack struct {
	OK bool `json:"ok"`
}

var done = ack{OK: true}
