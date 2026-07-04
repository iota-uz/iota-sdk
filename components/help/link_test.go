package help

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDocURL(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		path     string
		want     string
	}{
		{
			name: "default base",
			path: "modules/crm.md",
			want: "/help/doc/modules/crm.md",
		},
		{
			name:     "custom base",
			basePath: "/support",
			path:     "modules/crm.md",
			want:     "/support/doc/modules/crm.md",
		},
		{
			name: "doc-prefixed path",
			path: "doc/modules/crm.md",
			want: "/help/doc/modules/crm.md",
		},
		{
			name: "absolute local path",
			path: "/help/doc/modules/crm.md",
			want: "/help/doc/modules/crm.md",
		},
		{
			name: "absolute external path",
			path: "https://example.com/help",
			want: "https://example.com/help",
		},
		{
			name: "empty path",
			want: "/help",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, DocURL(tt.basePath, tt.path))
		})
	}
}

func TestLink_Render(t *testing.T) {
	var buf bytes.Buffer
	err := Link(LinkProps{
		Path:    "granite/osago/policy-issue.md",
		Label:   "Open OSAGO policy issuing help",
		Tooltip: "Read policy issuing guidance",
		NewTab:  true,
	}).Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	require.Contains(t, html, `href="/help/doc/granite/osago/policy-issue.md"`)
	require.Contains(t, html, `aria-label="Open OSAGO policy issuing help"`)
	require.Contains(t, html, `x-tooltip.raw="Read policy issuing guidance"`)
	require.Contains(t, html, `target="_blank"`)
	require.Contains(t, html, `rel="noopener noreferrer"`)
}
