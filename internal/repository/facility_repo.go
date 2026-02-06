package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/vtuos/vtuos/internal/models"
)

// FacilityRepository handles facility system data access.
type FacilityRepository struct {
	db *sql.DB
}

// NewFacilityRepository creates a new facility repository.
func NewFacilityRepository(db *sql.DB) *FacilityRepository {
	return &FacilityRepository{db: db}
}

// Create inserts a new facility system into the database.
func (r *FacilityRepository) Create(ctx context.Context, tx *sql.Tx, system *models.FacilitySystem) error {
	if err := system.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	query := `
		INSERT INTO facility_systems (
			id, system_code, name, category, location_sector, location_level,
			status, efficiency_percent, capacity_rating, capacity_unit, current_output,
			install_date, last_maintenance_date, next_maintenance_due,
			maintenance_interval_days, mtbf_hours, total_runtime_hours,
			notes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var execer interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}
	if tx != nil {
		execer = tx
	} else {
		execer = r.db
	}

	now := time.Now().UTC()
	system.CreatedAt = now
	system.UpdatedAt = now

	_, err := execer.ExecContext(ctx, query,
		system.ID,
		system.SystemCode,
		system.Name,
		string(system.Category),
		system.LocationSector,
		system.LocationLevel,
		string(system.Status),
		system.EfficiencyPercent,
		system.CapacityRating,
		nullableString(system.CapacityUnit),
		system.CurrentOutput,
		system.InstallDate.Format(time.DateOnly),
		nullableTimePtr(system.LastMaintenanceDate),
		nullableTimePtr(system.NextMaintenanceDue),
		system.MaintenanceIntervalDays,
		system.MTBFHours,
		system.TotalRuntimeHours,
		nullableString(system.Notes),
		system.CreatedAt.Format(time.RFC3339),
		system.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting facility system: %w", err)
	}

	return nil
}

// GetByID retrieves a facility system by ID.
func (r *FacilityRepository) GetByID(ctx context.Context, id string) (*models.FacilitySystem, error) {
	query := `
		SELECT id, system_code, name, category, location_sector, location_level,
			status, efficiency_percent, capacity_rating, capacity_unit, current_output,
			install_date, last_maintenance_date, next_maintenance_due,
			maintenance_interval_days, mtbf_hours, total_runtime_hours,
			notes, created_at, updated_at
		FROM facility_systems
		WHERE id = ?`

	return r.scanSystem(r.db.QueryRowContext(ctx, query, id))
}

// GetByCode retrieves a facility system by system code.
func (r *FacilityRepository) GetByCode(ctx context.Context, code string) (*models.FacilitySystem, error) {
	query := `
		SELECT id, system_code, name, category, location_sector, location_level,
			status, efficiency_percent, capacity_rating, capacity_unit, current_output,
			install_date, last_maintenance_date, next_maintenance_due,
			maintenance_interval_days, mtbf_hours, total_runtime_hours,
			notes, created_at, updated_at
		FROM facility_systems
		WHERE system_code = ?`

	return r.scanSystem(r.db.QueryRowContext(ctx, query, code))
}

// Update modifies an existing facility system.
func (r *FacilityRepository) Update(ctx context.Context, tx *sql.Tx, system *models.FacilitySystem) error {
	if err := system.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	query := `
		UPDATE facility_systems SET
			system_code = ?, name = ?, category = ?, location_sector = ?, location_level = ?,
			status = ?, efficiency_percent = ?, capacity_rating = ?, capacity_unit = ?,
			current_output = ?, install_date = ?, last_maintenance_date = ?,
			next_maintenance_due = ?, maintenance_interval_days = ?, mtbf_hours = ?,
			total_runtime_hours = ?, notes = ?, updated_at = ?
		WHERE id = ?`

	var execer interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}
	if tx != nil {
		execer = tx
	} else {
		execer = r.db
	}

	system.UpdatedAt = time.Now().UTC()

	result, err := execer.ExecContext(ctx, query,
		system.SystemCode,
		system.Name,
		string(system.Category),
		system.LocationSector,
		system.LocationLevel,
		string(system.Status),
		system.EfficiencyPercent,
		system.CapacityRating,
		nullableString(system.CapacityUnit),
		system.CurrentOutput,
		system.InstallDate.Format(time.DateOnly),
		nullableTimePtr(system.LastMaintenanceDate),
		nullableTimePtr(system.NextMaintenanceDue),
		system.MaintenanceIntervalDays,
		system.MTBFHours,
		system.TotalRuntimeHours,
		nullableString(system.Notes),
		system.UpdatedAt.Format(time.RFC3339),
		system.ID,
	)
	if err != nil {
		return fmt.Errorf("updating facility system: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("facility system not found: %s", system.ID)
	}

	return nil
}

// Delete removes a facility system from the database.
func (r *FacilityRepository) Delete(ctx context.Context, tx *sql.Tx, id string) error {
	query := `DELETE FROM facility_systems WHERE id = ?`

	var execer interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}
	if tx != nil {
		execer = tx
	} else {
		execer = r.db
	}

	result, err := execer.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting facility system: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("facility system not found: %s", id)
	}

	return nil
}

// List retrieves facility systems with filtering and pagination.
func (r *FacilityRepository) List(ctx context.Context, filter models.FacilitySystemFilter, page models.Pagination) (*models.FacilitySystemList, error) {
	var conditions []string
	var args []any

	if filter.Category != nil {
		conditions = append(conditions, "category = ?")
		args = append(args, string(*filter.Category))
	}
	if filter.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, string(*filter.Status))
	}
	if filter.Sector != "" {
		conditions = append(conditions, "location_sector = ?")
		args = append(args, filter.Sector)
	}
	if filter.SearchTerm != "" {
		conditions = append(conditions, "(name LIKE ? OR system_code LIKE ?)")
		pattern := "%" + filter.SearchTerm + "%"
		args = append(args, pattern, pattern)
	}
	if filter.OverdueOnly {
		conditions = append(conditions, "next_maintenance_due IS NOT NULL AND next_maintenance_due < datetime('now')")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM facility_systems %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting facility systems: %w", err)
	}

	// Get page
	query := fmt.Sprintf(`
		SELECT id, system_code, name, category, location_sector, location_level,
			status, efficiency_percent, capacity_rating, capacity_unit, current_output,
			install_date, last_maintenance_date, next_maintenance_due,
			maintenance_interval_days, mtbf_hours, total_runtime_hours,
			notes, created_at, updated_at
		FROM facility_systems
		%s
		ORDER BY category, name
		LIMIT ? OFFSET ?`, whereClause)

	args = append(args, page.Limit(), page.Offset())
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying facility systems: %w", err)
	}
	defer rows.Close()

	var systems []*models.FacilitySystem
	for rows.Next() {
		system, err := r.scanSystemRow(rows)
		if err != nil {
			return nil, err
		}
		systems = append(systems, system)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating facility systems: %w", err)
	}

	return &models.FacilitySystemList{
		Systems:    systems,
		Total:      total,
		Page:       page.Page,
		PageSize:   page.Limit(),
		TotalPages: page.TotalPages(total),
	}, nil
}

// CountByStatus returns counts of facility systems by status.
func (r *FacilityRepository) CountByStatus(ctx context.Context) (map[models.SystemStatus]int, error) {
	query := `SELECT status, COUNT(*) FROM facility_systems GROUP BY status`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("counting by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[models.SystemStatus]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scanning status count: %w", err)
		}
		counts[models.SystemStatus(status)] = count
	}

	return counts, rows.Err()
}

// GetAverageEfficiency returns the average efficiency of operational systems.
func (r *FacilityRepository) GetAverageEfficiency(ctx context.Context) (float64, error) {
	query := `SELECT COALESCE(AVG(efficiency_percent), 0) FROM facility_systems WHERE status IN ('OPERATIONAL', 'DEGRADED')`
	var avg float64
	if err := r.db.QueryRowContext(ctx, query).Scan(&avg); err != nil {
		return 0, fmt.Errorf("calculating average efficiency: %w", err)
	}
	return avg, nil
}

// GetOverdueCount returns the count of systems overdue for maintenance.
func (r *FacilityRepository) GetOverdueCount(ctx context.Context, asOf time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM facility_systems WHERE next_maintenance_due IS NOT NULL AND next_maintenance_due < ?`
	var count int
	if err := r.db.QueryRowContext(ctx, query, asOf.Format(time.RFC3339)).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting overdue systems: %w", err)
	}
	return count, nil
}

