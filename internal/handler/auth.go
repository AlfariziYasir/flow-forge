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

	"go.uber.org/zap"
)

type AuthHandler struct {
	us  services.UserService
	log *logger.Logger
}

func NewAuthHandler(us services.UserService, l *logger.Logger) *AuthHandler {
	return &AuthHandler{
		us:  us,
		log: l,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed decode request body", zap.Error(err))
		errorx.HttpNewError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	accToken, refToken, err := h.us.Login(r.Context(), req)
	if err != nil {
		h.log.Error("failed login", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "login success", model.UserLoginResponse{
		AccessToken:  accToken,
		RefreshToken: refToken,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	refUuid := jwt.GetRefUuid(r.Context())
	if refUuid == "" {
		h.log.Error("failed get ref uuid")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}

	userID := jwt.GetUser(r.Context())
	if userID == "" {
		h.log.Error("failed get user id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}

	role := jwt.GetRole(r.Context())
	if role == "" {
		h.log.Error("failed get role")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}

	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("failed get tenant id")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}

	accToken, err := h.us.Refresh(r.Context(), refUuid, userID, role, tenantID)
	if err != nil {
		h.log.Error("failed refresh", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessWithData(w, http.StatusOK, "refresh success", model.UserRefreshResponse{
		AccessToken: accToken,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	refUuid := jwt.GetRefUuid(r.Context())
	if refUuid == "" {
		h.log.Error("failed get ref uuid")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}

	accUuid := jwt.GetAccUuid(r.Context())
	if accUuid == "" {
		h.log.Error("failed get acc uuid")
		errorx.HttpNewError(w, http.StatusUnauthorized, "invalid request")
		return
	}

	err := h.us.Logout(r.Context(), accUuid, refUuid)
	if err != nil {
		h.log.Error("failed logout", zap.Error(err))
		errorx.MapError(err).Write(w)
		return
	}

	response.SuccessMessage(w, http.StatusOK, "logout success")
}
