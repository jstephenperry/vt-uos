// Package protocol defines the wire types shared between the VT-UOS master
// server, its web administration console, and connected client terminals.
//
// These types are deliberately dependency-free (standard library only) so they
// can be marshalled to JSON and exchanged over HTTP without coupling the
// transport layers to the internal domain models. They describe operational
// state — the same information an operator would read off a control board — not
// game state.
package protocol

import "time"

// Version is the protocol revision. Clients and the master compare this on
// registration to detect incompatible deployments.
const Version = "1.0"

// SimStatus describes the run state of the simulation control core.
type SimStatus string

const (
	// SimStopped indicates the engine is not advancing vault time.
	SimStopped SimStatus = "STOPPED"
	// SimRunning indicates the engine is advancing vault time automatically.
	SimRunning SimStatus = "RUNNING"
	// SimPaused indicates the engine is loaded but holding vault time steady.
	SimPaused SimStatus = "PAUSED"
)

// AlertLevel classifies the severity of an operational alert.
type AlertLevel string

const (
	AlertInfo     AlertLevel = "INFO"
	AlertWarning  AlertLevel = "WARNING"
	AlertCritical AlertLevel = "CRITICAL"
)

// Alert is an actionable condition surfaced by the control core.
type Alert struct {
	Code         string     `json:"code"`
	Level        AlertLevel `json:"level"`
	Message      string     `json:"message"`
	Subsystem    string     `json:"subsystem"`
	Raised       time.Time  `json:"raised"`
	Acknowledged bool       `json:"acknowledged"`
}

// EventRecord is an entry in the operational event log produced by the engine.
// Events correspond to real occurrences (equipment faults, spoilage, medical
// incidents, recovered caches) and are mirrored to the persistent audit log.
type EventRecord struct {
	ID       string     `json:"id"`
	VaultTime time.Time `json:"vault_time"`
	Category string     `json:"category"`
	Type     string     `json:"type"`
	Severity AlertLevel `json:"severity"`
	Summary  string     `json:"summary"`
	Detail   string     `json:"detail,omitempty"`
}

// ResourceStatus summarises a single resource category for the operations board.
type ResourceStatus struct {
	Category     string  `json:"category"`
	Label        string  `json:"label"`
	OnHand       float64 `json:"on_hand"`
	Unit         string  `json:"unit"`
	DailyUse     float64 `json:"daily_use"`
	RunwayDays   int     `json:"runway_days"` // -1 means effectively unlimited
	Status       string  `json:"status"`      // OK | WATCH | WARNING | CRITICAL
	FillFraction float64 `json:"fill_fraction"`
}

// SystemStatus summarises a single facility system for the operations board.
type SystemStatus struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Category   string  `json:"category"`
	Status     string  `json:"status"`
	Efficiency float64 `json:"efficiency"`
	Critical   bool    `json:"critical"`
}

// PowerStatus describes the vault's electrical balance.
type PowerStatus struct {
	GenerationKW  float64 `json:"generation_kw"`
	ConsumptionKW float64 `json:"consumption_kw"`
	BalanceKW     float64 `json:"balance_kw"`
	ReservePct    float64 `json:"reserve_pct"`
}

// PopulationStatus summarises the census for the operations board.
type PopulationStatus struct {
	Active      int     `json:"active"`
	Capacity    int     `json:"capacity"`
	Deceased    int     `json:"deceased"`
	Births      int     `json:"births"`
	Deaths      int     `json:"deaths"`
	Quarantined int     `json:"quarantined"`
	AverageAge  float64 `json:"average_age"`
	LoadPct     float64 `json:"load_pct"`
}

// SystemsSummary is the aggregate health of all facility systems.
type SystemsSummary struct {
	Total         int     `json:"total"`
	Operational   int     `json:"operational"`
	Degraded      int     `json:"degraded"`
	Offline       int     `json:"offline"`
	Failed        int     `json:"failed"`
	AvgEfficiency float64 `json:"avg_efficiency"`
	OverdueMaint  int     `json:"overdue_maintenance"`
}

