package handler

import (
	"encoding/json"
	"net/http"

	"flowforge/internal/model"
	"flowforge/internal/services"
	"flowforge/pkg/logger"

	"github.com/go-chi/chi/v5"
)

type TenantHandler struct {
	ts services.TenantService
	l  *logger.Logger
}

func NewTenantHandler(ts services.TenantService, l *logger.Logger) *TenantHandler {
	return &TenantHandler{
		ts: ts,
		l:  l,
	}
}

func (h *TenantHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.TenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	tenant, err := h.ts.Create(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tenant)
}

func (h *TenantHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tenant, err := h.ts.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(tenant)
}

func (h *TenantHandler) List(w http.ResponseWriter, r *http.Request) {
	req := model.ListTenantRequest{
		PageSize: 50,
	}

	tenants, _, _, err := h.ts.List(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(tenants)
}

func (h *TenantHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.TenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	tenant, err := h.ts.Update(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(tenant)
}

func (h *TenantHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.ts.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
