package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/embedding"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/llm/gpt-functions"
	"net/http"
)

type EmbeddingService struct {
	app application.Application
}

func NewEmbeddingService(app application.Application) *EmbeddingService {
	return &EmbeddingService{
		app: app,
	}
}

func (s *EmbeddingService) Search(ctx context.Context, query string) ([]*embedding.SearchResult, error) {
	body := &bytes.Buffer{}
	err := json.NewEncoder(body).Encode(map[string]interface{}{"query": query, "top_k": 5, "threshold": 0})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"http://localhost:8000/embeddings/search",
		body,
	)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("failed to search knowledge base")
	}
	var results []*embedding.SearchResult
	err = json.NewDecoder(response.Body).Decode(&results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

type searchKnowledgeBase struct {
	embeddingService *EmbeddingService
}

func NewSearchKnowledgeBase(service *EmbeddingService) functions.ChatFunctionDefinition {
	return searchKnowledgeBase{
		embeddingService: service,
	}
}

func (s searchKnowledgeBase) Name() string {
	return "search_knowledge_base"
}

func (s searchKnowledgeBase) Description() string {
	return "Search the knowledge using vector search"
}

func (s searchKnowledgeBase) Arguments() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Query to search the knowledge base",
			},
		},
		"required": []string{"query"},
	}
}

func (s searchKnowledgeBase) Execute(args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", errors.New("keyword is required")
	}
	results, err := s.embeddingService.Search(context.Background(), query)
	if err != nil {
		return "", err
	}
	records := make([]map[string]interface{}, len(results))
	for i, result := range results {
		records[i] = map[string]interface{}{
			"text": result.Text,
		}
	}
	jsonBytes, err := json.Marshal(records)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

//
// func (s *EmbeddingService) GetByUUID(ctx context.Context, uuid string) (*composables.Embedding, error) {
//	return s.app.Embeddings.GetByUUID(ctx, uuid)
//}
//
// func (s *EmbeddingService) Create(ctx context.Context, embedding *composables.Embedding) error {
//	return s.app.Embeddings.Create(ctx, embedding)
//}
