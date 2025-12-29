package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/vtuos/vtuos/internal/models"
)

// ResourceRepository handles resource data access.
type ResourceRepository struct {
	db *sql.DB
}

// NewResourceRepository creates a new resource repository.
func NewResourceRepository(db *sql.DB) *ResourceRepository {
	return &ResourceRepository{db: db}
}

// ============================================================================
// CATEGORIES
// ============================================================================

// CreateCategory inserts a new resource category.
func (r *ResourceRepository) CreateCategory(ctx context.Context, tx *sql.Tx, cat *models.ResourceCategory) error {
	query := `
		INSERT INTO resource_categories (
			id, code, name, description, unit_of_measure,
			is_consumable, is_critical, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	execer := r.getExecer(tx)
	cat.CreatedAt = time.Now().UTC()

	_, err := execer.ExecContext(ctx, query,
		cat.ID,
		cat.Code,
		cat.Name,
		nullableString(cat.Description),
		cat.UnitOfMeasure,
		boolToInt(cat.IsConsumable),
		boolToInt(cat.IsCritical),
		cat.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting category: %w", err)
	}
	return nil
}

// GetCategory retrieves a category by ID.
func (r *ResourceRepository) GetCategory(ctx context.Context, id string) (*models.ResourceCategory, error) {
	query := `
		SELECT id, code, name, description, unit_of_measure,
			is_consumable, is_critical, created_at
		FROM resource_categories
		WHERE id = ?`

	return r.scanCategory(r.db.QueryRowContext(ctx, query, id))
}

// GetCategoryByCode retrieves a category by code.
func (r *ResourceRepository) GetCategoryByCode(ctx context.Context, code string) (*models.ResourceCategory, error) {
	query := `
		SELECT id, code, name, description, unit_of_measure,
			is_consumable, is_critical, created_at
		FROM resource_categories
		WHERE code = ?`

	return r.scanCategory(r.db.QueryRowContext(ctx, query, code))
}

// ListCategories retrieves all resource categories.
func (r *ResourceRepository) ListCategories(ctx context.Context) ([]*models.ResourceCategory, error) {
	query := `
		SELECT id, code, name, description, unit_of_measure,
			is_consumable, is_critical, created_at
		FROM resource_categories
		ORDER BY code`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying categories: %w", err)
	}
	defer rows.Close()

	var categories []*models.ResourceCategory
	for rows.Next() {
		cat, err := r.scanCategoryRow(rows)
		if err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}
	return categories, rows.Err()
}

// ============================================================================
// ITEMS
// ============================================================================

// CreateItem inserts a new resource item.
func (r *ResourceRepository) CreateItem(ctx context.Context, tx *sql.Tx, item *models.ResourceItem) error {
	query := `
		INSERT INTO resource_items (
			id, category_id, item_code, name, description, unit_of_measure,
			calories_per_unit, shelf_life_days, storage_requirements,
			is_producible, production_rate_per_day, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	execer := r.getExecer(tx)
	now := time.Now().UTC()
	item.CreatedAt = now
	item.UpdatedAt = now

	_, err := execer.ExecContext(ctx, query,
		item.ID,
		item.CategoryID,
		item.ItemCode,
		item.Name,
		nullableString(item.Description),
		item.UnitOfMeasure,
		item.CaloriesPerUnit,
		item.ShelfLifeDays,
		nullableString(item.StorageRequirements),
		boolToInt(item.IsProducible),
		item.ProductionRatePerDay,
		item.CreatedAt.Format(time.RFC3339),
		item.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting item: %w", err)
	}
	return nil
}

// GetItem retrieves an item by ID.
func (r *ResourceRepository) GetItem(ctx context.Context, id string) (*models.ResourceItem, error) {
	query := `
		SELECT i.id, i.category_id, i.item_code, i.name, i.description, i.unit_of_measure,
			i.calories_per_unit, i.shelf_life_days, i.storage_requirements,
			i.is_producible, i.production_rate_per_day, i.created_at, i.updated_at,
			c.id, c.code, c.name, c.description, c.unit_of_measure,
			c.is_consumable, c.is_critical, c.created_at
		FROM resource_items i
		LEFT JOIN resource_categories c ON i.category_id = c.id
		WHERE i.id = ?`

	return r.scanItemWithCategory(r.db.QueryRowContext(ctx, query, id))
}

// GetItemByCode retrieves an item by code.
func (r *ResourceRepository) GetItemByCode(ctx context.Context, code string) (*models.ResourceItem, error) {
	query := `
		SELECT i.id, i.category_id, i.item_code, i.name, i.description, i.unit_of_measure,
			i.calories_per_unit, i.shelf_life_days, i.storage_requirements,
			i.is_producible, i.production_rate_per_day, i.created_at, i.updated_at,
			c.id, c.code, c.name, c.description, c.unit_of_measure,
			c.is_consumable, c.is_critical, c.created_at
		FROM resource_items i
		LEFT JOIN resource_categories c ON i.category_id = c.id
		WHERE i.item_code = ?`

	return r.scanItemWithCategory(r.db.QueryRowContext(ctx, query, code))
}

// ListItems retrieves items with optional category filter.
func (r *ResourceRepository) ListItems(ctx context.Context, categoryID string, page models.Pagination) (*models.ItemList, error) {
	var conditions []string
	var args []any

	if categoryID != "" {
		conditions = append(conditions, "i.category_id = ?")
		args = append(args, categoryID)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM resource_items i %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting items: %w", err)
	}

	// Get page
	query := fmt.Sprintf(`
		SELECT i.id, i.category_id, i.item_code, i.name, i.description, i.unit_of_measure,
			i.calories_per_unit, i.shelf_life_days, i.storage_requirements,
			i.is_producible, i.production_rate_per_day, i.created_at, i.updated_at
		FROM resource_items i
		%s
		ORDER BY i.item_code
		LIMIT ? OFFSET ?`, whereClause)

	args = append(args, page.Limit(), page.Offset())
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying items: %w", err)
	}
	defer rows.Close()

	var items []*models.ResourceItem
	for rows.Next() {
		item, err := r.scanItemRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return &models.ItemList{
		Items:      items,
		Total:      total,
		Page:       page.Page,
		TotalPages: page.TotalPages(total),
	}, rows.Err()
}

// ============================================================================
// STOCKS
// ============================================================================

// CreateStock inserts a new resource stock.
func (r *ResourceRepository) CreateStock(ctx context.Context, tx *sql.Tx, stock *models.ResourceStock) error {
	query := `
		INSERT INTO resource_stocks (
			id, item_id, lot_number, quantity, quantity_reserved,
			storage_location, received_date, expiration_date, status,
			last_audit_date, last_audit_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	execer := r.getExecer(tx)
	now := time.Now().UTC()
	stock.CreatedAt = now
	stock.UpdatedAt = now

	_, err := execer.ExecContext(ctx, query,
		stock.ID,
		stock.ItemID,
		stock.LotNumber,
		stock.Quantity,
		stock.QuantityReserved,
		stock.StorageLocation,
		stock.ReceivedDate.Format(time.RFC3339),
		nullableTimePtrRFC3339(stock.ExpirationDate),
		string(stock.Status),
		nullableTimePtrRFC3339(stock.LastAuditDate),
		stock.LastAuditBy,
		stock.CreatedAt.Format(time.RFC3339),
		stock.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting stock: %w", err)
	}
	return nil
}

// GetStock retrieves a stock by ID.
func (r *ResourceRepository) GetStock(ctx context.Context, id string) (*models.ResourceStock, error) {
	query := `
		SELECT s.id, s.item_id, s.lot_number, s.quantity, s.quantity_reserved,
			s.storage_location, s.received_date, s.expiration_date, s.status,
			s.last_audit_date, s.last_audit_by, s.created_at, s.updated_at,
			i.id, i.category_id, i.item_code, i.name, i.unit_of_measure
		FROM resource_stocks s
		LEFT JOIN resource_items i ON s.item_id = i.id
		WHERE s.id = ?`

	return r.scanStockWithItem(r.db.QueryRowContext(ctx, query, id))
}

// UpdateStock updates a stock record.
func (r *ResourceRepository) UpdateStock(ctx context.Context, tx *sql.Tx, stock *models.ResourceStock) error {
	query := `
		UPDATE resource_stocks SET
			quantity = ?, quantity_reserved = ?, status = ?,
			last_audit_date = ?, last_audit_by = ?, updated_at = ?
		WHERE id = ?`

	execer := r.getExecer(tx)
	stock.UpdatedAt = time.Now().UTC()

	result, err := execer.ExecContext(ctx, query,
		stock.Quantity,
		stock.QuantityReserved,
		string(stock.Status),
		nullableTimePtrRFC3339(stock.LastAuditDate),
		stock.LastAuditBy,
		stock.UpdatedAt.Format(time.RFC3339),
		stock.ID,
	)
	if err != nil {
		return fmt.Errorf("updating stock: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("stock not found: %s", stock.ID)
	}
	return nil
}

// ListStocks retrieves stocks with filtering and pagination.
func (r *ResourceRepository) ListStocks(ctx context.Context, filter models.StockFilter, page models.Pagination) (*models.StockList, error) {
	var conditions []string
	var args []any

	if filter.ItemID != "" {
		conditions = append(conditions, "s.item_id = ?")
		args = append(args, filter.ItemID)
	}
	if filter.CategoryID != "" {
		conditions = append(conditions, "i.category_id = ?")
		args = append(args, filter.CategoryID)
	}
	if filter.Status != nil {
		conditions = append(conditions, "s.status = ?")
		args = append(args, string(*filter.Status))
	}
	if filter.StorageLocation != "" {
		conditions = append(conditions, "s.storage_location = ?")
		args = append(args, filter.StorageLocation)
	}
	if filter.ExpiringWithin != nil {
		conditions = append(conditions, "s.expiration_date <= date('now', '+' || ? || ' days')")
		args = append(args, *filter.ExpiringWithin)
	}
	if filter.MinQuantity != nil {
		conditions = append(conditions, "s.quantity >= ?")
		args = append(args, *filter.MinQuantity)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM resource_stocks s
		LEFT JOIN resource_items i ON s.item_id = i.id
		%s`, whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting stocks: %w", err)
	}

	// Get page
	query := fmt.Sprintf(`
		SELECT s.id, s.item_id, s.lot_number, s.quantity, s.quantity_reserved,
			s.storage_location, s.received_date, s.expiration_date, s.status,
			s.last_audit_date, s.last_audit_by, s.created_at, s.updated_at,
			i.id, i.category_id, i.item_code, i.name, i.unit_of_measure
		FROM resource_stocks s
		LEFT JOIN resource_items i ON s.item_id = i.id
		%s
		ORDER BY s.expiration_date ASC NULLS LAST, s.received_date ASC
		LIMIT ? OFFSET ?`, whereClause)

	args = append(args, page.Limit(), page.Offset())
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying stocks: %w", err)
	}
	defer rows.Close()

	var stocks []*models.ResourceStock
	for rows.Next() {
		stock, err := r.scanStockWithItemRow(rows)
		if err != nil {
			return nil, err
		}
		stocks = append(stocks, stock)
	}

	return &models.StockList{
		Stocks:     stocks,
		Total:      total,
		Page:       page.Page,
		TotalPages: page.TotalPages(total),
	}, rows.Err()
}

