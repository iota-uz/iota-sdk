package types

// Citation represents a source citation for generated content.
type Citation struct {
	Type       string `json:"type"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Excerpt    string `json:"excerpt"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
}
