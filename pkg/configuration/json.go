package configuration

import (
	"encoding/json"
	"os"
)

var jsonConfigSingleton *ErpJsonConfig

type ErpJsonConfig struct {
	Modules []string `json:"modules"`
}

func UseJsonConfig() *ErpJsonConfig {
	if jsonConfigSingleton == nil {
		jsonConfigSingleton = &ErpJsonConfig{}
	}
	f, err := os.Open("erp.config.json")
	if err != nil {
		panic(err)
	}
	if err := json.NewDecoder(f).Decode(jsonConfigSingleton); err != nil {
		panic(err)
	}
	return jsonConfigSingleton
}
