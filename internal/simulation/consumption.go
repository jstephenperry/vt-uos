package simulation

import (
	"context"
	"fmt"
	"time"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/protocol"
	"github.com/vtuos/vtuos/internal/services/resources"
)

// defaultFoodCaloriesPerUnit is used when a food item does not declare a
// calorie density, so consumption accounting can still proceed.
const defaultFoodCaloriesPerUnit = 1500.0 // kcal per kg, ration-pack baseline

// consumeDailyResources draws one day of consumption from inventory based on the
// vault's household ration requirements. Water is metered in litres and food in
// calories (converted to mass via each item's calorie density). A shortfall —
// inventory unable to meet demand — is a reportable operational condition.
func (e *Engine) consumeDailyResources(ctx context.Context, asOf time.Time) {
	reqs, err := e.res.GetVaultDailyRequirements(ctx)
	if err != nil {
		e.log.Warn("computing daily requirements failed", "error", err)
		return
	}

	waterVar := e.variance(e.cfg.Simulation.Consumption.WaterVariance)
	calVar := e.variance(e.cfg.Simulation.Consumption.CalorieVariance)

	waterDemand := reqs.TotalWaterL * waterVar
	calorieDemand := reqs.TotalCalories * calVar

	waterUsed, waterShort := e.consumeByQuantity(ctx, "WATER", waterDemand, "Daily potable water draw")
	foodKg, calShort := e.consumeFoodByCalories(ctx, calorieDemand, "Daily ration distribution")

	// Modest medical/consumable draw proportional to population keeps clinical
	// stocks moving so runway forecasting stays meaningful.
	medDemand := float64(e.state.Population.Active) * 0.02
	medUsed, _ := e.consumeByQuantity(ctx, "MEDICAL", medDemand, "Routine clinical consumption")

	e.mu.Lock()
	e.dailyUse["WATER"] = waterDemand
	e.dailyUse["FOOD"] = foodKg
	e.dailyUse["MEDICAL"] = medUsed
	_ = waterUsed
	e.mu.Unlock()

	if waterShort > 0.5 {
		e.emitEvent(ctx, asOf, catResource, "WATER_SHORTFALL", protocol.AlertCritical,
			"Potable water shortfall",
			fmt.Sprintf("Demand exceeded supply by %.0f L; rationing recommended.", waterShort))
	}
	if calShort > 1000 {
		e.emitEvent(ctx, asOf, catResource, "FOOD_SHORTFALL", protocol.AlertCritical,
			"Ration shortfall",
			fmt.Sprintf("Caloric demand exceeded supply by %.0f kcal; ration class review required.", calShort))
	}
}

// consumeByQuantity consumes the requested quantity (in the category's unit)
// across the available items of a category, oldest stock first. It returns the
// amount actually consumed and any unmet shortfall.
func (e *Engine) consumeByQuantity(ctx context.Context, categoryCode string, demand float64, reason string) (consumed, shortfall float64) {
	if demand <= 0 {
		return 0, 0
	}
	cat, err := e.res.GetCategoryByCode(ctx, categoryCode)
	if err != nil || cat == nil {
		return 0, demand
	}
	items, err := e.res.ListItems(ctx, cat.ID, models.Pagination{Page: 1, PageSize: 100})
	if err != nil {
		return 0, demand
	}

	remaining := demand
	for _, item := range items.Items {
		if remaining <= 0 {
			break
		}
		avail := e.availableForItem(ctx, item.ID)
		if avail <= 0 {
			continue
		}
		take := remaining
		if take > avail {
			take = avail
		}
		if err := e.res.RecordConsumption(ctx, resources.ConsumptionInput{
			ItemID:            item.ID,
			Quantity:          take,
			Reason:            reason,
			RelatedEntityType: "VAULT",
		}); err != nil {
			continue
		}
		remaining -= take
		consumed += take
	}
	if remaining > 0 {
		shortfall = remaining
	}
	return consumed, shortfall
}

