// Package client implements the VT-UOS client terminal (vtuos connect): a
// managed display/operator node that registers with a master server, renders
// the live operational state it streams, reports telemetry back on a heartbeat,
// and executes remote operations the master issues.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/vtuos/vtuos/internal/protocol"
)

// Conn is the HTTP transport between a client terminal and the master server.
type Conn struct {
	base    string
	hc      *http.Client
	name    string
	kind    protocol.ClientKind
	version string

	id        string
	token     string
	heartbeat int
}

// NewConn creates a transport to the master at addr (host:port or full URL).
func NewConn(addr, name string, kind protocol.ClientKind, version string) *Conn {
	base := strings.TrimRight(addr, "/")
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	return &Conn{
		base:      base,
		hc:        &http.Client{Timeout: 15 * time.Second},
		name:      name,
		kind:      kind,
		version:   version,
		heartbeat: 5,
	}
}

// HeartbeatInterval returns the cadence the master requested.
func (c *Conn) HeartbeatInterval() time.Duration {
	if c.heartbeat <= 0 {
		return 5 * time.Second
	}
	return time.Duration(c.heartbeat) * time.Second
}

// VaultName returns the registered vault designation (after Register).
func (c *Conn) ID() string { return c.id }

// Register admits this terminal to the master and stores its credentials.
func (c *Conn) Register(ctx context.Context) (protocol.RegisterResponse, error) {
	body, _ := json.Marshal(protocol.RegisterRequest{
		Protocol: protocol.Version,
		Name:     c.name,
		Kind:     c.kind,
		Version:  c.version,
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", c.base+"/api/v1/clients/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return protocol.RegisterResponse{}, fmt.Errorf("registering: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return protocol.RegisterResponse{}, fmt.Errorf("register rejected: %s", resp.Status)
	}
	var rr protocol.RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return protocol.RegisterResponse{}, fmt.Errorf("decoding registration: %w", err)
	}
	c.id = rr.ClientID
	c.token = rr.Token
	if rr.HeartbeatSeconds > 0 {
		c.heartbeat = rr.HeartbeatSeconds
	}
	return rr, nil
}

// StreamStates opens the master's SSE stream and delivers each state frame to
// out until the context is cancelled or the stream fails.
func (c *Conn) StreamStates(ctx context.Context, out chan<- protocol.VaultState) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.base+"/api/v1/stream", nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("opening stream: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stream rejected: %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var data strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			// End of an event: flush any accumulated data as a state frame.
			if data.Len() > 0 {
				var st protocol.VaultState
				if err := json.Unmarshal([]byte(data.String()), &st); err == nil {
					select {
					case out <- st:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				data.Reset()
			}
		case strings.HasPrefix(line, "data:"):
			data.WriteString(strings.TrimSpace(line[5:]))
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return scanner.Err()
}

// Heartbeat reports telemetry and returns any pending remote operations.
func (c *Conn) Heartbeat(ctx context.Context, tel protocol.Telemetry) ([]protocol.Command, error) {
	body, _ := json.Marshal(tel)
	url := fmt.Sprintf("%s/api/v1/clients/%s/heartbeat", c.base, c.id)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Token", c.token)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("heartbeat rejected: %s", resp.Status)
	}
	var hr protocol.HeartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&hr); err != nil {
		return nil, err
	}
	return hr.Commands, nil
}

// ReportResult reports the outcome of a remote operation to the master.
func (c *Conn) ReportResult(ctx context.Context, res protocol.CommandResult) error {
	body, _ := json.Marshal(res)
	url := fmt.Sprintf("%s/api/v1/clients/%s/result", c.base, c.id)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Token", c.token)
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
