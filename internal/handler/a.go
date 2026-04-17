package handler

import (
	"encoding/json"
	"net/http"

	"flowforge/internal/services"
	"flowforge/pkg/logger"
)

type AIHandler struct {
	ais services.AIService
	l   *logger.Logger
}

func NewAIHandler(ais services.AIService, l *logger.Logger) *AIHandler {
	return &AIHandler{
		ais: ais,
		l:   l,
	}
}

type AIDAGRequest struct {
	Prompt string `json:"prompt"`
}

type AIAnalyzeRequest struct {
	ErrorLog string `json:"error_log"`
}

func (h *AIHandler) GenerateDAG(w http.ResponseWriter, r *http.Request) {
	var req AIDAGRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	dag, err := h.ais.GenerateDAGFromText(r.Context(), req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(dag)
}

func (h *AIHandler) AnalyzeFailure(w http.ResponseWriter, r *http.Request) {
	var req AIAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	analysis, err := h.ais.AnalyzeFailure(r.Context(), req.ErrorLog)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"analysis": analysis})
}
