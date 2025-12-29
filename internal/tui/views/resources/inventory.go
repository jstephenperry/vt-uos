// Package resources provides TUI views for resource management.
package resources

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/services/resources"
	"github.com/vtuos/vtuos/internal/tui/components"
)

// InventoryView displays the resource inventory list.
type InventoryView struct {
	service    *resources.Service
	table      *components.Table
	stocks     []*models.ResourceStock
	categories []*models.ResourceCategory
	page       models.Pagination
	filter     models.StockFilter
	loading    bool
	err        error
	search     string
	vaultTime  time.Time

	// Currently selected category (nil = all)
	selectedCategory *string
}

// MaxContentWidth is the maximum width for content display
const MaxContentWidth = 120

// NewInventoryView creates a new inventory view.
func NewInventoryView(service *resources.Service) *InventoryView {
	columns := []components.Column{
		{Title: "Item Code", Width: 18},
		{Title: "Name", Width: 25},
		{Title: "Category", Width: 10},
		{Title: "Quantity", Width: 12, Align: lipgloss.Right},
		{Title: "Unit", Width: 8},
		{Title: "Status", Width: 10},
		{Title: "Expires", Width: 12},
	}

	table := components.NewTable(columns)
	table.SetVisibleRows(20)
	table.Focus(true)

	return &InventoryView{
		service: service,
		table:   table,
		page:    models.Pagination{Page: 1, PageSize: 20},
	}
}

// Load fetches stocks from the database.
func (v *InventoryView) Load(ctx context.Context) error {
	v.loading = true
	v.err = nil

	// Load categories for display
	if v.categories == nil {
		cats, err := v.service.ListCategories(ctx)
		if err == nil {
			v.categories = cats
		}
	}

	// Apply category filter if selected
	filter := v.filter
	if v.selectedCategory != nil {
		filter.CategoryID = *v.selectedCategory
	}

	result, err := v.service.ListStocks(ctx, filter, v.page)
	if err != nil {
		v.loading = false
		v.err = err
		return err
	}

	v.stocks = result.Stocks
	v.loading = false

	// Convert to table rows
	rows := make([][]string, len(v.stocks))
	for i, s := range v.stocks {
		catCode := "-"
		if s.Item != nil && s.Item.Category != nil {
			catCode = s.Item.Category.Code
		} else if s.Item != nil {
			// Try to find category from our cached list
			for _, cat := range v.categories {
				if cat.ID == s.Item.CategoryID {
					catCode = cat.Code
					break
				}
			}
		}

		itemCode := "-"
		itemName := "-"
		unit := "-"
		if s.Item != nil {
			itemCode = s.Item.ItemCode
			itemName = s.Item.Name
			unit = s.Item.UnitOfMeasure
		}

		expires := "-"
		if s.ExpirationDate != nil {
			days := s.DaysUntilExpiration(v.vaultTime)
			if days < 0 {
				expires = "EXPIRED"
			} else if days == 0 {
				expires = "TODAY"
			} else if days < 30 {
				expires = fmt.Sprintf("%dd", days)
			} else {
				expires = s.ExpirationDate.Format("2006-01-02")
			}
		}

		rows[i] = []string{
			itemCode,
			itemName,
			catCode,
			fmt.Sprintf("%.1f", s.Quantity),
			unit,
			string(s.Status),
			expires,
		}
	}

	v.table.SetRows(rows)
	v.table.SetPagination(result.Page, result.TotalPages, result.Total)

	return nil
}

// SetVaultTime sets the current vault time.
func (v *InventoryView) SetVaultTime(t time.Time) {
	v.vaultTime = t
}

// SetCategoryFilter sets the category filter.
func (v *InventoryView) SetCategoryFilter(categoryID *string) {
	v.selectedCategory = categoryID
	v.page.Page = 1
}

// NextPage moves to the next page.
func (v *InventoryView) NextPage() {
	v.page.Page++
}

// PrevPage moves to the previous page.
func (v *InventoryView) PrevPage() {
	if v.page.Page > 1 {
		v.page.Page--
	}
}

// MoveUp moves the selection up.
func (v *InventoryView) MoveUp() {
	v.table.MoveUp()
}

// MoveDown moves the selection down.
func (v *InventoryView) MoveDown() {
	v.table.MoveDown()
}

// SelectedStock returns the currently selected stock.
func (v *InventoryView) SelectedStock() *models.ResourceStock {
	idx := v.table.Selected()
	if idx >= 0 && idx < len(v.stocks) {
		return v.stocks[idx]
	}
	return nil
}

// GetCategories returns the available categories.
func (v *InventoryView) GetCategories() []*models.ResourceCategory {
	return v.categories
}

