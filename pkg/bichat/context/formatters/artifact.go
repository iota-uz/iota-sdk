package formatters

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// ArtifactListFormatter formats artifact list results as markdown tables.
type ArtifactListFormatter struct{}

// NewArtifactListFormatter creates a new artifact list formatter.
func NewArtifactListFormatter() *ArtifactListFormatter {
	return &ArtifactListFormatter{}
}

// Format renders an ArtifactListPayload as markdown.
func (f *ArtifactListFormatter) Format(payload any, opts context.FormatOptions) (string, error) {
	p, ok := payload.(ArtifactListPayload)
	if !ok {
		return "", fmt.Errorf("ArtifactListFormatter: expected ArtifactListPayload, got %T", payload)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Artifacts (page %d/%d)\n", p.Page, p.TotalPages)
	b.WriteString("| id | type | name | mime | size_bytes | created_at |\n")
	b.WriteString("| --- | --- | --- | --- | ---: | --- |\n")

	for _, a := range p.Artifacts {
		fmt.Fprintf(&b,
			"| %s | %s | %s | %s | %d | %s |\n",
			a.ID,
			a.Type,
			escapeTableCell(a.Name),
			escapeTableCell(a.MimeType),
			a.SizeBytes,
			a.CreatedAt,
		)
	}

	fmt.Fprintf(&b, "\nhas_next_page: %t", p.HasNext)
	if p.HasNext {
		fmt.Fprintf(&b, " (use page=%d)", p.Page+1)
	}
	if p.HitCap {
		b.WriteString("\n\nNote: artifact listing reached tool cap and may be truncated.")
	}

	return b.String(), nil
}

// ArtifactContentFormatter formats artifact content for reading.
type ArtifactContentFormatter struct{}

// NewArtifactContentFormatter creates a new artifact content formatter.
func NewArtifactContentFormatter() *ArtifactContentFormatter {
	return &ArtifactContentFormatter{}
}

// Format renders an ArtifactContentPayload as markdown.
func (f *ArtifactContentFormatter) Format(payload any, opts context.FormatOptions) (string, error) {
	p, ok := payload.(ArtifactContentPayload)
	if !ok {
		return "", fmt.Errorf("ArtifactContentFormatter: expected ArtifactContentPayload, got %T", payload)
	}

	var b strings.Builder
	b.WriteString("## Artifact Read\n")
	fmt.Fprintf(&b, "- id: %s\n", p.ID)
	fmt.Fprintf(&b, "- type: %s\n", p.Type)
	fmt.Fprintf(&b, "- name: %s\n", p.Name)
	fmt.Fprintf(&b, "- mime: %s\n", p.MimeType)
	fmt.Fprintf(&b, "- page: %d/%d\n", p.Page, p.TotalPages)
	fmt.Fprintf(&b, "- page_size: %d\n\n", p.PageSize)

	if p.OutOfRange {
		b.WriteString("Requested page is out of range for this artifact content.\n")
	} else if p.Content == "" {
		b.WriteString("(no content on this page)\n")
	} else {
		b.WriteString(p.Content)
		b.WriteString("\n")
	}

	if p.HasNext {
		fmt.Fprintf(&b, "\nhas_next_page: true (use page=%d)", p.Page+1)
	} else {
		b.WriteString("\nhas_next_page: false")
	}

	return b.String(), nil
}

func escapeTableCell(value string) string {
	replacer := strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ")
	return replacer.Replace(value)
}
