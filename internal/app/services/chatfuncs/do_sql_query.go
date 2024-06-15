package chatfuncs

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
)

type doSQLQuery struct {
	db *gorm.DB
}

func NewDoSQLQuery(db *gorm.DB) ChatFunctionDefinition {
	return doSQLQuery{db: db}
}

func (d doSQLQuery) Name() string {
	return "do_sql_query"
}

func (d doSQLQuery) Description() string {
	return "Executes a SQL query and return the results. Input should be a fully formed SQL query"
}

func (d doSQLQuery) Arguments() map[string]interface{} {
	return map[string]interface{}{
		"query": map[string]interface{}{
			"type": "string",
			"description": `SQL query extracting info to answer the user's question.
								SQL should be written in PostgreSQL dialect using the schema from "get_schema" function.
								The query should be returned in plain text, not in JSON.`,
		},
	}
}

func (d doSQLQuery) Execute(args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", errors.New("query is required")
	}
	var records []map[string]interface{}
	err := d.db.Raw(query, &records).Error
	if err != nil {
		return "", err
	}
	jsonBytes, err := json.Marshal(records)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
