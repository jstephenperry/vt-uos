package resources

import (
	"strings"
	"testing"
	"time"

	"github.com/vtuos/vtuos/internal/models"
)

func TestInventoryView_New(t *testing.T) {
	view := NewInventoryView(nil)
	if view == nil {
		t.Fatal("expected non-nil view")
	}
	if view.table == nil {
		t.Fatal("expected non-nil table")
	}
}

func TestInventoryView_EmptyRender(t *testing.T) {
	view := NewInventoryView(nil)
	output := view.Render(120, 40)

	if !strings.Contains(output, "RESOURCE INVENTORY") {
		t.Error("expected title in output")
	}
	if !strings.Contains(output, "No inventory found") {
		t.Error("expected empty state message")
	}
}

func TestInventoryView_RenderHelp_Wide(t *testing.T) {
	view := NewInventoryView(nil)
	output := view.Render(120, 40)

	if !strings.Contains(output, "PgUp/Dn:Page") {
		t.Error("expected full help text on wide terminal")
	}
}

func TestInventoryView_RenderHelp_Narrow(t *testing.T) {
	view := NewInventoryView(nil)
	output := view.Render(50, 40)

	if !strings.Contains(output, "c:Cat") {
		t.Error("expected compact help text on narrow terminal")
	}
}

func TestInventoryView_RenderDetail_NilStock(t *testing.T) {
	view := NewInventoryView(nil)
	output := view.RenderDetail(nil, 120)

	if !strings.Contains(output, "No stock selected") {
		t.Error("expected 'No stock selected' for nil stock")
	}
}

func TestInventoryView_RenderDetail_WithStock(t *testing.T) {
	view := NewInventoryView(nil)
	now := time.Now().UTC()
	view.SetVaultTime(now)

	expiration := now.AddDate(0, 6, 0)
	lastAudit := now.AddDate(0, -1, 0)
	lotNumber := "LOT-2077-001"
	calories := 250.0

	stock := &models.ResourceStock{
		ID:               "stock-001",
		ItemID:           "item-001",
		Quantity:         500.0,
		QuantityReserved: 50.0,
		StorageLocation:  "STORAGE-A-12",
		ReceivedDate:     now.AddDate(-1, 0, 0),
		ExpirationDate:   &expiration,
		LastAuditDate:    &lastAudit,
		LotNumber:        &lotNumber,
		Status:           models.StockStatusAvailable,
		Item: &models.ResourceItem{
			ID:              "item-001",
			ItemCode:        "FOOD-PROTEIN-001",
			Name:            "Protein Ration",
			UnitOfMeasure:   "unit",
			CaloriesPerUnit: &calories,
		},
	}

	output := view.RenderDetail(stock, 120)

	checks := []struct {
		label string
		value string
	}{
		{"title", "STOCK DETAILS"},
		{"item code", "FOOD-PROTEIN-001"},
		{"name", "Protein Ration"},
		{"unit", "unit"},
		{"calories", "250"},
		{"quantity", "500.00"},
		{"reserved", "50.00"},
		{"available", "450.00"},
		{"status", "AVAILABLE"},
		{"location", "STORAGE-A-12"},
		{"lot number", "LOT-2077-001"},
		{"help", "Esc:Back"},
	}

	for _, check := range checks {
		if !strings.Contains(output, check.value) {
			t.Errorf("expected %s (%q) in detail output", check.label, check.value)
		}
	}
}

func TestInventoryView_RenderDetail_ExpiredStock(t *testing.T) {
	view := NewInventoryView(nil)
	now := time.Now().UTC()
	view.SetVaultTime(now)

	expiration := now.AddDate(0, 0, -7) // Expired 7 days ago

	stock := &models.ResourceStock{
		ID:              "stock-002",
		ItemID:          "item-001",
		Quantity:        100.0,
		StorageLocation: "STORAGE-B-03",
		ReceivedDate:    now.AddDate(-2, 0, 0),
		ExpirationDate:  &expiration,
		Status:          models.StockStatusExpired,
		Item: &models.ResourceItem{
			ID:            "item-001",
			ItemCode:      "FOOD-001",
			Name:          "Expired Ration",
			UnitOfMeasure: "unit",
		},
	}

	output := view.RenderDetail(stock, 120)

	if !strings.Contains(output, "EXPIRED") {
		t.Error("expected EXPIRED indicator in output")
	}
}

