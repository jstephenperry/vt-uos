# Module Specifications

## Overview

VT-UOS is organized into domain-focused service modules. Each module encapsulates business logic, exposes a clean API, and operates on repository abstractions.

## Module: Population Registry

**Purpose:** Manage the vault's population census, vital records, and demographic analysis.

**Capabilities:**

1. **Resident CRUD** - Create, read, update resident records
2. **Vital Records** - Birth registration, death registration, marriage/union records
3. **Lineage Tracking** - Family tree, ancestry, genetic relationship mapping
4. **Inbreeding Detection** - Calculate coefficient of inbreeding (COI) for potential pairings
5. **Demographics** - Age distribution, sex ratio, population projections
6. **Search & Filter** - Find residents by name, status, vocation, household, etc.

**Key Algorithms:**

*Coefficient of Inbreeding (COI):*

```plaintext
COI = Σ (0.5)^(n1+n2+1) × (1 + FA)

Where:
- n1 = generations from individual to common ancestor through parent 1
- n2 = generations from individual to common ancestor through parent 2  
- FA = COI of the common ancestor
```

Implement Wright's path coefficient method. Flag pairings with COI > 0.0625 (first cousin level).

*Population Projection:*

- Track birth rate, death rate, net replacement
- Project population at 5, 10, 25, 50 year intervals
- Alert if population trajectory threatens viability

**API (Service Interface):**

```go
type PopulationService interface {
    // Resident management
    CreateResident(ctx context.Context, input CreateResidentInput) (*Resident, error)
    GetResident(ctx context.Context, id string) (*Resident, error)
    UpdateResident(ctx context.Context, id string, input UpdateResidentInput) (*Resident, error)
    ListResidents(ctx context.Context, filter ResidentFilter, page Pagination) (*ResidentList, error)
    
    // Vital records
    RegisterBirth(ctx context.Context, input BirthRegistration) (*Resident, error)
    RegisterDeath(ctx context.Context, residentID string, input DeathRegistration) error
    
    // Lineage
    GetAncestry(ctx context.Context, residentID string, generations int) (*FamilyTree, error)
    GetDescendants(ctx context.Context, residentID string, generations int) (*FamilyTree, error)
    CalculateCOI(ctx context.Context, parent1ID, parent2ID string) (float64, error)
    FindCommonAncestors(ctx context.Context, resident1ID, resident2ID string) ([]Resident, error)
    
    // Demographics
    GetPopulationStats(ctx context.Context) (*PopulationStats, error)
    GetAgeDistribution(ctx context.Context) (*AgeDistribution, error)
    ProjectPopulation(ctx context.Context, years int) (*PopulationProjection, error)
}
```

---

## Module: Resource Management

**Purpose:** Track all consumable and material resources in the closed-loop vault environment.

**Capabilities:**

1. **Inventory Management** - Stock levels, locations, lot tracking
2. **Consumption Tracking** - Record all resource usage with attribution
3. **Production Tracking** - Track internally produced resources (food, water)
4. **Expiration Management** - Track perishables, alert on approaching expiration
5. **Rationing** - Calculate and enforce allocation by household/ration class
6. **Forecasting** - Project resource depletion, runway calculations

**Ration Classes:**

| Class | Calorie Target | Water (L/day) | Use Case |
| ------- | --------------- | --------------- | ---------- |
| MINIMAL | 1500 | 2.0 | Punishment, scarcity |
| STANDARD | 2000 | 3.0 | Normal residents |
| ENHANCED | 2500 | 3.5 | Pregnant, recovering |
| LABOR_INTENSIVE | 3000 | 4.0 | Heavy physical work |
| MEDICAL | Variable | Variable | Clinical determination |

**Key Calculations:**

*Daily Consumption Estimate:*

```plaintext
For each household:
  base_calories = SUM(member_calorie_needs based on age, sex, activity)
  adjusted_calories = base_calories × ration_class_modifier
  water_needs = SUM(member_water_needs)
  
Aggregate to vault daily totals
Compare against production + inventory
Calculate runway (days until depletion)
```

*Expiration Priority Queue:*

- Always consume oldest stock first (FIFO)
- Alert on items expiring within 30, 7, 1 days
- Auto-mark expired items, generate spoilage transactions

**API (Service Interface):**

