package handler

import (
	"fmt"
	"net/http"
	"time"

	"flowforge/internal/broadcaster"
	"flowforge/pkg/jwt"
	"flowforge/pkg/logger"
)

type SSEHandler struct {
	log *logger.Logger
}

func NewSSEHandler(l *logger.Logger) *SSEHandler {
	return &SSEHandler{log: l}
}

func (h *SSEHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.log.Error("streaming not supported")
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	tenantID := jwt.GetTenant(r.Context())
	if tenantID == "" {
		h.log.Error("tenant_id missing from context")
		http.Error(w, "Unauthorized: tenant_id missing", http.StatusUnauthorized)
		return
	}

	clientChan := make(chan []byte, 10)

	b := broadcaster.Get()
	b.Register(tenantID, clientChan)

	defer func() {
		b.Unregister(tenantID, clientChan)
	}()

	fmt.Fprintf(w, "event: connected\ndata: {\"status\": \"connected\"}\n\n")
	flusher.Flush()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-time.After(30 * time.Second):
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
