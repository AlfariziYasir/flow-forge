package handler_test

import (
	"context"
	"errors"
	"flowforge/internal/handler"
	"flowforge/pkg/jwt"
	"flowforge/pkg/logger"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockCache struct {
	mock.Mock
}

func (m *mockCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return m.Called(ctx, key, value, ttl).Error(0)
}
func (m *mockCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}
func (m *mockCache) GetStruct(ctx context.Context, key string, dest any) error {
	return m.Called(ctx, key, dest).Error(0)
}
func (m *mockCache) Delete(ctx context.Context, keys ...string) error {
	return m.Called(ctx, keys).Error(0)
}
func (m *mockCache) DeleteByPattern(ctx context.Context, pattern string) error {
	return m.Called(ctx, pattern).Error(0)
}
func (m *mockCache) Client() redis.UniversalClient {
	return nil
}
func (m *mockCache) Publish(ctx context.Context, channel string, message any) error {
	return m.Called(ctx, channel, message).Error(0)
}
func (m *mockCache) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return nil
}
func (m *mockCache) RPush(ctx context.Context, key string, values ...any) error {
	return m.Called(ctx, key, values).Error(0)
}
func (m *mockCache) BLPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	args := m.Called(ctx, timeout, keys)
	return args.Get(0).([]string), args.Error(1)
}
func (m *mockCache) Allow(ctx context.Context, key string, limit int, rate int) (bool, error) {
	args := m.Called(ctx, key, limit, rate)
	return args.Bool(0), args.Error(1)
}
func (m *mockCache) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, key, value, ttl)
	return args.Bool(0), args.Error(1)
}

func TestRateLimiter_UnderLimit(t *testing.T) {
	cache := new(mockCache)
	// Global limit
	cache.On("Allow", mock.Anything, "rate_limit:global", 1000, 1000).Return(true, nil)
	// IP limit (RemoteAddr defaults to 192.0.2.1:1234 in httptest)
	cache.On("Allow", mock.Anything, "rate_limit:ip:192.0.2.1", 5, 5).Return(true, nil)

	mw := handler.RateLimiter(logger.NewNop(), cache, 5, 5, 1000, 1000)
	
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimiter_OverLimit(t *testing.T) {
	cache := new(mockCache)
	// Global limit
	cache.On("Allow", mock.Anything, "rate_limit:global", 1000, 1000).Return(true, nil)
	// First call allowed, second denied
	cache.On("Allow", mock.Anything, "rate_limit:ip:192.0.2.1", 1, 1).Return(true, nil).Once()
	cache.On("Allow", mock.Anything, "rate_limit:ip:192.0.2.1", 1, 1).Return(false, nil).Once()

	mw := handler.RateLimiter(logger.NewNop(), cache, 1, 1, 1000, 1000)
	
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

func TestRateLimiter_FailOpen(t *testing.T) {
	cache := new(mockCache)
	// Return an error from Redis
	cache.On("Allow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, errors.New("redis down"))

	mw := handler.RateLimiter(logger.NewNop(), cache, 5, 5, 1000, 1000)
	
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Should FAIL OPEN and allow the request
	assert.Equal(t, http.StatusOK, rr.Code)
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
