-- +migrate Up
-- VT-UOS Initial Schema
-- Vault-Tec Unified Operating System Database

-- ============================================================================
-- VAULT METADATA
-- ============================================================================

CREATE TABLE vault_metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO vault_metadata (key, value) VALUES ('schema_version', '1');
INSERT INTO vault_metadata (key, value) VALUES ('vault_time', '2077-10-23T09:47:00Z');

-- ============================================================================
-- QUARTERS (Physical living spaces)
-- ============================================================================

CREATE TABLE quarters (
    id TEXT PRIMARY KEY,
    unit_code TEXT UNIQUE NOT NULL,
    sector TEXT NOT NULL,
    level INTEGER NOT NULL,
    unit_type TEXT NOT NULL CHECK (unit_type IN ('SINGLE', 'DOUBLE', 'FAMILY', 'DORMITORY', 'EXECUTIVE')),
    capacity INTEGER NOT NULL,
    square_meters REAL NOT NULL,
    amenities TEXT,
    status TEXT NOT NULL DEFAULT 'AVAILABLE' CHECK (status IN ('AVAILABLE', 'OCCUPIED', 'MAINTENANCE', 'CONDEMNED')),
    assigned_household_id TEXT,
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_quarters_status ON quarters(status);
CREATE INDEX idx_quarters_sector ON quarters(sector);
CREATE INDEX idx_quarters_unit_type ON quarters(unit_type);

-- ============================================================================
-- VOCATIONS (Job categories and positions)
-- ============================================================================

CREATE TABLE vocations (
    id TEXT PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    department TEXT NOT NULL CHECK (department IN ('ENGINEERING', 'MEDICAL', 'SECURITY', 'FOOD_PRODUCTION', 'ADMINISTRATION', 'EDUCATION', 'SANITATION', 'RESEARCH')),
    required_clearance INTEGER NOT NULL DEFAULT 1,
    required_skills TEXT,
    headcount_authorized INTEGER NOT NULL,
    headcount_minimum INTEGER NOT NULL,
    shift_pattern TEXT NOT NULL DEFAULT 'STANDARD' CHECK (shift_pattern IN ('STANDARD', 'ROTATING', 'ON_CALL', 'CONTINUOUS')),
    hazard_level TEXT NOT NULL DEFAULT 'NONE' CHECK (hazard_level IN ('NONE', 'LOW', 'MODERATE', 'HIGH', 'EXTREME')),
    description TEXT,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_vocations_department ON vocations(department);
CREATE INDEX idx_vocations_active ON vocations(is_active);

-- ============================================================================
-- HOUSEHOLDS (Groupings of residents)
-- ============================================================================

CREATE TABLE households (
    id TEXT PRIMARY KEY,
    designation TEXT UNIQUE NOT NULL,
    household_type TEXT NOT NULL CHECK (household_type IN ('FAMILY', 'INDIVIDUAL', 'COMMUNAL', 'TEMPORARY')),
    head_of_household_id TEXT,
    quarters_id TEXT REFERENCES quarters(id),
    ration_class TEXT NOT NULL DEFAULT 'STANDARD' CHECK (ration_class IN ('MINIMAL', 'STANDARD', 'ENHANCED', 'MEDICAL', 'LABOR_INTENSIVE')),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'DISSOLVED', 'MERGED')),
    formed_date TEXT NOT NULL,
    dissolved_date TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_households_status ON households(status);
CREATE INDEX idx_households_quarters ON households(quarters_id);

-- ============================================================================
-- RESIDENTS (Vault dwellers)
-- ============================================================================

CREATE TABLE residents (
    id TEXT PRIMARY KEY,
    registry_number TEXT UNIQUE NOT NULL,
    surname TEXT NOT NULL,
    given_names TEXT NOT NULL,
    date_of_birth TEXT NOT NULL,
    date_of_death TEXT,
    sex TEXT NOT NULL CHECK (sex IN ('M', 'F')),
    blood_type TEXT CHECK (blood_type IN ('A+', 'A-', 'B+', 'B-', 'AB+', 'AB-', 'O+', 'O-')),
    entry_type TEXT NOT NULL CHECK (entry_type IN ('ORIGINAL', 'VAULT_BORN', 'ADMITTED')),
    entry_date TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'DECEASED', 'EXILED', 'SURFACE_MISSION', 'QUARANTINE')),
    biological_parent_1_id TEXT REFERENCES residents(id),
    biological_parent_2_id TEXT REFERENCES residents(id),
    household_id TEXT REFERENCES households(id),
    quarters_id TEXT REFERENCES quarters(id),
    primary_vocation_id TEXT REFERENCES vocations(id),
    clearance_level INTEGER NOT NULL DEFAULT 1 CHECK (clearance_level BETWEEN 1 AND 10),
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_residents_status ON residents(status);
CREATE INDEX idx_residents_household ON residents(household_id);
CREATE INDEX idx_residents_vocation ON residents(primary_vocation_id);
CREATE INDEX idx_residents_surname ON residents(surname);
CREATE INDEX idx_residents_registry ON residents(registry_number);

