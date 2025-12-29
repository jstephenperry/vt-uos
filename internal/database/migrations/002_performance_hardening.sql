-- +migrate Up
-- Performance Hardening Migration
-- Adds missing indexes, composite indexes, partial indexes, and covering indexes
-- for optimal query performance on mission-critical vault operations.

-- ============================================================================
-- ANALYSIS NOTES:
-- 1. SQLite does NOT auto-create indexes on foreign key columns
-- 2. Composite indexes should have high-selectivity columns first
-- 3. Partial indexes reduce index size for filtered queries
-- 4. Covering indexes avoid table lookups for common queries
-- ============================================================================

-- ============================================================================
-- RESIDENTS - Core entity, heavily queried
-- ============================================================================

-- FK indexes (missing from original schema)
CREATE INDEX idx_residents_parent1 ON residents(biological_parent_1_id)
    WHERE biological_parent_1_id IS NOT NULL;
CREATE INDEX idx_residents_parent2 ON residents(biological_parent_2_id)
    WHERE biological_parent_2_id IS NOT NULL;
CREATE INDEX idx_residents_quarters ON residents(quarters_id)
    WHERE quarters_id IS NOT NULL;

-- Composite indexes for common query patterns
-- Active residents by name (census search)
CREATE INDEX idx_residents_active_name ON residents(status, surname, given_names)
    WHERE status = 'ACTIVE';

-- Household member lookup (very frequent)
CREATE INDEX idx_residents_household_status ON residents(household_id, status)
    WHERE household_id IS NOT NULL;

-- Age-based queries (demographics, work eligibility)
CREATE INDEX idx_residents_dob ON residents(date_of_birth);

-- Clearance-based access control
CREATE INDEX idx_residents_clearance ON residents(clearance_level, status)
    WHERE status = 'ACTIVE';

-- Lineage queries (COI calculation) - covering index
CREATE INDEX idx_residents_lineage ON residents(id, biological_parent_1_id, biological_parent_2_id);

-- Entry type filtering (original vs vault-born)
CREATE INDEX idx_residents_entry ON residents(entry_type, entry_date);

-- ============================================================================
-- HOUSEHOLDS - Rationing and housing queries
-- ============================================================================

-- FK index
CREATE INDEX idx_households_head ON households(head_of_household_id)
    WHERE head_of_household_id IS NOT NULL;

-- Rationing calculations (daily operation)
CREATE INDEX idx_households_active_ration ON households(status, ration_class)
    WHERE status = 'ACTIVE';

-- Housing assignment queries
CREATE INDEX idx_households_type_status ON households(household_type, status);

-- ============================================================================
-- QUARTERS - Housing availability
-- ============================================================================

-- FK index
CREATE INDEX idx_quarters_household ON quarters(assigned_household_id)
    WHERE assigned_household_id IS NOT NULL;

-- Find available quarters by location (housing assignment)
CREATE INDEX idx_quarters_available ON quarters(status, sector, level, unit_type)
    WHERE status = 'AVAILABLE';

-- Capacity planning
CREATE INDEX idx_quarters_capacity ON quarters(unit_type, capacity);

-- ============================================================================
-- VOCATIONS - Staffing queries
-- ============================================================================

-- Active positions by department (staffing reports)
CREATE INDEX idx_vocations_dept_active ON vocations(department, is_active)
    WHERE is_active = 1;

-- Clearance-based job matching
CREATE INDEX idx_vocations_clearance ON vocations(required_clearance, is_active)
    WHERE is_active = 1;

-- ============================================================================
-- WORK_ASSIGNMENTS - Shift scheduling, labor allocation
-- ============================================================================

-- Active assignments by resident (schedule lookup)
CREATE INDEX idx_work_assignments_resident_active ON work_assignments(resident_id, status, shift)
    WHERE status = 'ACTIVE';

-- Staffing by vocation (vacancy tracking)
CREATE INDEX idx_work_assignments_vocation_active ON work_assignments(vocation_id, status, assignment_type)
    WHERE status = 'ACTIVE';

-- Shift roster generation
CREATE INDEX idx_work_assignments_shift ON work_assignments(shift, status, start_date)
    WHERE status = 'ACTIVE';

-- Date range queries (scheduling)
CREATE INDEX idx_work_assignments_dates ON work_assignments(start_date, end_date);

-- FK index
CREATE INDEX idx_work_assignments_assigned_by ON work_assignments(assigned_by)
    WHERE assigned_by IS NOT NULL;

