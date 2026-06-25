package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vtuos/vtuos/internal/protocol"
)

// writeJSON encodes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("encoding JSON response failed", "error", err)
	}
}

// writeError writes a JSON error envelope.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, protocol.APIError{Error: msg})
}

// adminToken extracts the operator token from the request, accepting either an
// Authorization: Bearer header or an X-Admin-Token header.
func adminToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(strings.ToLower(h), "bearer ") {
			return strings.TrimSpace(h[7:])
		}
	}
	return r.Header.Get("X-Admin-Token")
}

// adminAuthorized reports whether the request carries a valid operator token.
// When no admin token is configured the control surface is open (and a warning
// is logged at startup).
func (s *Server) adminAuthorized(r *http.Request) bool {
	if s.cfg.Server.AdminToken == "" {
		return true
	}
	return adminToken(r) == s.cfg.Server.AdminToken
}

// requireAdmin wraps a handler so it only runs for authorised operators.
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.adminAuthorized(r) {
			writeError(w, http.StatusUnauthorized, "operator token required")
			return
		}
		next(w, r)
	}
}
