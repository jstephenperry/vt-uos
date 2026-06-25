package server

import (
	"sync"
	"time"

	"github.com/vtuos/vtuos/internal/protocol"
	"github.com/vtuos/vtuos/internal/util"
)

// clientEntry is the master's record of a connected terminal.
type clientEntry struct {
	info    protocol.ClientInfo
	token   string
	pending []protocol.Command
	results []protocol.CommandResult
}

// ClientRegistry tracks connected client terminals, the remote-operation queue
// for each, and their reported telemetry. It is safe for concurrent use.
type ClientRegistry struct {
	mu      sync.RWMutex
	clients map[string]*clientEntry
	timeout time.Duration
	idGen   *util.IDGenerator
	nowFn   func() time.Time
}

// NewClientRegistry creates a registry that considers a client offline after it
// has been silent for longer than timeout.
func NewClientRegistry(timeout time.Duration) *ClientRegistry {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &ClientRegistry{
		clients: make(map[string]*clientEntry),
		timeout: timeout,
		idGen:   util.NewIDGenerator(),
		nowFn:   time.Now,
	}
}

// Register admits a new client terminal and returns its assigned id and token.
func (r *ClientRegistry) Register(req protocol.RegisterRequest, addr string) protocol.RegisterResponse {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := r.idGen.NewID()
	token := r.idGen.NewID()
	now := r.nowFn()
	kind := req.Kind
	if kind != protocol.ClientOperator {
		kind = protocol.ClientDisplay
	}
	name := req.Name
	if name == "" {
		name = "terminal-" + id[:8]
	}

	r.clients[id] = &clientEntry{
		token: token,
		info: protocol.ClientInfo{
			ID:        id,
			Name:      name,
			Kind:      kind,
			Address:   addr,
			Version:   req.Version,
			Connected: now,
			LastSeen:  now,
			Online:    true,
		},
	}
	return protocol.RegisterResponse{
		ClientID:   id,
		Token:      token,
		Protocol:   protocol.Version,
		ServerTime: now,
	}
}

// Heartbeat records a client's telemetry and returns (and clears) any pending
// remote operations queued for it.
func (r *ClientRegistry) Heartbeat(id, token string, tel protocol.Telemetry) ([]protocol.Command, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[id]
	if !ok || c.token != token {
		return nil, false
	}
	c.info.LastSeen = r.nowFn()
	c.info.Online = true
	c.info.Telemetry = tel
	cmds := c.pending
	c.pending = nil
	c.info.PendingCmds = 0
	return cmds, true
}

// ReportResult records the outcome of a remote operation reported by a client.
func (r *ClientRegistry) ReportResult(id, token string, res protocol.CommandResult) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[id]
	if !ok || c.token != token {
		return false
	}
	c.results = append(c.results, res)
	if len(c.results) > 20 {
		c.results = c.results[len(c.results)-20:]
	}
	return true
}

// IssueCommand queues a remote operation for the given client.
func (r *ClientRegistry) IssueCommand(id string, cmd protocol.Command) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[id]
	if !ok {
		return false
	}
	if cmd.ID == "" {
		cmd.ID = r.idGen.NewID()
	}
	if cmd.Issued.IsZero() {
		cmd.Issued = r.nowFn()
	}
	c.pending = append(c.pending, cmd)
	c.info.PendingCmds = len(c.pending)
	return true
}

// List returns a snapshot of all known clients with online status computed from
// the heartbeat timeout.
func (r *ClientRegistry) List() []protocol.ClientInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := r.nowFn()
	out := make([]protocol.ClientInfo, 0, len(r.clients))
	for _, c := range r.clients {
		info := c.info
		info.Online = now.Sub(info.LastSeen) < r.timeout
		info.PendingCmds = len(c.pending)
		out = append(out, info)
	}
	return out
}

// Get returns a single client's info, or false if unknown.
func (r *ClientRegistry) Get(id string) (protocol.ClientInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.clients[id]
	if !ok {
		return protocol.ClientInfo{}, false
	}
	info := c.info
	info.Online = r.nowFn().Sub(info.LastSeen) < r.timeout
	info.PendingCmds = len(c.pending)
	return info, true
}

// Count returns the number of clients and how many are currently online.
func (r *ClientRegistry) Count() (total, online int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := r.nowFn()
	for _, c := range r.clients {
		total++
		if now.Sub(c.info.LastSeen) < r.timeout {
			online++
		}
	}
	return total, online
}