-- Add foreign key for head of household now that residents table exists
-- SQLite doesn't support ALTER TABLE ADD CONSTRAINT, so this is handled at application level

-- ============================================================================
-- WORK ASSIGNMENTS
-- ============================================================================

CREATE TABLE work_assignments (
    id TEXT PRIMARY KEY,
    resident_id TEXT NOT NULL REFERENCES residents(id),
    vocation_id TEXT NOT NULL REFERENCES vocations(id),
    assignment_type TEXT NOT NULL CHECK (assignment_type IN ('PRIMARY', 'SECONDARY', 'TEMPORARY', 'TRAINING')),
    start_date TEXT NOT NULL,
    end_date TEXT,
    shift TEXT CHECK (shift IN ('ALPHA', 'BETA', 'GAMMA')),
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

-- ============================================================================
-- RESOURCES
-- ============================================================================

CREATE TABLE resource_categories (
    id TEXT PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    unit_of_measure TEXT NOT NULL,
    is_consumable INTEGER NOT NULL DEFAULT 1,
    is_critical INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE resource_items (
    id TEXT PRIMARY KEY,
    category_id TEXT NOT NULL REFERENCES resource_categories(id),
    item_code TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    unit_of_measure TEXT NOT NULL,
    calories_per_unit REAL,
    shelf_life_days INTEGER,
    storage_requirements TEXT,
    is_producible INTEGER NOT NULL DEFAULT 0,
    production_rate_per_day REAL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_resource_items_category ON resource_items(category_id);

CREATE TABLE resource_stocks (
    id TEXT PRIMARY KEY,
    item_id TEXT NOT NULL REFERENCES resource_items(id),
    lot_number TEXT,
    quantity REAL NOT NULL CHECK (quantity >= 0),
    quantity_reserved REAL NOT NULL DEFAULT 0 CHECK (quantity_reserved >= 0),
    storage_location TEXT NOT NULL,
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
CREATE INDEX idx_resource_stocks_location ON resource_stocks(storage_location);

CREATE TABLE resource_transactions (
    id TEXT PRIMARY KEY,
    stock_id TEXT REFERENCES resource_stocks(id),
    item_id TEXT NOT NULL REFERENCES resource_items(id),
    transaction_type TEXT NOT NULL CHECK (transaction_type IN ('CONSUMPTION', 'PRODUCTION', 'ADJUSTMENT', 'SPOILAGE', 'TRANSFER', 'AUDIT_CORRECTION')),
    quantity REAL NOT NULL,
    balance_after REAL NOT NULL,
    reason TEXT,
    authorized_by TEXT REFERENCES residents(id),
    related_entity_type TEXT,
    related_entity_id TEXT,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_resource_transactions_item ON resource_transactions(item_id);
CREATE INDEX idx_resource_transactions_timestamp ON resource_transactions(timestamp);
CREATE INDEX idx_resource_transactions_type ON resource_transactions(transaction_type);

-- ============================================================================
-- FACILITY SYSTEMS
-- ============================================================================

CREATE TABLE facility_systems (
    id TEXT PRIMARY KEY,
    system_code TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('POWER', 'WATER', 'HVAC', 'SECURITY', 'MEDICAL', 'FOOD_PRODUCTION', 'WASTE', 'COMMUNICATIONS', 'STRUCTURAL')),
    location_sector TEXT NOT NULL,
    location_level INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPERATIONAL' CHECK (status IN ('OPERATIONAL', 'DEGRADED', 'MAINTENANCE', 'OFFLINE', 'FAILED', 'DESTROYED')),
    efficiency_percent REAL NOT NULL DEFAULT 100.0 CHECK (efficiency_percent BETWEEN 0 AND 100),
    capacity_rating REAL,
    capacity_unit TEXT,
    current_output REAL,
    install_date TEXT NOT NULL,
    last_maintenance_date TEXT,
    next_maintenance_due TEXT,
    maintenance_interval_days INTEGER NOT NULL DEFAULT 90,
    mtbf_hours INTEGER,
    total_runtime_hours REAL NOT NULL DEFAULT 0,
    telemetry_json TEXT,
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
    description TEXT NOT NULL,
    work_performed TEXT,
    parts_consumed TEXT,
    lead_technician_id TEXT REFERENCES residents(id),
    crew_member_ids TEXT,
    scheduled_date TEXT,
    started_at TEXT,
    completed_at TEXT,
    estimated_hours REAL,
    actual_hours REAL,
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

-- ============================================================================
-- MEDICAL RECORDS
-- ============================================================================

CREATE TABLE medical_records (
    id TEXT PRIMARY KEY,
    resident_id TEXT NOT NULL REFERENCES residents(id),
    record_type TEXT NOT NULL CHECK (record_type IN ('EXAMINATION', 'TREATMENT', 'VACCINATION', 'INCIDENT', 'PSYCHOLOGICAL', 'RADIATION', 'CHRONIC_CONDITION', 'LAB_RESULT')),
    chief_complaint TEXT,
    diagnosis_codes TEXT,
    diagnosis_text TEXT,
    treatment_provided TEXT,
    medications_prescribed TEXT,
    vitals_json TEXT,
    radiation_dose_msv REAL,
    radiation_cumulative_msv REAL,
    provider_id TEXT REFERENCES residents(id),
    facility_location TEXT,
    encounter_date TEXT NOT NULL,
    follow_up_date TEXT,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'RESOLVED', 'CHRONIC', 'FOLLOW_UP_REQUIRED')),
    confidentiality_level INTEGER NOT NULL DEFAULT 1,
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
    resolution_date TEXT,
    severity TEXT NOT NULL CHECK (severity IN ('MILD', 'MODERATE', 'SEVERE', 'CRITICAL')),
    is_chronic INTEGER NOT NULL DEFAULT 0,
    is_genetic INTEGER NOT NULL DEFAULT 0,
    is_contagious INTEGER NOT NULL DEFAULT 0,
    treatment_plan TEXT,
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_medical_conditions_resident ON medical_conditions(resident_id);
CREATE INDEX idx_medical_conditions_chronic ON medical_conditions(is_chronic);
CREATE INDEX idx_medical_conditions_contagious ON medical_conditions(is_contagious);

-- ============================================================================
-- SECURITY
-- ============================================================================

CREATE TABLE security_zones (
    id TEXT PRIMARY KEY,
    zone_code TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    sector TEXT NOT NULL,
    required_clearance INTEGER NOT NULL DEFAULT 1,
    is_restricted INTEGER NOT NULL DEFAULT 0,
    access_schedule TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE access_log (
    id TEXT PRIMARY KEY,
    resident_id TEXT NOT NULL REFERENCES residents(id),
    zone_id TEXT NOT NULL REFERENCES security_zones(id),
    access_point TEXT NOT NULL,
    direction TEXT NOT NULL CHECK (direction IN ('ENTRY', 'EXIT')),
    access_result TEXT NOT NULL CHECK (access_result IN ('GRANTED', 'DENIED', 'OVERRIDE', 'EMERGENCY')),
    denial_reason TEXT,
    override_by TEXT REFERENCES residents(id),
    timestamp TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_access_log_resident ON access_log(resident_id);
CREATE INDEX idx_access_log_zone ON access_log(zone_id);
CREATE INDEX idx_access_log_timestamp ON access_log(timestamp);
CREATE INDEX idx_access_log_result ON access_log(access_result);

CREATE TABLE security_incidents (
    id TEXT PRIMARY KEY,
    incident_number TEXT UNIQUE NOT NULL,
    incident_type TEXT NOT NULL CHECK (incident_type IN ('ALTERCATION', 'THEFT', 'VANDALISM', 'UNAUTHORIZED_ACCESS', 'CONTRABAND', 'INSUBORDINATION', 'ASSAULT', 'OTHER')),
    severity TEXT NOT NULL CHECK (severity IN ('MINOR', 'MODERATE', 'MAJOR', 'CRITICAL')),
    description TEXT NOT NULL,
    location_sector TEXT,
    location_detail TEXT,
    reported_by TEXT REFERENCES residents(id),
    involved_resident_ids TEXT,
    witness_resident_ids TEXT,
    responding_officer_ids TEXT,
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'INVESTIGATING', 'PENDING_REVIEW', 'RESOLVED', 'CLOSED')),
    resolution TEXT,
    disciplinary_action TEXT,
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

-- ============================================================================
-- GOVERNANCE
-- ============================================================================

CREATE TABLE directives (
    id TEXT PRIMARY KEY,
    directive_number TEXT UNIQUE NOT NULL,
    directive_type TEXT NOT NULL CHECK (directive_type IN ('POLICY', 'EMERGENCY', 'OPERATIONAL', 'PERSONNEL', 'RESOURCE', 'SECURITY')),
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    full_text TEXT NOT NULL,
    issued_by TEXT NOT NULL REFERENCES residents(id),
    authority_level TEXT NOT NULL CHECK (authority_level IN ('OVERSEER', 'DEPARTMENT_HEAD', 'VAULT_TEC_CENTRAL')),
    affected_departments TEXT,
    affected_clearance_levels TEXT,
    status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'ACTIVE', 'SUSPENDED', 'SUPERSEDED', 'RESCINDED')),
    effective_date TEXT,
    expiration_date TEXT,
    supersedes_directive_id TEXT REFERENCES directives(id),
    superseded_by_directive_id TEXT REFERENCES directives(id),
    classification_level TEXT NOT NULL DEFAULT 'GENERAL' CHECK (classification_level IN ('GENERAL', 'RESTRICTED', 'CONFIDENTIAL', 'OVERSEER_ONLY')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_directives_type ON directives(directive_type);
CREATE INDEX idx_directives_status ON directives(status);
CREATE INDEX idx_directives_classification ON directives(classification_level);

-- ============================================================================
-- AUDIT LOG (Immutable)
-- ============================================================================

CREATE TABLE audit_log (
    id TEXT PRIMARY KEY,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    actor_type TEXT NOT NULL CHECK (actor_type IN ('USER', 'SYSTEM', 'SIMULATION')),
    actor_id TEXT,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    old_values TEXT,
    new_values TEXT,
    session_id TEXT,
    ip_address TEXT,
    terminal_id TEXT
);

CREATE INDEX idx_audit_log_timestamp ON audit_log(timestamp);
CREATE INDEX idx_audit_log_actor ON audit_log(actor_id);
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);

-- ============================================================================
-- SIMULATION STATE
-- ============================================================================

CREATE TABLE simulation_events (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    scheduled_time TEXT NOT NULL,
    processed_at TEXT,
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED', 'CANCELLED')),
    priority INTEGER NOT NULL DEFAULT 0,
    payload TEXT,
    result TEXT,
    error_message TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_simulation_events_status ON simulation_events(status);
CREATE INDEX idx_simulation_events_scheduled ON simulation_events(scheduled_time);

-- +migrate Down
DROP TABLE IF EXISTS simulation_events;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS directives;
DROP TABLE IF EXISTS security_incidents;
DROP TABLE IF EXISTS access_log;
DROP TABLE IF EXISTS security_zones;
DROP TABLE IF EXISTS medical_conditions;
DROP TABLE IF EXISTS medical_records;
DROP TABLE IF EXISTS maintenance_records;
DROP TABLE IF EXISTS facility_systems;
DROP TABLE IF EXISTS resource_transactions;
DROP TABLE IF EXISTS resource_stocks;
DROP TABLE IF EXISTS resource_items;
DROP TABLE IF EXISTS resource_categories;
DROP TABLE IF EXISTS work_assignments;
DROP TABLE IF EXISTS residents;
DROP TABLE IF EXISTS households;
DROP TABLE IF EXISTS vocations;
DROP TABLE IF EXISTS quarters;
DROP TABLE IF EXISTS vault_metadata;
