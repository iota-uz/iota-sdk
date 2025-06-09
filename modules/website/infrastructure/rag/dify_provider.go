package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type SearchMethod string

const (
	KeywordSearch  SearchMethod = "keyword_search"
	SemanticSearch SearchMethod = "semantic_search"
	FullTextSearch SearchMethod = "full_text_search"
	HybridSearch   SearchMethod = "hybrid_search"
)

type DifyProvider struct {
	baseURL        string
	apiKey         string
	datasetID      string
	retrievalModel RetrievalModel
	client         *http.Client
}

type DifyConfig struct {
	BaseURL        string
	APIKey         string
	DatasetID      string
	RetrievalModel *RetrievalModel
}

type RetrievalModel struct {
	SearchMethod          SearchMethod   `json:"search_method"`
	RerankingEnable       bool           `json:"reranking_enable"`
	RerankingMode         *RerankingMode `json:"reranking_mode"`
	Weights               *float64       `json:"weights"`
	TopK                  int            `json:"top_k"`
	ScoreThresholdEnabled bool           `json:"score_threshold_enabled"`
	ScoreThreshold        *float64       `json:"score_threshold"`
}

type RerankingMode struct {
	RerankingProviderName string `json:"reranking_provider_name"`
	RerankingModelName    string `json:"reranking_model_name"`
}

type DifyRequest struct {
	Query          string         `json:"query"`
	RetrievalModel RetrievalModel `json:"retrieval_model"`
}

type DifyResponse struct {
	Query   Query    `json:"query"`
	Records []Record `json:"records"`
}

type Query struct {
	Content string `json:"content"`
}

type Record struct {
	Segment Segment `json:"segment"`
	Score   float64 `json:"score"`
}

type Segment struct {
	ID         string   `json:"id"`
	Position   int      `json:"position"`
	DocumentID string   `json:"document_id"`
	Content    string   `json:"content"`
	Answer     *string  `json:"answer"`
	WordCount  int      `json:"word_count"`
	Tokens     int      `json:"tokens"`
	Keywords   []string `json:"keywords"`
	Document   Document `json:"document"`
}

type Document struct {
	ID             string `json:"id"`
	DataSourceType string `json:"data_source_type"`
	Name           string `json:"name"`
}

func NewDifyProvider(config DifyConfig) *DifyProvider {
	retrievalModel := RetrievalModel{
		SearchMethod:          KeywordSearch,
		RerankingEnable:       false,
		RerankingMode:         nil,
		Weights:               nil,
		TopK:                  5,
		ScoreThresholdEnabled: false,
		ScoreThreshold:        nil,
	}

	if config.RetrievalModel != nil {
		retrievalModel = *config.RetrievalModel
	}

	return &DifyProvider{
		baseURL:        config.BaseURL,
		apiKey:         config.APIKey,
		datasetID:      config.DatasetID,
		retrievalModel: retrievalModel,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (d *DifyProvider) SearchRelevantContext(ctx context.Context, query string) ([]string, error) {
	req := DifyRequest{
		Query:          query,
		RetrievalModel: d.retrievalModel,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/datasets/%s/retrieve", d.baseURL, d.datasetID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var difyResp DifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&difyResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	contexts := make([]string, 0, len(difyResp.Records))
	for _, record := range difyResp.Records {
		contexts = append(contexts, record.Segment.Content)
	}

	return contexts, nil
}
