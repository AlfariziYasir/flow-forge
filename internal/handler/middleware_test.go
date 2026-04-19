package handler_test

import (

	"flowforge/internal/handler"
	"flowforge/pkg/jwt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_UnderLimit(t *testing.T) {
	// 5 requests per second
	mw := handler.RateLimiter(5)
	
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimiter_OverLimit(t *testing.T) {
	// 1 request per second
	mw := handler.RateLimiter(1)
	
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	
	// First request succeeds
	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, req)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Second request fails
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req)
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
}

func TestRBACMiddleware(t *testing.T) {
	mw := handler.RBACMiddleware("admin")
	
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("Authorized Admin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := req.Context()
		ctx = jwt.SetContext(ctx, jwt.RoleKey, "admin")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req.WithContext(ctx))
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Unauthorized Viewer", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := req.Context()
		ctx = jwt.SetContext(ctx, jwt.RoleKey, "viewer")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req.WithContext(ctx))
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

func TestInputSanitizer(t *testing.T) {
	mw := handler.InputSanitizer()
	
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("Clean Input", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name": "test"}`))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Malicious Input", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name": "<script>alert(1)</script>"}`))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
