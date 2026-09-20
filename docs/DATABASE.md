# Database Schema

## Overview

VT-UOS persists all state in a single SQLite file opened through `modernc.org/sqlite` (pure Go, no CGO) (`internal/database/database.go`). Every table, index, and constraint below comes from the two embedded migration files (`internal/database/migrations/001_initial.sql`, `internal/database/migrations/002_performance_hardening.sql`); no other file defines schema. There are no `CREATE TRIGGER` statements anywhere in the migrations — the schema has no triggers.

Twenty domain tables exist. `001_initial.sql` creates all twenty; `002_performance_hardening.sql` adds indexes only (plus one `ANALYZE`) and creates no new tables. A twenty-first table, `schema_migrations`, is created directly by Go code rather than by a migration file (`internal/database/migrate.go`) and is described under "Migration mechanism" below.

## Vault Metadata

### `vault_metadata`

Key/value store for schema version, the simulated vault clock, and simulation engine counters that must survive a restart.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `key` | TEXT | PRIMARY KEY | — |
| `value` | TEXT | NOT NULL | — |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes: none beyond the primary key. Foreign keys: none. Triggers: none.

The migration itself seeds two rows: `schema_version`='1' and `vault_time`='2077-10-23T09:47:00Z' (`internal/database/migrations/001_initial.sql`). At runtime the simulation engine reads/writes four keys via an upsert (`INSERT ... ON CONFLICT(key) DO UPDATE`) — `vault_time`, `sim_births`, `sim_deaths`, `sim_tick_count` — so the vault clock and cumulative birth/death/tick counters survive a restart (`internal/simulation/persistence.go`).

## Population & Housing

### `quarters`

Physical living-space units that residents and households occupy.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `unit_code` | TEXT | UNIQUE NOT NULL | — |
| `sector` | TEXT | NOT NULL | — |
| `level` | INTEGER | NOT NULL | — |
| `unit_type` | TEXT | NOT NULL, CHECK IN (`SINGLE`,`DOUBLE`,`FAMILY`,`DORMITORY`,`EXECUTIVE`) | — |
| `capacity` | INTEGER | NOT NULL | — |
| `square_meters` | REAL | NOT NULL | — |
| `amenities` | TEXT | — | — |
| `status` | TEXT | NOT NULL, CHECK IN (`AVAILABLE`,`OCCUPIED`,`MAINTENANCE`,`CONDEMNED`) | `'AVAILABLE'` |
| `assigned_household_id` | TEXT | — (no FK, see note) | — |
| `notes` | TEXT | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes: `idx_quarters_status(status)`, `idx_quarters_sector(sector)`, `idx_quarters_unit_type(unit_type)` (`001_initial.sql`); `idx_quarters_household(assigned_household_id)` partial WHERE NOT NULL, `idx_quarters_available(status, sector, level, unit_type)` partial WHERE `status='AVAILABLE'`, `idx_quarters_capacity(unit_type, capacity)` (`002_performance_hardening.sql`).

Foreign keys: none. `assigned_household_id` has no `REFERENCES households(id)` clause — a comment in the migration explains this is deliberate: "SQLite doesn't support ALTER TABLE ADD CONSTRAINT, so this is handled at application level" (`internal/database/migrations/001_initial.sql`).

### `vocations`

Job categories and specific positions residents can be assigned to.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `code` | TEXT | UNIQUE NOT NULL | — |
| `title` | TEXT | NOT NULL | — |
| `department` | TEXT | NOT NULL, CHECK IN (`ENGINEERING`,`MEDICAL`,`SECURITY`,`FOOD_PRODUCTION`,`ADMINISTRATION`,`EDUCATION`,`SANITATION`,`RESEARCH`) | — |
| `required_clearance` | INTEGER | NOT NULL | `1` |
| `required_skills` | TEXT | — | — |
| `headcount_authorized` | INTEGER | NOT NULL | — |
| `headcount_minimum` | INTEGER | NOT NULL | — |
| `shift_pattern` | TEXT | NOT NULL, CHECK IN (`STANDARD`,`ROTATING`,`ON_CALL`,`CONTINUOUS`) | `'STANDARD'` |
| `hazard_level` | TEXT | NOT NULL, CHECK IN (`NONE`,`LOW`,`MODERATE`,`HIGH`,`EXTREME`) | `'NONE'` |
| `description` | TEXT | — | — |
| `is_active` | INTEGER | NOT NULL | `1` |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes: `idx_vocations_department(department)`, `idx_vocations_active(is_active)` (`001_initial.sql`); `idx_vocations_dept_active(department, is_active)` partial WHERE `is_active=1`, `idx_vocations_clearance(required_clearance, is_active)` partial WHERE `is_active=1` (`002_performance_hardening.sql`).

Foreign keys: none. Triggers: none.

### `households`

Groupings of residents that share a ration class and (usually) quarters.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `designation` | TEXT | UNIQUE NOT NULL | — |
| `household_type` | TEXT | NOT NULL, CHECK IN (`FAMILY`,`INDIVIDUAL`,`COMMUNAL`,`TEMPORARY`) | — |
| `head_of_household_id` | TEXT | — (no FK, see note) | — |
| `quarters_id` | TEXT | REFERENCES `quarters(id)` | — |
| `ration_class` | TEXT | NOT NULL, CHECK IN (`MINIMAL`,`STANDARD`,`ENHANCED`,`MEDICAL`,`LABOR_INTENSIVE`) | `'STANDARD'` |
| `status` | TEXT | NOT NULL, CHECK IN (`ACTIVE`,`DISSOLVED`,`MERGED`) | `'ACTIVE'` |
| `formed_date` | TEXT | NOT NULL | — |
| `dissolved_date` | TEXT | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes: `idx_households_status(status)`, `idx_households_quarters(quarters_id)` (`001_initial.sql`); `idx_households_head(head_of_household_id)` partial WHERE NOT NULL, `idx_households_active_ration(status, ration_class)` partial WHERE `status='ACTIVE'`, `idx_households_type_status(household_type, status)` (`002_performance_hardening.sql`).

