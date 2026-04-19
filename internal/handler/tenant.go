package handler

import (
	"encoding/json"
	"net/http"

	"flowforge/internal/model"
	"flowforge/internal/services"
	"flowforge/pkg/errorx"
	"flowforge/pkg/logger"
	"flowforge/pkg/response"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type TenantHandler struct {
	ts  services.TenantService
	log *logger.Logger
}

func NewTenantHandler(ts services.TenantService, l *logger.Logger) *TenantHandler {
	return &TenantHandler{
		ts:  ts,
		log: l,
	}
}

func (h *TenantHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.TenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed decode request body", zap.Error(err))
		errorx.HttpNewError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tenant, err := h.ts.Create(r.Context(), req)
	if err != nil {
		h.log.Error("failed create tenant", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusCreated, "success", tenant)
}

func (h *TenantHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tenant, err := h.ts.Get(r.Context(), id)
	if err != nil {
		h.log.Error("failed get tenant", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", tenant)
}

func (h *TenantHandler) List(w http.ResponseWriter, r *http.Request) {
	req := model.ListTenantRequest{
		PageSize: 50,
	}

	tenants, count, _, err := h.ts.List(r.Context(), req)
	if err != nil {
		h.log.Error("failed list tenants", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", map[string]interface{}{
		"data": tenants,
		"total": count,
	})
}

func (h *TenantHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.TenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed decode request body", zap.Error(err))
		errorx.HttpNewError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tenant, err := h.ts.Update(r.Context(), req)
	if err != nil {
		h.log.Error("failed update tenant", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", tenant)
}

func (h *TenantHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.ts.Delete(r.Context(), id); err != nil {
		h.log.Error("failed delete tenant", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessMessage(w, http.StatusOK, "success")
}