func TestInventoryView_RenderDetail_NearExpiration(t *testing.T) {
	view := NewInventoryView(nil)
	now := time.Now().UTC()
	view.SetVaultTime(now)

	expiration := now.AddDate(0, 0, 5) // 5 days from now

	stock := &models.ResourceStock{
		ID:              "stock-003",
		ItemID:          "item-001",
		Quantity:        50.0,
		StorageLocation: "STORAGE-C-01",
		ReceivedDate:    now.AddDate(-1, 0, 0),
		ExpirationDate:  &expiration,
		Status:          models.StockStatusAvailable,
		Item: &models.ResourceItem{
			ID:            "item-001",
			ItemCode:      "MED-001",
			Name:          "Medical Supply",
			UnitOfMeasure: "unit",
		},
	}

	output := view.RenderDetail(stock, 120)

	if !strings.Contains(output, "5 days") {
		t.Error("expected '5 days' expiration in output")
	}
}

func TestInventoryView_RenderDetail_Responsive(t *testing.T) {
	view := NewInventoryView(nil)
	now := time.Now().UTC()
	view.SetVaultTime(now)

	stock := &models.ResourceStock{
		ID:              "stock-001",
		ItemID:          "item-001",
		Quantity:        100.0,
		StorageLocation: "STORAGE-A",
		ReceivedDate:    now,
		Status:          models.StockStatusAvailable,
		Item: &models.ResourceItem{
			ID:            "item-001",
			ItemCode:      "ITEM-001",
			Name:          "Test Item",
			UnitOfMeasure: "kg",
		},
	}

	wide := view.RenderDetail(stock, 120)
	narrow := view.RenderDetail(stock, 50)

	if !strings.Contains(wide, "ITEM-001") {
		t.Error("expected item code in wide output")
	}
	if !strings.Contains(narrow, "ITEM-001") {
		t.Error("expected item code in narrow output")
	}
}

func TestInventoryView_CategoryFilter(t *testing.T) {
	view := NewInventoryView(nil)

	catID := "cat-001"
	view.SetCategoryFilter(&catID)

	if view.selectedCategory == nil {
		t.Error("expected category filter to be set")
	}
	if *view.selectedCategory != "cat-001" {
		t.Errorf("expected cat-001, got %s", *view.selectedCategory)
	}

	view.SetCategoryFilter(nil)
	if view.selectedCategory != nil {
		t.Error("expected nil after clearing filter")
	}
}

func TestInventoryView_CategoryFilter_ResetsPage(t *testing.T) {
	view := NewInventoryView(nil)
	view.page.Page = 5

	catID := "cat-001"
	view.SetCategoryFilter(&catID)
	if view.page.Page != 1 {
		t.Errorf("expected page 1 after filter, got %d", view.page.Page)
	}
}

func TestInventoryView_Navigation_Empty(t *testing.T) {
	view := NewInventoryView(nil)

	view.MoveUp()
	view.MoveDown()

	if view.SelectedStock() != nil {
		t.Error("expected nil selected stock with no data")
	}
}

func TestInventoryView_Pagination(t *testing.T) {
	view := NewInventoryView(nil)

	view.NextPage()
	view.PrevPage()
	view.PrevPage() // Should not go below 1
}

func TestInventoryView_SetVaultTime(t *testing.T) {
	view := NewInventoryView(nil)
	now := time.Now().UTC()
	view.SetVaultTime(now)

	if view.vaultTime.IsZero() {
		t.Error("expected non-zero vault time")
	}
}

func TestInventoryView_SetVisibleRows(t *testing.T) {
	view := NewInventoryView(nil)
	view.SetVisibleRows(15)
	// Should not panic
}

func TestInventoryView_GetCategories_Empty(t *testing.T) {
	view := NewInventoryView(nil)
	cats := view.GetCategories()

	if cats != nil {
		t.Error("expected nil categories initially")
	}
}