// GetExpiringStocks retrieves stocks expiring within the given days.
func (r *ResourceRepository) GetExpiringStocks(ctx context.Context, days int) ([]*models.ResourceStock, error) {
	query := `
		SELECT s.id, s.item_id, s.lot_number, s.quantity, s.quantity_reserved,
			s.storage_location, s.received_date, s.expiration_date, s.status,
			s.last_audit_date, s.last_audit_by, s.created_at, s.updated_at,
			i.id, i.category_id, i.item_code, i.name, i.unit_of_measure
		FROM resource_stocks s
		LEFT JOIN resource_items i ON s.item_id = i.id
		WHERE s.expiration_date IS NOT NULL
		  AND s.expiration_date <= date('now', '+' || ? || ' days')
		  AND s.status = 'AVAILABLE'
		ORDER BY s.expiration_date ASC`

	rows, err := r.db.QueryContext(ctx, query, days)
	if err != nil {
		return nil, fmt.Errorf("querying expiring stocks: %w", err)
	}
	defer rows.Close()

	var stocks []*models.ResourceStock
	for rows.Next() {
		stock, err := r.scanStockWithItemRow(rows)
		if err != nil {
			return nil, err
		}
		stocks = append(stocks, stock)
	}
	return stocks, rows.Err()
}

