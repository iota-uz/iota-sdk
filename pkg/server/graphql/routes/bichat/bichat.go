package bichat

import (
	"context"
	"errors"
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/service"
	"github.com/jmoiron/sqlx"
	"github.com/sashabaranov/go-openai"
	"io"
)

func GetPrompt(db *sqlx.DB, id string) (string, error) {
	prompt := &Prompt{}
	err := db.Get(prompt, "SELECT * FROM prompts WHERE id = $1", id)
	if err != nil {
		return "", err
	}
	return prompt.Prompt, nil
}

func StartChat(db *sqlx.DB) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		message, ok := p.Args["message"].(string)
		if !ok {
			return nil, errors.New("message is required")
		}
		if len(message) > 1000 {
			return nil, errors.New("message is too long")
		}
		model, ok := p.Args["model"].(string)
		if !ok {
			return nil, errors.New("model is required")
		}
		dialogue := &Dialogue{}
		dialogueId, ok := p.Args["dialogueId"].(int)
		if ok {
			q := "SELECT * FROM dialogues WHERE id = $1"
			if err := db.Get(dialogue, q, dialogueId); err != nil {
				return nil, err
			}
		} else {
			prompt, err := GetPrompt(db, "bi-chat")
			if err != nil {
				return nil, err
			}
			dialogue.Messages = append(dialogue.Messages, &openai.ChatCompletionMessage{
				Role:    "system",
				Content: prompt,
			})
			q := "INSERT INTO dialogues (label, messages) VALUES ($1, $2) RETURNING *"
			if err := db.Get(dialogue, q, "New chat", dialogue.Messages); err != nil {
				return nil, err
			}
		}
		dialogue.Messages = append(dialogue.Messages, &openai.ChatCompletionMessage{
			Role:    "user",
			Content: message,
		})
		client := openai.NewClient("")
		var messages []openai.ChatCompletionMessage
		for _, m := range dialogue.Messages {
			messages = append(messages, *m)
		}
		resp, err := client.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
			Stream:   true,
		})
		if err != nil {
			return nil, err
		}
		for {
			chunk, err := resp.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			fmt.Println(chunk)
		}
		return dialogue, nil
	}
}

func GraphQL(db *sqlx.DB) (*graphql.Object, *graphql.Object) {
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "Dialogues",
		Description: "Dialogues between users with GPT",
		Fields: graphql.Fields{
			"dialogues": &graphql.Field{
				Type: graphql.NewList(graphql.NewObject(
					graphql.ObjectConfig{
						Name: "Dialogue",
						Fields: graphql.Fields{
							"label": &graphql.Field{
								Type: graphql.String,
							},
							"messages": &graphql.Field{
								Type: graphql.NewList(
									graphql.NewObject(
										graphql.ObjectConfig{
											Name: "Message",
											Fields: graphql.Fields{
												"id": &graphql.Field{
													Type: graphql.Int,
												},
												"content": &graphql.Field{
													Type: graphql.String,
												},
												"role": &graphql.Field{
													Type: graphql.String,
												},
												"function_call": &graphql.Field{
													Type: graphql.String,
												},
												"tool_calls": &graphql.Field{
													Type: graphql.String,
												},
												"created_at": &graphql.Field{
													Type: graphql.String,
												},
											},
										},
									),
								),
							},
						},
					},
				)),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					query := service.ResolveToQuery(p, &service.Model{
						Table: "dialogues",
						Pk: &service.Field{
							Name: "id",
							Type: service.Serial,
						},
						Fields: []*service.Field{
							{
								Name: "label",
								Type: service.CharacterVarying,
							},
							{
								Name: "messages",
								Type: service.Jsonb,
							},
						},
					})
					_sortBy, ok := p.Info.VariableValues["sortBy"].([]interface{})
					if ok {
						var sortBy []string
						for _, s := range _sortBy {
							sortBy = append(sortBy, s.(string))
						}
						query.Order(service.OrderStringToExpression(sortBy)...)
					}
					data, err := service.Find(db, query)
					if err != nil {
						return nil, err
					}
					return data, nil
				},
			},
		},
	})
	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ChatMutations",
		Fields: graphql.Fields{
			"StartChat": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "StartChat",
					Fields: graphql.Fields{
						"message": &graphql.Field{
							Type: graphql.String,
						},
						"model": &graphql.Field{
							Type: graphql.String,
						},
						"dialogueId": &graphql.Field{
							Type: graphql.Int,
						},
					},
				}),
				Resolve: StartChat(db),
			},
		},
	})
	return queryType, mutationType
}
