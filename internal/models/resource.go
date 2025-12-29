package models

import (
	"time"
)

// ResourceCategory represents a category of resources.
type ResourceCategory struct {
	ID            string
	Code          string // "FOOD", "WATER", "MEDICAL", etc.
	Name          string
	Description   string
	UnitOfMeasure string // "kg", "liters", "units", "doses"
	IsConsumable  bool
	IsCritical    bool // Triggers alerts at low levels
	CreatedAt     time.Time
}

// ResourceItem represents a specific resource item within a category.
type ResourceItem struct {
	ID                   string
	CategoryID           string
	ItemCode             string // "FOOD-PROTEIN-001"
	Name                 string
	Description          string
	UnitOfMeasure        string
	CaloriesPerUnit      *float64 // For food items
	ShelfLifeDays        *int     // NULL for non-perishables
	StorageRequirements  string   // JSON: {"temp_max_c": 4, "humidity_max_pct": 60}
	IsProducible         bool     // Can vault produce this?
	ProductionRatePerDay *float64 // If producible
	CreatedAt            time.Time
	UpdatedAt            time.Time

	// Joined fields
	Category *ResourceCategory
}

// StockStatus represents the status of a resource stock.
type StockStatus string

const (
	StockStatusAvailable  StockStatus = "AVAILABLE"
	StockStatusReserved   StockStatus = "RESERVED"
	StockStatusQuarantine StockStatus = "QUARANTINE"
	StockStatusExpired    StockStatus = "EXPIRED"
	StockStatusDepleted   StockStatus = "DEPLETED"
)

func (s StockStatus) String() string {
	return string(s)
}

// ResourceStock represents inventory of a specific resource item.
type ResourceStock struct {
	ID               string
	ItemID           string
	LotNumber        *string
	Quantity         float64
	QuantityReserved float64
	StorageLocation  string // "STORAGE-A-12"
	ReceivedDate     time.Time
	ExpirationDate   *time.Time
	Status           StockStatus
	LastAuditDate    *time.Time
	LastAuditBy      *string
	CreatedAt        time.Time
	UpdatedAt        time.Time

	// Joined fields
	Item *ResourceItem
}

// AvailableQuantity returns the quantity available for consumption.
func (s *ResourceStock) AvailableQuantity() float64 {
	return s.Quantity - s.QuantityReserved
}

// IsExpired checks if the stock is expired based on expiration date.
func (s *ResourceStock) IsExpired(now time.Time) bool {
	if s.ExpirationDate == nil {
		return false
	}
	return now.After(*s.ExpirationDate)
}

// DaysUntilExpiration returns days until expiration, -1 if no expiration.
func (s *ResourceStock) DaysUntilExpiration(now time.Time) int {
	if s.ExpirationDate == nil {
		return -1
	}
	duration := s.ExpirationDate.Sub(now)
	return int(duration.Hours() / 24)
}

// TransactionType represents the type of resource transaction.
type TransactionType string

const (
	TransactionTypeConsumption     TransactionType = "CONSUMPTION"
	TransactionTypeProduction      TransactionType = "PRODUCTION"
	TransactionTypeAdjustment      TransactionType = "ADJUSTMENT"
	TransactionTypeSpoilage        TransactionType = "SPOILAGE"
	TransactionTypeTransfer        TransactionType = "TRANSFER"
	TransactionTypeAuditCorrection TransactionType = "AUDIT_CORRECTION"
)

func (t TransactionType) String() string {
	return string(t)
}

// ResourceTransaction represents a resource inventory transaction.
type ResourceTransaction struct {
	ID                string
	StockID           *string // NULL for production events
	ItemID            string
	TransactionType   TransactionType
	Quantity          float64 // Positive for additions, negative for removals
	BalanceAfter      float64 // Running balance
	Reason            string
	AuthorizedBy      *string
	RelatedEntityType *string // 'RESIDENT', 'HOUSEHOLD', 'FACILITY', etc.
	RelatedEntityID   *string
	Timestamp         time.Time
	CreatedAt         time.Time

	// Joined fields
	Item  *ResourceItem
	Stock *ResourceStock
}

// StockFilter defines filters for querying stocks.
type StockFilter struct {
	ItemID          string
	CategoryID      string
	Status          *StockStatus
	StorageLocation string
	ExpiringWithin  *int // Days until expiration
	MinQuantity     *float64
}

// TransactionFilter defines filters for querying transactions.
type TransactionFilter struct {
	ItemID            string
	StockID           string
	TransactionType   *TransactionType
	StartDate         *time.Time
	EndDate           *time.Time
	RelatedEntityType string
	RelatedEntityID   string
}

// StockList represents a paginated list of stocks.
type StockList struct {
	Stocks     []*ResourceStock
	Total      int
	Page       int
	TotalPages int
}

// TransactionList represents a paginated list of transactions.
type TransactionList struct {
	Transactions []*ResourceTransaction
	Total        int
	Page         int
	TotalPages   int
}

// ItemList represents a paginated list of resource items.
type ItemList struct {
	Items      []*ResourceItem
	Total      int
	Page       int
	TotalPages int
}

// DailyRequirements represents the vault's daily resource requirements.
type DailyRequirements struct {
	TotalCalories float64
	TotalWaterL   float64
	ByHousehold   map[string]HouseholdRequirement
}

// HouseholdRequirement represents a single household's requirements.
type HouseholdRequirement struct {
	HouseholdID string
	RationClass RationClass
	MemberCount int
	CaloriesDay float64
	WaterLDay   float64
}

// RunwayProjection represents how long resources will last.
type RunwayProjection struct {
	ItemID           string
	ItemName         string
	CurrentStock     float64
	DailyConsumption float64
	DaysRemaining    int
	ProjectedRunout  *time.Time
	Status           string // "CRITICAL", "WARNING", "OK"
}

// RationAllocation represents resource allocation for a household.
type RationAllocation struct {
	HouseholdID   string
	RationClass   RationClass
	DailyCalories float64
	DailyWaterL   float64
	WeeklyItems   []AllocationItem
}

// AllocationItem represents a specific item allocation.
type AllocationItem struct {
	ItemID   string
	ItemName string
	Quantity float64
	Unit     string
}