// GetTotalStockByItem returns total quantity for an item.
func (r *ResourceRepository) GetTotalStockByItem(ctx context.Context, itemID string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(quantity - quantity_reserved), 0)
		FROM resource_stocks
		WHERE item_id = ? AND status = 'AVAILABLE'`

	var total float64
	err := r.db.QueryRowContext(ctx, query, itemID).Scan(&total)
	return total, err
}

// ============================================================================
// TRANSACTIONS
// ============================================================================

// CreateTransaction inserts a new resource transaction.
func (r *ResourceRepository) CreateTransaction(ctx context.Context, tx *sql.Tx, txn *models.ResourceTransaction) error {
	query := `
		INSERT INTO resource_transactions (
			id, stock_id, item_id, transaction_type, quantity, balance_after,
			reason, authorized_by, related_entity_type, related_entity_id,
			timestamp, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	execer := r.getExecer(tx)
	now := time.Now().UTC()
	if txn.Timestamp.IsZero() {
		txn.Timestamp = now
	}
	txn.CreatedAt = now

	_, err := execer.ExecContext(ctx, query,
		txn.ID,
		txn.StockID,
		txn.ItemID,
		string(txn.TransactionType),
		txn.Quantity,
		txn.BalanceAfter,
		nullableString(txn.Reason),
		txn.AuthorizedBy,
		txn.RelatedEntityType,
		txn.RelatedEntityID,
		txn.Timestamp.Format(time.RFC3339),
		txn.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting transaction: %w", err)
	}
	return nil
}

