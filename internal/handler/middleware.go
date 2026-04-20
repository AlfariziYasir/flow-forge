package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"flowforge/pkg/jwt"
	"flowforge/pkg/logger"
	"flowforge/pkg/redis"

	"go.uber.org/zap"
)

func RateLimiter(log *logger.Logger, cache redis.Cache, tenantLimit, tenantRate int, globalLimit, globalRate int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 1. Global Rate Limit
			globalKey := "rate_limit:global"
			allowed, err := cache.Allow(ctx, globalKey, globalLimit, globalRate)
			if err != nil {
				log.Error("global rate limiter error, failing open", zap.Error(err), zap.String("key", globalKey))
			}
			if !allowed && err == nil {
				http.Error(w, "Global rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// 2. Scoped Rate Limit (Tenant or IP)
			var scopeKey string
			tenantID := jwt.GetTenant(ctx)
			if tenantID != "" {
				scopeKey = fmt.Sprintf("rate_limit:tenant:%s", tenantID)
			} else {
				ip := r.Header.Get("X-Forwarded-For")
				if ip == "" {
					ip = r.RemoteAddr
					if idx := strings.LastIndex(ip, ":"); idx != -1 {
						ip = ip[:idx]
					}
				} else {
					ips := strings.Split(ip, ",")
					ip = strings.TrimSpace(ips[0])
				}
				scopeKey = fmt.Sprintf("rate_limit:ip:%s", ip)
			}

			allowed, err = cache.Allow(ctx, scopeKey, tenantLimit, tenantRate)
			if err != nil {
				log.Error("scoped rate limiter error, failing open", zap.Error(err), zap.String("key", scopeKey))
			}
			if !allowed && err == nil {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RBACMiddleware(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := jwt.GetRole(r.Context())

			allowed := false
			for _, rr := range requiredRoles {
				if role == rr {
					allowed = true
					break
				}
			}

			if !allowed {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func InputSanitizer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost || r.Method == http.MethodPut {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Error reading request body", http.StatusInternalServerError)
					return
				}
				r.Body.Close()

				r.Body = io.NopCloser(bytes.NewBuffer(body))

				if bytes.Contains(bytes.ToLower(body), []byte("<script")) {
					http.Error(w, "Malicious input detected", http.StatusBadRequest)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
