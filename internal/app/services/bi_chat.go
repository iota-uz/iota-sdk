package services

//func GetPrompt(db *sqlx.DB, id string) (string, error) {
//	prompt := &Prompt{}
//	err := db.Get(prompt, "SELECT * FROM prompts WHERE id = $1", id)
//	if err != nil {
//		return "", err
//	}
//	return prompt.Prompt, nil
//}
//
//func StartChat(db *sqlx.DB) graphql.FieldResolveFn {
//	return func(p graphql.ResolveParams) (interface{}, error) {
//		message, ok := p.Args["message"].(string)
//		if !ok {
//			return nil, errors.New("message is required")
//		}
//		if len(message) > 1000 {
//			return nil, errors.New("message is too long")
//		}
//		model, ok := p.Args["model"].(string)
//		if !ok {
//			return nil, errors.New("model is required")
//		}
//		dialogue := &Dialogue{}
//		dialogueId, ok := p.Args["dialogueId"].(int)
//		if ok {
//			q := "SELECT * FROM dialogues WHERE id = $1"
//			if err := db.Get(dialogue, q, dialogueId); err != nil {
//				return nil, err
//			}
//		} else {
//			prompt, err := GetPrompt(db, "bi-chat")
//			if err != nil {
//				return nil, err
//			}
//			dialogue.Messages = append(dialogue.Messages, &openai.ChatCompletionMessage{
//				Role:    "system",
//				Content: prompt,
//			})
//			q := "INSERT INTO dialogues (label, messages) VALUES ($1, $2) RETURNING *"
//			if err := db.Get(dialogue, q, "New chat", dialogue.Messages); err != nil {
//				return nil, err
//			}
//		}
//		dialogue.Messages = append(dialogue.Messages, &openai.ChatCompletionMessage{
//			Role:    "user",
//			Content: message,
//		})
//		client := openai.NewClient("")
//		var messages []openai.ChatCompletionMessage
//		for _, m := range dialogue.Messages {
//			messages = append(messages, *m)
//		}
//		resp, err := client.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{
//			Model:    model,
//			Messages: messages,
//			Stream:   true,
//		})
//		if err != nil {
//			return nil, err
//		}
//		for {
//			chunk, err := resp.Recv()
//			if errors.Is(err, io.EOF) {
//				break
//			}
//			if err != nil {
//				return nil, err
//			}
//		}
//		return dialogue, nil
//	}
//}
