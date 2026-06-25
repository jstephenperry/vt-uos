package client

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vtuos/vtuos/internal/config"
	"github.com/vtuos/vtuos/internal/protocol"
	"github.com/vtuos/vtuos/internal/tui"
)

// model is the Bubble Tea model for a client display terminal.
type model struct {
	conn  *Conn
	theme *tui.Theme
	start time.Time

	width, height int
	state         protocol.VaultState
	haveState     bool
	connected     bool
	vault         string

	page        string
	banner      string
	bannerUntil time.Time
	kiosk       bool
	frames      uint64
	quitting    bool

	stateCh chan protocol.VaultState
	connCh  chan bool
}

type stateMsg protocol.VaultState
type connMsg bool
type hbTickMsg struct{}
type hbResultMsg struct {
	commands []protocol.Command
	err      error
}
type secondMsg struct{}

func newModel(conn *Conn, theme *tui.Theme, vault string) *model {
	return &model{
		conn:    conn,
		theme:   theme,
		start:   time.Now(),
		vault:   vault,
		page:    "dashboard",
		stateCh: make(chan protocol.VaultState, 8),
		connCh:  make(chan bool, 8),
	}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.listenState(),
		m.listenConn(),
		hbTickCmd(m.conn.HeartbeatInterval()),
		secondCmd(),
	)
}

func (m *model) listenState() tea.Cmd {
	return func() tea.Msg {
		st, ok := <-m.stateCh
		if !ok {
			return nil
		}
		return stateMsg(st)
	}
}

func (m *model) listenConn() tea.Cmd {
	return func() tea.Msg {
		v, ok := <-m.connCh
		if !ok {
			return nil
		}
		return connMsg(v)
	}
}

func hbTickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return hbTickMsg{} })
}

func secondCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return secondMsg{} })
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case stateMsg:
		m.state = protocol.VaultState(msg)
		m.haveState = true
		m.connected = true
		if m.state.VaultDesignation != "" {
			m.vault = m.state.VaultDesignation
		}
		return m, m.listenState()

	case connMsg:
		if !bool(msg) {
			m.connected = false
		}
		return m, m.listenConn()

	case hbTickMsg:
		return m, tea.Batch(m.doHeartbeat(), hbTickCmd(m.conn.HeartbeatInterval()))

	case hbResultMsg:
		if msg.err != nil {
			m.connected = false
			return m, nil
		}
		return m.applyCommands(msg.commands)

	case secondMsg:
		m.frames++
		if m.banner != "" && time.Now().After(m.bannerUntil) {
			m.banner = ""
		}
		if m.quitting {
			return m, tea.Quit
		}
		return m, secondCmd()
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}
	if m.kiosk {
		return m, nil // locked down for unattended display
	}
	switch msg.String() {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit
	case "1":
		m.page = "dashboard"
	case "2":
		m.page = "systems"
	case "3":
		m.page = "resources"
	case "4":
		m.page = "events"
	case "5":
		m.page = "population"
	}
	return m, nil
}

// doHeartbeat snapshots telemetry on the main goroutine and reports it.
func (m *model) doHeartbeat() tea.Cmd {
	tel := protocol.Telemetry{
		UptimeSeconds:  int64(time.Since(m.start).Seconds()),
		CurrentView:    m.page,
		KioskMode:      m.kiosk,
		FramesRendered: m.frames,
		WidthCols:      m.width,
		HeightRows:     m.height,
	}
	conn := m.conn
	return func() tea.Msg {
		cmds, err := conn.Heartbeat(context.Background(), tel)
		return hbResultMsg{commands: cmds, err: err}
	}
}

// applyCommands executes remote operations and acknowledges them to the master.
func (m *model) applyCommands(cmds []protocol.Command) (tea.Model, tea.Cmd) {
	m.connected = true
	var effects []tea.Cmd
	for _, c := range cmds {
		ok, message := true, "applied"
		switch c.Type {
		case protocol.CmdSwitchView:
			if v := c.Args["view"]; v != "" {
				m.page = v
			}
		case protocol.CmdSetKiosk:
			m.kiosk = c.Args["enabled"] == "true"
		case protocol.CmdMessage:
			m.banner = c.Args["text"]
			m.bannerUntil = time.Now().Add(10 * time.Second)
		case protocol.CmdIdentify:
			m.banner = "◢◤  THIS TERMINAL — " + m.conn.ID()[:8] + "  ◥◣"
			m.bannerUntil = time.Now().Add(6 * time.Second)
		case protocol.CmdRefresh:
			message = "refreshed"
		case protocol.CmdReboot:
			m.banner = "RECONNECTING TERMINAL…"
			m.bannerUntil = time.Now().Add(5 * time.Second)
		case protocol.CmdShutdown:
			m.quitting = true
			effects = append(effects, tea.Quit)
		default:
			ok, message = false, "unsupported command"
		}
		effects = append(effects, m.reportResult(c.ID, ok, message))
	}
	return m, tea.Batch(effects...)
}

func (m *model) reportResult(cmdID string, ok bool, message string) tea.Cmd {
	conn := m.conn
	res := protocol.CommandResult{CommandID: cmdID, OK: ok, Message: message, Completed: time.Now().UTC()}
	return func() tea.Msg {
		_ = conn.ReportResult(context.Background(), res)
		return nil
	}
}

// Run registers with the master and runs the client display terminal.
func Run(ctx context.Context, conn *Conn, cfg *config.Config) error {
	rr, err := registerWithRetry(ctx, conn)
	if err != nil {
		return err
	}
	theme := tui.NewTheme(cfg.Display.ColorScheme)
	m := newModel(conn, theme, rr.VaultDesignation)

	// Stream live state in the background, reconnecting on failure.
	go func() {
		for ctx.Err() == nil {
			if err := conn.StreamStates(ctx, m.stateCh); err != nil {
				select {
				case m.connCh <- false:
				default:
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}()

	p := tea.NewProgram(m, tea.WithAltScreen())
	go func() {
		<-ctx.Done()
		p.Quit()
	}()
	_, err = p.Run()
	return err
}

func registerWithRetry(ctx context.Context, conn *Conn) (protocol.RegisterResponse, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		rr, err := conn.Register(ctx)
		if err == nil {
			return rr, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return protocol.RegisterResponse{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
	}
	return protocol.RegisterResponse{}, fmt.Errorf("could not reach master server: %w", lastErr)
}