```go
type ResourceService interface {
    // Inventory
    GetStock(ctx context.Context, itemID string) (*ResourceStock, error)
    ListStocks(ctx context.Context, filter StockFilter, page Pagination) (*StockList, error)
    AdjustStock(ctx context.Context, stockID string, adjustment StockAdjustment) error
    
    // Transactions
    RecordConsumption(ctx context.Context, input ConsumptionInput) error
    RecordProduction(ctx context.Context, input ProductionInput) error
    GetTransactionHistory(ctx context.Context, filter TransactionFilter, page Pagination) (*TransactionList, error)
    
    // Rationing
    CalculateHouseholdAllocation(ctx context.Context, householdID string) (*RationAllocation, error)
    GetVaultDailyRequirements(ctx context.Context) (*DailyRequirements, error)
    
    // Forecasting
    GetResourceRunway(ctx context.Context, itemID string) (*RunwayProjection, error)
    GetExpiringItems(ctx context.Context, withinDays int) ([]ResourceStock, error)
    
    // Auditing
    PerformInventoryAudit(ctx context.Context, stockID string, actualQty float64, auditorID string) error
}
```

---

## Module: Labor Allocation

**Purpose:** Manage work assignments, shift scheduling, and workforce planning.

**Capabilities:**

1. **Vocation Management** - Define positions, requirements, headcounts
2. **Assignment Management** - Assign residents to vocations
3. **Shift Scheduling** - Three 8-hour shifts (Alpha, Beta, Gamma)
4. **Skill Matching** - Match resident skills to vocation requirements
5. **Vacancy Tracking** - Identify understaffed positions
6. **Succession Planning** - Track aging workforce, training needs

**Shift Schedule:**

| Shift | Hours | Typical Roles |
| ----- | ----- | ------------- |
| ALPHA | 0600-1400 | Administration, education, most services |
| BETA | 1400-2200 | Maintenance, production |
| GAMMA | 2200-0600 | Security, essential systems monitoring |

**Assignment Rules:**

- Residents age 16-18: TRAINING assignments only
- Residents age 18-65: Full assignments
- Residents age 65+: Reduced hours, advisory roles
- Minimum rest: 8 hours between shifts
- Maximum: 1 PRIMARY + 1 SECONDARY assignment

**API (Service Interface):**

```go
type LaborService interface {
    // Vocations
    CreateVocation(ctx context.Context, input CreateVocationInput) (*Vocation, error)
    GetVocation(ctx context.Context, id string) (*Vocation, error)
    ListVocations(ctx context.Context, filter VocationFilter) ([]Vocation, error)
    GetVocationStaffing(ctx context.Context, vocationID string) (*StaffingStatus, error)
    
    // Assignments
    AssignResident(ctx context.Context, input AssignmentInput) (*WorkAssignment, error)
    EndAssignment(ctx context.Context, assignmentID string, reason string) error
    GetResidentAssignments(ctx context.Context, residentID string) ([]WorkAssignment, error)
    
    // Scheduling
    GetShiftRoster(ctx context.Context, date string, shift string) (*ShiftRoster, error)
    GetResidentSchedule(ctx context.Context, residentID string, startDate, endDate string) (*Schedule, error)
    
    // Analysis
    GetStaffingReport(ctx context.Context) (*StaffingReport, error)
    GetVacancies(ctx context.Context) ([]Vacancy, error)
    FindQualifiedResidents(ctx context.Context, vocationID string) ([]Resident, error)
}
```

---

## Module: Facility Operations

**Purpose:** Monitor and maintain vault infrastructure systems.

**Capabilities:**

1. **System Monitoring** - Track status, efficiency, telemetry
2. **Maintenance Scheduling** - Preventive maintenance calendar
3. **Work Orders** - Create, assign, track maintenance work
4. **Parts Management** - Track parts consumption from resource inventory
5. **Failure Prediction** - MTBF-based alerts
6. **Dependency Mapping** - Understand system interdependencies

**System Categories:**

| Category | Critical | Examples |
| -------- | -------- | -------- |
| POWER | Yes | Reactor, generators, power distribution |
| WATER | Yes | Purification, recycling, distribution |
| HVAC | Yes | Air filtration, temperature, humidity |
| WASTE | Yes | Sewage processing, recycling |
| SECURITY | Yes | Door controls, surveillance |
| MEDICAL | Partial | Medical equipment, pharmacy |
| FOOD_PRODUCTION | Partial | Hydroponics, food processing |
| COMMUNICATIONS | No | Intercom, terminals |

**Efficiency Impact:**

