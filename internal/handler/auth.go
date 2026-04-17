package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"flowforge/internal/model"
	"flowforge/internal/repository"
	"flowforge/pkg/jwt"
	"flowforge/pkg/logger"

	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	userRepo repository.UserRepository
	tm       jwt.TokenManager
	l        *logger.Logger
}

func NewAuthHandler(userRepo repository.UserRepository, tm jwt.TokenManager, l *logger.Logger) *AuthHandler {
	return &AuthHandler{
		userRepo: userRepo,
		tm:       tm,
		l:        l,
	}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"` // In MVP, we might just compare clear text or mock
}

type LoginResponse struct {
	Token string `json:"token"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var user model.User
	err := h.userRepo.Get(context.Background(), map[string]any{"email": req.Email}, true, &user)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate token (valid for 24h)
	token, err := h.tm.Generate(user.ID, user.TenantID, user.Role, 24*time.Hour)
	if err != nil {
		h.l.Error("failed to generate token")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{Token: token})
}