-- ============================================================================
-- RESOURCE_STOCKS - Inventory management (high frequency)
-- ============================================================================

-- Available inventory by item (consumption queries)
CREATE INDEX idx_resource_stocks_available ON resource_stocks(item_id, status, quantity)
    WHERE status = 'AVAILABLE' AND quantity > 0;

-- Expiring items alert (daily check)
CREATE INDEX idx_resource_stocks_expiring ON resource_stocks(status, expiration_date)
    WHERE status = 'AVAILABLE' AND expiration_date IS NOT NULL;

-- FIFO consumption (oldest first)
CREATE INDEX idx_resource_stocks_fifo ON resource_stocks(item_id, received_date)
    WHERE status = 'AVAILABLE';

-- Audit trail
CREATE INDEX idx_resource_stocks_audit ON resource_stocks(last_audit_by)
    WHERE last_audit_by IS NOT NULL;

-- ============================================================================
-- RESOURCE_TRANSACTIONS - Append-only audit trail
-- ============================================================================

-- FK index
CREATE INDEX idx_resource_transactions_stock ON resource_transactions(stock_id)
    WHERE stock_id IS NOT NULL;

-- Item history with time ordering
CREATE INDEX idx_resource_transactions_item_time ON resource_transactions(item_id, timestamp DESC);

-- Transaction type analysis
CREATE INDEX idx_resource_transactions_type_time ON resource_transactions(transaction_type, timestamp DESC);

-- Authorization audit
CREATE INDEX idx_resource_transactions_auth ON resource_transactions(authorized_by, timestamp)
    WHERE authorized_by IS NOT NULL;

-- ============================================================================
-- FACILITY_SYSTEMS - Critical infrastructure monitoring
-- ============================================================================

-- Critical system status (dashboard)
CREATE INDEX idx_facility_critical ON facility_systems(category, status, efficiency_percent)
    WHERE category IN ('POWER', 'WATER', 'HVAC', 'WASTE', 'SECURITY');

-- Maintenance scheduling
CREATE INDEX idx_facility_maintenance_due ON facility_systems(next_maintenance_due, status)
    WHERE status IN ('OPERATIONAL', 'DEGRADED');

-- Location-based queries
CREATE INDEX idx_facility_location ON facility_systems(location_sector, location_level);

-- Degraded systems alert
CREATE INDEX idx_facility_degraded ON facility_systems(status, efficiency_percent)
    WHERE status = 'DEGRADED' OR efficiency_percent < 80;

-- ============================================================================
-- MAINTENANCE_RECORDS - Work order management
-- ============================================================================

-- FK indexes
CREATE INDEX idx_maintenance_technician ON maintenance_records(lead_technician_id)
    WHERE lead_technician_id IS NOT NULL;

-- Scheduled maintenance
CREATE INDEX idx_maintenance_scheduled ON maintenance_records(scheduled_date, system_id)
    WHERE outcome IS NULL;

-- Completed work history
CREATE INDEX idx_maintenance_completed ON maintenance_records(system_id, completed_at DESC)
    WHERE completed_at IS NOT NULL;

-- Outcome analysis
CREATE INDEX idx_maintenance_outcome ON maintenance_records(outcome, maintenance_type);

-- ============================================================================
-- MEDICAL_RECORDS - Patient care (confidentiality-aware)
-- ============================================================================

-- Patient history (most common query)
CREATE INDEX idx_medical_patient_history ON medical_records(resident_id, encounter_date DESC);

-- Specific record types per patient
CREATE INDEX idx_medical_patient_type ON medical_records(resident_id, record_type, encounter_date DESC);

-- Provider workload
CREATE INDEX idx_medical_provider ON medical_records(provider_id, encounter_date)
    WHERE provider_id IS NOT NULL;

-- Follow-up scheduling
CREATE INDEX idx_medical_followup ON medical_records(follow_up_date, status)
    WHERE follow_up_date IS NOT NULL AND status = 'FOLLOW_UP_REQUIRED';

-- Radiation tracking (safety critical)
CREATE INDEX idx_medical_radiation ON medical_records(resident_id, radiation_cumulative_msv)
    WHERE radiation_dose_msv IS NOT NULL;

-- ============================================================================
-- MEDICAL_CONDITIONS - Epidemiology and genetics
-- ============================================================================

-- Active chronic conditions per patient
CREATE INDEX idx_conditions_chronic ON medical_conditions(resident_id, is_chronic, severity)
    WHERE resolution_date IS NULL;

