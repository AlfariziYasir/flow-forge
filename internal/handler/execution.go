package handler

import (
	"encoding/json"
	"net/http"

	"flowforge/internal/model"
	"flowforge/internal/services"
	"flowforge/pkg/logger"

	"github.com/go-chi/chi/v5"
)

type ExecutionHandler struct {
	es services.ExecutionService
	l  *logger.Logger
}

func NewExecutionHandler(es services.ExecutionService, l *logger.Logger) *ExecutionHandler {
	return &ExecutionHandler{
		es: es,
		l:  l,
	}
}

func (h *ExecutionHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value("tenant_id").(string)
	id := chi.URLParam(r, "id")

	exe, err := h.es.Get(r.Context(), tenantID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(exe)
}

func (h *ExecutionHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value("tenant_id").(string)
	
	req := model.ListExecutionRequest{
		TenantID: tenantID,
		PageSize: 50,
	}

	dtos, _, _, err := h.es.List(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(dtos)
}

func (h *ExecutionHandler) Retry(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value("tenant_id").(string)
	id := chi.URLParam(r, "id")

	if err := h.es.Retry(r.Context(), tenantID, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *ExecutionHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value("tenant_id").(string)
	id := chi.URLParam(r, "id")

	if err := h.es.Cancel(r.Context(), tenantID, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