// ListTransactions retrieves transactions with filtering and pagination.
func (r *ResourceRepository) ListTransactions(ctx context.Context, filter models.TransactionFilter, page models.Pagination) (*models.TransactionList, error) {
	var conditions []string
	var args []any

	if filter.ItemID != "" {
		conditions = append(conditions, "t.item_id = ?")
		args = append(args, filter.ItemID)
	}
	if filter.StockID != "" {
		conditions = append(conditions, "t.stock_id = ?")
		args = append(args, filter.StockID)
	}
	if filter.TransactionType != nil {
		conditions = append(conditions, "t.transaction_type = ?")
		args = append(args, string(*filter.TransactionType))
	}
	if filter.StartDate != nil {
		conditions = append(conditions, "t.timestamp >= ?")
		args = append(args, filter.StartDate.Format(time.RFC3339))
	}
	if filter.EndDate != nil {
		conditions = append(conditions, "t.timestamp <= ?")
		args = append(args, filter.EndDate.Format(time.RFC3339))
	}
	if filter.RelatedEntityType != "" {
		conditions = append(conditions, "t.related_entity_type = ?")
		args = append(args, filter.RelatedEntityType)
	}
	if filter.RelatedEntityID != "" {
		conditions = append(conditions, "t.related_entity_id = ?")
		args = append(args, filter.RelatedEntityID)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM resource_transactions t %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting transactions: %w", err)
	}

	// Get page
	query := fmt.Sprintf(`
		SELECT t.id, t.stock_id, t.item_id, t.transaction_type, t.quantity,
			t.balance_after, t.reason, t.authorized_by, t.related_entity_type,
			t.related_entity_id, t.timestamp, t.created_at,
			i.item_code, i.name
		FROM resource_transactions t
		LEFT JOIN resource_items i ON t.item_id = i.id
		%s
		ORDER BY t.timestamp DESC
		LIMIT ? OFFSET ?`, whereClause)

	args = append(args, page.Limit(), page.Offset())
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*models.ResourceTransaction
	for rows.Next() {
		txn, err := r.scanTransactionRow(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, txn)
	}

	return &models.TransactionList{
		Transactions: transactions,
		Total:        total,
		Page:         page.Page,
		TotalPages:   page.TotalPages(total),
	}, rows.Err()
}

// GetDailyConsumption calculates daily consumption for an item over a period.
func (r *ResourceRepository) GetDailyConsumption(ctx context.Context, itemID string, days int) (float64, error) {
	query := `
		SELECT COALESCE(SUM(ABS(quantity)), 0)
		FROM resource_transactions
		WHERE item_id = ?
		  AND transaction_type = 'CONSUMPTION'
		  AND timestamp >= date('now', '-' || ? || ' days')`

	var totalConsumed float64
	err := r.db.QueryRowContext(ctx, query, itemID, days).Scan(&totalConsumed)
	if err != nil {
		return 0, err
	}

	if days > 0 {
		return totalConsumed / float64(days), nil
	}
	return 0, nil
}

