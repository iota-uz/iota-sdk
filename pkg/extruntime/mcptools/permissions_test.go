package mcptools_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/mcptools"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

// dummy builds an input that satisfies the schema's shape (the values are
// placeholders: the tool is expected to be refused or to fail on them, never
// to succeed).
func dummy(s *jsonschema.Schema, text string) any {
	if s == nil {
		return text
	}
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	switch s.Type {
	case "object":
		out := map[string]any{}
		for _, name := range s.Required {
			out[name] = dummy(s.Properties[name], text)
		}
		return out
	case "array":
		return []any{}
	case "integer", "number":
		return 1
	case "boolean":
		return false
	}
	for _, t := range s.Types {
		if t != "null" {
			return dummy(&jsonschema.Schema{Type: t}, text)
		}
	}
	return text
}

func extPermission(name string) permission.Permission {
	for _, p := range []permission.Permission{corePerms.ExtensionsRead, corePerms.ExtensionsAuthor, corePerms.ExtensionsReview, corePerms.ExtensionsPublish, corePerms.ExtensionsOperate} {
		if p.Name() == name {
			return p
		}
	}
	panic("unknown permission " + name)
}

// Every management tool tells MCP clients which RBAC permissions it needs
// (Spec.Meta and the description). The service is what enforces them, so the
// advertised list must be what the service really checks: with one advertised
// permission missing the tool is refused, and with exactly the advertised
// ones it is not refused for rights.
func TestAdvertisedPermissionsAreWhatTheServiceEnforces(t *testing.T) {
	e := newEnv(t)
	all := []string{manage.PermRead, manage.PermAuthor, manage.PermReview, manage.PermPublish, manage.PermOperate}
	checked, skipped := make([]string, 0, 64), []string{}
	next := uint(100)
	actor := func(perms []string) uint {
		next++
		ps := make([]permission.Permission, 0, len(perms))
		for _, p := range perms {
			ps = append(ps, extPermission(p))
		}
		e.users[next] = account(t, e.tenant, next, ps...)
		return next
	}
	for _, spec := range e.engine.Registry().Specs() {
		advertised, ok := spec.Meta[mcptools.MetaPermissions].([]string)
		if !ok || len(advertised) == 0 || !slices.ContainsFunc(advertised, func(p string) bool { return slices.Contains(all, p) }) {
			continue
		}
		// The permissions listed in the description are the same as in the meta.
		for _, p := range advertised {
			require.Contains(t, spec.Description, p, "%s: the description names every permission of its meta", spec.Name)
		}
		var input any
		determinable := false
		// A placeholder text of the shapes the tools take: a namespace, then an ID.
		for _, text := range []string{"acme.perm", "7c9e6679-7425-40de-944b-e07fc1f90ae7"} {
			raw, err := json.Marshal(dummy(spec.Input, text))
			require.NoError(t, err, spec.Name)
			require.NoError(t, json.Unmarshal(raw, &input))
			determinable = true
			for _, missing := range advertised {
				held := slices.DeleteFunc(slices.Clone(all), func(p string) bool { return p == missing })
				res := e.call(actor(held), spec.Name, input)
				require.False(t, res.OK, "%s without %s", spec.Name, missing)
				if res.Error.Code == mcp.CodeInvalidInput {
					// The placeholder input is refused before rights are looked at: try the next shape.
					determinable = false
					break
				}
				require.Equal(t, mcp.CodeForbidden, res.Error.Code,
					"%s advertises %s but is not refused without it (%+v); the permission it really enforces is another one", spec.Name, missing, res.Error)
			}
			if determinable {
				break
			}
		}
		if !determinable {
			skipped = append(skipped, spec.Name)
			continue
		}
		res := e.call(actor(advertised), spec.Name, input)
		if !res.OK {
			require.NotEqual(t, mcp.CodeForbidden, res.Error.Code,
				"%s advertises %v but refuses a user holding exactly those: %+v", spec.Name, advertised, res.Error)
		}
		checked = append(checked, spec.Name)
	}
	t.Logf("checked %d tools; input not expressible for %v", len(checked), skipped)
	require.GreaterOrEqual(t, len(checked), 25, "most management tools can be checked; skipped: %v", skipped)
	// Not vacuous: every service-enforced management tool took part.
	for _, want := range []string{"ext_publish", "ext_activate", "ext_deactivate", "ext_purge", "ext_review", "ext_bind", "ext_package_list", "ext_file_write"} {
		require.Contains(t, checked, want, fmt.Sprint(checked))
	}
}

// The management API authenticates with the session cookie, which any site
// can make a browser send: a state-changing call from another site is refused
// before it reaches the service (the configurator's own calls are same-origin).
func TestManageAPIRefusesCrossSiteStateChanges(t *testing.T) {
	e := newEnv(t)
	create := func(ns string, headers ...string) int {
		status, _ := e.http(adminID, "POST", "/packages", map[string]string{"namespace": ns}, headers...)
		return status
	}
	for name, headers := range map[string][]string{
		"a cross-site fetch":      {"Sec-Fetch-Site", "cross-site"},
		"a same-site fetch":       {"Sec-Fetch-Site", "same-site"},
		"an older browser's POST": {"Origin", "https://evil.example.net"},
	} {
		require.Equal(t, 403, create("acme.cross", headers...), name)
	}
	status, _ := e.http(adminID, "GET", "/packages/acme.cross", nil)
	require.Equal(t, 404, status, "nothing was created by the refused calls")

	status, body := e.http(adminID, "POST", "/packages", map[string]string{"namespace": "acme.cross"}, "Sec-Fetch-Site", "cross-site")
	require.Equal(t, 403, status)
	require.Contains(t, string(body), "runtime.crossSite")

	require.Less(t, create("acme.cross", "Sec-Fetch-Site", "same-origin"), 300, "the same origin may")
	status, _ = e.http(adminID, "GET", "/packages/acme.cross", nil, "Sec-Fetch-Site", "cross-site")
	require.Equal(t, 200, status, "a safe method is not state-changing")
}
