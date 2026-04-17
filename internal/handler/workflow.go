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
	
	req := model.ListWorkflowRequest{
		TenantID: tenantID,
		PageSize: 50,
	}

	dtos, _, _, err := h.ws.List(r.Context(), req)
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

func (h *WorkflowHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.WorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.ws.Update(r.Context(), &req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WorkflowHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value("tenant_id").(string)
	name := chi.URLParam(r, "name")

	if err := h.ws.Delete(r.Context(), tenantID, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *WorkflowHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	var req model.WorkflowRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.ws.Rollback(r.Context(), &req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WorkflowHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value("tenant_id").(string)
	name := chi.URLParam(r, "name")

	versions, err := h.ws.ListVersions(r.Context(), tenantID, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(versions)
}
