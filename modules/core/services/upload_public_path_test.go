package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/modules/core/services"
)

// One predicate for the download route and for whoever hands out its links: inside the public root, outside the
// private namespace, compared as the route compares them.
func TestPublicUploadPath(t *testing.T) {
	for name, tc := range map[string]struct {
		root, path string
		want       bool
	}{
		"a public file":                      {"static", "static/abc.png", true},
		"a nested public file":               {"static", "static/2026/abc.png", true},
		"an absolute root":                   {"/srv/uploads", "/srv/uploads/abc.png", true},
		"the private namespace":              {"static", "static/.private/abc.png", false},
		"the private directory itself":       {"static", "static/.private", false},
		"outside the root":                   {"static", "elsewhere/abc.png", false},
		"a root with the same prefix":        {"static", "static2/abc.png", false},
		"a root that leaves the working dir": {"../uploads", "../uploads/abc.txt", false}, // the route normalizes the path and answers 404
		"no root":                            {"", "static/abc.png", false},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, services.PublicUploadPath(tc.root, tc.path))
		})
	}
}
