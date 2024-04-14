package chatfuncs

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"net/http"
)

var SupportedUnits = []string{
	"m", "cm", "mm", "km", "ft", "in", "yd", "mi",
	"kg", "g", "mg", "t", "lb", "oz",
	"m/s", "km/h", "ft/s", "mph", "kn", "mach",
	"m2", "cm2", "mm2", "km2", "ft2", "in2", "yd2", "mi2",
	"m3", "cm3", "mm3", "km3", "ft3", "in3", "yd3", "mi3",
	"l", "ml", "gal", "qt", "pt", "cup", "fl oz",
	"kg/m3", "g/cm3", "mg/mm3", "t/km3", "lb/ft3", "oz/in3", "oz/yd3", "t/m3",
}

func GetExchangeRate(from string, to string) (float64, error) {
	response, err := http.Get("https://www.cbr-xml-daily.ru/latest.js")
	if err != nil {
		return 0, err
	}
	if response.StatusCode != 200 {
		return 0, errors.New("failed to get exchange rates")
	}
	var responseData struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(response.Body).Decode(&responseData); err != nil {
		return 0, err
	}
	exchangeRates := responseData.Rates
	exchangeRates["RUB"] = 1
	return exchangeRates[from] / exchangeRates[to], nil
}

func NewUnitConversion(db *sqlx.DB) ChatFunctionDefinition {
	return &unitConversion{db: db}
}

type unitConversion struct {
	db *sqlx.DB
}

func (u unitConversion) Name() string {
	return "unit_conversion"
}

func (u unitConversion) Description() string {
	return "Converts units"
}

func (u unitConversion) Arguments() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"amount": map[string]interface{}{
				"type":        "number",
				"description": "Amount of units to convert.",
			},
			"from": map[string]interface{}{
				"type":        "string",
				"description": "Unit to convert from.",
				"enum":        SupportedUnits,
			},
			"to": map[string]interface{}{
				"type":        "string",
				"description": "Unit to convert to.",
				"enum":        SupportedUnits,
			},
		},
	}
}

func (u unitConversion) Execute(args map[string]interface{}) (string, error) {
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
	result, err := UnitConversion(amount, from, to)
	if err != nil {
		return "", err
	}
	jsonBytes, err := json.Marshal(map[string]interface{}{
		"result": result,
	})
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

func UnitConversion(amount float64, from string, to string) (float64, error) {
	lengthUnitMap := map[string]float64{
		"m":  1,
		"cm": 0.01,
		"mm": 0.001,
		"km": 1000,
		"ft": 0.3048,
		"in": 0.0254,
		"yd": 0.9144,
		"mi": 1609.344,
	}
	massUnitMap := map[string]float64{
		"kg": 1,
		"g":  0.001,
		"mg": 0.000001,
		"t":  1000,
		"lb": 0.45359237,
		"oz": 0.0283495231,
	}
	speedUnitMap := map[string]float64{
		"m/s":  1,
		"km/h": 0.277777778,
		"ft/s": 0.3048,
		"mph":  0.44704,
		"kn":   0.514444444,
		"mach": 340.3,
	}
	areaUnitMap := map[string]float64{
		"m2":  1,
		"cm2": 0.0001,
		"mm2": 0.000001,
		"km2": 1000000,
		"ft2": 0.09290304,
		"in2": 0.00064516,
		"yd2": 0.83612736,
		"mi2": 2589988.11,
	}
	volumeUnitMap := map[string]float64{
		"m3":  1,
		"cm3": 0.000001,
		"mm3": 0.000000001,
		"km3": 1000000000,
		"ft3": 0.0283168466,
		"in3": 0.0000163871,
		"yd3": 0.764554858,
		"mi3": 4168181825.4,
		"l":   1,
		"ml":  0.001,
		"gal": 3.785411,
	}
	densityUnitMap := map[string]float64{
		"kg/m3":  1,
		"g/cm3":  1000,
		"mg/mm3": 0.000001,
		"t/km3":  0.000,
	}
	unitMaps := []map[string]float64{
		lengthUnitMap,
		massUnitMap,
		speedUnitMap,
		areaUnitMap,
		volumeUnitMap,
		densityUnitMap,
	}
	for _, unitMap := range unitMaps {
		if unitMap[from] != 0 && unitMap[to] != 0 {
			return amount * unitMap[from] / unitMap[to], nil
		}
	}
	return 0, errors.New(fmt.Sprintf("Cannot convert from %s to %s", from, to))
}
