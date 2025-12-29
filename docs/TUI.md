# TUI Design Specification

## Visual Design Principles

1. **Monochrome green phosphor aesthetic** - Primary: #00FF00, dimmed: #00AA00, highlight: #66FF66
2. **Box drawing characters** for structure
3. **Minimal animation** - Cursor blink, occasional scan lines
4. **Dense information display** - Maximize data per screen
5. **Keyboard-only navigation** - No mouse required

## Screen Structure

```plaintext
┌──────────────────────────────────────────────────────────────────────────────┐
│ VAULT-TEC UNIFIED OPERATING SYSTEM v1.0.0          VAULT 076 | POP: 487      │
│ ════════════════════════════════════════════════════════════════════════════ │
│ 2077-11-15 14:32:07 | ALERT: Water Purification efficiency at 78%            │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│                          [ MAIN CONTENT AREA ]                               │
│                                                                              │
│                                                                              │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│ [F1]Help [F2]Dashboard [F3]Population [F4]Resources [F5]Facilities [F10]Quit │
└──────────────────────────────────────────────────────────────────────────────┘
```

## Navigation Structure

```plaintext
Main Menu
├── Dashboard (F2)
│   ├── Population Summary
│   ├── Resource Status
│   ├── System Status
│   └── Active Alerts
├── Population (F3)
│   ├── Census
│   │   ├── Browse All
│   │   ├── Search
│   │   └── Add Resident
│   ├── Households
│   ├── Vital Records
│   │   ├── Register Birth
│   │   ├── Register Death
│   │   └── View Records
│   ├── Demographics
│   └── Lineage Browser
├── Resources (F4)
│   ├── Inventory
│   │   ├── By Category
│   │   └── Search
│   ├── Transactions
│   ├── Rationing
│   │   ├── Allocation Table
│   │   └── Adjust Ration Class
│   ├── Expiring Items
│   └── Forecasting
├── Facilities (F5)
│   ├── System Status
│   │   ├── All Systems
│   │   ├── By Category
│   │   └── Critical Only
│   ├── Maintenance
│   │   ├── Schedule
│   │   ├── Overdue
│   │   └── Create Work Order
│   └── Telemetry
├── Labor (F6)
│   ├── Assignments
│   ├── Shift Roster
│   ├── Vacancies
│   └── Vocations
├── Medical (F7)
│   ├── Patient Records
│   ├── Active Conditions
│   ├── Radiation Tracking
│   └── Epidemiology
├── Security (F8)
│   ├── Access Log
│   ├── Incidents
│   │   ├── Open
│   │   ├── All
│   │   └── Report Incident
│   └── Zone Management
├── Governance (F9)
│   ├── Directives
│   │   ├── Active
│   │   ├── All
│   │   └── Issue Directive
│   └── Audit Log
└── Settings
    ├── Vault Configuration
    ├── Simulation Controls
    ├── User Preferences
    └── About
```

## Key Bindings

### Global

| Key | Action |
| --- | ------ |
| F1 | Context help |
| F2-F9 | Module shortcuts |
| F10 | Quit (with confirmation) |
| Tab | Next field/element |
| Shift+Tab | Previous field/element |
| Enter | Select/confirm |
| Escape | Back/cancel |
| / | Search (in lists) |
| ? | Help |
| Ctrl+C | Force quit |

### Navigation

| Key | Action |
| --- | ------ |
| ↑/k | Move up |
| ↓/j | Move down |
| ←/h | Move left/back |
| →/l | Move right/forward |
| PgUp | Page up |
| PgDn | Page down |
| Home | First item |
| End | Last item |

### List Views

| Key | Action |
| --- | ------ |
| / | Search/filter |
| n | Next search result |
| N | Previous search result |
| s | Sort options |
| r | Refresh |

## Component Library

Build these reusable Bubble Tea components:

### 1. Header

Displays vault name, population, current time, and rotating alerts.

```go
type Header struct {
    VaultName      string
    VaultNumber    int
    Population     int
    CurrentTime    time.Time
    Alerts         []string
    alertIndex     int
}
```

### 2. Menu

Vertical selectable list for navigation.

```go
type Menu struct {
    Title       string
    Items       []MenuItem
    Selected    int
    Width       int
}

type MenuItem struct {
    Label       string
    Key         string
    Action      func()
}
```

### 3. Table

Sortable, paginated data table.

```go
type Table struct {
    Columns     []Column
    Rows        []Row
    Selected    int
    SortColumn  int
    SortDir     SortDirection
    Page        int
    PageSize    int
    TotalRows   int
}

type Column struct {
    Header      string
    Width       int
    Align       Alignment
    Sortable    bool
}
```

### 4. Form

Input fields with validation.

