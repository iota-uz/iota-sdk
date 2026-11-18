package runtime_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
)

var pathParam = regexp.MustCompile(`\{([^}]+)\}`)

// concretePath fills the parameters of an OpenAPI path with values that
// parse (a uuid for ids) but name nothing.
func concretePath(path string) string {
	return pathParam.ReplaceAllStringFunc(path, func(p string) string {
		switch p {
		case "{id}":
			return "123e4567-e89b-12d3-a456-426614174000"
		case "{ns}":
			return "acme.none"
		case "{path}":
			return "a/b.txt"
		case "{module}":
			return "@iota-kit/core.tgz"
		}
		return "x"
	})
}

// The management routes table is the source of the API's rights: a caller
// without the permission a route declares is refused before the handler runs,
// and the permission it declares is enough to get past that check.
func TestEveryManagementRouteEnforcesItsDeclaredPermission(t *testing.T) {
	e := newAPIEnv(t)
	const stranger = 3
	audits, alternatives := runtime.RouteAudits(), runtime.RouteAlternatives()

	refused := make([]string, 0, len(runtime.ManageRoutes()))
	for _, r := range runtime.ManageRoutes() {
		key := r.Method + " " + r.Path
		path := strings.TrimPrefix(concretePath(r.Path), runtime.APIPrefix)
		if r.Permission == "" {
			continue
		}
		if alternatives[key] != "" {
			// The service decides: the assignee of a task may decide without the permission.
			status, _, _ := e.do(stranger, r.Method, path, map[string]any{"decision": "x"})
			require.NotEqual(t, http.StatusForbidden, status, "%s has an alternative to %s and leaves the decision to the service", key, r.Permission)
			continue
		}

		var body any
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			body = []byte("{}")
		}
		status, out, _ := e.do(stranger, r.Method, path, body)
		require.Equal(t, http.StatusForbidden, status, "%s must refuse a caller without %s: %s", key, r.Permission, out)
		var env struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(out, &env), string(out))
		require.Contains(t, env.Error.Message, r.Permission, key)
		refused = append(refused, key)

		// holding exactly the declared permission gets past the check (the
		// call itself may still fail: nothing is installed here)
		status, out, _ = e.do(stranger, r.Method, path, body, "X-Perms", r.Permission)
		require.NotEqual(t, http.StatusForbidden, status, "%s must accept %s alone: %s", key, r.Permission, out)
	}
	require.GreaterOrEqual(t, len(refused), 30, "the table's permissioned routes were exercised")

	// A refusal is audited like the service's own: under the route's action.
	denied := map[string]bool{}
	rows, err := e.db.Query(t.Context(), `SELECT action FROM extruntime.audit_log WHERE outcome = 'denied' AND actor_user_id = $1`, int64(stranger))
	require.NoError(t, err)
	for rows.Next() {
		var action string
		require.NoError(t, rows.Scan(&action))
		denied[action] = true
	}
	require.NoError(t, rows.Err())
	rows.Close()
	for _, key := range refused {
		if action := audits[key]; action != "" {
			require.True(t, denied[action], "%s: a refused request is audited as %q", key, action)
		}
	}
}

func TestEveryMutatingRouteNamesItsAuditAction(t *testing.T) {
	t.Parallel()
	alternatives := runtime.RouteAlternatives()
	for key, action := range runtime.RouteAudits() {
		if strings.HasPrefix(key, http.MethodGet+" ") || alternatives[key] != "" {
			continue
		}
		require.NotEmpty(t, action, "%s changes state: its refusals must be audited", key)
	}
}

func TestEmptyListsAreEncodedAsArrays(t *testing.T) {
	e := newAPIEnv(t)
	for _, path := range []string{"/packages", "/bindings", "/owners", "/audit"} {
		status, out, _ := e.do(1, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, status, path)
		require.Equal(t, "[]", strings.TrimSpace(string(out)), "%s: an empty list is [], not null", path)
	}
}

func TestRetryTakesNoResult(t *testing.T) {
	e := newAPIEnv(t)
	const path = "/workflows/123e4567-e89b-12d3-a456-426614174000/retry"
	e.json(1, http.MethodPost, path, map[string]any{"name": "step", "n": 1, "result": map[string]any{}}, http.StatusBadRequest, nil)
	// without it the request is well formed; no engine is attached in this test
	e.json(1, http.MethodPost, path, map[string]any{"name": "step", "n": 1}, http.StatusConflict, nil)
}

type emptyProbe struct {
	List    []string          `json:"list"`
	Omitted []string          `json:"omitted,omitempty"`
	Map     map[string]string `json:"map"`
	Inner   *struct {
		Deep []int `json:"deep"`
	} `json:"inner"`
	Rows []struct {
		Cells []int `json:"cells"`
	} `json:"rows"`
	Raw  json.RawMessage `json:"raw"`
	Blob []byte          `json:"blob"`
	When time.Time       `json:"when"`
	Any  any             `json:"any"`
	ID   uuid.UUID       `json:"id"`
}

func TestWithEmptySlicesTouchesOnlyNilSlicesAndMaps(t *testing.T) {
	t.Parallel()
	in := emptyProbe{Inner: &struct {
		Deep []int `json:"deep"`
	}{}, Rows: []struct {
		Cells []int `json:"cells"`
	}{{}}, Any: []string(nil)}
	got, err := json.Marshal(runtime.WithEmptySlices(in))
	require.NoError(t, err)
	require.JSONEq(t, `{"list":[],"map":{},"inner":{"deep":[]},"rows":[{"cells":[]}],"raw":null,"blob":null,
		"when":"0001-01-01T00:00:00Z","any":[],"id":"00000000-0000-0000-0000-000000000000"}`, string(got))
	require.Nil(t, in.List, "the input is not modified")
	require.Nil(t, in.Inner.Deep)

	for _, v := range []any{nil, "text", 7, []string{"a"}, map[string]int{"a": 1}} {
		want, err := json.Marshal(v)
		require.NoError(t, err)
		got, err := json.Marshal(runtime.WithEmptySlices(v))
		require.NoError(t, err)
		require.JSONEq(t, string(want), string(got))
	}
}

// A refusal made by the transport reads like one made by the service: the
// namespace, version and target the request named are in the audit row, also
// when only the body carries them.
func TestRefusedRequestsAreAuditedWithTheFieldsOfTheirBody(t *testing.T) {
	e := newAPIEnv(t)
	e.json(3, http.MethodPost, "/packages", map[string]string{"namespace": "acme.audited"}, http.StatusForbidden, nil)
	e.json(3, http.MethodPost, "/packages/acme.audited/publish", map[string]string{"version": "2.0.0"}, http.StatusForbidden, nil)
	e.json(3, http.MethodPost, "/packages/acme.audited/move", map[string]string{"from": "a.ts", "to": "b.ts"}, http.StatusForbidden, nil)
	e.json(3, http.MethodGet, "/packages/acme.audited/export?version=1.0.0", nil, http.StatusForbidden, nil)

	type row struct{ Action, Namespace, Version, Target, Outcome string }
	rows, err := e.db.Query(t.Context(), `SELECT action, namespace, version, target, outcome FROM extruntime.audit_log WHERE actor_user_id = 3 ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	var got []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.Action, &r.Namespace, &r.Version, &r.Target, &r.Outcome))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []row{
		{"package.create", "acme.audited", "", "", "denied"},
		{"publish", "acme.audited", "2.0.0", "", "denied"},
		{"draft.move", "acme.audited", "", "a.ts -> b.ts", "denied"},
		{"export", "acme.audited", "1.0.0", "version", "denied"},
	}, got)
}
