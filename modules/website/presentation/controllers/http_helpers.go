package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers/dtos"
)

func writeJSONError(w http.ResponseWriter, statusCode int, message, errorCode string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(dtos.NewAPIError(message, errorCode)); err != nil {
		panic(err)
	}
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		panic(err)
	}
}
