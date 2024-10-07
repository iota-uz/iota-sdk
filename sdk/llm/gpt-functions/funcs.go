package functions

import (
	"encoding/json"

	"github.com/sashabaranov/go-openai"
	"gorm.io/gorm"
)

type CompletionFunc func(args map[string]interface{}) (string, error)

func New() *ChatTools {
	return &ChatTools{} //nolint:exhaustruct
}

func Default(db *gorm.DB) *ChatTools {
	return &ChatTools{
		Definitions: []ChatFunctionDefinition{
			NewGetSchema(db),
		},
	}
}

type ChatTools struct {
	Definitions []ChatFunctionDefinition
}

func (c *ChatTools) Add(def ChatFunctionDefinition) {
	c.Definitions = append(c.Definitions, def)
}

func (c *ChatTools) OpenAiTools() []openai.Tool {
	tools := make([]openai.Tool, 0, len(c.Definitions))
	for _, def := range c.Definitions {
		tools = append(tools, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{ //nolint:exhaustruct
				Name:        def.Name(),
				Description: def.Description(),
				Parameters:  def.Arguments(),
			},
		})
	}
	return tools
}

func (c *ChatTools) Funcs() map[string]CompletionFunc {
	funcs := map[string]CompletionFunc{}
	for _, def := range c.Definitions {
		funcs[def.Name()] = def.Execute
	}
	return funcs
}

func (c *ChatTools) Call(name string, args string) (string, error) {
	if fn, ok := c.Funcs()[name]; ok {
		parsedArgs := map[string]interface{}{}
		if err := json.Unmarshal([]byte(args), &parsedArgs); err != nil {
			return "", err
		}
		return fn(parsedArgs)
	}
	return "", nil
}
