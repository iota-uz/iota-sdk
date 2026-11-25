package composition

import "testing"

// A route variable with a pattern matches only what its pattern does, as the router: a literal
// sibling route (/automations/tasks) never takes the policy of /automations/{id:<uuid>}.
func TestRoutePatternMatchesHonorsVariablePatterns(t *testing.T) {
	t.Parallel()
	uuid := `/automations/{id:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}}`
	for _, tc := range []struct {
		pattern, path string
		want          bool
	}{
		{uuid, "/automations/tasks", false},
		{uuid, "/automations/6f0c1c1e-0000-4000-8000-000000000001", true},
		{"/users/{id:[0-9]+}", "/users/42", true},
		{"/users/{id:[0-9]+}", "/users/new", false},
		{"/reports/{id}", "/reports/anything", true},
		{"/reports/{id:(}", "/reports/anything", true}, // an invalid pattern constrains nothing
	} {
		if got := routePatternMatches(tc.pattern, tc.path); got != tc.want {
			t.Errorf("%s ~ %s = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}
