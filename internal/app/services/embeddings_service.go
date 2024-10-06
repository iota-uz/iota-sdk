package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/iota-agency/iota-erp/internal/domain/entities/embedding"
	functions "github.com/iota-agency/iota-erp/sdk/llm/gpt-functions"
)

type EmbeddingService struct {
	app *Application
}

func NewEmbeddingService(app *Application) *EmbeddingService {
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
	response, err := http.DefaultClient.Post("http://localhost:8000/embeddings/search", "application/json", body)
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
	var records []map[string]interface{}
	for _, result := range results {
		records = append(records, map[string]interface{}{
			"text": result.Text,
		})
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
//func (s *EmbeddingService) Create(ctx context.Context, embedding *composables.Embedding) error {
//	return s.app.Embeddings.Create(ctx, embedding)
//}
