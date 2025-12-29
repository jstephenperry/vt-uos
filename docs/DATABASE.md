# Database Schema Specification

## Overview

VT-UOS uses SQLite via `modernc.org/sqlite` (pure Go, no CGO) for all data persistence. The schema is designed for a closed-loop vault environment managing populations, resources, facilities, and operations over multi-generational timespans.

## Core Entities

### Resident

The central entity representing a vault dweller.

```sql
CREATE TABLE residents (
    -- Identity
    id TEXT PRIMARY KEY,                              -- UUIDv7
    registry_number TEXT UNIQUE NOT NULL,             -- "V076-00001" format
    
    -- Biographic Data
    surname TEXT NOT NULL,
    given_names TEXT NOT NULL,
    date_of_birth TEXT NOT NULL,                      -- ISO8601 date
    date_of_death TEXT,                               -- NULL if alive
    sex TEXT NOT NULL CHECK (sex IN ('M', 'F')),
    blood_type TEXT CHECK (blood_type IN ('A+', 'A-', 'B+', 'B-', 'AB+', 'AB-', 'O+', 'O-')),
    
    -- Origin & Status
    entry_type TEXT NOT NULL CHECK (entry_type IN ('ORIGINAL', 'VAULT_BORN', 'ADMITTED')),
    entry_date TEXT NOT NULL,                         -- ISO8601 datetime
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'DECEASED', 'EXILED', 'SURFACE_MISSION', 'QUARANTINE')),
    
    -- Lineage (for genetic tracking)
    biological_parent_1_id TEXT REFERENCES residents(id),
    biological_parent_2_id TEXT REFERENCES residents(id),
    
    -- Current Assignments
    household_id TEXT REFERENCES households(id),
    quarters_id TEXT REFERENCES quarters(id),
    primary_vocation_id TEXT REFERENCES vocations(id),
    clearance_level INTEGER NOT NULL DEFAULT 1 CHECK (clearance_level BETWEEN 1 AND 10),
    
    -- Metadata
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_residents_status ON residents(status);
CREATE INDEX idx_residents_household ON residents(household_id);
CREATE INDEX idx_residents_vocation ON residents(primary_vocation_id);
```

**Business Rules:**

- `registry_number` format: `V{vault_number}-{5-digit sequence}` (e.g., V076-00001)
- `date_of_birth` for VAULT_BORN residents must be after vault seal date
- `biological_parent_*` required for VAULT_BORN, NULL for ORIGINAL/ADMITTED
- `date_of_death` required when status changes to DECEASED
- Clearance level 10 reserved for Overseer

### Household

Grouping of residents sharing living quarters.