// VaultState is the complete operational snapshot broadcast to clients and the
// web console. It is rebuilt by the engine after every processed interval.
type VaultState struct {
	SchemaVersion string    `json:"schema_version"`
	Generated     time.Time `json:"generated"`

	// Identity
	VaultDesignation string `json:"vault_designation"`
	VaultNumber      int    `json:"vault_number"`

	// Simulation control core
	Status       SimStatus `json:"status"`
	TimeScale    float64   `json:"time_scale"`
	VaultTime    time.Time `json:"vault_time"`
	SealDate     time.Time `json:"seal_date"`
	ElapsedDays  int       `json:"elapsed_days"`
	ElapsedYears int       `json:"elapsed_years"`
	TickCount    uint64    `json:"tick_count"`

	// Operational telemetry
	Population PopulationStatus `json:"population"`
	Resources  []ResourceStatus `json:"resources"`
	Systems    SystemsSummary   `json:"systems"`
	SystemList []SystemStatus   `json:"system_list"`
	Power      PowerStatus      `json:"power"`

	Alerts       []Alert       `json:"alerts"`
	RecentEvents []EventRecord `json:"recent_events"`
}

// ClientKind distinguishes how a connected terminal is intended to be used.
type ClientKind string

const (
	// ClientDisplay is an unattended exhibit screen (read-only, kiosk).
	ClientDisplay ClientKind = "DISPLAY"
	// ClientOperator is an attended operator terminal.
	ClientOperator ClientKind = "OPERATOR"
)

// RegisterRequest is sent by a client terminal when it joins the master.
type RegisterRequest struct {
	Protocol string     `json:"protocol"`
	Name     string     `json:"name"`
	Kind     ClientKind `json:"kind"`
	Version  string     `json:"version"`
}

// RegisterResponse is returned to a client that successfully registers.
type RegisterResponse struct {
	ClientID         string    `json:"client_id"`
	Token            string    `json:"token"`
	Protocol         string    `json:"protocol"`
	VaultDesignation string    `json:"vault_designation"`
	ServerTime       time.Time `json:"server_time"`
	HeartbeatSeconds int       `json:"heartbeat_seconds"`
}

// Telemetry is the health information a client reports back to the master on
// each heartbeat — this is the "report back to the server" channel.
type Telemetry struct {
	UptimeSeconds int64  `json:"uptime_seconds"`
	CurrentView   string `json:"current_view"`
	KioskMode     bool   `json:"kiosk_mode"`
	FramesRendered uint64 `json:"frames_rendered"`
	Errors        uint64 `json:"errors"`
	WidthCols     int    `json:"width_cols"`
	HeightRows    int    `json:"height_rows"`
	Note          string `json:"note,omitempty"`
}

// HeartbeatResponse is returned to a client heartbeat. It carries any pending
// remote operations the master wants the client to perform.
type HeartbeatResponse struct {
	Acknowledged bool      `json:"acknowledged"`
	ServerTime   time.Time `json:"server_time"`
	Commands     []Command `json:"commands"`
}

// CommandType enumerates the remote operations a master may issue to a client.
type CommandType string

const (
	CmdSwitchView CommandType = "SWITCH_VIEW" // args: view
	CmdSetKiosk   CommandType = "SET_KIOSK"   // args: enabled (true|false)
	CmdMessage    CommandType = "MESSAGE"     // args: text
	CmdIdentify   CommandType = "IDENTIFY"    // flash an identifying banner
	CmdRefresh    CommandType = "REFRESH"     // force a state refresh
	CmdReboot     CommandType = "REBOOT"      // reconnect/restart the client loop
	CmdShutdown   CommandType = "SHUTDOWN"    // gracefully terminate the client
)

// Command is a single remote operation targeted at a client terminal.
type Command struct {
	ID     string            `json:"id"`
	Type   CommandType       `json:"type"`
	Args   map[string]string `json:"args,omitempty"`
	Issued time.Time         `json:"issued"`
}

// CommandResult is reported by a client after it executes (or rejects) a command.
type CommandResult struct {
	CommandID string    `json:"command_id"`
	OK        bool      `json:"ok"`
	Message   string    `json:"message,omitempty"`
	Completed time.Time `json:"completed"`
}

// ClientInfo is the master's view of a connected terminal, surfaced to operators
// in the web console and over the API.
type ClientInfo struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Kind        ClientKind `json:"kind"`
	Address     string     `json:"address"`
	Version     string     `json:"version"`
	Connected   time.Time  `json:"connected"`
	LastSeen    time.Time  `json:"last_seen"`
	Online      bool       `json:"online"`
	Telemetry   Telemetry  `json:"telemetry"`
	PendingCmds int        `json:"pending_commands"`
}

// ControlRequest is the body for simulation control endpoints that take a value
// (e.g. setting the time scale or stepping a number of hours).
type ControlRequest struct {
	TimeScale float64 `json:"time_scale,omitempty"`
	Hours     int     `json:"hours,omitempty"`
	Code      string  `json:"code,omitempty"`
}

// APIError is the standard error envelope returned by the master API.
type APIError struct {
	Error string `json:"error"`
}
