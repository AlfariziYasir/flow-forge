package handler

import (
	"context"
	"net/http"
	"strings"

	"flowforge/pkg/jwt"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func NewRouter(authHandler *AuthHandler, wfHandler *WorkflowHandler, tm jwt.TokenManager) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"}, // For MVP
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

	r.Post("/api/v1/auth/login", authHandler.Login)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMiddleware(tm))

		r.Get("/monitor/stream", SSEHandler)

		r.Route("/workflows", func(r chi.Router) {
			r.Post("/", wfHandler.Create)
			r.Get("/", wfHandler.List)
			r.Get("/{id}", wfHandler.Get)
			r.Post("/{id}/trigger", wfHandler.Trigger)
		})
	})

	return r
}

func authMiddleware(tm jwt.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}

			claims, err := tm.Verify(parts[1])
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
			ctx = context.WithValue(ctx, "tenant_id", claims.TenantID)
			ctx = context.WithValue(ctx, "role", claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
