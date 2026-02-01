package domain

// Citation represents a source reference in an AI response.
// This is a struct (not interface) following idiomatic Go patterns.
type Citation struct {
	Source  string
	Title   string
	URL     string
	Excerpt string
}

// NewCitation creates a new citation with the given parameters
func NewCitation(source, title, url, excerpt string) Citation {
	return Citation{
		Source:  source,
		Title:   title,
		URL:     url,
		Excerpt: excerpt,
	}
}

// HasURL returns true if the citation has a URL
func (c Citation) HasURL() bool {
	return c.URL != ""
}

// HasExcerpt returns true if the citation has an excerpt
func (c Citation) HasExcerpt() bool {
	return c.Excerpt != ""
}
