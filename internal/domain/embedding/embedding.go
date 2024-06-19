package embedding

type SearchResult struct {
	Uuid        string  `json:"uuid"`
	Text        string  `json:"text"`
	ReferenceId string  `json:"reference_id"`
	Score       float64 `json:"score"`
}