- Systems below 80% efficiency: WARNING
- Systems below 50% efficiency: CRITICAL
- POWER degradation cascades to dependent systems
- HVAC failure triggers population health events

**API (Service Interface):**

```go
type FacilityService interface {
    // Systems
    GetSystem(ctx context.Context, id string) (*FacilitySystem, error)
    ListSystems(ctx context.Context, filter SystemFilter) ([]FacilitySystem, error)
    UpdateSystemStatus(ctx context.Context, id string, status SystemStatusUpdate) error
    RecordTelemetry(ctx context.Context, systemID string, telemetry map[string]any) error
    
    // Maintenance
    CreateMaintenanceRecord(ctx context.Context, input MaintenanceInput) (*MaintenanceRecord, error)
    GetMaintenanceSchedule(ctx context.Context, startDate, endDate string) ([]ScheduledMaintenance, error)
    GetOverdueMaintenance(ctx context.Context) ([]FacilitySystem, error)
    CompleteMaintenanceRecord(ctx context.Context, id string, outcome MaintenanceOutcome) error
    
    // Analysis
    GetSystemHealth(ctx context.Context) (*SystemHealthReport, error)
    GetFailurePredictions(ctx context.Context) ([]FailurePrediction, error)
    GetDependencyGraph(ctx context.Context) (*DependencyGraph, error)
}
```

---

## Module: Simulation Engine

**Purpose:** Progress vault state over time with realistic resource consumption, population dynamics, and random events.

**Core Loop:**

```plaintext
Every simulation tick (configurable, default 1 game-hour):
  1. Advance vault clock
  2. Process resource consumption
  3. Process resource production
  4. Age residents (on day boundaries)
  5. Check for scheduled events (maintenance due, expiration)
  6. Roll for random events
  7. Process event queue
  8. Update system degradation
  9. Check alert conditions
  10. Persist state
```

**Time Scaling:**

| Scale | Real Time | Game Time | Use Case |
| ----- | --------- | --------- | -------- |
| 1:1 | 1 hour | 1 hour | Real-time demo |
| 60:1 | 1 minute | 1 hour | Accelerated play |
| 1440:1 | 1 minute | 1 day | Fast-forward |
| Paused | - | - | Data entry, review |

**Random Event Categories:**

1. **Population Events** - Illness, accidents, births, deaths, conflicts
2. **Facility Events** - Equipment failures, malfunctions, wear
3. **Resource Events** - Spoilage, contamination, discovery
4. **Security Events** - Incidents, breaches, external threats
5. **Discovery Events** - Found caches, recovered data, innovations

**Event Probability Modifiers:**

- Population density affects conflict probability
- System efficiency affects failure probability
- Resource scarcity affects theft/hoarding probability
- Time since seal affects external threat probability

**API (Service Interface):**

```go
type SimulationEngine interface {
    // Control
    Start(ctx context.Context) error
    Pause(ctx context.Context) error
    Resume(ctx context.Context) error
    SetTimeScale(scale float64) error
    GetStatus() SimulationStatus
    
    // Time
    GetVaultTime() time.Time
    AdvanceTime(duration time.Duration) error  // Manual advance when paused
    
    // Events
    GetPendingEvents() []ScheduledEvent
    QueueEvent(event Event) error
    GetEventHistory(filter EventFilter, page Pagination) (*EventList, error)
    
    // State
    GetVaultSummary() *VaultSummary
    GetAlerts() []Alert
}
```

---

## Module: Medical

**Purpose:** Track resident health, manage medical records, and monitor epidemiological trends.

**Capabilities:**

1. **Patient Records** - Medical history, encounters, treatments
2. **Condition Tracking** - Chronic conditions, contagious diseases
3. **Radiation Monitoring** - Individual and population exposure tracking
4. **Epidemiology** - Disease spread, outbreak detection

---

## Module: Security

**Purpose:** Monitor access control, incident tracking, and vault security.

**Capabilities:**

1. **Access Control** - Zone restrictions, clearance enforcement
2. **Access Logging** - Track all zone entry/exit
3. **Incident Management** - Report, investigate, resolve security events
4. **Threat Assessment** - Analyze security trends

---

## Module: Governance

**Purpose:** Manage vault directives, policies, and administrative decisions.

**Capabilities:**

1. **Directive Management** - Issue, track, supersede official directives
2. **Policy Enforcement** - Link directives to system behaviors
3. **Audit Trail** - Immutable log of all system changes
4. **Classification Control** - Manage document access levels