Foreign keys: `quarters_id` → `quarters(id)`. `head_of_household_id` has no `REFERENCES residents(id)` clause, for the same application-level-enforcement reason noted in the migration's own comment (`internal/database/migrations/001_initial.sql`).

### `residents`

The central entity: a vault dweller, with identity, lineage, and current assignments.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `registry_number` | TEXT | UNIQUE NOT NULL | — |
| `surname` | TEXT | NOT NULL | — |
| `given_names` | TEXT | NOT NULL | — |
| `date_of_birth` | TEXT | NOT NULL | — |
| `date_of_death` | TEXT | — | — |
| `sex` | TEXT | NOT NULL, CHECK IN (`M`,`F`) | — |
| `blood_type` | TEXT | CHECK IN (`A+`,`A-`,`B+`,`B-`,`AB+`,`AB-`,`O+`,`O-`) | — |
| `entry_type` | TEXT | NOT NULL, CHECK IN (`ORIGINAL`,`VAULT_BORN`,`ADMITTED`) | — |
| `entry_date` | TEXT | NOT NULL | — |
| `status` | TEXT | NOT NULL, CHECK IN (`ACTIVE`,`DECEASED`,`EXILED`,`SURFACE_MISSION`,`QUARANTINE`) | `'ACTIVE'` |
| `biological_parent_1_id` | TEXT | REFERENCES `residents(id)` | — |
| `biological_parent_2_id` | TEXT | REFERENCES `residents(id)` | — |
| `household_id` | TEXT | REFERENCES `households(id)` | — |
| `quarters_id` | TEXT | REFERENCES `quarters(id)` | — |
| `primary_vocation_id` | TEXT | REFERENCES `vocations(id)` | — |
| `clearance_level` | INTEGER | NOT NULL, CHECK BETWEEN 1 AND 10 | `1` |
| `notes` | TEXT | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes (`001_initial.sql`): `idx_residents_status(status)`, `idx_residents_household(household_id)`, `idx_residents_vocation(primary_vocation_id)`, `idx_residents_surname(surname)`, `idx_residents_registry(registry_number)`.

Indexes (`002_performance_hardening.sql`): `idx_residents_parent1(biological_parent_1_id)` partial WHERE NOT NULL, `idx_residents_parent2(biological_parent_2_id)` partial WHERE NOT NULL, `idx_residents_quarters(quarters_id)` partial WHERE NOT NULL, `idx_residents_active_name(status, surname, given_names)` partial WHERE `status='ACTIVE'`, `idx_residents_household_status(household_id, status)` partial WHERE NOT NULL, `idx_residents_dob(date_of_birth)`, `idx_residents_clearance(clearance_level, status)` partial WHERE `status='ACTIVE'`, `idx_residents_lineage(id, biological_parent_1_id, biological_parent_2_id)` (covering index for COI/lineage lookups), `idx_residents_entry(entry_type, entry_date)`.

Foreign keys: `biological_parent_1_id`/`biological_parent_2_id` (self-referential), `household_id` → `households(id)`, `quarters_id` → `quarters(id)`, `primary_vocation_id` → `vocations(id)`. Triggers: none.

The `QUARANTINE` status is written by the simulation's illness incident with a raw `UPDATE` (`internal/simulation/incidents.go`); no code path returns a resident to `ACTIVE`.

