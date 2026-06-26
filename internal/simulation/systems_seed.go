package simulation

import (
	"context"
	"log/slog"
	"time"

	"github.com/vtuos/vtuos/internal/database"
	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/services/facilities"
)

// coreSystem describes a piece of vault infrastructure that any commissioned
// vault possesses. These are reference data, not random content.
type coreSystem struct {
	code     string
	name     string
	category models.SystemCategory
	sector   string
	level    int
	capacity float64
	unit     string
	interval int
	mtbf     int
}

var coreSystems = []coreSystem{
	{"PWR-RX-01", "Fusion Reactor Core", models.SystemCategoryPower, "CORE", 5, 480, "kW", 180, 87600},
	{"PWR-GEN-01", "Backup Generator Bank", models.SystemCategoryPower, "CORE", 5, 160, "kW", 120, 43800},
	{"PWR-DIST-01", "Power Distribution Grid", models.SystemCategoryPower, "CORE", 4, 600, "kW", 90, 26280},
	{"WTR-PUR-01", "Water Purification Plant", models.SystemCategoryWater, "UTILITY", 3, 12000, "L/day", 60, 17520},
	{"WTR-REC-01", "Greywater Recycling Unit", models.SystemCategoryWater, "UTILITY", 3, 8000, "L/day", 60, 13140},
	{"HVAC-FILT-01", "Primary Air Filtration", models.SystemCategoryHVAC, "UTILITY", 2, 100, "%", 45, 21900},
	{"HVAC-CLIM-01", "Climate Control System", models.SystemCategoryHVAC, "UTILITY", 2, 100, "%", 90, 26280},
	{"WST-PROC-01", "Sewage Processing Plant", models.SystemCategoryWaste, "UTILITY", 1, 100, "%", 60, 17520},
	{"SEC-DOOR-01", "Vault Door Control", models.SystemCategorySecurity, "ENTRANCE", 1, 100, "%", 30, 35040},
	{"SEC-SURV-01", "Surveillance Network", models.SystemCategorySecurity, "CORE", 4, 100, "%", 90, 26280},
	{"MED-CLIN-01", "Medical Bay Equipment", models.SystemCategoryMedical, "MEDICAL", 2, 100, "%", 90, 21900},
	{"FOOD-HYDRO-01", "Hydroponics Bay A", models.SystemCategoryFoodProduction, "AGRI", 3, 600, "kg/day", 75, 13140},
	{"FOOD-HYDRO-02", "Hydroponics Bay B", models.SystemCategoryFoodProduction, "AGRI", 3, 600, "kg/day", 75, 13140},
	{"COM-INT-01", "Internal Communications", models.SystemCategoryCommunications, "CORE", 4, 100, "%", 180, 43800},
	{"STR-INT-01", "Structural Integrity Monitor", models.SystemCategoryStructural, "CORE", 6, 100, "%", 365, 87600},
}

// EnsureCoreSystems creates the vault's core facility systems if none exist.
// It is idempotent and self-healing: an operational vault always has
// infrastructure to manage.
func EnsureCoreSystems(ctx context.Context, db *database.DB, fac *facilities.Service, installDate time.Time) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM facility_systems`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if installDate.IsZero() {
		installDate = time.Now().UTC()
	}
	created := 0
	for _, cs := range coreSystems {
		cap := cs.capacity
		mtbf := cs.mtbf
		_, err := fac.CreateSystem(ctx, facilities.CreateSystemInput{
			SystemCode:              cs.code,
			Name:                    cs.name,
			Category:                cs.category,
			LocationSector:          cs.sector,
			LocationLevel:           cs.level,
			CapacityRating:          &cap,
			CapacityUnit:            cs.unit,
			InstallDate:             installDate,
			MaintenanceIntervalDays: cs.interval,
			MTBFHours:               &mtbf,
			Notes:                   "Commissioned core system",
		})
		if err != nil {
			slog.Warn("creating core system failed", "code", cs.code, "error", err)
			continue
		}
		created++
	}
	slog.Info("core facility systems ensured", "created", created)
	return nil
}
