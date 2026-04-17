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

func NewRouter(
	authHandler *AuthHandler,
	wfHandler *WorkflowHandler,
	execHandler *ExecutionHandler,
	userHandler *UserHandler,
	tenantHandler *TenantHandler,
	aiHandler *AIHandler,
	tm jwt.TokenManager,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(RateLimiter(50)) // 50 req/sec global
	r.Use(InputSanitizer())
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
			r.Put("/{id}", wfHandler.Update)
			r.Delete("/{name}", wfHandler.Delete)
			r.Post("/{id}/trigger", wfHandler.Trigger)
			r.Post("/{id}/rollback", wfHandler.Rollback)
			r.Get("/{name}/versions", wfHandler.ListVersions)
		})

		r.Route("/executions", func(r chi.Router) {
			r.Get("/", execHandler.List)
			r.Get("/{id}", execHandler.Get)
			r.Post("/{id}/retry", execHandler.Retry)
			r.Post("/{id}/cancel", execHandler.Cancel)
		})

		r.Route("/ai", func(r chi.Router) {
			r.Post("/generate", aiHandler.GenerateDAG)
			r.Post("/analyze", aiHandler.AnalyzeFailure)
		})

		// Admin only routes
		r.Group(func(r chi.Router) {
			r.Use(RBACMiddleware("admin"))
			
			r.Route("/tenants", func(r chi.Router) {
				r.Post("/", tenantHandler.Create)
				r.Get("/", tenantHandler.List)
				r.Get("/{id}", tenantHandler.Get)
				r.Put("/{id}", tenantHandler.Update)
				r.Delete("/{id}", tenantHandler.Delete)
			})

			r.Route("/users", func(r chi.Router) {
				r.Post("/", userHandler.Create)
				r.Get("/", userHandler.List)
				r.Get("/{id}", userHandler.Get)
				r.Put("/{id}", userHandler.Update)
				r.Delete("/{id}", userHandler.Delete)
			})
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
