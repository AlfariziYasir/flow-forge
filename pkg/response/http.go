package response

import (
	"encoding/json"
	"net/http"
)

type Response[T any] struct {
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
}

func Success[T any](w http.ResponseWriter, code int, message string, data T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(Response[T]{
		Message: message,
		Data:    data,
	})
}