// CreateMaintenanceRecord inserts a new maintenance record.
func (r *FacilityRepository) CreateMaintenanceRecord(ctx context.Context, tx *sql.Tx, record *models.MaintenanceRecord) error {
	if err := record.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	query := `
		INSERT INTO maintenance_records (
			id, system_id, maintenance_type, description, work_performed, parts_consumed,
			lead_technician_id, crew_member_ids, scheduled_date, started_at, completed_at,
			estimated_hours, actual_hours, outcome, system_status_before, system_status_after,
			efficiency_before, efficiency_after, notes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var execer interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}
	if tx != nil {
		execer = tx
	} else {
		execer = r.db
	}

	now := time.Now().UTC()
	record.CreatedAt = now
	record.UpdatedAt = now

	var outcomeStr sql.NullString
	if record.Outcome != nil {
		outcomeStr = sql.NullString{String: string(*record.Outcome), Valid: true}
	}

	_, err := execer.ExecContext(ctx, query,
		record.ID,
		record.SystemID,
		string(record.MaintenanceType),
		record.Description,
		nullableString(record.WorkPerformed),
		nullableString(record.PartsConsumed),
		record.LeadTechnicianID,
		nullableString(record.CrewMemberIDs),
		nullableTimePtr(record.ScheduledDate),
		nullableTimePtr(record.StartedAt),
		nullableTimePtr(record.CompletedAt),
		record.EstimatedHours,
		record.ActualHours,
		outcomeStr,
		nullableString(record.SystemStatusBefore),
		nullableString(record.SystemStatusAfter),
		record.EfficiencyBefore,
		record.EfficiencyAfter,
		nullableString(record.Notes),
		record.CreatedAt.Format(time.RFC3339),
		record.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting maintenance record: %w", err)
	}

	return nil
}

// ListMaintenanceRecords retrieves maintenance records with filtering and pagination.
func (r *FacilityRepository) ListMaintenanceRecords(ctx context.Context, filter models.MaintenanceRecordFilter, page models.Pagination) (*models.MaintenanceRecordList, error) {
	var conditions []string
	var args []any

	if filter.SystemID != nil {
		conditions = append(conditions, "system_id = ?")
		args = append(args, *filter.SystemID)
	}
	if filter.MaintenanceType != nil {
		conditions = append(conditions, "maintenance_type = ?")
		args = append(args, string(*filter.MaintenanceType))
	}
	if filter.Outcome != nil {
		conditions = append(conditions, "outcome = ?")
		args = append(args, string(*filter.Outcome))
	}
	if filter.SearchTerm != "" {
		conditions = append(conditions, "(description LIKE ? OR work_performed LIKE ?)")
		pattern := "%" + filter.SearchTerm + "%"
		args = append(args, pattern, pattern)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM maintenance_records %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting maintenance records: %w", err)
	}

	// Get page
	query := fmt.Sprintf(`
		SELECT id, system_id, maintenance_type, description, work_performed, parts_consumed,
			lead_technician_id, crew_member_ids, scheduled_date, started_at, completed_at,
			estimated_hours, actual_hours, outcome, system_status_before, system_status_after,
			efficiency_before, efficiency_after, notes, created_at, updated_at
		FROM maintenance_records
		%s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, whereClause)

	args = append(args, page.Limit(), page.Offset())
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying maintenance records: %w", err)
	}
	defer rows.Close()

	var records []*models.MaintenanceRecord
	for rows.Next() {
		record, err := r.scanMaintenanceRecordRow(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating maintenance records: %w", err)
	}

	return &models.MaintenanceRecordList{
		Records:    records,
		Total:      total,
		Page:       page.Page,
		PageSize:   page.Limit(),
		TotalPages: page.TotalPages(total),
	}, nil
}