```go
type Form struct {
    Title       string
    Fields      []Field
    Current     int
    Errors      map[string]string
}

type Field struct {
    Name        string
    Label       string
    Type        FieldType  // Text, Number, Date, Select
    Value       string
    Required    bool
    Validator   func(string) error
    Options     []string   // For select fields
}
```

### 5. Modal

Overlay dialogs for confirmations.

```go
type Modal struct {
    Title       string
    Message     string
    Type        ModalType  // Confirm, Alert, Info
    Buttons     []Button
    Selected    int
}

type Button struct {
    Label       string
    Primary     bool
    Action      func()
}
```

### 6. StatusBar

Bottom bar showing available actions.

```go
type StatusBar struct {
    Left        string
    Right       string
    Shortcuts   []Shortcut
}

type Shortcut struct {
    Key         string
    Label       string
}
```

### 7. AlertBanner

Rotating alert display.

```go
type AlertBanner struct {
    Alerts      []Alert
    Current     int
    Ticker      time.Ticker
}

type Alert struct {
    Level       AlertLevel  // Info, Warning, Critical
    Message     string
    Timestamp   time.Time
}
```

### 8. Pagination

Page controls for large datasets.

```go
type Pagination struct {
    Current     int
    Total       int
    PageSize    int
}
```

### 9. Tabs

Horizontal tab navigation.

```go
type Tabs struct {
    Tabs        []Tab
    Active      int
}

type Tab struct {
    Label       string
    Content     tea.Model
}
```

### 10. Tree

Hierarchical data (family trees, org charts).

```go
type Tree struct {
    Root        *TreeNode
    Selected    *TreeNode
    Expanded    map[string]bool
}

type TreeNode struct {
    ID          string
    Label       string
    Children    []*TreeNode
    Parent      *TreeNode
    Data        interface{}
}
```

## Example Screens

### Dashboard

```plaintext
┌──────────────────────────────────────────────────────────────────────────────┐
│ VAULT-TEC UNIFIED OPERATING SYSTEM v1.0.0          VAULT 076 | POP: 487      │
│ ════════════════════════════════════════════════════════════════════════════ │
│ 2077-11-15 14:32:07 | ALERT: Water Purification efficiency at 78%            │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│ ╔══════════════════════════════════════════════════════════════════════════╗ │
│ ║ POPULATION STATUS                                                        ║ │
│ ╠══════════════════════════════════════════════════════════════════════════╣ │
│ ║ Total Population: 487                     Active Residents: 465          ║ │
│ ║ Births (30d): 3                           Deaths (30d): 2                ║ │
│ ║ Average Age: 34.2 years                   Sex Ratio: 1.02 M:F            ║ │
│ ╚══════════════════════════════════════════════════════════════════════════╝ │
│                                                                              │
│ ╔══════════════════════════════════════════════════════════════════════════╗ │
│ ║ CRITICAL RESOURCES                                                       ║ │
│ ╠══════════════════════════════════════════════════════════════════════════╣ │
│ ║ Water: [████████████████████░░] 82% | 127 days runway                   ║ │
│ ║ Food:  [███████████████████░░░] 76% | 94 days runway                    ║ │
│ ║ Power: [██████████████████████] 94% | Stable                            ║ │
│ ╚══════════════════════════════════════════════════════════════════════════╝ │
│                                                                              │
│ ╔══════════════════════════════════════════════════════════════════════════╗ │
│ ║ SYSTEM STATUS                                                            ║ │
│ ╠══════════════════════════════════════════════════════════════════════════╣ │
│ ║ PWR-REACTOR-01     [OPERATIONAL] 98%                                     ║ │
│ ║ WATER-PURIF-01     [DEGRADED]    78%  ⚠ Maintenance overdue             ║ │
│ ║ HVAC-CENTRAL-01    [OPERATIONAL] 92%                                     ║ │
│ ╚══════════════════════════════════════════════════════════════════════════╝ │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│ [F1]Help [F2]Dashboard [F3]Population [F4]Resources [F5]Facilities [F10]Quit │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Population Census List

```plaintext
┌──────────────────────────────────────────────────────────────────────────────┐
│ VAULT-TEC UNIFIED OPERATING SYSTEM v1.0.0          VAULT 076 | POP: 487      │
│ ════════════════════════════════════════════════════════════════════════════ │
│ 2077-11-15 14:32:07 | All systems nominal                                    │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│ POPULATION > CENSUS                                      [/]Search [r]Refresh │
│                                                                              │
│ ┌────────────┬───────────────────────┬─────┬──────────────┬─────────────┐   │
│ │ Registry # │ Name                  │ Age │ Vocation     │ Status      │   │
│ ├────────────┼───────────────────────┼─────┼──────────────┼─────────────┤   │
│ │ V076-00001 │ ANDERSON, James       │  45 │ Overseer     │ ACTIVE      │   │
│ │ V076-00002 │ BROOKS, Sarah         │  42 │ Medical      │ ACTIVE      │   │
│ │>V076-00003 │ CHEN, Michael         │  38 │ Engineering  │ ACTIVE      │   │
│ │ V076-00004 │ DAVIS, Emily          │  29 │ Security     │ ACTIVE      │   │
│ │ V076-00005 │ EVANS, Robert         │  51 │ Maintenance  │ ACTIVE      │   │
│ │ V076-00006 │ FISHER, Maria         │  33 │ Food Prod    │ ACTIVE      │   │
│ │ V076-00007 │ GARCIA, Thomas        │  27 │ Security     │ ACTIVE      │   │
│ │ V076-00008 │ HAYES, Jennifer       │  36 │ Education    │ ACTIVE      │   │
│ │ V076-00009 │ JACKSON, William      │  44 │ Engineering  │ ACTIVE      │   │
│ │ V076-00010 │ KELLY, Patricia       │  31 │ Medical      │ ACTIVE      │   │
│ └────────────┴───────────────────────┴─────┴──────────────┴─────────────┘   │
│                                                                              │
│ Page 1 of 49                          [Enter]View [a]Add [d]Delete [s]Sort   │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│ [F1]Help [F2]Dashboard [F3]Population [F4]Resources [F5]Facilities [F10]Quit │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Resident Detail View

