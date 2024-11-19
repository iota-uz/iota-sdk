package embedding

type SearchResult struct {
	UUID        string  `json:"uuid"`
	Text        string  `json:"text"`
	ReferenceID string  `json:"reference_id"`
	Score       float64 `json:"score"`
}
