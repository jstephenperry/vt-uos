package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleStream serves the live operational state as Server-Sent Events. Each
// connection subscribes to the control core and receives a fresh VaultState
// frame whenever the engine recomputes, plus periodic keep-alive comments.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	id, ch := s.engine.Subscribe()
	defer s.engine.Unsubscribe(id)

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-keepAlive.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		case st, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(st)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: state\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}
