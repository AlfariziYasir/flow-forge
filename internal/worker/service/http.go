package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPAction struct {
	client *http.Client
}

func NewHTTPAction() *HTTPAction {
	return &HTTPAction{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *HTTPAction) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	method, _ := params["method"].(string)
	if method == "" {
		method = "GET"
	}
	url, _ := params["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("missing 'url' parameter")
	}

	var bodyReader io.Reader
	if body, ok := params["body"].(string); ok && body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	if headers, ok := params["headers"].(map[string]any); ok {
		for k, v := range headers {
			if val, ok := v.(string); ok {
				req.Header.Set(k, val)
			}
		}
	}

	execID, _ := params["_execution_id"].(string)
	stepID, _ := params["_step_id"].(string)
	attempt := 0
	if a, ok := params["_attempt"].(int); ok {
		attempt = a
	}
	if execID != "" && stepID != "" {
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%s:%s:%d", execID, stepID, attempt)))
		idempotencyKey := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status_code": float64(resp.StatusCode),
		"body":        string(respBody),
	}, nil
}