// consumeFoodByCalories converts a caloric demand into mass drawn from food
// stocks using each item's calorie density. It returns the kilograms consumed
// and any unmet caloric shortfall.
func (e *Engine) consumeFoodByCalories(ctx context.Context, calorieDemand float64, reason string) (kg, calorieShortfall float64) {
	if calorieDemand <= 0 {
		return 0, 0
	}
	cat, err := e.res.GetCategoryByCode(ctx, "FOOD")
	if err != nil || cat == nil {
		return 0, calorieDemand
	}
	items, err := e.res.ListItems(ctx, cat.ID, models.Pagination{Page: 1, PageSize: 100})
	if err != nil {
		return 0, calorieDemand
	}

	remainingCal := calorieDemand
	for _, item := range items.Items {
		if remainingCal <= 0 {
			break
		}
		density := defaultFoodCaloriesPerUnit
		if item.CaloriesPerUnit != nil && *item.CaloriesPerUnit > 0 {
			density = *item.CaloriesPerUnit
		}
		avail := e.availableForItem(ctx, item.ID)
		if avail <= 0 {
			continue
		}
		neededUnits := remainingCal / density
		take := neededUnits
		if take > avail {
			take = avail
		}
		if err := e.res.RecordConsumption(ctx, resources.ConsumptionInput{
			ItemID:            item.ID,
			Quantity:          take,
			Reason:            reason,
			RelatedEntityType: "VAULT",
		}); err != nil {
			continue
		}
		kg += take
		remainingCal -= take * density
	}
	if remainingCal > 0 {
		calorieShortfall = remainingCal
	}
	return kg, calorieShortfall
}

// processExpirations marks perishable stock that has passed its expiration date
// as spoiled and reports a notable spoilage batch to the operational log.
func (e *Engine) processExpirations(ctx context.Context, asOf time.Time) {
	count, err := e.res.ProcessExpiredItems(ctx, asOf)
	if err != nil {
		e.log.Debug("processing expirations failed", "error", err)
		return
	}
	if count >= 3 {
		e.emitEvent(ctx, asOf, catResource, "SPOILAGE", protocol.AlertWarning,
			"Stock spoilage recorded",
			fmt.Sprintf("%d stock lot(s) passed expiration and were written off as spoilage.", count))
	}
}

// produceDailyResources records one day of internal production for producible
// items, scaled by the efficiency of the facility category that governs them
// (hydroponics for food, water treatment for water, and so on).
func (e *Engine) produceDailyResources(ctx context.Context, asOf time.Time) {
	cats, err := e.res.ListCategories(ctx)
	if err != nil {
		return
	}
	catCodeByID := make(map[string]string, len(cats))
	for _, c := range cats {
		catCodeByID[c.ID] = c.Code
	}

	eff := e.systemEfficiencyByCategory(ctx)

	for _, cat := range cats {
		items, err := e.res.ListItems(ctx, cat.ID, models.Pagination{Page: 1, PageSize: 100})
		if err != nil {
			continue
		}
		for _, item := range items.Items {
			if !item.IsProducible || item.ProductionRatePerDay == nil || *item.ProductionRatePerDay <= 0 {
				continue
			}
			factor := productionEfficiency(cat.Code, eff)
			qty := *item.ProductionRatePerDay * factor
			if qty <= 0 {
				continue
			}

			var expiration *time.Time
			if item.ShelfLifeDays != nil && *item.ShelfLifeDays > 0 {
				exp := asOf.AddDate(0, 0, *item.ShelfLifeDays)
				expiration = &exp
			}
			if _, err := e.res.RecordProduction(ctx, resources.ProductionInput{
				ItemID:          item.ID,
				Quantity:        qty,
				StorageLocation: fmt.Sprintf("PROD-%s", safePrefix(cat.Code)),
				ExpirationDate:  expiration,
				Reason:          "Daily internal production",
			}); err != nil {
				e.log.Debug("recording production failed", "item", item.ItemCode, "error", err)
			}
		}
	}
}

// productionEfficiency maps a resource category to the facility category whose
// efficiency gates its production, returning a 0..1 scaling factor.
func productionEfficiency(resourceCategory string, eff map[string]float64) float64 {
	var key string
	switch resourceCategory {
	case "FOOD":
		key = string(models.SystemCategoryFoodProduction)
	case "WATER":
		key = string(models.SystemCategoryWater)
	case "POWER":
		key = string(models.SystemCategoryPower)
	default:
		return 1.0
	}
	if v, ok := eff[key]; ok {
		return v
	}
	return 1.0
}

// availableForItem returns the total available (unreserved) quantity on hand for
// an item across all available stock lots.
func (e *Engine) availableForItem(ctx context.Context, itemID string) float64 {
	var total float64
	const q = `SELECT COALESCE(SUM(quantity - quantity_reserved), 0)
		FROM resource_stocks WHERE item_id = ? AND status = 'AVAILABLE'`
	if err := e.db.QueryRowContext(ctx, q, itemID).Scan(&total); err != nil {
		return 0
	}
	return total
}

// variance returns a multiplicative consumption variance factor in
// [1-v, 1+v] using the engine's deterministic RNG.
func (e *Engine) variance(v float64) float64 {
	if v <= 0 {
		return 1.0
	}
	return 1.0 + (e.rng.Float64()*2-1)*v
}

func safePrefix(s string) string {
	if len(s) >= 4 {
		return s[:4]
	}
	return s
}
