package handler

import (
	"net/http"

	"flowforge/internal/model"
	"flowforge/internal/services"
	"flowforge/pkg/errorx"
	"flowforge/pkg/jwt"
	"flowforge/pkg/logger"
	"flowforge/pkg/response"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ExecutionHandler struct {
	es  services.ExecutionService
	log *logger.Logger
}

func NewExecutionHandler(es services.ExecutionService, l *logger.Logger) *ExecutionHandler {
	return &ExecutionHandler{
		es:  es,
		log: l,
	}
}

func (h *ExecutionHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("failed get tenant id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}
	id := chi.URLParam(r, "id")

	exe, err := h.es.Get(r.Context(), tenantID, id)
	if err != nil {
		h.log.Error("failed get execution", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", exe)
}

func (h *ExecutionHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("failed get tenant id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}

	req := model.ListExecutionRequest{
		TenantID: tenantID,
	}

	dtos, count, _, err := h.es.List(r.Context(), req)
	if err != nil {
		h.log.Error("failed list executions", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", map[string]interface{}{
		"data":  dtos,
		"total": count,
	})
}

func (h *ExecutionHandler) Retry(w http.ResponseWriter, r *http.Request) {
	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("failed get tenant id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}
	id := chi.URLParam(r, "id")

	err := h.es.Retry(r.Context(), tenantID, id)
	if err != nil {
		h.log.Error("failed retry execution", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessMessage(w, http.StatusOK, "retry triggered")
}

func (h *ExecutionHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("failed get tenant id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}
	id := chi.URLParam(r, "id")

	if err := h.es.Cancel(r.Context(), tenantID, id); err != nil {
		h.log.Error("failed cancel execution", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessMessage(w, http.StatusOK, "execution cancelled")
}
