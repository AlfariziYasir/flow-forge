package handler

import (
	"encoding/json"
	"net/http"

	"flowforge/internal/services"
	"flowforge/pkg/errorx"
	"flowforge/pkg/logger"
	"flowforge/pkg/response"

	"go.uber.org/zap"
)

type AIHandler struct {
	ais services.AIService
	log *logger.Logger
}

func NewAIHandler(ais services.AIService, l *logger.Logger) *AIHandler {
	return &AIHandler{
		ais: ais,
		log: l,
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
		h.log.Error("failed decode request body", zap.Error(err))
		errorx.HttpNewError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	dag, err := h.ais.GenerateDAGFromText(r.Context(), req.Prompt)
	if err != nil {
		h.log.Error("failed generate dag", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", dag)
}

func (h *AIHandler) AnalyzeFailure(w http.ResponseWriter, r *http.Request) {
	var req AIAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed decode request body", zap.Error(err))
		errorx.HttpNewError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	analysis, err := h.ais.AnalyzeFailure(r.Context(), req.ErrorLog)
	if err != nil {
		h.log.Error("failed analyze failure", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", map[string]string{"analysis": analysis})
}
