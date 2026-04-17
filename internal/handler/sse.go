package handler

import (
	"fmt"
	"net/http"

	"flowforge/internal/broadcaster"
)

func SSEHandler(w http.ResponseWriter, r *http.Request) {
	// Setup headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Ensure the connection supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Create a channel for this client
	clientChan := make(chan []byte)

	// Register this client with the broadcaster
	b := broadcaster.Get()
	b.Register(clientChan)

	// Ensure cleanup when the client disconnects
	defer func() {
		b.Unregister(clientChan)
	}()

	// Send an initial connected message
	fmt.Fprintf(w, "event: connected\ndata: {\"status\": \"connected\"}\n\n")
	flusher.Flush()

	// Notify client connection lost
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return
		case msg := <-clientChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}
