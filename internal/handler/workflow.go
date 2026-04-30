package handler

import (
	"encoding/json"
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

type WorkflowHandler struct {
	ws  services.WorkflowService
	log *logger.Logger
}

func NewWorkflowHandler(ws services.WorkflowService, l *logger.Logger) *WorkflowHandler {
	return &WorkflowHandler{
		ws:  ws,
		log: l,
	}
}

func (h *WorkflowHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("failed get tenant id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}

	var req model.WorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed decode request body", zap.Error(err))
		errorx.HttpNewError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	wf, err := h.ws.Create(r.Context(), tenantID, &req)
	if err != nil {
		h.log.Error("failed create workflow", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusCreated, "success", wf)
}

func (h *WorkflowHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("failed get tenant id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}
	id := chi.URLParam(r, "id")

	wf, err := h.ws.Get(r.Context(), tenantID, id)
	if err != nil {
		h.log.Error("failed get workflow", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", wf)
}

func (h *WorkflowHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("failed get tenant id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}

	req := model.ListWorkflowRequest{
		TenantID: tenantID,
	}

	dtos, count, _, err := h.ws.List(r.Context(), req)
	if err != nil {
		h.log.Error("failed list workflows", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", map[string]interface{}{
		"data":  dtos,
		"total": count,
	})
}

func (h *WorkflowHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("failed get tenant id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}
	id := chi.URLParam(r, "id")

	exe, err := h.ws.Trigger(r.Context(), tenantID, id, "MANUAL")
	if err != nil {
		h.log.Error("failed trigger workflow", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusAccepted, "triggered", exe)
}

func (h *WorkflowHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.WorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed decode request body", zap.Error(err))
		errorx.HttpNewError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.ws.Update(r.Context(), &req); err != nil {
		h.log.Error("failed update workflow", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessMessage(w, http.StatusOK, "success")
}

func (h *WorkflowHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("failed get tenant id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}
	name := chi.URLParam(r, "name")

	if err := h.ws.Delete(r.Context(), tenantID, name); err != nil {
		h.log.Error("failed delete workflow", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessMessage(w, http.StatusOK, "success")
}

func (h *WorkflowHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	var req model.WorkflowRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed decode request body", zap.Error(err))
		errorx.HttpNewError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.ws.Rollback(r.Context(), &req); err != nil {
		h.log.Error("failed rollback workflow", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessMessage(w, http.StatusOK, "success")
}

func (h *WorkflowHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("failed get tenant id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}
	name := chi.URLParam(r, "name")

	versions, err := h.ws.ListVersions(r.Context(), tenantID, name)
	if err != nil {
		h.log.Error("failed list workflow versions", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", versions)
}