-- Contagious disease tracking (outbreak detection)
CREATE INDEX idx_conditions_contagious ON medical_conditions(is_contagious, onset_date, severity)
    WHERE is_contagious = 1 AND resolution_date IS NULL;

-- Genetic conditions (lineage analysis)
CREATE INDEX idx_conditions_genetic ON medical_conditions(is_genetic, condition_code)
    WHERE is_genetic = 1;

-- Severity-based alerts
CREATE INDEX idx_conditions_severe ON medical_conditions(severity, resident_id)
    WHERE severity IN ('SEVERE', 'CRITICAL') AND resolution_date IS NULL;

-- ============================================================================
-- SECURITY_ZONES - Access control
-- ============================================================================

-- Sector-based zone lookup
CREATE INDEX idx_zones_sector ON security_zones(sector, required_clearance);

-- Restricted areas
CREATE INDEX idx_zones_restricted ON security_zones(is_restricted, required_clearance)
    WHERE is_restricted = 1;

-- ============================================================================
-- ACCESS_LOG - High-write audit table (minimal indexes)
-- ============================================================================

-- CAUTION: This is a high-write table. Too many indexes slow inserts.
-- Only add indexes essential for security queries.

-- Resident access history (security investigation)
CREATE INDEX idx_access_resident_time ON access_log(resident_id, timestamp DESC);

-- Zone activity monitoring
CREATE INDEX idx_access_zone_time ON access_log(zone_id, timestamp DESC);

-- Denied access alerts (security review)
CREATE INDEX idx_access_denied ON access_log(access_result, timestamp DESC)
    WHERE access_result IN ('DENIED', 'OVERRIDE', 'EMERGENCY');

-- Drop redundant single-column indexes (replaced by composites)
DROP INDEX IF EXISTS idx_access_log_resident;
DROP INDEX IF EXISTS idx_access_log_zone;

-- ============================================================================
-- SECURITY_INCIDENTS - Incident management
-- ============================================================================

-- Open incidents by severity (dashboard)
CREATE INDEX idx_incidents_open ON security_incidents(status, severity, occurred_at DESC)
    WHERE status NOT IN ('RESOLVED', 'CLOSED');

-- Reporter lookup
CREATE INDEX idx_incidents_reporter ON security_incidents(reported_by)
    WHERE reported_by IS NOT NULL;

-- Location-based analysis
CREATE INDEX idx_incidents_location ON security_incidents(location_sector, occurred_at DESC);

-- ============================================================================
-- DIRECTIVES - Policy management
-- ============================================================================

-- Active directives (policy lookup)
CREATE INDEX idx_directives_active ON directives(status, effective_date, classification_level)
    WHERE status = 'ACTIVE';

-- Author lookup
CREATE INDEX idx_directives_author ON directives(issued_by, status);

-- Expiring directives
CREATE INDEX idx_directives_expiring ON directives(expiration_date, status)
    WHERE expiration_date IS NOT NULL AND status = 'ACTIVE';

-- ============================================================================
-- AUDIT_LOG - Immutable audit trail (high-write, careful with indexes)
-- ============================================================================

-- CAUTION: High-write table. Indexes must be minimal and essential.

-- Entity history (investigation)
CREATE INDEX idx_audit_entity_time ON audit_log(entity_type, entity_id, timestamp DESC);

-- Actor activity (user behavior analysis)
CREATE INDEX idx_audit_actor_time ON audit_log(actor_id, timestamp DESC)
    WHERE actor_id IS NOT NULL;

-- Recent actions by type (dashboard)
CREATE INDEX idx_audit_action_time ON audit_log(action, timestamp DESC);

-- Drop redundant index
DROP INDEX IF EXISTS idx_audit_log_entity;

-- ============================================================================
-- SIMULATION_EVENTS - Event queue processing
-- ============================================================================

-- Event processing queue (hot path - must be fast)
CREATE INDEX idx_sim_queue ON simulation_events(status, priority DESC, scheduled_time)
    WHERE status = 'PENDING';

-- Event history by type
CREATE INDEX idx_sim_type_time ON simulation_events(event_type, scheduled_time DESC);

-- Failed events (error handling)
CREATE INDEX idx_sim_failed ON simulation_events(status, scheduled_time)
    WHERE status = 'FAILED';

-- ============================================================================
-- ANALYZE all tables to update statistics
-- ============================================================================

ANALYZE;

-- +migrate Down