// ============================================================================
// HELPERS
// ============================================================================

func (r *ResourceRepository) getExecer(tx *sql.Tx) interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
} {
	if tx != nil {
		return tx
	}
	return r.db
}

func (r *ResourceRepository) scanCategory(row *sql.Row) (*models.ResourceCategory, error) {
	var cat models.ResourceCategory
	var desc sql.NullString
	var createdStr string
	var isConsumable, isCritical int

	err := row.Scan(
		&cat.ID, &cat.Code, &cat.Name, &desc, &cat.UnitOfMeasure,
		&isConsumable, &isCritical, &createdStr,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("category not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scanning category: %w", err)
	}

	if desc.Valid {
		cat.Description = desc.String
	}
	cat.IsConsumable = isConsumable == 1
	cat.IsCritical = isCritical == 1
	cat.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)

	return &cat, nil
}

func (r *ResourceRepository) scanCategoryRow(rows *sql.Rows) (*models.ResourceCategory, error) {
	var cat models.ResourceCategory
	var desc sql.NullString
	var createdStr string
	var isConsumable, isCritical int

	err := rows.Scan(
		&cat.ID, &cat.Code, &cat.Name, &desc, &cat.UnitOfMeasure,
		&isConsumable, &isCritical, &createdStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning category row: %w", err)
	}

	if desc.Valid {
		cat.Description = desc.String
	}
	cat.IsConsumable = isConsumable == 1
	cat.IsCritical = isCritical == 1
	cat.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)

	return &cat, nil
}

func (r *ResourceRepository) scanItemWithCategory(row *sql.Row) (*models.ResourceItem, error) {
	var item models.ResourceItem
	var cat models.ResourceCategory
	var itemDesc, storageReq sql.NullString
	var calories, prodRate sql.NullFloat64
	var shelfLife sql.NullInt64
	var isProducible int
	var createdStr, updatedStr string
	var catDesc sql.NullString
	var catCreatedStr string
	var catConsumable, catCritical int

	err := row.Scan(
		&item.ID, &item.CategoryID, &item.ItemCode, &item.Name, &itemDesc, &item.UnitOfMeasure,
		&calories, &shelfLife, &storageReq, &isProducible, &prodRate, &createdStr, &updatedStr,
		&cat.ID, &cat.Code, &cat.Name, &catDesc, &cat.UnitOfMeasure,
		&catConsumable, &catCritical, &catCreatedStr,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("item not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scanning item: %w", err)
	}

	if itemDesc.Valid {
		item.Description = itemDesc.String
	}
	if calories.Valid {
		item.CaloriesPerUnit = &calories.Float64
	}
	if shelfLife.Valid {
		v := int(shelfLife.Int64)
		item.ShelfLifeDays = &v
	}
	if storageReq.Valid {
		item.StorageRequirements = storageReq.String
	}
	item.IsProducible = isProducible == 1
	if prodRate.Valid {
		item.ProductionRatePerDay = &prodRate.Float64
	}
	item.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)

	if catDesc.Valid {
		cat.Description = catDesc.String
	}
	cat.IsConsumable = catConsumable == 1
	cat.IsCritical = catCritical == 1
	cat.CreatedAt, _ = time.Parse(time.RFC3339, catCreatedStr)
	item.Category = &cat

	return &item, nil
}

func (r *ResourceRepository) scanItemRow(rows *sql.Rows) (*models.ResourceItem, error) {
	var item models.ResourceItem
	var itemDesc, storageReq sql.NullString
	var calories, prodRate sql.NullFloat64
	var shelfLife sql.NullInt64
	var isProducible int
	var createdStr, updatedStr string

	err := rows.Scan(
		&item.ID, &item.CategoryID, &item.ItemCode, &item.Name, &itemDesc, &item.UnitOfMeasure,
		&calories, &shelfLife, &storageReq, &isProducible, &prodRate, &createdStr, &updatedStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning item row: %w", err)
	}

	if itemDesc.Valid {
		item.Description = itemDesc.String
	}
	if calories.Valid {
		item.CaloriesPerUnit = &calories.Float64
	}
	if shelfLife.Valid {
		v := int(shelfLife.Int64)
		item.ShelfLifeDays = &v
	}
	if storageReq.Valid {
		item.StorageRequirements = storageReq.String
	}
	item.IsProducible = isProducible == 1
	if prodRate.Valid {
		item.ProductionRatePerDay = &prodRate.Float64
	}
	item.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)

	return &item, nil
}

