package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

func valid() manifest.Manifest {
	return manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: "acme.loyalty",
		Resources: []manifest.Resource{
			{ID: "acme.loyalty.card", Kind: manifest.ResEntity},
			{ID: "acme.loyalty.reminder.v1", Kind: manifest.ResNotice},
		},
		Endpoints: []manifest.Endpoint{{ID: "acme.loyalty.hook", Export: "e", Method: "POST", Path: "/hooks/stripe"}},
		Schedules: []manifest.Schedule{{ID: "acme.loyalty.nightly", Export: "e", Cron: "0 3 * * *", Timezone: "Asia/Tashkent"}},
		Templates: []manifest.Template{{Export: "e", Notice: "acme.loyalty.reminder.v1", Channels: []string{"sms", "email", "push", "inApp"}}},
		Guards:    []manifest.Guard{{ID: "acme.loyalty.g", Export: "e", Target: "acme.loyalty.card", Phase: manifest.GuardWrite}},
	}
}

func TestValidateAcceptsTheClosedDomains(t *testing.T) {
	t.Parallel()
	m := valid()
	require.NoError(t, m.Validate())
	for _, method := range []string{"get", "HEAD", "Post", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		m := valid()
		m.Endpoints[0].Method = method
		require.NoError(t, m.Validate(), method)
	}
	m = valid()
	m.Schedules[0].Timezone = ""
	require.NoError(t, m.Validate(), "no zone means UTC")
}

func TestValidateRefusesValuesOutsideTheClosedDomains(t *testing.T) {
	t.Parallel()
	const hostile = "[x](https://evil.example)"
	for name, tc := range map[string]struct {
		mutate func(*manifest.Manifest)
		want   string
	}{
		"endpoint method":      {func(m *manifest.Manifest) { m.Endpoints[0].Method = "POST|" + hostile }, "unsupported method"},
		"endpoint TRACE":       {func(m *manifest.Manifest) { m.Endpoints[0].Method = "TRACE" }, "unsupported method"},
		"endpoint empty":       {func(m *manifest.Manifest) { m.Endpoints[0].Method = "" }, "unsupported method"},
		"endpoint path":        {func(m *manifest.Manifest) { m.Endpoints[0].Path = "hooks" }, "invalid path"},
		"endpoint path spaces": {func(m *manifest.Manifest) { m.Endpoints[0].Path = "/a b" }, "invalid path"},
		"endpoint path tick":   {func(m *manifest.Manifest) { m.Endpoints[0].Path = "/a`b" }, "invalid path"},
		"time zone":            {func(m *manifest.Manifest) { m.Schedules[0].Timezone = hostile }, "unknown time zone"},
		"time zone, made up":   {func(m *manifest.Manifest) { m.Schedules[0].Timezone = "Mars/Olympus" }, "unknown time zone"},
		"template channel":     {func(m *manifest.Manifest) { m.Templates[0].Channels = []string{"sms", hostile} }, "unknown channel"},
		"template channels":    {func(m *manifest.Manifest) { m.Templates[0].Channels = nil }, "no channel"},
		"guard phase":          {func(m *manifest.Manifest) { m.Guards[0].Phase = manifest.GuardPhase(hostile) }, "unknown phase"},
		"resource kind":        {func(m *manifest.Manifest) { m.Resources[0].Kind = manifest.ResourceKind(hostile) }, "unknown kind"},
		"key type":             {func(m *manifest.Manifest) { m.Resources[0].KeyType = hostile }, "unknown key type"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := valid()
			tc.mutate(&m)
			require.ErrorContains(t, m.Validate(), tc.want)
		})
	}
}
