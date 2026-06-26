package server

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed web/index.html.tmpl web/static/*
var webFS embed.FS

// webData is injected into the console template.
type webData struct {
	Vault       string
	VaultNumber int
	Protocol    string
	Heartbeat   int
}

// registerWebRoutes mounts the embedded web administration console.
func (s *Server) registerWebRoutes(mux *http.ServeMux) {
	tmpl := template.Must(template.ParseFS(webFS, "web/index.html.tmpl"))

	staticFS, err := fs.Sub(webFS, "web/static")
	if err != nil {
		s.log.Error("loading web assets failed", "error", err)
		return
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmpl.Execute(w, webData{
			Vault:       s.cfg.Vault.Designation,
			VaultNumber: s.cfg.Vault.Number,
			Protocol:    "1.0",
			Heartbeat:   s.heartbeatSeconds(),
		})
	})
}
