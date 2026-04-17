package service

import (
	"context"
	"io"
	"net/http"
	"strings"
)

type HTTPAction struct {
	client *http.Client
}

func NewHTTPAction() *HTTPAction {
	return &HTTPAction{
		client: &http.Client{},
	}
}

func (a *HTTPAction) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	method, _ := params["method"].(string)
	if method == "" {
		method = "GET"
	}
	url, _ := params["url"].(string)

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
