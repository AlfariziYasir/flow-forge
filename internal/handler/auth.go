package handler

import (
	"encoding/json"
	"net/http"

	"flowforge/internal/model"
	"flowforge/internal/services"
	"flowforge/pkg/errorx"
	"flowforge/pkg/logger"
	"flowforge/pkg/response"
)

type AuthHandler struct {
	svc services.UserService
	l   *logger.Logger
}

func NewAuthHandler(svc services.UserService, l *logger.Logger) *AuthHandler {
	return &AuthHandler{
		svc: svc,
		l:   l,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorx.HttpNewError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	accToken, refToken, err := h.svc.Login(r.Context(), req)
	if err != nil {
		errorx.MapError(err).Write(w)
		return
	}

	response.Success(w, http.StatusOK, "login success", model.UserLoginResponse{
		AccessToken:  accToken,
		RefreshToken: refToken,
	})
}
