package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/vtuos/vtuos/internal/models"
)

// HouseholdRepository handles household data access.
type HouseholdRepository struct {
	db *sql.DB
}

// NewHouseholdRepository creates a new household repository.
func NewHouseholdRepository(db *sql.DB) *HouseholdRepository {
	return &HouseholdRepository{db: db}
}

// Create inserts a new household into the database.
func (r *HouseholdRepository) Create(ctx context.Context, tx *sql.Tx, household *models.Household) error {
	if err := household.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	query := `
		INSERT INTO households (
			id, designation, household_type, head_of_household_id, quarters_id,
			ration_class, status, formed_date, dissolved_date, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var execer interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}
	if tx != nil {
		execer = tx
	} else {
		execer = r.db
	}

	now := time.Now().UTC()
	household.CreatedAt = now
	household.UpdatedAt = now

	_, err := execer.ExecContext(ctx, query,
		household.ID,
		household.Designation,
		string(household.HouseholdType),
		household.HeadOfHouseholdID,
		household.QuartersID,
		string(household.RationClass),
		string(household.Status),
		household.FormedDate.Format(time.DateOnly),
		nullableTimePtr(household.DissolvedDate),
		household.CreatedAt.Format(time.RFC3339),
		household.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting household: %w", err)
	}

	return nil
}

// GetByID retrieves a household by ID.
func (r *HouseholdRepository) GetByID(ctx context.Context, id string) (*models.Household, error) {
	query := `
		SELECT id, designation, household_type, head_of_household_id, quarters_id,
			ration_class, status, formed_date, dissolved_date, created_at, updated_at
		FROM households
		WHERE id = ?`

	household, err := r.scanHousehold(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return nil, err
	}

	// Get member count
	countQuery := `SELECT COUNT(*) FROM residents WHERE household_id = ? AND status = 'ACTIVE'`
	if err := r.db.QueryRowContext(ctx, countQuery, id).Scan(&household.MemberCount); err != nil {
		household.MemberCount = 0
	}

	return household, nil
}

// GetByDesignation retrieves a household by designation.
func (r *HouseholdRepository) GetByDesignation(ctx context.Context, designation string) (*models.Household, error) {
	query := `
		SELECT id, designation, household_type, head_of_household_id, quarters_id,
			ration_class, status, formed_date, dissolved_date, created_at, updated_at
		FROM households
		WHERE designation = ?`

	return r.scanHousehold(r.db.QueryRowContext(ctx, query, designation))
}

// Update modifies an existing household.
func (r *HouseholdRepository) Update(ctx context.Context, tx *sql.Tx, household *models.Household) error {
	if err := household.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	query := `
		UPDATE households SET
			designation = ?, household_type = ?, head_of_household_id = ?, quarters_id = ?,
			ration_class = ?, status = ?, formed_date = ?, dissolved_date = ?, updated_at = ?
		WHERE id = ?`

	var execer interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}
	if tx != nil {
		execer = tx
	} else {
		execer = r.db
	}

	household.UpdatedAt = time.Now().UTC()

	result, err := execer.ExecContext(ctx, query,
		household.Designation,
		string(household.HouseholdType),
		household.HeadOfHouseholdID,
		household.QuartersID,
		string(household.RationClass),
		string(household.Status),
		household.FormedDate.Format(time.DateOnly),
		nullableTimePtr(household.DissolvedDate),
		household.UpdatedAt.Format(time.RFC3339),
		household.ID,
	)
	if err != nil {
		return fmt.Errorf("updating household: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("household not found: %s", household.ID)
	}

	return nil
}

// List retrieves households with filtering and pagination.
func (r *HouseholdRepository) List(ctx context.Context, filter models.HouseholdFilter, page models.Pagination) (*models.HouseholdList, error) {
	var conditions []string
	var args []any

	if filter.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, string(*filter.Status))
	}
	if filter.HouseholdType != nil {
		conditions = append(conditions, "household_type = ?")
		args = append(args, string(*filter.HouseholdType))
	}
	if filter.RationClass != nil {
		conditions = append(conditions, "ration_class = ?")
		args = append(args, string(*filter.RationClass))
	}
	if filter.HasQuarters != nil {
		if *filter.HasQuarters {
			conditions = append(conditions, "quarters_id IS NOT NULL")
		} else {
			conditions = append(conditions, "quarters_id IS NULL")
		}
	}
	if filter.SearchTerm != "" {
		conditions = append(conditions, "designation LIKE ?")
		args = append(args, "%"+filter.SearchTerm+"%")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM households %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting households: %w", err)
	}

	// Get page with member counts
	query := fmt.Sprintf(`
		SELECT h.id, h.designation, h.household_type, h.head_of_household_id, h.quarters_id,
			h.ration_class, h.status, h.formed_date, h.dissolved_date, h.created_at, h.updated_at,
			(SELECT COUNT(*) FROM residents r WHERE r.household_id = h.id AND r.status = 'ACTIVE') as member_count
		FROM households h
		%s
		ORDER BY h.designation
		LIMIT ? OFFSET ?`, whereClause)

	args = append(args, page.Limit(), page.Offset())
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying households: %w", err)
	}
	defer rows.Close()

	var households []*models.Household
	for rows.Next() {
		household, err := r.scanHouseholdRowWithCount(rows)
		if err != nil {
			return nil, err
		}
		households = append(households, household)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating households: %w", err)
	}

	return &models.HouseholdList{
		Households: households,
		Total:      total,
		Page:       page.Page,
		PageSize:   page.Limit(),
		TotalPages: page.TotalPages(total),
	}, nil
}

// Delete removes a household from the database.
func (r *HouseholdRepository) Delete(ctx context.Context, tx *sql.Tx, id string) error {
	query := `DELETE FROM households WHERE id = ?`

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
		return fmt.Errorf("deleting household: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("household not found: %s", id)
	}

	return nil
}

// GetMemberCount returns the number of active residents in a household.
func (r *HouseholdRepository) GetMemberCount(ctx context.Context, householdID string) (int, error) {
	query := `SELECT COUNT(*) FROM residents WHERE household_id = ? AND status = 'ACTIVE'`
	var count int
	if err := r.db.QueryRowContext(ctx, query, householdID).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting household members: %w", err)
	}
	return count, nil
}

// GetNextDesignation generates the next available household designation.
func (r *HouseholdRepository) GetNextDesignation(ctx context.Context) (string, error) {
	query := `
		SELECT designation FROM households
		ORDER BY designation DESC
		LIMIT 1`

	var lastDesig string
	err := r.db.QueryRowContext(ctx, query).Scan(&lastDesig)
	if err == sql.ErrNoRows {
		return "H-0001", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting last designation: %w", err)
	}

	// Parse the number portion and increment
	var num int
	_, err = fmt.Sscanf(lastDesig, "H-%d", &num)
	if err != nil {
		// Fallback to count
		countQuery := `SELECT COUNT(*) FROM households`
		var count int
		if err := r.db.QueryRowContext(ctx, countQuery).Scan(&count); err != nil {
			return "", fmt.Errorf("counting households: %w", err)
		}
		return fmt.Sprintf("H-%04d", count+1), nil
	}

	return fmt.Sprintf("H-%04d", num+1), nil
}

// CountByStatus returns counts of households by status.
func (r *HouseholdRepository) CountByStatus(ctx context.Context) (map[models.HouseholdStatus]int, error) {
	query := `SELECT status, COUNT(*) FROM households GROUP BY status`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("counting by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[models.HouseholdStatus]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scanning status count: %w", err)
		}
		counts[models.HouseholdStatus(status)] = count
	}

	return counts, rows.Err()
}

// GetByRationClass retrieves all active households with a given ration class.
func (r *HouseholdRepository) GetByRationClass(ctx context.Context, rationClass models.RationClass) ([]*models.Household, error) {
	query := `
		SELECT id, designation, household_type, head_of_household_id, quarters_id,
			ration_class, status, formed_date, dissolved_date, created_at, updated_at
		FROM households
		WHERE ration_class = ? AND status = 'ACTIVE'
		ORDER BY designation`

	rows, err := r.db.QueryContext(ctx, query, string(rationClass))
	if err != nil {
		return nil, fmt.Errorf("querying by ration class: %w", err)
	}
	defer rows.Close()

	var households []*models.Household
	for rows.Next() {
		household, err := r.scanHouseholdRow(rows)
		if err != nil {
			return nil, err
		}
		households = append(households, household)
	}

	return households, rows.Err()
}

// scanHousehold scans a single row into a Household struct.
func (r *HouseholdRepository) scanHousehold(row *sql.Row) (*models.Household, error) {
	var household models.Household
	var formedStr, createdStr, updatedStr string
	var dissolvedStr, headID, quartersID sql.NullString

	err := row.Scan(
		&household.ID,
		&household.Designation,
		&household.HouseholdType,
		&headID,
		&quartersID,
		&household.RationClass,
		&household.Status,
		&formedStr,
		&dissolvedStr,
		&createdStr,
		&updatedStr,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("household not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scanning household: %w", err)
	}

	// Parse dates
	household.FormedDate, _ = time.Parse(time.DateOnly, formedStr)
	if dissolvedStr.Valid {
		dissolved, _ := time.Parse(time.DateOnly, dissolvedStr.String)
		household.DissolvedDate = &dissolved
	}
	household.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	household.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)

	// Set nullable fields
	if headID.Valid {
		household.HeadOfHouseholdID = &headID.String
	}
	if quartersID.Valid {
		household.QuartersID = &quartersID.String
	}

	return &household, nil
}

// scanHouseholdRow scans a row from a rows iterator.
func (r *HouseholdRepository) scanHouseholdRow(rows *sql.Rows) (*models.Household, error) {
	var household models.Household
	var formedStr, createdStr, updatedStr string
	var dissolvedStr, headID, quartersID sql.NullString

	err := rows.Scan(
		&household.ID,
		&household.Designation,
		&household.HouseholdType,
		&headID,
		&quartersID,
		&household.RationClass,
		&household.Status,
		&formedStr,
		&dissolvedStr,
		&createdStr,
		&updatedStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning household row: %w", err)
	}

	// Parse dates
	household.FormedDate, _ = time.Parse(time.DateOnly, formedStr)
	if dissolvedStr.Valid {
		dissolved, _ := time.Parse(time.DateOnly, dissolvedStr.String)
		household.DissolvedDate = &dissolved
	}
	household.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	household.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)

	// Set nullable fields
	if headID.Valid {
		household.HeadOfHouseholdID = &headID.String
	}
	if quartersID.Valid {
		household.QuartersID = &quartersID.String
	}

	return &household, nil
}

// scanHouseholdRowWithCount scans a row that includes member_count.
func (r *HouseholdRepository) scanHouseholdRowWithCount(rows *sql.Rows) (*models.Household, error) {
	var household models.Household
	var formedStr, createdStr, updatedStr string
	var dissolvedStr, headID, quartersID sql.NullString

	err := rows.Scan(
		&household.ID,
		&household.Designation,
		&household.HouseholdType,
		&headID,
		&quartersID,
		&household.RationClass,
		&household.Status,
		&formedStr,
		&dissolvedStr,
		&createdStr,
		&updatedStr,
		&household.MemberCount,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning household row: %w", err)
	}

	// Parse dates
	household.FormedDate, _ = time.Parse(time.DateOnly, formedStr)
	if dissolvedStr.Valid {
		dissolved, _ := time.Parse(time.DateOnly, dissolvedStr.String)
		household.DissolvedDate = &dissolved
	}
	household.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	household.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)

	// Set nullable fields
	if headID.Valid {
		household.HeadOfHouseholdID = &headID.String
	}
	if quartersID.Valid {
		household.QuartersID = &quartersID.String
	}

	return &household, nil
}

func nullableTimePtr(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.Format(time.DateOnly), Valid: true}
}