```plaintext
┌──────────────────────────────────────────────────────────────────────────────┐
│ VAULT-TEC UNIFIED OPERATING SYSTEM v1.0.0          VAULT 076 | POP: 487      │
│ ════════════════════════════════════════════════════════════════════════════ │
│ 2077-11-15 14:32:07 | All systems nominal                                    │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│ POPULATION > CENSUS > V076-00003                               [e]Edit [ESC]Back │
│                                                                              │
│ ╔══════════════════════════════════════════════════════════════════════════╗ │
│ ║ CHEN, Michael                                      Registry: V076-00003  ║ │
│ ╠══════════════════════════════════════════════════════════════════════════╣ │
│ ║ Date of Birth: 2039-03-15 (38 years)              Sex: Male              ║ │
│ ║ Blood Type: O+                                     Status: ACTIVE         ║ │
│ ║ Entry Type: ORIGINAL                               Entry: 2077-10-23     ║ │
│ ║ Clearance: Level 5                                                       ║ │
│ ╚══════════════════════════════════════════════════════════════════════════╝ │
│                                                                              │
│ ╔══════════════════════════════════════════════════════════════════════════╗ │
│ ║ ASSIGNMENTS                                                              ║ │
│ ╠══════════════════════════════════════════════════════════════════════════╣ │
│ ║ Primary: Engineering Technician (ENG-TECH-02)     Shift: BETA            ║ │
│ ║ Since: 2077-10-23                                                        ║ │
│ ║ Performance: 4.2/5.0                                                     ║ │
│ ╚══════════════════════════════════════════════════════════════════════════╝ │
│                                                                              │
│ ╔══════════════════════════════════════════════════════════════════════════╗ │
│ ║ HOUSEHOLD                                                                ║ │
│ ╠══════════════════════════════════════════════════════════════════════════╣ │
│ ║ Household: H-0127 (FAMILY)                        Head of Household: Self║ │
│ ║ Quarters: R-B-042 (FAMILY unit)                                          ║ │
│ ║ Members: 3 (Self, Spouse, 1 Child)                                       ║ │
│ ║ Ration Class: STANDARD                                                   ║ │
│ ╚══════════════════════════════════════════════════════════════════════════╝ │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│ [F1]Help [F2]Dashboard [F3]Population [F4]Resources [F5]Facilities [F10]Quit │
└──────────────────────────────────────────────────────────────────────────────┘
```

## Style Guide

### Colors (Green Phosphor Theme)

```go
const (
    ColorPrimary    = "#00FF00"  // Bright green
    ColorDim        = "#00AA00"  // Dimmed green
    ColorHighlight  = "#66FF66"  // Lighter green
    ColorBackground = "#000000"  // Black
    ColorBorder     = "#008800"  // Mid green
)
```

### Typography

- Primary font: Monospace (system default)
- All caps for headers and labels
- Mixed case for data values
- Right-align numbers
- Left-align text

### Box Drawing

Use Unicode box drawing characters:
- `┌─┐│└┘├┤┬┴┼` for light boxes
- `╔═╗║╚╝╠╣╦╩╬` for heavy boxes (emphasis)
- `▀▄█░▒▓` for progress bars
