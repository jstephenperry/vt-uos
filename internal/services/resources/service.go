// Package resources provides resource management services for VT-UOS.
package resources

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/repository"
	"github.com/vtuos/vtuos/internal/util"
)

// Service provides resource management operations.
type Service struct {
	db          *sql.DB
	resources   *repository.ResourceRepository
	households  *repository.HouseholdRepository
	residents   *repository.ResidentRepository
	idGenerator *util.IDGenerator
}

// NewService creates a new resource service.
func NewService(db *sql.DB) *Service {
	return &Service{
		db:          db,
		resources:   repository.NewResourceRepository(db),
		households:  repository.NewHouseholdRepository(db),
		residents:   repository.NewResidentRepository(db),
		idGenerator: util.NewIDGenerator(),
	}
}

// ============================================================================
// CATEGORIES
// ============================================================================

// CreateCategory creates a new resource category.
func (s *Service) CreateCategory(ctx context.Context, input CreateCategoryInput) (*models.ResourceCategory, error) {
	cat := &models.ResourceCategory{
		ID:            s.idGenerator.NewID(),
		Code:          input.Code,
		Name:          input.Name,
		Description:   input.Description,
		UnitOfMeasure: input.UnitOfMeasure,
		IsConsumable:  input.IsConsumable,
		IsCritical:    input.IsCritical,
	}

	if err := s.resources.CreateCategory(ctx, nil, cat); err != nil {
		return nil, fmt.Errorf("creating category: %w", err)
	}

	return cat, nil
}

// GetCategory retrieves a category by ID.
func (s *Service) GetCategory(ctx context.Context, id string) (*models.ResourceCategory, error) {
	return s.resources.GetCategory(ctx, id)
}

// GetCategoryByCode retrieves a category by code.
func (s *Service) GetCategoryByCode(ctx context.Context, code string) (*models.ResourceCategory, error) {
	return s.resources.GetCategoryByCode(ctx, code)
}

// ListCategories retrieves all resource categories.
func (s *Service) ListCategories(ctx context.Context) ([]*models.ResourceCategory, error) {
	return s.resources.ListCategories(ctx)
}

// ============================================================================
// ITEMS
// ============================================================================

// CreateItem creates a new resource item.
func (s *Service) CreateItem(ctx context.Context, input CreateItemInput) (*models.ResourceItem, error) {
	item := &models.ResourceItem{
		ID:                   s.idGenerator.NewID(),
		CategoryID:           input.CategoryID,
		ItemCode:             input.ItemCode,
		Name:                 input.Name,
		Description:          input.Description,
		UnitOfMeasure:        input.UnitOfMeasure,
		CaloriesPerUnit:      input.CaloriesPerUnit,
		ShelfLifeDays:        input.ShelfLifeDays,
		StorageRequirements:  input.StorageRequirements,
		IsProducible:         input.IsProducible,
		ProductionRatePerDay: input.ProductionRatePerDay,
	}

	if err := s.resources.CreateItem(ctx, nil, item); err != nil {
		return nil, fmt.Errorf("creating item: %w", err)
	}

	return item, nil
}

// GetItem retrieves an item by ID.
func (s *Service) GetItem(ctx context.Context, id string) (*models.ResourceItem, error) {
	return s.resources.GetItem(ctx, id)
}

// GetItemByCode retrieves an item by code.
func (s *Service) GetItemByCode(ctx context.Context, code string) (*models.ResourceItem, error) {
	return s.resources.GetItemByCode(ctx, code)
}

// ListItems retrieves items with optional category filter.
func (s *Service) ListItems(ctx context.Context, categoryID string, page models.Pagination) (*models.ItemList, error) {
	return s.resources.ListItems(ctx, categoryID, page)
}

// ============================================================================
// STOCKS
// ============================================================================

