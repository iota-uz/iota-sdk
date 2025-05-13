package functions

import (
	"encoding/json"

	"github.com/sashabaranov/go-openai"

	"gorm.io/gorm"
)

type CompletionFunc func(args map[string]interface{}) (string, error)

type ChatTools interface {
	Add(def ChatFunctionDefinition)
	OpenAiTools() []openai.Tool
	Funcs() map[string]CompletionFunc
	Call(name string, args string) (string, error)
}

func New() ChatTools {
	return &chatTools{}
}

func Default(db *gorm.DB) *chatTools {
	return &chatTools{
		Definitions: []ChatFunctionDefinition{
			NewGetSchema(db),
		},
	}
}

type chatTools struct {
	Definitions []ChatFunctionDefinition
}

func (c *chatTools) Add(def ChatFunctionDefinition) {
	c.Definitions = append(c.Definitions, def)
}

func (c *chatTools) OpenAiTools() []openai.Tool {
	tools := make([]openai.Tool, 0, len(c.Definitions))
	for _, def := range c.Definitions {
		tools = append(tools, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        def.Name(),
				Description: def.Description(),
				Parameters:  def.Arguments(),
			},
		})
	}
	return tools
}

func (c *chatTools) Funcs() map[string]CompletionFunc {
	funcs := map[string]CompletionFunc{}
	for _, def := range c.Definitions {
		funcs[def.Name()] = def.Execute
	}
	return funcs
}

func (c *chatTools) Call(name string, args string) (string, error) {
	if fn, ok := c.Funcs()[name]; ok {
		parsedArgs := map[string]interface{}{}
		if err := json.Unmarshal([]byte(args), &parsedArgs); err != nil {
			return "", err
		}
		return fn(parsedArgs)
	}
	return "", nil
}
