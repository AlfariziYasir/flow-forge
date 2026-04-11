package handler

import (
	"encoding/json"
	"net/http"

	"flowforge/internal/model"
	"flowforge/internal/services"
	"flowforge/pkg/logger"

	"github.com/go-chi/chi/v5"
)

type WorkflowHandler struct {
	ws services.WorkflowService
	l  *logger.Logger
}

func NewWorkflowHandler(ws services.WorkflowService, l *logger.Logger) *WorkflowHandler {
	return &WorkflowHandler{
		ws: ws,
		l:  l,
	}
}

func (h *WorkflowHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value("tenant_id").(string)

	var req model.WorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	wf, err := h.ws.Create(r.Context(), tenantID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(wf)
}

func (h *WorkflowHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value("tenant_id").(string)
	id := chi.URLParam(r, "id")

	wf, err := h.ws.Get(r.Context(), tenantID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(wf)
}

func (h *WorkflowHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value("tenant_id").(string)
	
	dtos, _, err := h.ws.List(r.Context(), tenantID, 50, 0, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(dtos)
}

func (h *WorkflowHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value("tenant_id").(string)
	id := chi.URLParam(r, "id")

	exe, err := h.ws.Trigger(r.Context(), tenantID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(exe)
}
