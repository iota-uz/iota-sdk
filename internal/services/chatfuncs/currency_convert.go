package chatfuncs

import (
	"encoding/json"
	"errors"

	functions "github.com/iota-agency/iota-sdk/sdk/llm/gpt-functions"
)

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

func NewCurrencyConvert() functions.ChatFunctionDefinition {
	return currencyConvert{}
}

type currencyConvert struct{}

func (c currencyConvert) Name() string {
	return "currency_convert"
}

func (c currencyConvert) Description() string {
	return "Converts currency"
}

func (c currencyConvert) Arguments() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"amount": map[string]interface{}{
				"type":        "number",
				"description": "Amount of currency to convert.",
			},
			"from": map[string]interface{}{
				"type":        "string",
				"enum":        SupportedCurrencies,
				"description": "Currency to convert from.",
			},
			"to": map[string]interface{}{
				"type":        "string",
				"enum":        SupportedCurrencies,
				"description": "Currency to convert to.",
			},
		},
	}
}

func (c currencyConvert) Execute(args map[string]interface{}) (string, error) {
	amount, ok := args["amount"].(float64)
	if !ok {
		return "", errors.New("amount is required")
	}
	from, ok := args["from"].(string)
	if !ok {
		return "", errors.New("from is required")
	}
	to, ok := args["to"].(string)
	if !ok {
		return "", errors.New("to is required")
	}
	rate, err := GetExchangeRate(from, to)
	if err != nil {
		return "", err
	}
	jsonBytes, err := json.Marshal(map[string]interface{}{
		"result": amount / rate,
	})
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