// scanSystem scans a single row into a FacilitySystem struct.
func (r *FacilityRepository) scanSystem(row *sql.Row) (*models.FacilitySystem, error) {
	var system models.FacilitySystem
	var installDateStr, createdStr, updatedStr string
	var lastMaintStr, nextMaintStr sql.NullString
	var capacityRating, currentOutput sql.NullFloat64
	var capacityUnit, notes sql.NullString
	var mtbfHours sql.NullInt64

	err := row.Scan(
		&system.ID,
		&system.SystemCode,
		&system.Name,
		&system.Category,
		&system.LocationSector,
		&system.LocationLevel,
		&system.Status,
		&system.EfficiencyPercent,
		&capacityRating,
		&capacityUnit,
		&currentOutput,
		&installDateStr,
		&lastMaintStr,
		&nextMaintStr,
		&system.MaintenanceIntervalDays,
		&mtbfHours,
		&system.TotalRuntimeHours,
		&notes,
		&createdStr,
		&updatedStr,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("facility system not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scanning facility system: %w", err)
	}

	system.InstallDate, _ = time.Parse(time.DateOnly, installDateStr)
	system.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	system.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)

	if lastMaintStr.Valid {
		t, _ := time.Parse(time.DateOnly, lastMaintStr.String)
		system.LastMaintenanceDate = &t
	}
	if nextMaintStr.Valid {
		t, _ := time.Parse(time.DateOnly, nextMaintStr.String)
		system.NextMaintenanceDue = &t
	}
	if capacityRating.Valid {
		system.CapacityRating = &capacityRating.Float64
	}
	if capacityUnit.Valid {
		system.CapacityUnit = capacityUnit.String
	}
	if currentOutput.Valid {
		system.CurrentOutput = &currentOutput.Float64
	}
	if notes.Valid {
		system.Notes = notes.String
	}
	if mtbfHours.Valid {
		v := int(mtbfHours.Int64)
		system.MTBFHours = &v
	}

	return &system, nil
}

