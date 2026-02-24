package types

// Citation represents a source citation for generated content.
// This is the canonical Citation type used across the entire bichat package.
//
// Fields from LLM streaming (populated by providers like OpenAI):
//   - Type: citation kind, e.g. "web"
//   - StartIndex / EndIndex: character offsets in the generated text
//
// Fields from domain/persistent context (populated when building messages):
//   - Source: human-readable source identifier (e.g. document name, dataset)
//
// Shared fields (populated in both contexts):
//   - Title, URL, Excerpt
type Citation struct {
	Source     string `json:"source,omitempty"`
	Type       string `json:"type,omitempty"`
	Title      string `json:"title,omitempty"`
	URL        string `json:"url,omitempty"`
	Excerpt    string `json:"excerpt,omitempty"`
	StartIndex int    `json:"start_index,omitempty"`
	EndIndex   int    `json:"end_index,omitempty"`
}

// NewCitation creates a new Citation with the given source metadata.
// Use this constructor when building citations from persistent/domain context
// (e.g. knowledge base results). For LLM streaming annotations, populate
// Type/StartIndex/EndIndex directly.
func NewCitation(source, title, url, excerpt string) Citation {
	return Citation{
		Source:  source,
		Title:   title,
		URL:     url,
		Excerpt: excerpt,
	}
}

// HasURL returns true if the citation has a URL.
func (c Citation) HasURL() bool {
	return c.URL != ""
}

// HasExcerpt returns true if the citation has an excerpt.
func (c Citation) HasExcerpt() bool {
	return c.Excerpt != ""
}
