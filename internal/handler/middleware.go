package handler

import (
	"bytes"
	"io"
	"net/http"

	"flowforge/pkg/jwt"

	"golang.org/x/time/rate"
)

func RateLimiter(r rate.Limit) func(http.Handler) http.Handler {
	limiter := rate.NewLimiter(r, int(r))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
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

				// Restore body for next handlers
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