```sql
CREATE TABLE households (
    id TEXT PRIMARY KEY,
    designation TEXT UNIQUE NOT NULL,                 -- "H-0042"
    household_type TEXT NOT NULL CHECK (household_type IN ('FAMILY', 'INDIVIDUAL', 'COMMUNAL', 'TEMPORARY')),
    head_of_household_id TEXT REFERENCES residents(id),
    quarters_id TEXT REFERENCES quarters(id),
    ration_class TEXT NOT NULL DEFAULT 'STANDARD' CHECK (ration_class IN ('MINIMAL', 'STANDARD', 'ENHANCED', 'MEDICAL', 'LABOR_INTENSIVE')),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'DISSOLVED', 'MERGED')),
    formed_date TEXT NOT NULL,
    dissolved_date TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

**Business Rules:**

- Household dissolves when all members deceased or reassigned
- Head of household must be ACTIVE adult resident (age 18+)
- Ration class affects resource allocation calculations

### Quarters

Physical living spaces within the vault.

```sql
CREATE TABLE quarters (
    id TEXT PRIMARY KEY,
    unit_code TEXT UNIQUE NOT NULL,                   -- "R-A-042" (Residential-Section A-Unit 42)
    sector TEXT NOT NULL,
    level INTEGER NOT NULL,
    unit_type TEXT NOT NULL CHECK (unit_type IN ('SINGLE', 'DOUBLE', 'FAMILY', 'DORMITORY', 'EXECUTIVE')),
    capacity INTEGER NOT NULL,
    square_meters REAL NOT NULL,
    amenities TEXT,                                   -- JSON array: ["PRIVATE_BATHROOM", "KITCHENETTE"]
    status TEXT NOT NULL DEFAULT 'AVAILABLE' CHECK (status IN ('AVAILABLE', 'OCCUPIED', 'MAINTENANCE', 'CONDEMNED')),
    assigned_household_id TEXT REFERENCES households(id),
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_quarters_status ON quarters(status);
CREATE INDEX idx_quarters_sector ON quarters(sector);
```

**Capacity by Type:**

- SINGLE: 1 person
- DOUBLE: 2 persons
- FAMILY: 4-6 persons
- DORMITORY: 8-20 persons
- EXECUTIVE: 2-4 persons (Overseer, department heads)

### Vocation

Job categories and specific positions.

```sql
CREATE TABLE vocations (
    id TEXT PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,                        -- "ENG-MAINT-01"
    title TEXT NOT NULL,                              -- "Maintenance Technician"
    department TEXT NOT NULL CHECK (department IN ('ENGINEERING', 'MEDICAL', 'SECURITY', 'FOOD_PRODUCTION', 'ADMINISTRATION', 'EDUCATION', 'SANITATION', 'RESEARCH')),
    required_clearance INTEGER NOT NULL DEFAULT 1,
    required_skills TEXT,                             -- JSON array
    headcount_authorized INTEGER NOT NULL,
    headcount_minimum INTEGER NOT NULL,
    shift_pattern TEXT NOT NULL DEFAULT 'STANDARD' CHECK (shift_pattern IN ('STANDARD', 'ROTATING', 'ON_CALL', 'CONTINUOUS')),
    hazard_level TEXT NOT NULL DEFAULT 'NONE' CHECK (hazard_level IN ('NONE', 'LOW', 'MODERATE', 'HIGH', 'EXTREME')),
    description TEXT,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE work_assignments (
    id TEXT PRIMARY KEY,
    resident_id TEXT NOT NULL REFERENCES residents(id),
    vocation_id TEXT NOT NULL REFERENCES vocations(id),
    assignment_type TEXT NOT NULL CHECK (assignment_type IN ('PRIMARY', 'SECONDARY', 'TEMPORARY', 'TRAINING')),
    start_date TEXT NOT NULL,
    end_date TEXT,                                    -- NULL for ongoing
    shift TEXT CHECK (shift IN ('ALPHA', 'BETA', 'GAMMA')),  -- 8-hour shifts
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'ON_LEAVE', 'SUSPENDED', 'COMPLETED')),
    performance_rating REAL CHECK (performance_rating BETWEEN 0 AND 5),
    assigned_by TEXT REFERENCES residents(id),
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_work_assignments_resident ON work_assignments(resident_id);
CREATE INDEX idx_work_assignments_vocation ON work_assignments(vocation_id);
CREATE INDEX idx_work_assignments_status ON work_assignments(status);
```

## Resources

Inventory tracking for all consumables and materials.

```sql
CREATE TABLE resource_categories (
    id TEXT PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,                        -- "FOOD", "WATER", "MEDICAL", etc.
    name TEXT NOT NULL,
    description TEXT,
    unit_of_measure TEXT NOT NULL,                    -- "kg", "liters", "units", "doses"
    is_consumable INTEGER NOT NULL DEFAULT 1,
    is_critical INTEGER NOT NULL DEFAULT 0,           -- Triggers alerts at low levels
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE resource_items (
    id TEXT PRIMARY KEY,
    category_id TEXT NOT NULL REFERENCES resource_categories(id),
    item_code TEXT UNIQUE NOT NULL,                   -- "FOOD-PROTEIN-001"
    name TEXT NOT NULL,
    description TEXT,
    unit_of_measure TEXT NOT NULL,
    calories_per_unit REAL,                           -- For food items
    shelf_life_days INTEGER,                          -- NULL for non-perishables
    storage_requirements TEXT,                        -- JSON: {"temp_max_c": 4, "humidity_max_pct": 60}
    is_producible INTEGER NOT NULL DEFAULT 0,         -- Can vault produce this?
    production_rate_per_day REAL,                     -- If producible
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE resource_stocks (
    id TEXT PRIMARY KEY,
    item_id TEXT NOT NULL REFERENCES resource_items(id),
    lot_number TEXT,                                  -- Batch tracking
    quantity REAL NOT NULL CHECK (quantity >= 0),
    quantity_reserved REAL NOT NULL DEFAULT 0 CHECK (quantity_reserved >= 0),
    storage_location TEXT NOT NULL,                   -- "STORAGE-A-12"
    received_date TEXT NOT NULL,
    expiration_date TEXT,
    status TEXT NOT NULL DEFAULT 'AVAILABLE' CHECK (status IN ('AVAILABLE', 'RESERVED', 'QUARANTINE', 'EXPIRED', 'DEPLETED')),
    last_audit_date TEXT,
    last_audit_by TEXT REFERENCES residents(id),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_resource_stocks_item ON resource_stocks(item_id);
CREATE INDEX idx_resource_stocks_status ON resource_stocks(status);
CREATE INDEX idx_resource_stocks_expiration ON resource_stocks(expiration_date);

CREATE TABLE resource_transactions (
    id TEXT PRIMARY KEY,
    stock_id TEXT REFERENCES resource_stocks(id),     -- NULL for production events
    item_id TEXT NOT NULL REFERENCES resource_items(id),
    transaction_type TEXT NOT NULL CHECK (transaction_type IN ('CONSUMPTION', 'PRODUCTION', 'ADJUSTMENT', 'SPOILAGE', 'TRANSFER', 'AUDIT_CORRECTION')),
    quantity REAL NOT NULL,                           -- Positive for additions, negative for removals
    balance_after REAL NOT NULL,                      -- Running balance
    reason TEXT,
    authorized_by TEXT REFERENCES residents(id),
    related_entity_type TEXT,                         -- 'RESIDENT', 'HOUSEHOLD', 'FACILITY', etc.
    related_entity_id TEXT,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_resource_transactions_item ON resource_transactions(item_id);
CREATE INDEX idx_resource_transactions_timestamp ON resource_transactions(timestamp);
CREATE INDEX idx_resource_transactions_type ON resource_transactions(transaction_type);
```

## Facility Systems

Infrastructure monitoring and maintenance.

```sql
CREATE TABLE facility_systems (
    id TEXT PRIMARY KEY,
    system_code TEXT UNIQUE NOT NULL,                 -- "PWR-REACTOR-01"
    name TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('POWER', 'WATER', 'HVAC', 'SECURITY', 'MEDICAL', 'FOOD_PRODUCTION', 'WASTE', 'COMMUNICATIONS', 'STRUCTURAL')),
    location_sector TEXT NOT NULL,
    location_level INTEGER NOT NULL,
    
    -- Status
    status TEXT NOT NULL DEFAULT 'OPERATIONAL' CHECK (status IN ('OPERATIONAL', 'DEGRADED', 'MAINTENANCE', 'OFFLINE', 'FAILED', 'DESTROYED')),
    efficiency_percent REAL NOT NULL DEFAULT 100.0 CHECK (efficiency_percent BETWEEN 0 AND 100),
    
    -- Specifications
    capacity_rating REAL,                             -- Depends on system type
    capacity_unit TEXT,                               -- "kW", "liters/day", etc.
    current_output REAL,
    
    -- Maintenance
    install_date TEXT NOT NULL,
    last_maintenance_date TEXT,
    next_maintenance_due TEXT,
    maintenance_interval_days INTEGER NOT NULL DEFAULT 90,
    mtbf_hours INTEGER,                               -- Mean time between failures
    total_runtime_hours REAL NOT NULL DEFAULT 0,
    
    -- Telemetry (latest readings)
    telemetry_json TEXT,                              -- System-specific sensor data
    telemetry_updated_at TEXT,
    
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_facility_systems_category ON facility_systems(category);
CREATE INDEX idx_facility_systems_status ON facility_systems(status);

CREATE TABLE maintenance_records (
    id TEXT PRIMARY KEY,
    system_id TEXT NOT NULL REFERENCES facility_systems(id),
    maintenance_type TEXT NOT NULL CHECK (maintenance_type IN ('PREVENTIVE', 'CORRECTIVE', 'EMERGENCY', 'INSPECTION', 'UPGRADE')),
    
    -- Work details
    description TEXT NOT NULL,
    work_performed TEXT,
    parts_consumed TEXT,                              -- JSON array of {item_id, quantity}
    
    -- Personnel
    lead_technician_id TEXT REFERENCES residents(id),
    crew_member_ids TEXT,                             -- JSON array of resident IDs
    
    -- Timing
    scheduled_date TEXT,
    started_at TEXT,
    completed_at TEXT,
    estimated_hours REAL,
    actual_hours REAL,
    
    -- Outcome
    outcome TEXT CHECK (outcome IN ('COMPLETED', 'PARTIAL', 'FAILED', 'DEFERRED', 'CANCELLED')),
    system_status_before TEXT,
    system_status_after TEXT,
    efficiency_before REAL,
    efficiency_after REAL,
    
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_maintenance_records_system ON maintenance_records(system_id);
CREATE INDEX idx_maintenance_records_type ON maintenance_records(maintenance_type);
```

## Medical Records

Health tracking and epidemiology.

```sql
CREATE TABLE medical_records (
    id TEXT PRIMARY KEY,
    resident_id TEXT NOT NULL REFERENCES residents(id),
    record_type TEXT NOT NULL CHECK (record_type IN ('EXAMINATION', 'TREATMENT', 'VACCINATION', 'INCIDENT', 'PSYCHOLOGICAL', 'RADIATION', 'CHRONIC_CONDITION', 'LAB_RESULT')),
    
    -- Clinical data
    chief_complaint TEXT,
    diagnosis_codes TEXT,                             -- JSON array of ICD-like codes
    diagnosis_text TEXT,
    treatment_provided TEXT,
    medications_prescribed TEXT,                      -- JSON array
    
    -- Vital signs (if applicable)
    vitals_json TEXT,                                 -- {"bp": "120/80", "hr": 72, "temp_c": 36.8, ...}
    
    -- Radiation tracking
    radiation_dose_msv REAL,                          -- Millisieverts this exposure
    radiation_cumulative_msv REAL,                    -- Running lifetime total
    
    -- Provider
    provider_id TEXT REFERENCES residents(id),
    facility_location TEXT,
    
    -- Timing
    encounter_date TEXT NOT NULL,
    follow_up_date TEXT,
    
    -- Status
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'RESOLVED', 'CHRONIC', 'FOLLOW_UP_REQUIRED')),
    confidentiality_level INTEGER NOT NULL DEFAULT 1, -- Higher = more restricted
    
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_medical_records_resident ON medical_records(resident_id);
CREATE INDEX idx_medical_records_type ON medical_records(record_type);
CREATE INDEX idx_medical_records_date ON medical_records(encounter_date);

CREATE TABLE medical_conditions (
    id TEXT PRIMARY KEY,
    resident_id TEXT NOT NULL REFERENCES residents(id),
    condition_code TEXT NOT NULL,
    condition_name TEXT NOT NULL,
    onset_date TEXT NOT NULL,
    resolution_date TEXT,                             -- NULL if ongoing
    severity TEXT NOT NULL CHECK (severity IN ('MILD', 'MODERATE', 'SEVERE', 'CRITICAL')),
    is_chronic INTEGER NOT NULL DEFAULT 0,
    is_genetic INTEGER NOT NULL DEFAULT 0,            -- Hereditary condition
    is_contagious INTEGER NOT NULL DEFAULT 0,
    treatment_plan TEXT,
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_medical_conditions_resident ON medical_conditions(resident_id);
CREATE INDEX idx_medical_conditions_chronic ON medical_conditions(is_chronic);
CREATE INDEX idx_medical_conditions_contagious ON medical_conditions(is_contagious);
```

## Security & Access Control

```sql
CREATE TABLE security_zones (
    id TEXT PRIMARY KEY,
    zone_code TEXT UNIQUE NOT NULL,                   -- "ZONE-ALPHA-1"
    name TEXT NOT NULL,
    description TEXT,
    sector TEXT NOT NULL,
    required_clearance INTEGER NOT NULL DEFAULT 1,
    is_restricted INTEGER NOT NULL DEFAULT 0,
    access_schedule TEXT,                             -- JSON: when zone is accessible
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE access_log (
    id TEXT PRIMARY KEY,
    resident_id TEXT NOT NULL REFERENCES residents(id),
    zone_id TEXT NOT NULL REFERENCES security_zones(id),
    access_point TEXT NOT NULL,                       -- Door/checkpoint ID
    direction TEXT NOT NULL CHECK (direction IN ('ENTRY', 'EXIT')),
    access_result TEXT NOT NULL CHECK (access_result IN ('GRANTED', 'DENIED', 'OVERRIDE', 'EMERGENCY')),
    denial_reason TEXT,
    override_by TEXT REFERENCES residents(id),        -- Who authorized override
    timestamp TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_access_log_resident ON access_log(resident_id);
CREATE INDEX idx_access_log_zone ON access_log(zone_id);
CREATE INDEX idx_access_log_timestamp ON access_log(timestamp);
CREATE INDEX idx_access_log_result ON access_log(access_result);

CREATE TABLE security_incidents (
    id TEXT PRIMARY KEY,
    incident_number TEXT UNIQUE NOT NULL,             -- "SI-2077-0042"
    incident_type TEXT NOT NULL CHECK (incident_type IN ('ALTERCATION', 'THEFT', 'VANDALISM', 'UNAUTHORIZED_ACCESS', 'CONTRABAND', 'INSUBORDINATION', 'ASSAULT', 'OTHER')),
    severity TEXT NOT NULL CHECK (severity IN ('MINOR', 'MODERATE', 'MAJOR', 'CRITICAL')),
    
    -- Details
    description TEXT NOT NULL,
    location_sector TEXT,
    location_detail TEXT,
    
    -- Parties involved
    reported_by TEXT REFERENCES residents(id),
    involved_resident_ids TEXT,                       -- JSON array
    witness_resident_ids TEXT,                        -- JSON array
    responding_officer_ids TEXT,                      -- JSON array
    
    -- Resolution
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'INVESTIGATING', 'PENDING_REVIEW', 'RESOLVED', 'CLOSED')),
    resolution TEXT,
    disciplinary_action TEXT,
    
    -- Timing
    occurred_at TEXT NOT NULL,
    reported_at TEXT NOT NULL,
    resolved_at TEXT,
    
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_security_incidents_type ON security_incidents(incident_type);
CREATE INDEX idx_security_incidents_status ON security_incidents(status);
CREATE INDEX idx_security_incidents_occurred ON security_incidents(occurred_at);
```

## Governance & Directives

```sql
CREATE TABLE directives (
    id TEXT PRIMARY KEY,
    directive_number TEXT UNIQUE NOT NULL,            -- "OD-2077-001" (Overseer Directive)
    directive_type TEXT NOT NULL CHECK (directive_type IN ('POLICY', 'EMERGENCY', 'OPERATIONAL', 'PERSONNEL', 'RESOURCE', 'SECURITY')),
    title TEXT NOT NULL,
    
    -- Content
    summary TEXT NOT NULL,
    full_text TEXT NOT NULL,
    
    -- Authority
    issued_by TEXT NOT NULL REFERENCES residents(id),
    authority_level TEXT NOT NULL CHECK (authority_level IN ('OVERSEER', 'DEPARTMENT_HEAD', 'VAULT_TEC_CENTRAL')),
    
    -- Scope
    affected_departments TEXT,                        -- JSON array, NULL = all
    affected_clearance_levels TEXT,                   -- JSON array, NULL = all
    
    -- Lifecycle
    status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'ACTIVE', 'SUSPENDED', 'SUPERSEDED', 'RESCINDED')),
    effective_date TEXT,
    expiration_date TEXT,
    supersedes_directive_id TEXT REFERENCES directives(id),
    superseded_by_directive_id TEXT REFERENCES directives(id),
    
    -- Classification
    classification_level TEXT NOT NULL DEFAULT 'GENERAL' CHECK (classification_level IN ('GENERAL', 'RESTRICTED', 'CONFIDENTIAL', 'OVERSEER_ONLY')),
    
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_directives_type ON directives(directive_type);
CREATE INDEX idx_directives_status ON directives(status);
CREATE INDEX idx_directives_classification ON directives(classification_level);
```

## Audit Log (Immutable)

```sql
CREATE TABLE audit_log (
    id TEXT PRIMARY KEY,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    
    -- Actor
    actor_type TEXT NOT NULL CHECK (actor_type IN ('USER', 'SYSTEM', 'SIMULATION')),
    actor_id TEXT,                                    -- resident_id if USER
    
    -- Action
    action TEXT NOT NULL,                             -- 'CREATE', 'UPDATE', 'DELETE', 'LOGIN', 'LOGOUT', etc.
    entity_type TEXT NOT NULL,                        -- Table name
    entity_id TEXT NOT NULL,
    
    -- Change details
    old_values TEXT,                                  -- JSON of changed fields (before)
    new_values TEXT,                                  -- JSON of changed fields (after)
    
    -- Context
    session_id TEXT,
    ip_address TEXT,
    terminal_id TEXT
);

CREATE INDEX idx_audit_log_timestamp ON audit_log(timestamp);
CREATE INDEX idx_audit_log_actor ON audit_log(actor_id);
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);
```