-- Residents
DROP INDEX IF EXISTS idx_residents_parent1;
DROP INDEX IF EXISTS idx_residents_parent2;
DROP INDEX IF EXISTS idx_residents_quarters;
DROP INDEX IF EXISTS idx_residents_active_name;
DROP INDEX IF EXISTS idx_residents_household_status;
DROP INDEX IF EXISTS idx_residents_dob;
DROP INDEX IF EXISTS idx_residents_clearance;
DROP INDEX IF EXISTS idx_residents_lineage;
DROP INDEX IF EXISTS idx_residents_entry;

-- Households
DROP INDEX IF EXISTS idx_households_head;
DROP INDEX IF EXISTS idx_households_active_ration;
DROP INDEX IF EXISTS idx_households_type_status;

-- Quarters
DROP INDEX IF EXISTS idx_quarters_household;
DROP INDEX IF EXISTS idx_quarters_available;
DROP INDEX IF EXISTS idx_quarters_capacity;

-- Vocations
DROP INDEX IF EXISTS idx_vocations_dept_active;
DROP INDEX IF EXISTS idx_vocations_clearance;

-- Work Assignments
DROP INDEX IF EXISTS idx_work_assignments_resident_active;
DROP INDEX IF EXISTS idx_work_assignments_vocation_active;
DROP INDEX IF EXISTS idx_work_assignments_shift;
DROP INDEX IF EXISTS idx_work_assignments_dates;
DROP INDEX IF EXISTS idx_work_assignments_assigned_by;

-- Resource Stocks
DROP INDEX IF EXISTS idx_resource_stocks_available;
DROP INDEX IF EXISTS idx_resource_stocks_expiring;
DROP INDEX IF EXISTS idx_resource_stocks_fifo;
DROP INDEX IF EXISTS idx_resource_stocks_audit;

-- Resource Transactions
DROP INDEX IF EXISTS idx_resource_transactions_stock;
DROP INDEX IF EXISTS idx_resource_transactions_item_time;
DROP INDEX IF EXISTS idx_resource_transactions_type_time;
DROP INDEX IF EXISTS idx_resource_transactions_auth;

-- Facility Systems
DROP INDEX IF EXISTS idx_facility_critical;
DROP INDEX IF EXISTS idx_facility_maintenance_due;
DROP INDEX IF EXISTS idx_facility_location;
DROP INDEX IF EXISTS idx_facility_degraded;

-- Maintenance Records
DROP INDEX IF EXISTS idx_maintenance_technician;
DROP INDEX IF EXISTS idx_maintenance_scheduled;
DROP INDEX IF EXISTS idx_maintenance_completed;
DROP INDEX IF EXISTS idx_maintenance_outcome;

-- Medical Records
DROP INDEX IF EXISTS idx_medical_patient_history;
DROP INDEX IF EXISTS idx_medical_patient_type;
DROP INDEX IF EXISTS idx_medical_provider;
DROP INDEX IF EXISTS idx_medical_followup;
DROP INDEX IF EXISTS idx_medical_radiation;

-- Medical Conditions
DROP INDEX IF EXISTS idx_conditions_chronic;
DROP INDEX IF EXISTS idx_conditions_contagious;
DROP INDEX IF EXISTS idx_conditions_genetic;
DROP INDEX IF EXISTS idx_conditions_severe;

-- Security Zones
DROP INDEX IF EXISTS idx_zones_sector;
DROP INDEX IF EXISTS idx_zones_restricted;

-- Access Log
DROP INDEX IF EXISTS idx_access_resident_time;
DROP INDEX IF EXISTS idx_access_zone_time;
DROP INDEX IF EXISTS idx_access_denied;
-- Recreate original indexes
CREATE INDEX idx_access_log_resident ON access_log(resident_id);
CREATE INDEX idx_access_log_zone ON access_log(zone_id);

-- Security Incidents
DROP INDEX IF EXISTS idx_incidents_open;
DROP INDEX IF EXISTS idx_incidents_reporter;
DROP INDEX IF EXISTS idx_incidents_location;

-- Directives
DROP INDEX IF EXISTS idx_directives_active;
DROP INDEX IF EXISTS idx_directives_author;
DROP INDEX IF EXISTS idx_directives_expiring;

-- Audit Log
DROP INDEX IF EXISTS idx_audit_entity_time;
DROP INDEX IF EXISTS idx_audit_actor_time;
DROP INDEX IF EXISTS idx_audit_action_time;
-- Recreate original index
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);

-- Simulation Events
DROP INDEX IF EXISTS idx_sim_queue;
DROP INDEX IF EXISTS idx_sim_type_time;
DROP INDEX IF EXISTS idx_sim_failed;