func (r *ResourceRepository) scanStockWithItem(row *sql.Row) (*models.ResourceStock, error) {
	var stock models.ResourceStock
	var item models.ResourceItem
	var lotNum, expDate, auditDate, auditBy sql.NullString
	var receivedStr, createdStr, updatedStr string

	err := row.Scan(
		&stock.ID, &stock.ItemID, &lotNum, &stock.Quantity, &stock.QuantityReserved,
		&stock.StorageLocation, &receivedStr, &expDate, &stock.Status,
		&auditDate, &auditBy, &createdStr, &updatedStr,
		&item.ID, &item.CategoryID, &item.ItemCode, &item.Name, &item.UnitOfMeasure,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("stock not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scanning stock: %w", err)
	}

	if lotNum.Valid {
		stock.LotNumber = &lotNum.String
	}
	stock.ReceivedDate, _ = time.Parse(time.RFC3339, receivedStr)
	if expDate.Valid {
		t, _ := time.Parse(time.RFC3339, expDate.String)
		stock.ExpirationDate = &t
	}
	if auditDate.Valid {
		t, _ := time.Parse(time.RFC3339, auditDate.String)
		stock.LastAuditDate = &t
	}
	if auditBy.Valid {
		stock.LastAuditBy = &auditBy.String
	}
	stock.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	stock.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	stock.Item = &item

	return &stock, nil
}

func (r *ResourceRepository) scanStockWithItemRow(rows *sql.Rows) (*models.ResourceStock, error) {
	var stock models.ResourceStock
	var item models.ResourceItem
	var lotNum, expDate, auditDate, auditBy sql.NullString
	var receivedStr, createdStr, updatedStr string

	err := rows.Scan(
		&stock.ID, &stock.ItemID, &lotNum, &stock.Quantity, &stock.QuantityReserved,
		&stock.StorageLocation, &receivedStr, &expDate, &stock.Status,
		&auditDate, &auditBy, &createdStr, &updatedStr,
		&item.ID, &item.CategoryID, &item.ItemCode, &item.Name, &item.UnitOfMeasure,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning stock row: %w", err)
	}

	if lotNum.Valid {
		stock.LotNumber = &lotNum.String
	}
	stock.ReceivedDate, _ = time.Parse(time.RFC3339, receivedStr)
	if expDate.Valid {
		t, _ := time.Parse(time.RFC3339, expDate.String)
		stock.ExpirationDate = &t
	}
	if auditDate.Valid {
		t, _ := time.Parse(time.RFC3339, auditDate.String)
		stock.LastAuditDate = &t
	}
	if auditBy.Valid {
		stock.LastAuditBy = &auditBy.String
	}
	stock.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	stock.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	stock.Item = &item

	return &stock, nil
}

func (r *ResourceRepository) scanTransactionRow(rows *sql.Rows) (*models.ResourceTransaction, error) {
	var txn models.ResourceTransaction
	var stockID, reason, authBy, relType, relID sql.NullString
	var timestampStr, createdStr string
	var itemCode, itemName sql.NullString

	err := rows.Scan(
		&txn.ID, &stockID, &txn.ItemID, &txn.TransactionType, &txn.Quantity,
		&txn.BalanceAfter, &reason, &authBy, &relType, &relID,
		&timestampStr, &createdStr,
		&itemCode, &itemName,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning transaction row: %w", err)
	}

	if stockID.Valid {
		txn.StockID = &stockID.String
	}
	if reason.Valid {
		txn.Reason = reason.String
	}
	if authBy.Valid {
		txn.AuthorizedBy = &authBy.String
	}
	if relType.Valid {
		txn.RelatedEntityType = &relType.String
	}
	if relID.Valid {
		txn.RelatedEntityID = &relID.String
	}
	txn.Timestamp, _ = time.Parse(time.RFC3339, timestampStr)
	txn.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)

	if itemCode.Valid && itemName.Valid {
		txn.Item = &models.ResourceItem{
			ItemCode: itemCode.String,
			Name:     itemName.String,
		}
	}

	return &txn, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableTimePtrRFC3339(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.Format(time.RFC3339), Valid: true}
}
