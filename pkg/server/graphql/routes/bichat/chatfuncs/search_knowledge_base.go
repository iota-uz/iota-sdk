package chatfuncs

import (
	"encoding/json"
	"errors"
	"github.com/jmoiron/sqlx"
)

type searchKnowledgeBase struct {
	Db *sqlx.DB
}

func NewSearchKnowledgeBase(db *sqlx.DB) ChatFunctionDefinition {
	return searchKnowledgeBase{Db: db}
}

func (s searchKnowledgeBase) Name() string {
	return "search_knowledge_base"
}

func (s searchKnowledgeBase) Description() string {
	return "Search the knowledge base by keyword."
}

func (s searchKnowledgeBase) Arguments() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"keyword": map[string]interface{}{
				"type":        "string",
				"description": "A single word.",
			},
		},
		"required": []string{"keyword"},
	}
}

func (s searchKnowledgeBase) Execute(args map[string]interface{}) (string, error) {
	keyword, ok := args["keyword"].(string)
	if !ok {
		return "", errors.New("keyword is required")
	}
	var records []map[string]interface{}
	err := s.Db.Select(&records, `SELECT * FROM articles WHERE content ILIKE $1 LIMIT 3;`, "%"+keyword+"%")
	if err != nil {
		return "", err
	}
	jsonBytes, err := json.Marshal(records)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