// scanSystemRow scans a row from a rows iterator.
func (r *FacilityRepository) scanSystemRow(rows *sql.Rows) (*models.FacilitySystem, error) {
	var system models.FacilitySystem
	var installDateStr, createdStr, updatedStr string
	var lastMaintStr, nextMaintStr sql.NullString
	var capacityRating, currentOutput sql.NullFloat64
	var capacityUnit, notes sql.NullString
	var mtbfHours sql.NullInt64

	err := rows.Scan(
		&system.ID,
		&system.SystemCode,
		&system.Name,
		&system.Category,
		&system.LocationSector,
		&system.LocationLevel,
		&system.Status,
		&system.EfficiencyPercent,
		&capacityRating,
		&capacityUnit,
		&currentOutput,
		&installDateStr,
		&lastMaintStr,
		&nextMaintStr,
		&system.MaintenanceIntervalDays,
		&mtbfHours,
		&system.TotalRuntimeHours,
		&notes,
		&createdStr,
		&updatedStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning facility system row: %w", err)
	}

	system.InstallDate, _ = time.Parse(time.DateOnly, installDateStr)
	system.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	system.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)

	if lastMaintStr.Valid {
		t, _ := time.Parse(time.DateOnly, lastMaintStr.String)
		system.LastMaintenanceDate = &t
	}
	if nextMaintStr.Valid {
		t, _ := time.Parse(time.DateOnly, nextMaintStr.String)
		system.NextMaintenanceDue = &t
	}
	if capacityRating.Valid {
		system.CapacityRating = &capacityRating.Float64
	}
	if capacityUnit.Valid {
		system.CapacityUnit = capacityUnit.String
	}
	if currentOutput.Valid {
		system.CurrentOutput = &currentOutput.Float64
	}
	if notes.Valid {
		system.Notes = notes.String
	}
	if mtbfHours.Valid {
		v := int(mtbfHours.Int64)
		system.MTBFHours = &v
	}

	return &system, nil
}

// scanMaintenanceRecordRow scans a maintenance record row.
func (r *FacilityRepository) scanMaintenanceRecordRow(rows *sql.Rows) (*models.MaintenanceRecord, error) {
	var record models.MaintenanceRecord
	var workPerformed, partsConsumed, crewMemberIDs sql.NullString
	var leadTechID sql.NullString
	var scheduledDate, startedAt, completedAt sql.NullString
	var estimatedHours, actualHours sql.NullFloat64
	var outcome, statusBefore, statusAfter, notes sql.NullString
	var efficiencyBefore, efficiencyAfter sql.NullFloat64
	var createdStr, updatedStr string

	err := rows.Scan(
		&record.ID,
		&record.SystemID,
		&record.MaintenanceType,
		&record.Description,
		&workPerformed,
		&partsConsumed,
		&leadTechID,
		&crewMemberIDs,
		&scheduledDate,
		&startedAt,
		&completedAt,
		&estimatedHours,
		&actualHours,
		&outcome,
		&statusBefore,
		&statusAfter,
		&efficiencyBefore,
		&efficiencyAfter,
		&notes,
		&createdStr,
		&updatedStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning maintenance record row: %w", err)
	}

	record.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	record.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)

	if workPerformed.Valid {
		record.WorkPerformed = workPerformed.String
	}
	if partsConsumed.Valid {
		record.PartsConsumed = partsConsumed.String
	}
	if leadTechID.Valid {
		record.LeadTechnicianID = &leadTechID.String
	}
	if crewMemberIDs.Valid {
		record.CrewMemberIDs = crewMemberIDs.String
	}
	if scheduledDate.Valid {
		t, _ := time.Parse(time.DateOnly, scheduledDate.String)
		record.ScheduledDate = &t
	}
	if startedAt.Valid {
		t, _ := time.Parse(time.RFC3339, startedAt.String)
		record.StartedAt = &t
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		record.CompletedAt = &t
	}
	if estimatedHours.Valid {
		record.EstimatedHours = &estimatedHours.Float64
	}
	if actualHours.Valid {
		record.ActualHours = &actualHours.Float64
	}
	if outcome.Valid {
		o := models.MaintenanceOutcome(outcome.String)
		record.Outcome = &o
	}
	if statusBefore.Valid {
		record.SystemStatusBefore = statusBefore.String
	}
	if statusAfter.Valid {
		record.SystemStatusAfter = statusAfter.String
	}
	if efficiencyBefore.Valid {
		record.EfficiencyBefore = &efficiencyBefore.Float64
	}
	if efficiencyAfter.Valid {
		record.EfficiencyAfter = &efficiencyAfter.Float64
	}
	if notes.Valid {
		record.Notes = notes.String
	}

	return &record, nil
}