// Render renders the inventory view.
func (v *InventoryView) Render(width, height int) string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#66FF66")).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("=== RESOURCE INVENTORY ==="))
	b.WriteString("\n\n")

	// Category filter info
	if v.selectedCategory != nil {
		catName := "Unknown"
		for _, cat := range v.categories {
			if cat.ID == *v.selectedCategory {
				catName = cat.Name
				break
			}
		}
		b.WriteString(labelStyle.Render("Category: "))
		b.WriteString(valueStyle.Render(catName))
		b.WriteString("\n\n")
	}

	// Error display
	if v.err != nil {
		b.WriteString(errStyle.Render("Error: " + v.err.Error()))
		b.WriteString("\n\n")
	}

	// Loading indicator
	if v.loading {
		b.WriteString(labelStyle.Render("Loading..."))
		b.WriteString("\n")
	} else if v.table.Empty() {
		b.WriteString(labelStyle.Render("No inventory found."))
		b.WriteString("\n")
	} else {
		// Table
		b.WriteString(v.table.Render())
	}

	// Help
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Up/Down:Select  Enter:Details  c:Category  PgUp/Dn:Page"))

	return b.String()
}

// RenderDetail renders the detail view for the selected stock.
func (v *InventoryView) RenderDetail(stock *models.ResourceStock) string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#66FF66")).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00")).Width(20)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))
	critStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA00"))

	if stock == nil {
		return labelStyle.Render("No stock selected")
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("=== STOCK DETAILS ==="))
	b.WriteString("\n\n")

	// Item Info
	b.WriteString(sectionStyle.Render("ITEM"))
	b.WriteString("\n")
	if stock.Item != nil {
		b.WriteString(labelStyle.Render("Item Code:") + " " + valueStyle.Render(stock.Item.ItemCode) + "\n")
		b.WriteString(labelStyle.Render("Name:") + " " + valueStyle.Render(stock.Item.Name) + "\n")
		b.WriteString(labelStyle.Render("Unit:") + " " + valueStyle.Render(stock.Item.UnitOfMeasure) + "\n")
		if stock.Item.CaloriesPerUnit != nil && *stock.Item.CaloriesPerUnit > 0 {
			b.WriteString(labelStyle.Render("Calories/Unit:") + " " + valueStyle.Render(fmt.Sprintf("%.0f", *stock.Item.CaloriesPerUnit)) + "\n")
		}
	}
	b.WriteString("\n")

	// Stock Info
	b.WriteString(sectionStyle.Render("STOCK"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Quantity:") + " " + valueStyle.Render(fmt.Sprintf("%.2f", stock.Quantity)) + "\n")
	b.WriteString(labelStyle.Render("Reserved:") + " " + valueStyle.Render(fmt.Sprintf("%.2f", stock.QuantityReserved)) + "\n")
	b.WriteString(labelStyle.Render("Available:") + " " + valueStyle.Render(fmt.Sprintf("%.2f", stock.AvailableQuantity())) + "\n")
	b.WriteString(labelStyle.Render("Status:") + " " + valueStyle.Render(string(stock.Status)) + "\n")
	b.WriteString(labelStyle.Render("Location:") + " " + valueStyle.Render(stock.StorageLocation) + "\n")
	if stock.LotNumber != nil {
		b.WriteString(labelStyle.Render("Lot Number:") + " " + valueStyle.Render(*stock.LotNumber) + "\n")
	}
	b.WriteString("\n")

	// Dates
	b.WriteString(sectionStyle.Render("DATES"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Received:") + " " + valueStyle.Render(stock.ReceivedDate.Format("2006-01-02")) + "\n")
	if stock.ExpirationDate != nil {
		days := stock.DaysUntilExpiration(v.vaultTime)
		expStr := stock.ExpirationDate.Format("2006-01-02")

		var daysStr string
		if days < 0 {
			daysStr = critStyle.Render("EXPIRED")
		} else if days == 0 {
			daysStr = critStyle.Render("TODAY")
		} else if days < 7 {
			daysStr = critStyle.Render(fmt.Sprintf("%d days", days))
		} else if days < 30 {
			daysStr = warnStyle.Render(fmt.Sprintf("%d days", days))
		} else {
			daysStr = valueStyle.Render(fmt.Sprintf("%d days", days))
		}

		b.WriteString(labelStyle.Render("Expires:") + " " + valueStyle.Render(expStr) + " (" + daysStr + ")\n")
	}
	if stock.LastAuditDate != nil {
		b.WriteString(labelStyle.Render("Last Audit:") + " " + valueStyle.Render(stock.LastAuditDate.Format("2006-01-02")) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Esc:Back  a:Adjust  u:Audit"))

	return b.String()
}
