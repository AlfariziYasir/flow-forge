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

type UserHandler struct {
	us  services.UserService
	log *logger.Logger
}

func NewUserHandler(us services.UserService, l *logger.Logger) *UserHandler {
	return &UserHandler{
		us:  us,
		log: l,
	}
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed decode request body", zap.Error(err))
		errorx.HttpNewError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.us.Create(r.Context(), req)
	if err != nil {
		h.log.Error("failed create user", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusCreated, "success", user)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	user, err := h.us.Get(r.Context(), id)
	if err != nil {
		h.log.Error("failed get user", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", user)
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	req := model.ListRequest{
		PageSize: 50,
	}

	users, count, _, err := h.us.List(r.Context(), req)
	if err != nil {
		h.log.Error("failed list users", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", map[string]interface{}{
		"data": users,
		"total": count,
	})
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed decode request body", zap.Error(err))
		errorx.HttpNewError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.us.Update(r.Context(), req)
	if err != nil {
		h.log.Error("failed update user", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "success", user)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.us.Delete(r.Context(), id); err != nil {
		h.log.Error("failed delete user", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessMessage(w, http.StatusOK, "success")
}
