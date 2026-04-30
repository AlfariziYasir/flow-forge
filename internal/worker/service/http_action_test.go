package service_test

import (
	"context"
	"flowforge/internal/worker/service"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHTTPAction_GET_Success(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	action := service.NewHTTPAction()
	params := map[string]any{
		"method": "GET",
		"url":    server.URL,
	}

	result, err := action.Execute(context.Background(), params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, `{"message": "success"}`, result["body"])
}

func TestHTTPAction_POST_WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, `{"key": "value"}`, string(body))
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status": "created"}`))
	}))
	defer server.Close()

	action := service.NewHTTPAction()
	params := map[string]any{
		"method": "POST",
		"url":    server.URL,
		"body":   `{"key": "value"}`,
	}

	result, err := action.Execute(context.Background(), params)

	assert.NoError(t, err)
	assert.Equal(t, float64(201), result["status_code"])
	assert.Equal(t, `{"status": "created"}`, result["body"])
}

func TestHTTPAction_Timeout(t *testing.T) {
	// Server that sleeps longer than action timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	action := service.NewHTTPAction()
	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	params := map[string]any{
		"method": "GET",
		"url":    server.URL,
	}

	_, err := action.Execute(ctx, params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestHTTPAction_InvalidURL(t *testing.T) {
	action := service.NewHTTPAction()
	params := map[string]any{
		"method": "GET",
		"url":    "not-a-valid-url",
	}

	_, err := action.Execute(context.Background(), params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported protocol scheme")
}

func TestHTTPAction_CustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	action := service.NewHTTPAction()
	params := map[string]any{
		"method": "GET",
		"url":    server.URL,
		"headers": map[string]any{
			"X-Test-Header": "test-value",
		},
	}

	_, err := action.Execute(context.Background(), params)

	assert.NoError(t, err)
}
