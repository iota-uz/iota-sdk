package types

// Citation represents a source citation for generated content.
type Citation struct {
	// Type indicates the type of citation (e.g., "web", "document", "database")
	Type string

	// Title is the title of the cited source
	Title string

	// URL is the URL of the cited source (if applicable)
	URL string

	// Excerpt is a relevant excerpt from the cited source
	Excerpt string

	// StartIndex is the character position where the citation starts in the content
	StartIndex int

	// EndIndex is the character position where the citation ends in the content
	EndIndex int
}