> ⚠ See [#31](https://github.com/jstephenperry/vt-uos/issues/31).

### `work_assignments`

Assigns a resident to a vocation with shift, status, and performance tracking.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `resident_id` | TEXT | NOT NULL, REFERENCES `residents(id)` | — |
| `vocation_id` | TEXT | NOT NULL, REFERENCES `vocations(id)` | — |
| `assignment_type` | TEXT | NOT NULL, CHECK IN (`PRIMARY`,`SECONDARY`,`TEMPORARY`,`TRAINING`) | — |
| `start_date` | TEXT | NOT NULL | — |
| `end_date` | TEXT | — | — |
| `shift` | TEXT | CHECK IN (`ALPHA`,`BETA`,`GAMMA`) | — |
| `status` | TEXT | NOT NULL, CHECK IN (`ACTIVE`,`ON_LEAVE`,`SUSPENDED`,`COMPLETED`) | `'ACTIVE'` |
| `performance_rating` | REAL | CHECK BETWEEN 0 AND 5 | — |
| `assigned_by` | TEXT | REFERENCES `residents(id)` | — |
| `notes` | TEXT | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes (`001_initial.sql`): `idx_work_assignments_resident(resident_id)`, `idx_work_assignments_vocation(vocation_id)`, `idx_work_assignments_status(status)`.

Indexes (`002_performance_hardening.sql`): `idx_work_assignments_resident_active(resident_id, status, shift)` partial WHERE `status='ACTIVE'`, `idx_work_assignments_vocation_active(vocation_id, status, assignment_type)` partial WHERE `status='ACTIVE'`, `idx_work_assignments_shift(shift, status, start_date)` partial WHERE `status='ACTIVE'`, `idx_work_assignments_dates(start_date, end_date)`, `idx_work_assignments_assigned_by(assigned_by)` partial WHERE NOT NULL.

Foreign keys: `resident_id` → `residents(id)`, `vocation_id` → `vocations(id)`, `assigned_by` → `residents(id)`. Triggers: none.

## Resources

### `resource_categories`

Top-level classification for consumables and materials (food, water, medical, etc.).

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `code` | TEXT | UNIQUE NOT NULL | — |
| `name` | TEXT | NOT NULL | — |
| `description` | TEXT | — | — |
| `unit_of_measure` | TEXT | NOT NULL | — |
| `is_consumable` | INTEGER | NOT NULL | `1` |
| `is_critical` | INTEGER | NOT NULL | `0` |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes: none in either migration. Foreign keys: none. Triggers: none.

### `resource_items`

A specific stock-keeping unit within a category.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `category_id` | TEXT | NOT NULL, REFERENCES `resource_categories(id)` | — |
| `item_code` | TEXT | UNIQUE NOT NULL | — |
| `name` | TEXT | NOT NULL | — |
| `description` | TEXT | — | — |
| `unit_of_measure` | TEXT | NOT NULL | — |
| `calories_per_unit` | REAL | — | — |
| `shelf_life_days` | INTEGER | — | — |
| `storage_requirements` | TEXT | — | — |
| `is_producible` | INTEGER | NOT NULL | `0` |
| `production_rate_per_day` | REAL | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes: `idx_resource_items_category(category_id)` (`001_initial.sql`). No `002` additions.

Foreign keys: `category_id` → `resource_categories(id)`. Triggers: none.

### `resource_stocks`

A physical lot of a resource item, tracked for FIFO consumption and expiration.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `item_id` | TEXT | NOT NULL, REFERENCES `resource_items(id)` | — |
| `lot_number` | TEXT | — | — |
| `quantity` | REAL | NOT NULL, CHECK >= 0 | — |
| `quantity_reserved` | REAL | NOT NULL, CHECK >= 0 | `0` |
| `storage_location` | TEXT | NOT NULL | — |
| `received_date` | TEXT | NOT NULL | — |
| `expiration_date` | TEXT | — | — |
| `status` | TEXT | NOT NULL, CHECK IN (`AVAILABLE`,`RESERVED`,`QUARANTINE`,`EXPIRED`,`DEPLETED`) | `'AVAILABLE'` |
| `last_audit_date` | TEXT | — | — |
| `last_audit_by` | TEXT | REFERENCES `residents(id)` | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes (`001_initial.sql`): `idx_resource_stocks_item(item_id)`, `idx_resource_stocks_status(status)`, `idx_resource_stocks_expiration(expiration_date)`, `idx_resource_stocks_location(storage_location)`.

Indexes (`002_performance_hardening.sql`): `idx_resource_stocks_available(item_id, status, quantity)` partial WHERE `status='AVAILABLE' AND quantity > 0`, `idx_resource_stocks_expiring(status, expiration_date)` partial WHERE `status='AVAILABLE' AND expiration_date IS NOT NULL`, `idx_resource_stocks_fifo(item_id, received_date)` partial WHERE `status='AVAILABLE'`, `idx_resource_stocks_audit(last_audit_by)` partial WHERE NOT NULL.

Foreign keys: `item_id` → `resource_items(id)`, `last_audit_by` → `residents(id)`. Triggers: none.

### `resource_transactions`

Append-only ledger of stock movements (consumption, production, adjustment, spoilage, transfer, audit correction).

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `stock_id` | TEXT | REFERENCES `resource_stocks(id)` | — |
| `item_id` | TEXT | NOT NULL, REFERENCES `resource_items(id)` | — |
| `transaction_type` | TEXT | NOT NULL, CHECK IN (`CONSUMPTION`,`PRODUCTION`,`ADJUSTMENT`,`SPOILAGE`,`TRANSFER`,`AUDIT_CORRECTION`) | — |
| `quantity` | REAL | NOT NULL | — |
| `balance_after` | REAL | NOT NULL | — |
| `reason` | TEXT | — | — |
| `authorized_by` | TEXT | REFERENCES `residents(id)` | — |
| `related_entity_type` | TEXT | — | — |
| `related_entity_id` | TEXT | — | — |
| `timestamp` | TEXT | NOT NULL | `datetime('now')` |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes (`001_initial.sql`): `idx_resource_transactions_item(item_id)`, `idx_resource_transactions_timestamp(timestamp)`, `idx_resource_transactions_type(transaction_type)`.

Indexes (`002_performance_hardening.sql`): `idx_resource_transactions_stock(stock_id)` partial WHERE NOT NULL, `idx_resource_transactions_item_time(item_id, timestamp DESC)`, `idx_resource_transactions_type_time(transaction_type, timestamp DESC)`, `idx_resource_transactions_auth(authorized_by, timestamp)` partial WHERE NOT NULL.

Foreign keys: `stock_id` → `resource_stocks(id)` (nullable — NULL represents a production event with no source lot), `item_id` → `resource_items(id)`, `authorized_by` → `residents(id)`. Triggers: none.

## Facility Systems

### `facility_systems`

Infrastructure units (power, water, HVAC, security, medical, food production, waste, communications, structural) monitored for status and maintenance.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `system_code` | TEXT | UNIQUE NOT NULL | — |
| `name` | TEXT | NOT NULL | — |
| `category` | TEXT | NOT NULL, CHECK IN (`POWER`,`WATER`,`HVAC`,`SECURITY`,`MEDICAL`,`FOOD_PRODUCTION`,`WASTE`,`COMMUNICATIONS`,`STRUCTURAL`) | — |
| `location_sector` | TEXT | NOT NULL | — |
| `location_level` | INTEGER | NOT NULL | — |
| `status` | TEXT | NOT NULL, CHECK IN (`OPERATIONAL`,`DEGRADED`,`MAINTENANCE`,`OFFLINE`,`FAILED`,`DESTROYED`) | `'OPERATIONAL'` |
| `efficiency_percent` | REAL | NOT NULL, CHECK BETWEEN 0 AND 100 | `100.0` |
| `capacity_rating` | REAL | — | — |
| `capacity_unit` | TEXT | — | — |
| `current_output` | REAL | — | — |
| `install_date` | TEXT | NOT NULL | — |
| `last_maintenance_date` | TEXT | — | — |
| `next_maintenance_due` | TEXT | — | — |
| `maintenance_interval_days` | INTEGER | NOT NULL | `90` |
| `mtbf_hours` | INTEGER | — | — |
| `total_runtime_hours` | REAL | NOT NULL | `0` |
| `telemetry_json` | TEXT | — | — |
| `telemetry_updated_at` | TEXT | — | — |
| `notes` | TEXT | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes (`001_initial.sql`): `idx_facility_systems_category(category)`, `idx_facility_systems_status(status)`.

Indexes (`002_performance_hardening.sql`): `idx_facility_critical(category, status, efficiency_percent)` partial WHERE `category IN ('POWER','WATER','HVAC','WASTE','SECURITY')`, `idx_facility_maintenance_due(next_maintenance_due, status)` partial WHERE `status IN ('OPERATIONAL','DEGRADED')`, `idx_facility_location(location_sector, location_level)`, `idx_facility_degraded(status, efficiency_percent)` partial WHERE `status='DEGRADED' OR efficiency_percent < 80`.

Foreign keys: none. Triggers: none.

`telemetry_json`/`telemetry_updated_at` are declared columns with no reader or writer anywhere in the Go code — `internal/repository/facility_repo.go`'s INSERT/SELECT/UPDATE column lists and `internal/models/facility.go`'s struct both omit them entirely.

> ⚠ See [#24](https://github.com/jstephenperry/vt-uos/issues/24).

### `maintenance_records`

A work order against a `facility_systems` row.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `system_id` | TEXT | NOT NULL, REFERENCES `facility_systems(id)` | — |
| `maintenance_type` | TEXT | NOT NULL, CHECK IN (`PREVENTIVE`,`CORRECTIVE`,`EMERGENCY`,`INSPECTION`,`UPGRADE`) | — |
| `description` | TEXT | NOT NULL | — |
| `work_performed` | TEXT | — | — |
| `parts_consumed` | TEXT | — | — |
| `lead_technician_id` | TEXT | REFERENCES `residents(id)` | — |
| `crew_member_ids` | TEXT | — | — |
| `scheduled_date` | TEXT | — | — |
| `started_at` | TEXT | — | — |
| `completed_at` | TEXT | — | — |
| `estimated_hours` | REAL | — | — |
| `actual_hours` | REAL | — | — |
| `outcome` | TEXT | CHECK IN (`COMPLETED`,`PARTIAL`,`FAILED`,`DEFERRED`,`CANCELLED`) | — |
| `system_status_before` | TEXT | — | — |
| `system_status_after` | TEXT | — | — |
| `efficiency_before` | REAL | — | — |
| `efficiency_after` | REAL | — | — |
| `notes` | TEXT | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes (`001_initial.sql`): `idx_maintenance_records_system(system_id)`, `idx_maintenance_records_type(maintenance_type)`.

Indexes (`002_performance_hardening.sql`): `idx_maintenance_technician(lead_technician_id)` partial WHERE NOT NULL, `idx_maintenance_scheduled(scheduled_date, system_id)` partial WHERE `outcome IS NULL`, `idx_maintenance_completed(system_id, completed_at DESC)` partial WHERE `completed_at IS NOT NULL`, `idx_maintenance_outcome(outcome, maintenance_type)`.

Foreign keys: `system_id` → `facility_systems(id)`, `lead_technician_id` → `residents(id)`. Triggers: none.

## Medical Records

### `medical_records`

An encounter record: examination, treatment, vaccination, incident, psychological, radiation, chronic condition, or lab result.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `resident_id` | TEXT | NOT NULL, REFERENCES `residents(id)` | — |
| `record_type` | TEXT | NOT NULL, CHECK IN (`EXAMINATION`,`TREATMENT`,`VACCINATION`,`INCIDENT`,`PSYCHOLOGICAL`,`RADIATION`,`CHRONIC_CONDITION`,`LAB_RESULT`) | — |
| `chief_complaint` | TEXT | — | — |
| `diagnosis_codes` | TEXT | — | — |
| `diagnosis_text` | TEXT | — | — |
| `treatment_provided` | TEXT | — | — |
| `medications_prescribed` | TEXT | — | — |
| `vitals_json` | TEXT | — | — |
| `radiation_dose_msv` | REAL | — | — |
| `radiation_cumulative_msv` | REAL | — | — |
| `provider_id` | TEXT | REFERENCES `residents(id)` | — |
| `facility_location` | TEXT | — | — |
| `encounter_date` | TEXT | NOT NULL | — |
| `follow_up_date` | TEXT | — | — |
| `status` | TEXT | NOT NULL, CHECK IN (`ACTIVE`,`RESOLVED`,`CHRONIC`,`FOLLOW_UP_REQUIRED`) | `'ACTIVE'` |
| `confidentiality_level` | INTEGER | NOT NULL | `1` |
| `notes` | TEXT | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes (`001_initial.sql`): `idx_medical_records_resident(resident_id)`, `idx_medical_records_type(record_type)`, `idx_medical_records_date(encounter_date)`.

Indexes (`002_performance_hardening.sql`): `idx_medical_patient_history(resident_id, encounter_date DESC)`, `idx_medical_patient_type(resident_id, record_type, encounter_date DESC)`, `idx_medical_provider(provider_id, encounter_date)` partial WHERE NOT NULL, `idx_medical_followup(follow_up_date, status)` partial WHERE `follow_up_date IS NOT NULL AND status='FOLLOW_UP_REQUIRED'`, `idx_medical_radiation(resident_id, radiation_cumulative_msv)` partial WHERE `radiation_dose_msv IS NOT NULL`.

Foreign keys: `resident_id` → `residents(id)`, `provider_id` → `residents(id)`. Triggers: none.

### `medical_conditions`

A diagnosed condition tracked over time, including whether it is chronic, genetic, or contagious.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `resident_id` | TEXT | NOT NULL, REFERENCES `residents(id)` | — |
| `condition_code` | TEXT | NOT NULL | — |
| `condition_name` | TEXT | NOT NULL | — |
| `onset_date` | TEXT | NOT NULL | — |
| `resolution_date` | TEXT | — | — |
| `severity` | TEXT | NOT NULL, CHECK IN (`MILD`,`MODERATE`,`SEVERE`,`CRITICAL`) | — |
| `is_chronic` | INTEGER | NOT NULL | `0` |
| `is_genetic` | INTEGER | NOT NULL | `0` |
| `is_contagious` | INTEGER | NOT NULL | `0` |
| `treatment_plan` | TEXT | — | — |
| `notes` | TEXT | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes (`001_initial.sql`): `idx_medical_conditions_resident(resident_id)`, `idx_medical_conditions_chronic(is_chronic)`, `idx_medical_conditions_contagious(is_contagious)`.

Indexes (`002_performance_hardening.sql`): `idx_conditions_chronic(resident_id, is_chronic, severity)` partial WHERE `resolution_date IS NULL`, `idx_conditions_contagious(is_contagious, onset_date, severity)` partial WHERE `is_contagious=1 AND resolution_date IS NULL`, `idx_conditions_genetic(is_genetic, condition_code)` partial WHERE `is_genetic=1`, `idx_conditions_severe(severity, resident_id)` partial WHERE `severity IN ('SEVERE','CRITICAL') AND resolution_date IS NULL`.

Foreign keys: `resident_id` → `residents(id)`. Triggers: none.

## Security & Access Control

### `security_zones`

A controlled-access area within the vault.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `zone_code` | TEXT | UNIQUE NOT NULL | — |
| `name` | TEXT | NOT NULL | — |
| `description` | TEXT | — | — |
| `sector` | TEXT | NOT NULL | — |
| `required_clearance` | INTEGER | NOT NULL | `1` |
| `is_restricted` | INTEGER | NOT NULL | `0` |
| `access_schedule` | TEXT | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes: none in `001_initial.sql`; `idx_zones_sector(sector, required_clearance)`, `idx_zones_restricted(is_restricted, required_clearance)` partial WHERE `is_restricted=1` (`002_performance_hardening.sql`).

Foreign keys: none. Triggers: none.

### `access_log`

Entry/exit events at a zone checkpoint.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `resident_id` | TEXT | NOT NULL, REFERENCES `residents(id)` | — |
| `zone_id` | TEXT | NOT NULL, REFERENCES `security_zones(id)` | — |
| `access_point` | TEXT | NOT NULL | — |
| `direction` | TEXT | NOT NULL, CHECK IN (`ENTRY`,`EXIT`) | — |
| `access_result` | TEXT | NOT NULL, CHECK IN (`GRANTED`,`DENIED`,`OVERRIDE`,`EMERGENCY`) | — |
| `denial_reason` | TEXT | — | — |
| `override_by` | TEXT | REFERENCES `residents(id)` | — |
| `timestamp` | TEXT | NOT NULL | `datetime('now')` |

Indexes: `idx_access_log_resident(resident_id)`, `idx_access_log_zone(zone_id)`, `idx_access_log_timestamp(timestamp)`, `idx_access_log_result(access_result)` (`001_initial.sql`). `002_performance_hardening.sql` drops `idx_access_log_resident` and `idx_access_log_zone` as redundant and replaces them with composites: `idx_access_resident_time(resident_id, timestamp DESC)`, `idx_access_zone_time(zone_id, timestamp DESC)`, `idx_access_denied(access_result, timestamp DESC)` partial WHERE `access_result IN ('DENIED','OVERRIDE','EMERGENCY')`. `idx_access_log_timestamp` and `idx_access_log_result` from `001` are never dropped and remain live alongside the new composites.

Foreign keys: `resident_id` → `residents(id)`, `zone_id` → `security_zones(id)`, `override_by` → `residents(id)`. Triggers: none.

### `security_incidents`

A logged incident (altercation, theft, vandalism, unauthorized access, contraband, insubordination, assault, other) with investigation/resolution state.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `incident_number` | TEXT | UNIQUE NOT NULL | — |
| `incident_type` | TEXT | NOT NULL, CHECK IN (`ALTERCATION`,`THEFT`,`VANDALISM`,`UNAUTHORIZED_ACCESS`,`CONTRABAND`,`INSUBORDINATION`,`ASSAULT`,`OTHER`) | — |
| `severity` | TEXT | NOT NULL, CHECK IN (`MINOR`,`MODERATE`,`MAJOR`,`CRITICAL`) | — |
| `description` | TEXT | NOT NULL | — |
| `location_sector` | TEXT | — | — |
| `location_detail` | TEXT | — | — |
| `reported_by` | TEXT | REFERENCES `residents(id)` | — |
| `involved_resident_ids` | TEXT | — | — |
| `witness_resident_ids` | TEXT | — | — |
| `responding_officer_ids` | TEXT | — | — |
| `status` | TEXT | NOT NULL, CHECK IN (`OPEN`,`INVESTIGATING`,`PENDING_REVIEW`,`RESOLVED`,`CLOSED`) | `'OPEN'` |
| `resolution` | TEXT | — | — |
| `disciplinary_action` | TEXT | — | — |
| `occurred_at` | TEXT | NOT NULL | — |
| `reported_at` | TEXT | NOT NULL | — |
| `resolved_at` | TEXT | — | — |
| `notes` | TEXT | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes (`001_initial.sql`): `idx_security_incidents_type(incident_type)`, `idx_security_incidents_status(status)`, `idx_security_incidents_occurred(occurred_at)`.

Indexes (`002_performance_hardening.sql`): `idx_incidents_open(status, severity, occurred_at DESC)` partial WHERE `status NOT IN ('RESOLVED','CLOSED')`, `idx_incidents_reporter(reported_by)` partial WHERE NOT NULL, `idx_incidents_location(location_sector, occurred_at DESC)`.

Foreign keys: `reported_by` → `residents(id)`. `involved_resident_ids`/`witness_resident_ids`/`responding_officer_ids` are freeform TEXT columns (JSON arrays of ids), not enforced foreign keys. Triggers: none.

## Governance

### `directives`

An Overseer/department-head/Vault-Tec-Central policy document with a lifecycle (draft → active → suspended/superseded/rescinded).

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `directive_number` | TEXT | UNIQUE NOT NULL | — |
| `directive_type` | TEXT | NOT NULL, CHECK IN (`POLICY`,`EMERGENCY`,`OPERATIONAL`,`PERSONNEL`,`RESOURCE`,`SECURITY`) | — |
| `title` | TEXT | NOT NULL | — |
| `summary` | TEXT | NOT NULL | — |
| `full_text` | TEXT | NOT NULL | — |
| `issued_by` | TEXT | NOT NULL, REFERENCES `residents(id)` | — |
| `authority_level` | TEXT | NOT NULL, CHECK IN (`OVERSEER`,`DEPARTMENT_HEAD`,`VAULT_TEC_CENTRAL`) | — |
| `affected_departments` | TEXT | — | — |
| `affected_clearance_levels` | TEXT | — | — |
| `status` | TEXT | NOT NULL, CHECK IN (`DRAFT`,`ACTIVE`,`SUSPENDED`,`SUPERSEDED`,`RESCINDED`) | `'DRAFT'` |
| `effective_date` | TEXT | — | — |
| `expiration_date` | TEXT | — | — |
| `supersedes_directive_id` | TEXT | REFERENCES `directives(id)` | — |
| `superseded_by_directive_id` | TEXT | REFERENCES `directives(id)` | — |
| `classification_level` | TEXT | NOT NULL, CHECK IN (`GENERAL`,`RESTRICTED`,`CONFIDENTIAL`,`OVERSEER_ONLY`) | `'GENERAL'` |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |
| `updated_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes (`001_initial.sql`): `idx_directives_type(directive_type)`, `idx_directives_status(status)`, `idx_directives_classification(classification_level)`.

Indexes (`002_performance_hardening.sql`): `idx_directives_active(status, effective_date, classification_level)` partial WHERE `status='ACTIVE'`, `idx_directives_author(issued_by, status)`, `idx_directives_expiring(expiration_date, status)` partial WHERE `expiration_date IS NOT NULL AND status='ACTIVE'`.

Foreign keys: `issued_by` → `residents(id)`, `supersedes_directive_id`/`superseded_by_directive_id` (self-referential). Triggers: none.

## Audit Log

### `audit_log`

Append-only record of create/update/delete/login/logout-style actions across the system.

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `timestamp` | TEXT | NOT NULL | `datetime('now')` |
| `actor_type` | TEXT | NOT NULL, CHECK IN (`USER`,`SYSTEM`,`SIMULATION`) | — |
| `actor_id` | TEXT | — | — |
| `action` | TEXT | NOT NULL | — |
| `entity_type` | TEXT | NOT NULL | — |
| `entity_id` | TEXT | NOT NULL | — |
| `old_values` | TEXT | — | — |
| `new_values` | TEXT | — | — |
| `session_id` | TEXT | — | — |
| `ip_address` | TEXT | — | — |
| `terminal_id` | TEXT | — | — |

Indexes (`001_initial.sql`): `idx_audit_log_timestamp(timestamp)`, `idx_audit_log_actor(actor_id)`, `idx_audit_log_entity(entity_type, entity_id)`.

`002_performance_hardening.sql` drops `idx_audit_log_entity` and replaces it with `idx_audit_entity_time(entity_type, entity_id, timestamp DESC)`; it also adds `idx_audit_actor_time(actor_id, timestamp DESC)` partial WHERE NOT NULL and `idx_audit_action_time(action, timestamp DESC)`. `idx_audit_log_timestamp` and `idx_audit_log_actor` from `001` are not dropped and remain live.

Foreign keys: none — `entity_type` stores a table name as free text rather than a typed reference, and `entity_id` is not constrained to any single table's primary key. Triggers: none (nothing in the schema or Go code prevents `UPDATE`/`DELETE` against this table; "immutable" is a convention, not an enforced property).

## Simulation State

### `simulation_events`

A queued or completed simulation event (used by the control-core engine's event log).

| Column | Type | Constraints | Default |
| --- | --- | --- | --- |
| `id` | TEXT | PRIMARY KEY | — |
| `event_type` | TEXT | NOT NULL | — |
| `scheduled_time` | TEXT | NOT NULL | — |
| `processed_at` | TEXT | — | — |
| `status` | TEXT | NOT NULL, CHECK IN (`PENDING`,`PROCESSING`,`COMPLETED`,`FAILED`,`CANCELLED`) | `'PENDING'` |
| `priority` | INTEGER | NOT NULL | `0` |
| `payload` | TEXT | — | — |
| `result` | TEXT | — | — |
| `error_message` | TEXT | — | — |
| `created_at` | TEXT | NOT NULL | `datetime('now')` |

Indexes (`001_initial.sql`): `idx_simulation_events_status(status)`, `idx_simulation_events_scheduled(scheduled_time)`.

Indexes (`002_performance_hardening.sql`): `idx_sim_queue(status, priority DESC, scheduled_time)` partial WHERE `status='PENDING'`, `idx_sim_type_time(event_type, scheduled_time DESC)`, `idx_sim_failed(status, scheduled_time)` partial WHERE `status='FAILED'`.

Foreign keys: none. Triggers: none.

The simulation engine writes rows here for completed events (`id`, `event_type`, `scheduled_time`, `processed_at`, `status='COMPLETED'`, `priority`, `payload`, `result`, `created_at`) and, alongside each, an `audit_log` row with `actor_type='SIMULATION'`, `actor_id='control-core'`, `terminal_id='SIM'` (`internal/simulation/events.go`).

## PRAGMAs

Every `database.Open` call runs eight pragmas, in this order (`internal/database/database.go`), each with the rationale given in the code's own comments:

| Pragma | Value | Rationale (from code comment) |
| --- | --- | --- |
| `journal_mode` | `WAL` | "WAL mode for power-loss resilience" |
| `synchronous` | `NORMAL` | "Synchronous NORMAL balances safety and performance" |
| `busy_timeout` | `5000` | "5 second busy timeout for concurrent access" |
| `foreign_keys` | `ON` | "Enable foreign key constraints" |
| `page_size` | `4096` | "Use 4KB page size (matches typical filesystem block size)" |
| `cache_size` | `-16000` (16MB) | "16MB cache for good performance" |
| `mmap_size` | `268435456` (256MB) | "Enable memory-mapped I/O for reads (256MB)" |
| `secure_delete` | `ON` | "Secure delete for sensitive data" |

Any pragma failure aborts `Open` with a wrapped error (`internal/database/database.go`).

The connection string also carries `_txlock=immediate&_timeout=5000&_fk=true` (`internal/database/database.go`), and the pool is capped to `SetMaxOpenConns(1)` / `SetMaxIdleConns(1)` — SQLite supports only one writer, so the process holds a single connection (`internal/database/database.go`). A separate in-memory test helper (`internal/database/testhelper.go`) opens `:memory:` and sets only `PRAGMA foreign_keys = ON` — no WAL, synchronous, or other pragma, since `:memory:` cannot use WAL.

> ⚠ See [#15](https://github.com/jstephenperry/vt-uos/issues/15) and [#46](https://github.com/jstephenperry/vt-uos/issues/46).

## Migration mechanism

Migrations are embedded at compile time via `//go:embed migrations/*.sql` (`internal/database/migrate.go`) and discovered by matching each embedded filename against `^(\d{3})_(.+)\.sql$`; a non-matching filename is skipped with a `slog.Warn` rather than failing the load (`internal/database/migrate.go`). Each file is split into an "up" and a "down" section on the literal markers `-- +migrate Up` / `-- +migrate Down` (`internal/database/migrate.go`); both shipped files use `Up` before `Down`.

Applied migrations are tracked in a table created directly by Go code (not by a migration file itself) the first time a `Migrator` is constructed:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    description TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (datetime('now')),
    checksum TEXT
)
```

(`internal/database/migrate.go`). The current schema version is `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`, defaulting to 0 on an empty table (`internal/database/migrate.go`). `checksum` is declared but never populated — `applyMigration`'s `INSERT INTO schema_migrations` supplies only `version` and `description` — and is never read or compared anywhere, so edited migration content between deployments would not be detected.

> ⚠ See [#44](https://github.com/jstephenperry/vt-uos/issues/44).

`MigrateUp` applies each pending migration (`version > current`) in its own call to `applyMigration`, and **each migration file is its own transaction** — a later migration failing leaves earlier ones in the batch already committed, it does not roll back the whole run (`internal/database/migrate.go`). Inside a transaction, the up-SQL is split into individual statements by a hand-rolled `splitStatements` that walks the SQL text byte-by-byte, toggling an in-string flag on `'`/`"` and splitting on top-level `;` (`internal/database/migrate.go`); it does not strip `--` line comments before splitting, and it treats a preceding backslash — not SQL's standard doubled-quote — as the string-escape character, so a future migration with a literal apostrophe or an inline comment containing a semicolon could be mis-split. After all statements in a migration run successfully, its `(version, description)` row is inserted into `schema_migrations` in the same transaction; if any statement fails, `WithTransaction` rolls the whole migration back and no row is recorded for it (`internal/database/database.go`, `internal/database/migrate.go`). `MigrateDown`, `MigrateTo`, `DryRun`, and `Status` are exported and implemented (rollback of the top applied version, migration to an arbitrary target, dry-run listing, and full status) but have no caller anywhere outside `migrate.go` itself and its tests — the CLI only ever calls `MigrateUp` (`cmd/vtuos/main.go`).

> ⚠ See [#25](https://github.com/jstephenperry/vt-uos/issues/25) and [#44](https://github.com/jstephenperry/vt-uos/issues/44).

## Recovery behavior

`AttemptRecovery(dbPath, backupDir)` runs once at startup, only when the database file already exists, in four phases (`internal/database/recovery.go`):

1. **Integrity check** — opens the file read-only (`?mode=ro`) and runs `PRAGMA integrity_check`, expecting a single row equal to `"ok"`. Success returns `RecoverySuccess` immediately.
2. **WAL recovery** — only attempted if a `-wal` sidecar file exists; opens the database normally, issues `PRAGMA wal_checkpoint(RESTART)`, then re-runs the integrity check. Success returns `RecoverySuccess` with `WALRecovered=true`.
3. **Backup restoration** — lists `backupDir`, keeps files ending in `.db`, sorts newest-first by modification time, and tries each in turn: runs the same integrity check against the candidate backup, moves the corrupted live file aside to `<path>.corrupted.<timestamp>` (best-effort; a failure here is only logged), removes any `-wal`/`-shm` sidecars (errors ignored), and copies the backup over the live path. The loop stops at the first backup that passes its own integrity check — it does not confirm the backup's schema version or vault identity, only that SQLite itself considers the file uncorrupted. Success returns `RecoveryFromBackup`.
4. **Failure** — if no phase above succeeds, returns `RecoveryFailed` with a generic `"all recovery attempts failed"` error; the per-phase detail is available on `RecoveryReport.Steps` but the CLI's caller does not inspect it (`cmd/vtuos/main.go`).
   > ⚠ See [#16](https://github.com/jstephenperry/vt-uos/issues/16) and [#17](https://github.com/jstephenperry/vt-uos/issues/17).

The file copy in phase 3 (`copyFile`, `internal/database/recovery.go`) explicitly calls `dstFile.Sync()` after `io.Copy` before returning, so a restored backup is fsynced to disk.

`DiagnoseDatabase(dbPath)` is a separate, read-only helper (not part of the `AttemptRecovery` flow) that reports file existence/size/mtime/permissions, WAL/SHM sidecar presence, and — if the file opens — SQLite version, schema version, user version, journal mode, page size/count, freelist count, and a `PRAGMA quick_check` result (`internal/database/recovery.go`).

## Snapshot mechanism

`Engine.CreateSnapshot(ctx)` (`internal/simulation/snapshot.go`) first calls `saveState` to flush the current vault clock and birth/death/tick counters into `vault_metadata` (warning-only if that fails), then calls `db.Backup(ctx)`. `Backup` (`internal/database/database.go`) checkpoints the WAL first (`PRAGMA wal_checkpoint(TRUNCATE)`, best-effort — a failure here only logs a warning), then runs `VACUUM INTO '<timestamped-path>'` against a filename built as `vault-<YYYYMMDD-HHMMSS>.db` under the configured backup directory. The backup path is interpolated into the SQL string with `fmt.Sprintf` rather than passed as a bound parameter; it is server-generated from a timestamp today, not user input. If `BackupRetentionDays > 0`, `Backup` spawns an untracked `go db.cleanOldBackups()` goroutine that deletes backup files older than the retention window — with no mechanism to wait for it or cap concurrent instances if backups run in quick succession.

A snapshot file produced this way is restored via the `-restore <path>` flag (`cmd/vtuos/main.go`), which copies it over the live database path (removing any `-wal`/`-shm` sidecars first) before `AttemptRecovery`, `database.Open`, or migrations run — an invalid or wrong-schema snapshot is not validated before installation; the subsequent integrity check and migration step are the only safety net.

> ⚠ See [#38](https://github.com/jstephenperry/vt-uos/issues/38).

## Data directory / default path

`database.path` defaults to `vault.db` (`internal/config/config.go`). `config.EnsureDataDir` resolves this at startup (`internal/config/loader.go`): an absolute path is used as-is (creating its parent directory); a relative path is placed under `$XDG_DATA_HOME/vtuos/` if set, else `~/.local/share/vtuos/`, and if `MkdirAll` on that directory fails, the function silently falls back to the bare relative path in the current working directory with no logged warning. `config.BackupDir` resolves the backup directory the same way but as a `backups` subdirectory next to the database (or `$XDG_DATA_HOME/vtuos/backups`, or a bare `backups` directory as a last resort), always calling `MkdirAll` and returning a hard error if that fails — unlike `EnsureDataDir`'s silent fallback (`internal/config/loader.go`).

## Seed data shape

`seed.Generator.Generate` (`internal/database/seed/generator.go`) runs entirely inside one `*sql.Tx`, seeding tables in this order, all via parameterized (`?`) inserts:

1. **`quarters`** — one row per unit across `QuartersSectors` (`A`,`B`,`C`,`D`) × `QuartersLevels` (5) × a fixed per-level mix of 5×`SINGLE` (capacity 1), 8×`DOUBLE` (capacity 2), 8×`FAMILY` (capacity 5), 2×`EXECUTIVE` (capacity 3), 2×`DORMITORY` (capacity 12) — 25 units per level, 500 quarters total for the default 4-sector/5-level configuration (`internal/database/seed/generator.go`, `internal/database/seed/names.go`).
2. **`vocations`** — one row per entry in `DepartmentVocations`: 36 vocations across 8 departments (`ENGINEERING` 6, `MEDICAL` 6, `SECURITY` 4, `FOOD_PRODUCTION` 5, `ADMINISTRATION` 5, `EDUCATION` 4, `SANITATION` 3, `RESEARCH` 3), each with `headcount_authorized`/`headcount_minimum` of 10/3 (5/2 for `ADMINISTRATION`, 1/1 for the single `ADM-OVSR-01` Overseer vocation) (`internal/database/seed/generator.go`, `internal/database/seed/names.go`).
3. **`households` + `residents`** — family households (default config target: 100) each insert one `households` row plus 2 adult `residents` rows and 0–4 child `residents` rows (uniformly random), followed by single-person households (default target: 80) each inserting one `households` row and one `residents` row, followed by additional single-person households generated one at a time until the resident count reaches `TargetPopulation` (default 500) (`internal/database/seed/generator.go`). `DefaultConfig` sets `RandomSeed: 2077`, so a given target/household configuration produces the same population deterministically (`internal/database/seed/generator.go`).
4. **`resource_categories`** — 8 rows (`FOOD`, `WATER`, `MEDICAL`, `POWER`, `PARTS`, `CLOTHING`, `TOOLS`, `CHEMICALS`) (`internal/database/seed/names.go`).
5. **`resource_items`** — 28 rows across those categories, each also generating exactly one **`resource_stocks`** row (lot `LOT-<item_code>-2077`) with an initial quantity derived from `TargetPopulation` — 90 days of food at 0.5 kg/person/day, 30 days of water at 3 L/person/day, 2 units/person for medical, 0.5 units/person for everything else (`internal/database/seed/generator.go`, `internal/database/seed/names.go`).

`main.runSeed` (`cmd/vtuos/main.go`) runs the generator only if `SELECT COUNT(*) FROM residents` succeeds and returns 0; if that query itself errors (e.g. the table doesn't exist), the error is ignored and seeding proceeds anyway rather than surfacing the underlying schema problem.