// CreateStock creates a new stock record.
func (s *Service) CreateStock(ctx context.Context, input CreateStockInput) (*models.ResourceStock, error) {
	stock := &models.ResourceStock{
		ID:              s.idGenerator.NewID(),
		ItemID:          input.ItemID,
		LotNumber:       input.LotNumber,
		Quantity:        input.Quantity,
		StorageLocation: input.StorageLocation,
		ReceivedDate:    input.ReceivedDate,
		ExpirationDate:  input.ExpirationDate,
		Status:          models.StockStatusAvailable,
	}

	if err := s.resources.CreateStock(ctx, nil, stock); err != nil {
		return nil, fmt.Errorf("creating stock: %w", err)
	}

	// Record the receipt transaction
	txn := &models.ResourceTransaction{
		ID:              s.idGenerator.NewID(),
		StockID:         &stock.ID,
		ItemID:          input.ItemID,
		TransactionType: models.TransactionTypeProduction,
		Quantity:        input.Quantity,
		BalanceAfter:    input.Quantity,
		Reason:          "Initial stock receipt",
	}
	if err := s.resources.CreateTransaction(ctx, nil, txn); err != nil {
		return nil, fmt.Errorf("recording receipt transaction: %w", err)
	}

	return stock, nil
}

// GetStock retrieves a stock by ID.
func (s *Service) GetStock(ctx context.Context, id string) (*models.ResourceStock, error) {
	return s.resources.GetStock(ctx, id)
}

// ListStocks retrieves stocks with filtering and pagination.
func (s *Service) ListStocks(ctx context.Context, filter models.StockFilter, page models.Pagination) (*models.StockList, error) {
	return s.resources.ListStocks(ctx, filter, page)
}

// AdjustStock adjusts the quantity of a stock.
func (s *Service) AdjustStock(ctx context.Context, stockID string, adjustment StockAdjustment) error {
	stock, err := s.resources.GetStock(ctx, stockID)
	if err != nil {
		return fmt.Errorf("getting stock: %w", err)
	}

	newQty := stock.Quantity + adjustment.QuantityChange
	if newQty < 0 {
		return fmt.Errorf("adjustment would result in negative quantity")
	}

	stock.Quantity = newQty
	if newQty == 0 {
		stock.Status = models.StockStatusDepleted
	}

	if err := s.resources.UpdateStock(ctx, nil, stock); err != nil {
		return fmt.Errorf("updating stock: %w", err)
	}

	// Record the transaction
	txn := &models.ResourceTransaction{
		ID:              s.idGenerator.NewID(),
		StockID:         &stockID,
		ItemID:          stock.ItemID,
		TransactionType: adjustment.Type,
		Quantity:        adjustment.QuantityChange,
		BalanceAfter:    newQty,
		Reason:          adjustment.Reason,
		AuthorizedBy:    adjustment.AuthorizedBy,
	}
	if err := s.resources.CreateTransaction(ctx, nil, txn); err != nil {
		return fmt.Errorf("recording transaction: %w", err)
	}

	return nil
}

// RecordConsumption records resource consumption.
func (s *Service) RecordConsumption(ctx context.Context, input ConsumptionInput) error {
	// Find available stock (FIFO - oldest first by expiration/received date)
	filter := models.StockFilter{
		ItemID: input.ItemID,
		Status: ptr(models.StockStatusAvailable),
	}
	stocks, err := s.resources.ListStocks(ctx, filter, models.Pagination{Page: 1, PageSize: 100})
	if err != nil {
		return fmt.Errorf("listing stocks: %w", err)
	}

	remaining := input.Quantity
	for _, stock := range stocks.Stocks {
		if remaining <= 0 {
			break
		}

		available := stock.AvailableQuantity()
		if available <= 0 {
			continue
		}

		consume := remaining
		if consume > available {
			consume = available
		}

		adjustment := StockAdjustment{
			QuantityChange: -consume,
			Type:           models.TransactionTypeConsumption,
			Reason:         input.Reason,
			AuthorizedBy:   input.AuthorizedBy,
		}
		if err := s.AdjustStock(ctx, stock.ID, adjustment); err != nil {
			return fmt.Errorf("consuming from stock %s: %w", stock.ID, err)
		}

		remaining -= consume
	}

	if remaining > 0 {
		return fmt.Errorf("insufficient stock: %.2f units remaining", remaining)
	}

	return nil
}

