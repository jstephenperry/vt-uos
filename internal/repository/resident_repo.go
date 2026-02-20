// Package repository provides data access layer for VT-UOS entities.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/vtuos/vtuos/internal/models"
)

// ResidentRepository handles resident data access.
type ResidentRepository struct {
	db *sql.DB
}

// NewResidentRepository creates a new resident repository.
func NewResidentRepository(db *sql.DB) *ResidentRepository {
	return &ResidentRepository{db: db}
}

// Create inserts a new resident into the database.
func (r *ResidentRepository) Create(ctx context.Context, tx *sql.Tx, resident *models.Resident) error {
	if err := resident.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	query := `
		INSERT INTO residents (
			id, registry_number, surname, given_names, date_of_birth, date_of_death,
			sex, blood_type, entry_type, entry_date, status,
			biological_parent_1_id, biological_parent_2_id,
			household_id, quarters_id, primary_vocation_id, clearance_level,
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
	resident.CreatedAt = now
	resident.UpdatedAt = now

	_, err := execer.ExecContext(ctx, query,
		resident.ID,
		resident.RegistryNumber,
		resident.Surname,
		resident.GivenNames,
		resident.DateOfBirth.Format(time.DateOnly),
		nullableTime(resident.DateOfDeath),
		string(resident.Sex),
		nullableString(string(resident.BloodType)),
		string(resident.EntryType),
		resident.EntryDate.Format(time.RFC3339),
		string(resident.Status),
		resident.BiologicalParent1ID,
		resident.BiologicalParent2ID,
		resident.HouseholdID,
		resident.QuartersID,
		resident.PrimaryVocationID,
		resident.ClearanceLevel,
		nullableString(resident.Notes),
		resident.CreatedAt.Format(time.RFC3339),
		resident.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting resident: %w", err)
	}

	return nil
}

// GetByID retrieves a resident by ID.
func (r *ResidentRepository) GetByID(ctx context.Context, id string) (*models.Resident, error) {
	query := `
		SELECT id, registry_number, surname, given_names, date_of_birth, date_of_death,
			sex, blood_type, entry_type, entry_date, status,
			biological_parent_1_id, biological_parent_2_id,
			household_id, quarters_id, primary_vocation_id, clearance_level,
			notes, created_at, updated_at
		FROM residents
		WHERE id = ?`

	return r.scanResident(r.db.QueryRowContext(ctx, query, id))
}

// GetByRegistryNumber retrieves a resident by registry number.
func (r *ResidentRepository) GetByRegistryNumber(ctx context.Context, regNum string) (*models.Resident, error) {
	query := `
		SELECT id, registry_number, surname, given_names, date_of_birth, date_of_death,
			sex, blood_type, entry_type, entry_date, status,
			biological_parent_1_id, biological_parent_2_id,
			household_id, quarters_id, primary_vocation_id, clearance_level,
			notes, created_at, updated_at
		FROM residents
		WHERE registry_number = ?`

	return r.scanResident(r.db.QueryRowContext(ctx, query, regNum))
}

// Update modifies an existing resident.
func (r *ResidentRepository) Update(ctx context.Context, tx *sql.Tx, resident *models.Resident) error {
	if err := resident.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	query := `
		UPDATE residents SET
			surname = ?, given_names = ?, date_of_birth = ?, date_of_death = ?,
			sex = ?, blood_type = ?, entry_type = ?, entry_date = ?, status = ?,
			biological_parent_1_id = ?, biological_parent_2_id = ?,
			household_id = ?, quarters_id = ?, primary_vocation_id = ?, clearance_level = ?,
			notes = ?, updated_at = ?
		WHERE id = ?`

	var execer interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}
	if tx != nil {
		execer = tx
	} else {
		execer = r.db
	}

	resident.UpdatedAt = time.Now().UTC()

	result, err := execer.ExecContext(ctx, query,
		resident.Surname,
		resident.GivenNames,
		resident.DateOfBirth.Format(time.DateOnly),
		nullableTime(resident.DateOfDeath),
		string(resident.Sex),
		nullableString(string(resident.BloodType)),
		string(resident.EntryType),
		resident.EntryDate.Format(time.RFC3339),
		string(resident.Status),
		resident.BiologicalParent1ID,
		resident.BiologicalParent2ID,
		resident.HouseholdID,
		resident.QuartersID,
		resident.PrimaryVocationID,
		resident.ClearanceLevel,
		nullableString(resident.Notes),
		resident.UpdatedAt.Format(time.RFC3339),
		resident.ID,
	)
	if err != nil {
		return fmt.Errorf("updating resident: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("resident not found: %s", resident.ID)
	}

	return nil
}

// List retrieves residents with filtering and pagination.
func (r *ResidentRepository) List(ctx context.Context, filter models.ResidentFilter, page models.Pagination) (*models.ResidentList, error) {
	var conditions []string
	var args []any

	if filter.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, string(*filter.Status))
	}
	if filter.HouseholdID != nil {
		conditions = append(conditions, "household_id = ?")
		args = append(args, *filter.HouseholdID)
	}
	if filter.VocationID != nil {
		conditions = append(conditions, "primary_vocation_id = ?")
		args = append(args, *filter.VocationID)
	}
	if filter.Sex != nil {
		conditions = append(conditions, "sex = ?")
		args = append(args, string(*filter.Sex))
	}
	if filter.EntryType != nil {
		conditions = append(conditions, "entry_type = ?")
		args = append(args, string(*filter.EntryType))
	}
	if filter.SearchTerm != "" {
		conditions = append(conditions, "(surname LIKE ? OR given_names LIKE ?)")
		searchPattern := "%" + filter.SearchTerm + "%"
		args = append(args, searchPattern, searchPattern)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM residents %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting residents: %w", err)
	}

	// Get page
	query := fmt.Sprintf(`
		SELECT id, registry_number, surname, given_names, date_of_birth, date_of_death,
			sex, blood_type, entry_type, entry_date, status,
			biological_parent_1_id, biological_parent_2_id,
			household_id, quarters_id, primary_vocation_id, clearance_level,
			notes, created_at, updated_at
		FROM residents
		%s
		ORDER BY surname, given_names
		LIMIT ? OFFSET ?`, whereClause)

	args = append(args, page.Limit(), page.Offset())
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying residents: %w", err)
	}
	defer rows.Close()

	var residents []*models.Resident
	for rows.Next() {
		resident, err := r.scanResidentRow(rows)
		if err != nil {
			return nil, err
		}
		residents = append(residents, resident)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating residents: %w", err)
	}

	return &models.ResidentList{
		Residents:  residents,
		Total:      total,
		Page:       page.Page,
		PageSize:   page.Limit(),
		TotalPages: page.TotalPages(total),
	}, nil
}

// Delete removes a resident from the database.
func (r *ResidentRepository) Delete(ctx context.Context, tx *sql.Tx, id string) error {
	query := `DELETE FROM residents WHERE id = ?`

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
		return fmt.Errorf("deleting resident: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("resident not found: %s", id)
	}

	return nil
}

// GetNextRegistryNumber generates the next available registry number.
func (r *ResidentRepository) GetNextRegistryNumber(ctx context.Context, vaultNumber int) (string, error) {
	query := `
		SELECT registry_number FROM residents
		ORDER BY registry_number DESC
		LIMIT 1`

	var lastNum string
	err := r.db.QueryRowContext(ctx, query).Scan(&lastNum)
	if err == sql.ErrNoRows {
		return fmt.Sprintf("V%03d-00001", vaultNumber), nil
	}
	if err != nil {
		return "", fmt.Errorf("getting last registry number: %w", err)
	}

	// Parse the number portion and increment
	var num int
	_, err = fmt.Sscanf(lastNum, fmt.Sprintf("V%03d-%%05d", vaultNumber), &num)
	if err != nil {
		// Fallback to sequential scan
		countQuery := `SELECT COUNT(*) FROM residents`
		var count int
		if err := r.db.QueryRowContext(ctx, countQuery).Scan(&count); err != nil {
			return "", fmt.Errorf("counting residents: %w", err)
		}
		return fmt.Sprintf("V%03d-%05d", vaultNumber, count+1), nil
	}

	return fmt.Sprintf("V%03d-%05d", vaultNumber, num+1), nil
}

// GetByHousehold retrieves all residents in a household.
func (r *ResidentRepository) GetByHousehold(ctx context.Context, householdID string) ([]*models.Resident, error) {
	query := `
		SELECT id, registry_number, surname, given_names, date_of_birth, date_of_death,
			sex, blood_type, entry_type, entry_date, status,
			biological_parent_1_id, biological_parent_2_id,
			household_id, quarters_id, primary_vocation_id, clearance_level,
			notes, created_at, updated_at
		FROM residents
		WHERE household_id = ?
		ORDER BY date_of_birth`

	rows, err := r.db.QueryContext(ctx, query, householdID)
	if err != nil {
		return nil, fmt.Errorf("querying household members: %w", err)
	}
	defer rows.Close()

	var residents []*models.Resident
	for rows.Next() {
		resident, err := r.scanResidentRow(rows)
		if err != nil {
			return nil, err
		}
		residents = append(residents, resident)
	}

	return residents, rows.Err()
}

// GetChildren retrieves biological children of a resident.
func (r *ResidentRepository) GetChildren(ctx context.Context, parentID string) ([]*models.Resident, error) {
	query := `
		SELECT id, registry_number, surname, given_names, date_of_birth, date_of_death,
			sex, blood_type, entry_type, entry_date, status,
			biological_parent_1_id, biological_parent_2_id,
			household_id, quarters_id, primary_vocation_id, clearance_level,
			notes, created_at, updated_at
		FROM residents
		WHERE biological_parent_1_id = ? OR biological_parent_2_id = ?
		ORDER BY date_of_birth`

	rows, err := r.db.QueryContext(ctx, query, parentID, parentID)
	if err != nil {
		return nil, fmt.Errorf("querying children: %w", err)
	}
	defer rows.Close()

	var children []*models.Resident
	for rows.Next() {
		child, err := r.scanResidentRow(rows)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}

	return children, rows.Err()
}

// GetParents retrieves biological parents of a resident.
func (r *ResidentRepository) GetParents(ctx context.Context, residentID string) ([]*models.Resident, error) {
	// First get the resident to find parent IDs
	resident, err := r.GetByID(ctx, residentID)
	if err != nil {
		return nil, err
	}

	var parents []*models.Resident
	if resident.BiologicalParent1ID != nil {
		parent1, err := r.GetByID(ctx, *resident.BiologicalParent1ID)
		if err == nil {
			parents = append(parents, parent1)
		}
	}
	if resident.BiologicalParent2ID != nil {
		parent2, err := r.GetByID(ctx, *resident.BiologicalParent2ID)
		if err == nil {
			parents = append(parents, parent2)
		}
	}

	return parents, nil
}

// CountByStatus returns counts of residents by status.
func (r *ResidentRepository) CountByStatus(ctx context.Context) (map[models.ResidentStatus]int, error) {
	query := `SELECT status, COUNT(*) FROM residents GROUP BY status`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("counting by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[models.ResidentStatus]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scanning status count: %w", err)
		}
		counts[models.ResidentStatus(status)] = count
	}

	return counts, rows.Err()
}

// scanResident scans a single row into a Resident struct.
func (r *ResidentRepository) scanResident(row *sql.Row) (*models.Resident, error) {
	var resident models.Resident
	var dobStr, entryDateStr, createdStr, updatedStr string
	var dodStr, bloodType, notes sql.NullString
	var parent1ID, parent2ID, householdID, quartersID, vocationID sql.NullString

	err := row.Scan(
		&resident.ID,
		&resident.RegistryNumber,
		&resident.Surname,
		&resident.GivenNames,
		&dobStr,
		&dodStr,
		&resident.Sex,
		&bloodType,
		&resident.EntryType,
		&entryDateStr,
		&resident.Status,
		&parent1ID,
		&parent2ID,
		&householdID,
		&quartersID,
		&vocationID,
		&resident.ClearanceLevel,
		&notes,
		&createdStr,
		&updatedStr,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("resident not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scanning resident: %w", err)
	}

	// Parse dates
	resident.DateOfBirth, _ = time.Parse(time.DateOnly, dobStr)
	if dodStr.Valid {
		dod, _ := time.Parse(time.DateOnly, dodStr.String)
		resident.DateOfDeath = &dod
	}
	resident.EntryDate, _ = time.Parse(time.RFC3339, entryDateStr)
	resident.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	resident.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)

	// Set nullable fields
	if bloodType.Valid {
		resident.BloodType = models.BloodType(bloodType.String)
	}
	if notes.Valid {
		resident.Notes = notes.String
	}
	if parent1ID.Valid {
		resident.BiologicalParent1ID = &parent1ID.String
	}
	if parent2ID.Valid {
		resident.BiologicalParent2ID = &parent2ID.String
	}
	if householdID.Valid {
		resident.HouseholdID = &householdID.String
	}
	if quartersID.Valid {
		resident.QuartersID = &quartersID.String
	}
	if vocationID.Valid {
		resident.PrimaryVocationID = &vocationID.String
	}

	return &resident, nil
}

// scanResidentRow scans a row from a rows iterator.
func (r *ResidentRepository) scanResidentRow(rows *sql.Rows) (*models.Resident, error) {
	var resident models.Resident
	var dobStr, entryDateStr, createdStr, updatedStr string
	var dodStr, bloodType, notes sql.NullString
	var parent1ID, parent2ID, householdID, quartersID, vocationID sql.NullString

	err := rows.Scan(
		&resident.ID,
		&resident.RegistryNumber,
		&resident.Surname,
		&resident.GivenNames,
		&dobStr,
		&dodStr,
		&resident.Sex,
		&bloodType,
		&resident.EntryType,
		&entryDateStr,
		&resident.Status,
		&parent1ID,
		&parent2ID,
		&householdID,
		&quartersID,
		&vocationID,
		&resident.ClearanceLevel,
		&notes,
		&createdStr,
		&updatedStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning resident row: %w", err)
	}

	// Parse dates
	resident.DateOfBirth, _ = time.Parse(time.DateOnly, dobStr)
	if dodStr.Valid {
		dod, _ := time.Parse(time.DateOnly, dodStr.String)
		resident.DateOfDeath = &dod
	}
	resident.EntryDate, _ = time.Parse(time.RFC3339, entryDateStr)
	resident.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	resident.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)

	// Set nullable fields
	if bloodType.Valid {
		resident.BloodType = models.BloodType(bloodType.String)
	}
	if notes.Valid {
		resident.Notes = notes.String
	}
	if parent1ID.Valid {
		resident.BiologicalParent1ID = &parent1ID.String
	}
	if parent2ID.Valid {
		resident.BiologicalParent2ID = &parent2ID.String
	}
	if householdID.Valid {
		resident.HouseholdID = &householdID.String
	}
	if quartersID.Valid {
		resident.QuartersID = &quartersID.String
	}
	if vocationID.Valid {
		resident.PrimaryVocationID = &vocationID.String
	}

	return &resident, nil
}

// Helper functions for nullable values
func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullableTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.Format(time.DateOnly), Valid: true}
}
