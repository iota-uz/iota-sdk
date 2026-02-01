package codecs

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// KBSearchResult represents a single knowledge base search result.
type KBSearchResult struct {
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
	Source  string  `json:"source,omitempty"`
}

// KBSearchResultsPayload represents knowledge base search results block.
type KBSearchResultsPayload struct {
	Query   string           `json:"query"`
	Results []KBSearchResult `json:"results"`
	TopK    int              `json:"top_k"`
}

// KBSearchResultsCodec handles knowledge base search results blocks.
type KBSearchResultsCodec struct {
	*context.BaseCodec
}

// NewKBSearchResultsCodec creates a new KB search results codec.
func NewKBSearchResultsCodec() *KBSearchResultsCodec {
	return &KBSearchResultsCodec{
		BaseCodec: context.NewBaseCodec("kb-search-results", "1.0.0"),
	}
}

// Validate validates the KB search results payload.
func (c *KBSearchResultsCodec) Validate(payload any) error {
	switch v := payload.(type) {
	case KBSearchResultsPayload:
		if v.Query == "" {
			return fmt.Errorf("search query cannot be empty")
		}
		return nil
	case map[string]any:
		if query, ok := v["query"].(string); !ok || query == "" {
			return fmt.Errorf("search query cannot be empty")
		}
		return nil
	default:
		return fmt.Errorf("invalid KB search results payload type: %T", payload)
	}
}

// Canonicalize converts the payload to canonical form.
func (c *KBSearchResultsCodec) Canonicalize(payload any) ([]byte, error) {
	var results KBSearchResultsPayload

	switch v := payload.(type) {
	case KBSearchResultsPayload:
		results = v
	case map[string]any:
		if query, ok := v["query"].(string); ok {
			results.Query = normalizeWhitespace(query)
		}
		if res, ok := v["results"].([]any); ok {
			for _, r := range res {
				if resMap, ok := r.(map[string]any); ok {
					result := KBSearchResult{}
					if title, ok := resMap["title"].(string); ok {
						result.Title = title
					}
					if content, ok := resMap["content"].(string); ok {
						result.Content = normalizeWhitespace(content)
					}
					if score, ok := resMap["score"].(float64); ok {
						result.Score = score
					}
					if source, ok := resMap["source"].(string); ok {
						result.Source = source
					}
					results.Results = append(results.Results, result)
				}
			}
		}
		if topK, ok := v["top_k"].(int); ok {
			results.TopK = topK
		}
	default:
		return nil, fmt.Errorf("invalid KB search results payload type: %T", payload)
	}

	return context.SortedJSONBytes(results)
}
