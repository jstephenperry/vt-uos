// Package server implements the VT-UOS master server: an authoritative
// simulation host that exposes operational state and control over a stdlib
// net/http JSON API, streams live state via Server-Sent Events, manages
// connected client terminals (registration, telemetry, and remote operations),
// and serves an embedded web administration console.
//
// It uses only the Go standard library for transport — no web framework — in
// keeping with the project's single-static-binary constraint.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/vtuos/vtuos/internal/config"
	"github.com/vtuos/vtuos/internal/simulation"
)

// Server is the master server.
type Server struct {
	engine  *simulation.Engine
	cfg     *config.Config
	clients *ClientRegistry
	log     *slog.Logger
	httpSrv *http.Server
}

// New constructs a master server bound to the given control core and config.
func New(engine *simulation.Engine, cfg *config.Config) (*Server, error) {
	timeout := time.Duration(cfg.Server.ClientTimeoutSeconds) * time.Second
	s := &Server{
		engine:  engine,
		cfg:     cfg,
		clients: NewClientRegistry(timeout),
		log:     slog.Default().With("component", "server"),
	}
	s.httpSrv = &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s, nil
}

// Registry exposes the client registry (used by tests and introspection).
func (s *Server) Registry() *ClientRegistry { return s.clients }

func (s *Server) heartbeatSeconds() int {
	if s.cfg.Server.HeartbeatSeconds > 0 {
		return s.cfg.Server.HeartbeatSeconds
	}
	return 5
}

// routes builds the HTTP request multiplexer.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Liveness and machine-readable health.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)

	// Operational read endpoints (open so display terminals can poll freely).
	mux.HandleFunc("GET /api/v1/state", s.handleState)
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/events", s.handleEvents)
	mux.HandleFunc("GET /api/v1/alerts", s.handleAlerts)
	mux.HandleFunc("GET /api/v1/systems", s.handleSystems)
	mux.HandleFunc("GET /api/v1/population", s.handlePopulation)
	mux.HandleFunc("GET /api/v1/resources", s.handleResources)
	mux.HandleFunc("GET /api/v1/stream", s.handleStream)

	// Simulation control (operator token required).
	mux.HandleFunc("POST /api/v1/sim/pause", s.requireAdmin(s.handleSimPause))
	mux.HandleFunc("POST /api/v1/sim/resume", s.requireAdmin(s.handleSimResume))
	mux.HandleFunc("POST /api/v1/sim/scale", s.requireAdmin(s.handleSimScale))
	mux.HandleFunc("POST /api/v1/sim/step", s.requireAdmin(s.handleSimStep))
	mux.HandleFunc("POST /api/v1/sim/snapshot", s.requireAdmin(s.handleSimSnapshot))
	mux.HandleFunc("POST /api/v1/sim/ack", s.requireAdmin(s.handleAckAlert))

	// Client terminals: registration/heartbeat/result use per-client tokens.
	mux.HandleFunc("POST /api/v1/clients/register", s.handleClientRegister)
	mux.HandleFunc("POST /api/v1/clients/{id}/heartbeat", s.handleClientHeartbeat)
	mux.HandleFunc("POST /api/v1/clients/{id}/result", s.handleClientResult)

	// Client management (operator token required).
	mux.HandleFunc("GET /api/v1/clients", s.requireAdmin(s.handleClientList))
	mux.HandleFunc("POST /api/v1/clients/{id}/command", s.requireAdmin(s.handleClientCommand))

	// Embedded web administration console.
	if s.cfg.Server.EnableWeb {
		s.registerWebRoutes(mux)
	}

	return logRequests(s.log, mux)
}

// Start begins serving and blocks until the server is shut down.
func (s *Server) Start() error {
	if s.cfg.Server.AdminToken == "" {
		s.log.Warn("no admin_token configured: simulation control and client management endpoints are UNAUTHENTICATED")
	}
	s.log.Info("master server listening",
		"addr", s.cfg.Server.Listen,
		"web_console", s.cfg.Server.EnableWeb,
		"vault", s.cfg.Vault.Designation,
	)
	if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

// logRequests is a lightweight access-logging middleware.
func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The state stream is long-lived; don't log it per-request beyond open.
		if r.URL.Path == "/api/v1/stream" {
			log.Debug("stream opened", "remote", r.RemoteAddr)
		}
		next.ServeHTTP(w, r)
	})
}
