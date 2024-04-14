package chatfuncs

import (
	"github.com/jmoiron/sqlx"
	"github.com/sashabaranov/go-openai"
)

type CompletionFunc func(args map[string]interface{}) (string, error)

var SupportedCurrencies = []string{
	"AUD", "AZN", "GBP",
	"AMD", "BYN", "BGN",
	"BRL", "HUF", "VND",
	"HKD", "GEL", "DKK",
	"AED", "USD", "EUR",
	"EGP", "INR", "IDR",
	"KZT", "CAD", "QAR",
	"KGS", "CNY", "MDL",
	"NZD", "NOK", "PLN",
	"RON", "XDR", "SGD",
	"TJS", "THB", "TRY",
	"TMT", "UZS", "UAH",
	"CZK", "SEK", "CHF",
	"RSD", "ZAR", "KRW",
	"JPY",
}

type ChatTools struct {
	Tools []openai.Tool             `json:"tools"`
	Funcs map[string]CompletionFunc `json:"funcs"`
}

func GetTools(db *sqlx.DB) (*ChatTools, error) {
	funcDefinitions := []ChatFunctionDefinition{
		NewUnitConversion(db),
		NewCurrencyConvert(),
		NewSearchKnowledgeBase(db),
		NewDoSQLQuery(db),
		NewGetSchema(db),
	}

	var tools []openai.Tool
	funcs := map[string]CompletionFunc{}
	for _, def := range funcDefinitions {
		tools = append(tools, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        def.Name(),
				Description: def.Description(),
				Parameters:  def.Arguments(),
			},
		})
		funcs[def.Name()] = def.Execute
	}
	return &ChatTools{
		Tools: tools,
		Funcs: funcs,
	}, nil
}
