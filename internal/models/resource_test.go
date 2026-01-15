package models

import (
	"testing"
	"time"
)

func TestStockStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status StockStatus
		want   string
	}{
		{"Available", StockStatusAvailable, "AVAILABLE"},
		{"Reserved", StockStatusReserved, "RESERVED"},
		{"Quarantine", StockStatusQuarantine, "QUARANTINE"},
		{"Expired", StockStatusExpired, "EXPIRED"},
		{"Depleted", StockStatusDepleted, "DEPLETED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("StockStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceStock_AvailableQuantity(t *testing.T) {
	tests := []struct {
		name             string
		quantity         float64
		quantityReserved float64
		want             float64
	}{
		{"No reservations", 100.0, 0.0, 100.0},
		{"Some reservations", 100.0, 25.0, 75.0},
		{"Fully reserved", 100.0, 100.0, 0.0},
		{"Over-reserved (edge case)", 100.0, 110.0, -10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stock := &ResourceStock{
				Quantity:         tt.quantity,
				QuantityReserved: tt.quantityReserved,
			}
			if got := stock.AvailableQuantity(); got != tt.want {
				t.Errorf("ResourceStock.AvailableQuantity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceStock_IsExpired(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		expirationDate *time.Time
		now            time.Time
		want           bool
	}{
		{
			name:           "Not expired",
			expirationDate: timePtr(now.AddDate(0, 6, 0)),
			now:            now,
			want:           false,
		},
		{
			name:           "Expired yesterday",
			expirationDate: timePtr(now.AddDate(0, 0, -1)),
			now:            now,
			want:           true,
		},
		{
			name:           "Expires today (not yet expired)",
			expirationDate: timePtr(now),
			now:            now.Add(-time.Hour),
			want:           false,
		},
		{
			name:           "Expires today (now expired)",
			expirationDate: timePtr(now),
			now:            now.Add(time.Hour),
			want:           true,
		},
		{
			name:           "No expiration date",
			expirationDate: nil,
			now:            now,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stock := &ResourceStock{
				ExpirationDate: tt.expirationDate,
			}
			if got := stock.IsExpired(tt.now); got != tt.want {
				t.Errorf("ResourceStock.IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceStock_DaysUntilExpiration(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		expirationDate *time.Time
		now            time.Time
		want           int
	}{
		{
			name:           "Expires in 30 days",
			expirationDate: timePtr(now.AddDate(0, 0, 30)),
			now:            now,
			want:           30,
		},
		{
			name:           "Expires tomorrow",
			expirationDate: timePtr(now.AddDate(0, 0, 1)),
			now:            now,
			want:           1,
		},
		{
			name:           "Expires today",
			expirationDate: timePtr(now),
			now:            now,
			want:           0,
		},
		{
			name:           "Expired yesterday",
			expirationDate: timePtr(now.AddDate(0, 0, -1)),
			now:            now,
			want:           -1,
		},
		{
			name:           "No expiration date",
			expirationDate: nil,
			now:            now,
			want:           -1,
		},
		{
			name:           "Expires in 1 year",
			expirationDate: timePtr(now.AddDate(1, 0, 0)),
			now:            now,
			want:           365,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stock := &ResourceStock{
				ExpirationDate: tt.expirationDate,
			}
			if got := stock.DaysUntilExpiration(tt.now); got != tt.want {
				t.Errorf("ResourceStock.DaysUntilExpiration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTransactionType_String(t *testing.T) {
	tests := []struct {
		name            string
		transactionType TransactionType
		want            string
	}{
		{"Consumption", TransactionTypeConsumption, "CONSUMPTION"},
		{"Production", TransactionTypeProduction, "PRODUCTION"},
		{"Adjustment", TransactionTypeAdjustment, "ADJUSTMENT"},
		{"Spoilage", TransactionTypeSpoilage, "SPOILAGE"},
		{"Transfer", TransactionTypeTransfer, "TRANSFER"},
		{"Audit correction", TransactionTypeAuditCorrection, "AUDIT_CORRECTION"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.transactionType.String(); got != tt.want {
				t.Errorf("TransactionType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function for tests
func timePtr(t time.Time) *time.Time {
	return &t
}