// RecordProduction records resource production.
func (s *Service) RecordProduction(ctx context.Context, input ProductionInput) (*models.ResourceStock, error) {
	stock := &models.ResourceStock{
		ID:              s.idGenerator.NewID(),
		ItemID:          input.ItemID,
		LotNumber:       input.LotNumber,
		Quantity:        input.Quantity,
		StorageLocation: input.StorageLocation,
		ReceivedDate:    time.Now(),
		ExpirationDate:  input.ExpirationDate,
		Status:          models.StockStatusAvailable,
	}

	if err := s.resources.CreateStock(ctx, nil, stock); err != nil {
		return nil, fmt.Errorf("creating stock: %w", err)
	}

	txn := &models.ResourceTransaction{
		ID:              s.idGenerator.NewID(),
		StockID:         &stock.ID,
		ItemID:          input.ItemID,
		TransactionType: models.TransactionTypeProduction,
		Quantity:        input.Quantity,
		BalanceAfter:    input.Quantity,
		Reason:          input.Reason,
		AuthorizedBy:    input.AuthorizedBy,
	}
	if err := s.resources.CreateTransaction(ctx, nil, txn); err != nil {
		return nil, fmt.Errorf("recording production transaction: %w", err)
	}

	return stock, nil
}

// GetTransactionHistory retrieves transaction history.
func (s *Service) GetTransactionHistory(ctx context.Context, filter models.TransactionFilter, page models.Pagination) (*models.TransactionList, error) {
	return s.resources.ListTransactions(ctx, filter, page)
}

// ============================================================================
// EXPIRATION & FORECASTING
// ============================================================================

// GetExpiringItems returns items expiring within the given days.
func (s *Service) GetExpiringItems(ctx context.Context, withinDays int) ([]*models.ResourceStock, error) {
	return s.resources.GetExpiringStocks(ctx, withinDays)
}

// ProcessExpiredItems marks expired items and creates spoilage transactions.
func (s *Service) ProcessExpiredItems(ctx context.Context, now time.Time) (int, error) {
	// Get items expiring today or earlier
	stocks, err := s.resources.GetExpiringStocks(ctx, 0)
	if err != nil {
		return 0, fmt.Errorf("getting expired stocks: %w", err)
	}

	count := 0
	for _, stock := range stocks {
		if stock.ExpirationDate != nil && now.After(*stock.ExpirationDate) {
			// Mark as expired
			stock.Status = models.StockStatusExpired
			if err := s.resources.UpdateStock(ctx, nil, stock); err != nil {
				continue
			}

			// Record spoilage transaction
			txn := &models.ResourceTransaction{
				ID:              s.idGenerator.NewID(),
				StockID:         &stock.ID,
				ItemID:          stock.ItemID,
				TransactionType: models.TransactionTypeSpoilage,
				Quantity:        -stock.Quantity,
				BalanceAfter:    0,
				Reason:          "Expired",
			}
			s.resources.CreateTransaction(ctx, nil, txn)
			count++
		}
	}

	return count, nil
}

// GetResourceRunway calculates how long resources will last.
func (s *Service) GetResourceRunway(ctx context.Context, itemID string) (*models.RunwayProjection, error) {
	// Get total available stock
	totalStock, err := s.resources.GetTotalStockByItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("getting total stock: %w", err)
	}

	// Get item info
	item, err := s.resources.GetItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("getting item: %w", err)
	}

	// Calculate daily consumption (last 30 days average)
	dailyConsumption, err := s.resources.GetDailyConsumption(ctx, itemID, 30)
	if err != nil {
		return nil, fmt.Errorf("getting daily consumption: %w", err)
	}

	proj := &models.RunwayProjection{
		ItemID:           itemID,
		ItemName:         item.Name,
		CurrentStock:     totalStock,
		DailyConsumption: dailyConsumption,
	}

	if dailyConsumption > 0 {
		daysRemaining := int(totalStock / dailyConsumption)
		proj.DaysRemaining = daysRemaining

		runoutDate := time.Now().AddDate(0, 0, daysRemaining)
		proj.ProjectedRunout = &runoutDate

		if daysRemaining < 7 {
			proj.Status = "CRITICAL"
		} else if daysRemaining < 30 {
			proj.Status = "WARNING"
		} else {
			proj.Status = "OK"
		}
	} else {
		proj.DaysRemaining = -1 // Unlimited
		proj.Status = "OK"
	}

	return proj, nil
}

