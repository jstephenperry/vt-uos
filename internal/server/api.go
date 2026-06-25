package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/vtuos/vtuos/internal/protocol"
)

// decodeJSON reads a JSON request body into v with a sane size limit.
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	return dec.Decode(v)
}

// ----- Read endpoints (open) -----

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	st := s.engine.Snapshot()
	total, online := s.clients.Count()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"protocol":       protocol.Version,
		"vault":          st.VaultDesignation,
		"vault_number":   st.VaultNumber,
		"sim_status":     st.Status,
		"vault_time":     st.VaultTime,
		"server_time":    time.Now().UTC(),
		"clients_total":  total,
		"clients_online": online,
	})
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.Snapshot())
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	st := s.engine.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        st.Status,
		"time_scale":    st.TimeScale,
		"vault_time":    st.VaultTime,
		"elapsed_days":  st.ElapsedDays,
		"elapsed_years": st.ElapsedYears,
		"tick_count":    st.TickCount,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	writeJSON(w, http.StatusOK, s.engine.Events(limit))
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.Alerts())
}

func (s *Server) handleSystems(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.Snapshot().SystemList)
}

func (s *Server) handlePopulation(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.Snapshot().Population)
}

func (s *Server) handleResources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.Snapshot().Resources)
}

// ----- Simulation control endpoints (admin) -----

func (s *Server) handleSimPause(w http.ResponseWriter, r *http.Request) {
	s.engine.Pause()
	s.log.Info("simulation paused via API", "by", r.RemoteAddr)
	writeJSON(w, http.StatusOK, s.engine.Snapshot())
}

func (s *Server) handleSimResume(w http.ResponseWriter, r *http.Request) {
	s.engine.Resume()
	s.log.Info("simulation resumed via API", "by", r.RemoteAddr)
	writeJSON(w, http.StatusOK, s.engine.Snapshot())
}

func (s *Server) handleSimScale(w http.ResponseWriter, r *http.Request) {
	var req protocol.ControlRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.engine.SetTimeScale(req.TimeScale); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Info("time scale changed via API", "scale", req.TimeScale, "by", r.RemoteAddr)
	writeJSON(w, http.StatusOK, s.engine.Snapshot())
}

func (s *Server) handleSimStep(w http.ResponseWriter, r *http.Request) {
	var req protocol.ControlRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	hours := req.Hours
	if hours <= 0 {
		hours = 24
	}
	if hours > 8760 {
		hours = 8760 // cap a single manual step to one simulated year
	}
	if err := s.engine.Step(r.Context(), hours); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.log.Info("simulation stepped via API", "hours", hours, "by", r.RemoteAddr)
	writeJSON(w, http.StatusOK, s.engine.Snapshot())
}

func (s *Server) handleSimSnapshot(w http.ResponseWriter, r *http.Request) {
	path, err := s.engine.CreateSnapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"snapshot": path})
}

func (s *Server) handleAckAlert(w http.ResponseWriter, r *http.Request) {
	var req protocol.ControlRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ok := s.engine.AcknowledgeAlert(req.Code)
	writeJSON(w, http.StatusOK, map[string]bool{"acknowledged": ok})
}

// ----- Client management endpoints -----

func (s *Server) handleClientRegister(w http.ResponseWriter, r *http.Request) {
	var req protocol.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Protocol != "" && req.Protocol != protocol.Version {
		writeError(w, http.StatusConflict, "protocol version mismatch")
		return
	}
	resp := s.clients.Register(req, r.RemoteAddr)
	resp.VaultDesignation = s.cfg.Vault.Designation
	resp.HeartbeatSeconds = s.heartbeatSeconds()
	s.log.Info("client registered", "id", resp.ClientID, "name", req.Name, "addr", r.RemoteAddr)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleClientHeartbeat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := r.Header.Get("X-Client-Token")
	var tel protocol.Telemetry
	if err := decodeJSON(r, &tel); err != nil {
		writeError(w, http.StatusBadRequest, "invalid telemetry body")
		return
	}
	cmds, ok := s.clients.Heartbeat(id, token, tel)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unknown client or bad token")
		return
	}
	writeJSON(w, http.StatusOK, protocol.HeartbeatResponse{
		Acknowledged: true,
		ServerTime:   time.Now().UTC(),
		Commands:     cmds,
	})
}

func (s *Server) handleClientResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := r.Header.Get("X-Client-Token")
	var res protocol.CommandResult
	if err := decodeJSON(r, &res); err != nil {
		writeError(w, http.StatusBadRequest, "invalid result body")
		return
	}
	if !s.clients.ReportResult(id, token, res) {
		writeError(w, http.StatusUnauthorized, "unknown client or bad token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleClientList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.clients.List())
}

func (s *Server) handleClientCommand(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var cmd protocol.Command
	if err := decodeJSON(r, &cmd); err != nil {
		writeError(w, http.StatusBadRequest, "invalid command body")
		return
	}
	if cmd.Type == "" {
		writeError(w, http.StatusBadRequest, "command type required")
		return
	}
	if !s.clients.IssueCommand(id, cmd) {
		writeError(w, http.StatusNotFound, "unknown client")
		return
	}
	s.log.Info("remote operation queued", "client", id, "type", cmd.Type, "by", r.RemoteAddr)
	writeJSON(w, http.StatusAccepted, map[string]string{"queued": string(cmd.Type)})
}
