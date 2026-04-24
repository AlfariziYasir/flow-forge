package handler

import (
	"fmt"
	"net/http"
	"strings"

	"flowforge/config"
	"flowforge/pkg/errorx"
	"flowforge/pkg/jwt"
	"flowforge/pkg/logger"
	"flowforge/pkg/redis"

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
	cfg *config.Config,
	cache redis.Cache,
	log *logger.Logger,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(RateLimiter(log, cache, 50, 50, 1000, 1000))
	r.Use(InputSanitizer())
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

	r.Post("/api/v1/auth/login", authHandler.Login)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMiddleware(cfg, cache))

		r.Route("/auth/", func(r chi.Router) {
			r.Get("refresh", authHandler.Refresh)
			r.Get("logout", authHandler.Logout)
		})

		r.Get("/monitor/stream", SSEHandler)

		r.Route("/workflows", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(RBACMiddleware("admin", "editor"))
				r.Post("/", wfHandler.Create)
				r.Put("/{id}", wfHandler.Update)
				r.Delete("/{name}", wfHandler.Delete)
				r.Post("/{id}/trigger", wfHandler.Trigger)
				r.Post("/{id}/rollback", wfHandler.Rollback)
			})
			r.Get("/", wfHandler.List)
			r.Get("/{id}", wfHandler.Get)
			r.Get("/{name}/versions", wfHandler.ListVersions)
		})

		r.Route("/executions", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(RBACMiddleware("admin", "editor"))
				r.Post("/{id}/retry", execHandler.Retry)
				r.Post("/{id}/cancel", execHandler.Cancel)
			})
			r.Get("/", execHandler.List)
			r.Get("/{id}", execHandler.Get)
		})

		r.Route("/ai", func(r chi.Router) {
			r.Use(RBACMiddleware("admin", "editor"))
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

func authMiddleware(cfg *config.Config, cache redis.Cache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				errorx.HttpNewError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				errorx.HttpNewError(w, http.StatusUnauthorized, "invalid authorization header format")
				return
			}

			secretKey := ""
			if strings.Contains(r.URL.String(), "refresh") {
				secretKey = cfg.RefreshTokenKey
			} else {
				secretKey = cfg.AccessTokenKey
			}

			claims, err := jwt.TokenValid(parts[1], secretKey)
			if err != nil {
				errorx.HttpNewError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			jti, ok := claims["jti"].(string)
			if !ok {
				errorx.HttpNewError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			refUuid := ""
			accUuid := ""
			if strings.Contains(r.URL.String(), "refresh") {
				val, err := cache.Get(r.Context(), fmt.Sprintf("%s:%s", jwt.RefKey, jti))
				if val == "" {
					errorx.HttpNewError(w, http.StatusUnauthorized, "token has been revoked")
					return
				}

				if err != nil && err != redis.ErrCacheMiss {
					errorx.HttpNewError(w, http.StatusUnauthorized, "internal auth error")
					return
				}

				refUuid = jti
			} else {
				val, err := cache.Get(r.Context(), fmt.Sprintf("%s:%s", jwt.BlacklistKey, jti))
				if val == "" {
					errorx.HttpNewError(w, http.StatusUnauthorized, "token has been revoked")
					return
				}

				if err != nil && err != redis.ErrCacheMiss {
					errorx.HttpNewError(w, http.StatusUnauthorized, "internal auth error")
					return
				}

				accUuid = jti
				refUuid = claims["ref_uuid"].(string)
			}

			ctx := jwt.SetContext(r.Context(), jwt.UserKey, claims["user_id"].(string))
			ctx = jwt.SetContext(ctx, jwt.TenantKey, claims["tenant_id"].(string))
			ctx = jwt.SetContext(ctx, jwt.RoleKey, string(claims["role"].(string)))
			ctx = jwt.SetContext(ctx, jwt.AccUuidKey, accUuid)
			ctx = jwt.SetContext(ctx, jwt.RefUuidKey, refUuid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