// ============================================================================
// RATIONING
// ============================================================================

// CalculateHouseholdAllocation calculates resource allocation for a household.
func (s *Service) CalculateHouseholdAllocation(ctx context.Context, householdID string) (*models.RationAllocation, error) {
	household, err := s.households.GetByID(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("getting household: %w", err)
	}

	// Get household members
	members, err := s.residents.GetByHousehold(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("getting members: %w", err)
	}

	// Calculate totals based on ration class and member count
	baseCalories := float64(household.RationClass.CalorieTarget())
	baseWater := household.RationClass.WaterTarget()

	allocation := &models.RationAllocation{
		HouseholdID:   householdID,
		RationClass:   household.RationClass,
		DailyCalories: baseCalories * float64(len(members)),
		DailyWaterL:   baseWater * float64(len(members)),
	}

	return allocation, nil
}

// GetVaultDailyRequirements calculates total daily resource requirements.
func (s *Service) GetVaultDailyRequirements(ctx context.Context) (*models.DailyRequirements, error) {
	// Get all active households
	filter := models.HouseholdFilter{
		Status: ptr(models.HouseholdStatusActive),
	}
	households, err := s.households.List(ctx, filter, models.Pagination{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, fmt.Errorf("listing households: %w", err)
	}

	reqs := &models.DailyRequirements{
		ByHousehold: make(map[string]models.HouseholdRequirement),
	}

	for _, h := range households.Households {
		members, err := s.residents.GetByHousehold(ctx, h.ID)
		if err != nil {
			continue
		}
		memberCount := len(members)

		caloriesDay := float64(h.RationClass.CalorieTarget() * memberCount)
		waterDay := h.RationClass.WaterTarget() * float64(memberCount)

		reqs.TotalCalories += caloriesDay
		reqs.TotalWaterL += waterDay

		reqs.ByHousehold[h.ID] = models.HouseholdRequirement{
			HouseholdID: h.ID,
			RationClass: h.RationClass,
			MemberCount: memberCount,
			CaloriesDay: caloriesDay,
			WaterLDay:   waterDay,
		}
	}

	return reqs, nil
}

// ============================================================================
// AUDITING
// ============================================================================

// PerformInventoryAudit records an inventory audit adjustment.
func (s *Service) PerformInventoryAudit(ctx context.Context, stockID string, actualQty float64, auditorID string) error {
	stock, err := s.resources.GetStock(ctx, stockID)
	if err != nil {
		return fmt.Errorf("getting stock: %w", err)
	}

	difference := actualQty - stock.Quantity
	if difference == 0 {
		// No adjustment needed, just update audit date
		now := time.Now()
		stock.LastAuditDate = &now
		stock.LastAuditBy = &auditorID
		return s.resources.UpdateStock(ctx, nil, stock)
	}

	// Record the adjustment
	now := time.Now()
	stock.Quantity = actualQty
	stock.LastAuditDate = &now
	stock.LastAuditBy = &auditorID

	if actualQty == 0 {
		stock.Status = models.StockStatusDepleted
	}

	if err := s.resources.UpdateStock(ctx, nil, stock); err != nil {
		return fmt.Errorf("updating stock: %w", err)
	}

	txn := &models.ResourceTransaction{
		ID:              s.idGenerator.NewID(),
		StockID:         &stockID,
		ItemID:          stock.ItemID,
		TransactionType: models.TransactionTypeAuditCorrection,
		Quantity:        difference,
		BalanceAfter:    actualQty,
		Reason:          "Inventory audit correction",
		AuthorizedBy:    &auditorID,
	}
	if err := s.resources.CreateTransaction(ctx, nil, txn); err != nil {
		return fmt.Errorf("recording audit transaction: %w", err)
	}

	return nil
}

// Helper function
func ptr[T any](v T) *T {
	return &v
}
