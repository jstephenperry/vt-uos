package resources

import (
	"time"

	"github.com/vtuos/vtuos/internal/models"
)

// CreateCategoryInput contains data for creating a resource category.
type CreateCategoryInput struct {
	Code          string
	Name          string
	Description   string
	UnitOfMeasure string
	IsConsumable  bool
	IsCritical    bool
}

// CreateItemInput contains data for creating a resource item.
type CreateItemInput struct {
	CategoryID           string
	ItemCode             string
	Name                 string
	Description          string
	UnitOfMeasure        string
	CaloriesPerUnit      *float64
	ShelfLifeDays        *int
	StorageRequirements  string
	IsProducible         bool
	ProductionRatePerDay *float64
}

// CreateStockInput contains data for creating a stock record.
type CreateStockInput struct {
	ItemID          string
	LotNumber       *string
	Quantity        float64
	StorageLocation string
	ReceivedDate    time.Time
	ExpirationDate  *time.Time
}

// StockAdjustment contains data for adjusting stock quantity.
type StockAdjustment struct {
	QuantityChange float64
	Type           models.TransactionType
	Reason         string
	AuthorizedBy   *string
}

// ConsumptionInput contains data for recording consumption.
type ConsumptionInput struct {
	ItemID            string
	Quantity          float64
	Reason            string
	AuthorizedBy      *string
	RelatedEntityType string // RESIDENT, HOUSEHOLD, FACILITY
	RelatedEntityID   string
}

// ProductionInput contains data for recording production.
type ProductionInput struct {
	ItemID          string
	Quantity        float64
	LotNumber       *string
	StorageLocation string
	ExpirationDate  *time.Time
	Reason          string
	AuthorizedBy    *string
}
