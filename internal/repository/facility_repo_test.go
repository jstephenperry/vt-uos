package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/testutil"
)

func setupFacilityTest(t *testing.T) (*FacilityRepository, *testutil.TestDB, context.Context) {
	t.Helper()
	db := testutil.NewTestDB(t)
	migrationsDir := filepath.Join("..", "database", "migrations")
	db.RunMigrations(t, migrationsDir)
	repo := NewFacilityRepository(db.DB)
	ctx := context.Background()
	t.Cleanup(func() { db.Close(t) })
	return repo, db, ctx
}

func TestFacilityRepository_Create(t *testing.T) {
	repo, _, ctx := setupFacilityTest(t)

	system := testutil.FixtureFacilitySystem()

	t.Run("Create facility system", func(t *testing.T) {
		err := repo.Create(ctx, nil, system)
		if err != nil {
			t.Fatalf("failed to create system: %v", err)
		}

		got, err := repo.GetByID(ctx, system.ID)
		if err != nil {
			t.Fatalf("failed to get system: %v", err)
		}

		if got.SystemCode != system.SystemCode {
			t.Errorf("expected code %s, got %s", system.SystemCode, got.SystemCode)
		}
		if got.Name != system.Name {
			t.Errorf("expected name %s, got %s", system.Name, got.Name)
		}
		if got.Category != system.Category {
			t.Errorf("expected category %s, got %s", system.Category, got.Category)
		}
		if got.Status != system.Status {
			t.Errorf("expected status %s, got %s", system.Status, got.Status)
		}
		if got.EfficiencyPercent != system.EfficiencyPercent {
			t.Errorf("expected efficiency %.1f, got %.1f", system.EfficiencyPercent, got.EfficiencyPercent)
		}
	})

	t.Run("Create duplicate code fails", func(t *testing.T) {
		dup := testutil.FixtureFacilitySystem(func(s *models.FacilitySystem) {
			s.SystemCode = system.SystemCode
		})
		err := repo.Create(ctx, nil, dup)
		if err == nil {
			t.Error("expected error for duplicate system code")
		}
	})
}

func TestFacilityRepository_GetByID(t *testing.T) {
	repo, _, ctx := setupFacilityTest(t)

	system := testutil.FixtureFacilitySystem()
	if err := repo.Create(ctx, nil, system); err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("Get existing system", func(t *testing.T) {
		got, err := repo.GetByID(ctx, system.ID)
		if err != nil {
			t.Fatalf("failed to get system: %v", err)
		}
		if got.ID != system.ID {
			t.Errorf("expected ID %s, got %s", system.ID, got.ID)
		}
	})

	t.Run("Get non-existent system returns error", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "non-existent")
		if err == nil {
			t.Error("expected error for non-existent system")
		}
	})
}

func TestFacilityRepository_GetByCode(t *testing.T) {
	repo, _, ctx := setupFacilityTest(t)

	system := testutil.FixtureFacilitySystem()
	if err := repo.Create(ctx, nil, system); err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("Get existing system by code", func(t *testing.T) {
		got, err := repo.GetByCode(ctx, system.SystemCode)
		if err != nil {
			t.Fatalf("failed to get system: %v", err)
		}
		if got.ID != system.ID {
			t.Errorf("expected ID %s, got %s", system.ID, got.ID)
		}
	})
}

func TestFacilityRepository_Update(t *testing.T) {
	repo, _, ctx := setupFacilityTest(t)

	system := testutil.FixtureFacilitySystem()
	if err := repo.Create(ctx, nil, system); err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("Update system status and efficiency", func(t *testing.T) {
		system.Status = models.SystemStatusDegraded
		system.EfficiencyPercent = 72.5
		err := repo.Update(ctx, nil, system)
		if err != nil {
			t.Fatalf("failed to update: %v", err)
		}

		got, err := repo.GetByID(ctx, system.ID)
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}
		if got.Status != models.SystemStatusDegraded {
			t.Errorf("expected DEGRADED, got %s", got.Status)
		}
		if got.EfficiencyPercent != 72.5 {
			t.Errorf("expected 72.5, got %.1f", got.EfficiencyPercent)
		}
	})
}

func TestFacilityRepository_Delete(t *testing.T) {
	repo, _, ctx := setupFacilityTest(t)

	system := testutil.FixtureFacilitySystem()
	if err := repo.Create(ctx, nil, system); err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("Delete system", func(t *testing.T) {
		err := repo.Delete(ctx, nil, system.ID)
		if err != nil {
			t.Fatalf("failed to delete: %v", err)
		}

		_, err = repo.GetByID(ctx, system.ID)
		if err == nil {
			t.Error("expected error after delete")
		}
	})

	t.Run("Delete non-existent system", func(t *testing.T) {
		err := repo.Delete(ctx, nil, "non-existent")
		if err == nil {
			t.Error("expected error for non-existent system")
		}
	})
}

func TestFacilityRepository_List(t *testing.T) {
	repo, _, ctx := setupFacilityTest(t)

	// Create test systems
	systems := []*models.FacilitySystem{
		testutil.FixtureFacilitySystem(func(s *models.FacilitySystem) {
			s.SystemCode = "PWR-GEN-01"
			s.Name = "Primary Generator"
			s.Category = models.SystemCategoryPower
			s.LocationSector = "A"
		}),
		testutil.FixtureFacilitySystem(func(s *models.FacilitySystem) {
			s.SystemCode = "WTR-PUR-01"
			s.Name = "Water Purification"
			s.Category = models.SystemCategoryWater
			s.LocationSector = "B"
		}),
		testutil.FixtureDegradedSystem(func(s *models.FacilitySystem) {
			s.SystemCode = "WST-PROC-01"
			s.LocationSector = "C"
		}),
	}

	for _, sys := range systems {
		if err := repo.Create(ctx, nil, sys); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	t.Run("List all systems", func(t *testing.T) {
		result, err := repo.List(ctx, models.FacilitySystemFilter{}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if result.Total != 3 {
			t.Errorf("expected 3 systems, got %d", result.Total)
		}
	})

	t.Run("Filter by category", func(t *testing.T) {
		cat := models.SystemCategoryPower
		result, err := repo.List(ctx, models.FacilitySystemFilter{Category: &cat}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if result.Total != 1 {
			t.Errorf("expected 1 POWER system, got %d", result.Total)
		}
	})

	t.Run("Filter by status", func(t *testing.T) {
		status := models.SystemStatusDegraded
		result, err := repo.List(ctx, models.FacilitySystemFilter{Status: &status}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if result.Total != 1 {
			t.Errorf("expected 1 DEGRADED system, got %d", result.Total)
		}
	})

	t.Run("Filter by sector", func(t *testing.T) {
		result, err := repo.List(ctx, models.FacilitySystemFilter{Sector: "A"}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if result.Total != 1 {
			t.Errorf("expected 1 system in sector A, got %d", result.Total)
		}
	})

	t.Run("Search by name", func(t *testing.T) {
		result, err := repo.List(ctx, models.FacilitySystemFilter{SearchTerm: "Water"}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if result.Total != 1 {
			t.Errorf("expected 1 system matching 'Water', got %d", result.Total)
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		result, err := repo.List(ctx, models.FacilitySystemFilter{}, models.Pagination{Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if len(result.Systems) != 2 {
			t.Errorf("expected 2 systems on page, got %d", len(result.Systems))
		}
		if result.Total != 3 {
			t.Errorf("expected total 3, got %d", result.Total)
		}
		if result.TotalPages != 2 {
			t.Errorf("expected 2 pages, got %d", result.TotalPages)
		}
	})
}

func TestFacilityRepository_CountByStatus(t *testing.T) {
	repo, _, ctx := setupFacilityTest(t)

	systems := []*models.FacilitySystem{
		testutil.FixtureFacilitySystem(func(s *models.FacilitySystem) {
			s.SystemCode = "SYS-A"
			s.Status = models.SystemStatusOperational
		}),
		testutil.FixtureFacilitySystem(func(s *models.FacilitySystem) {
			s.SystemCode = "SYS-B"
			s.Status = models.SystemStatusOperational
		}),
		testutil.FixtureDegradedSystem(func(s *models.FacilitySystem) {
			s.SystemCode = "SYS-C"
		}),
	}

	for _, sys := range systems {
		if err := repo.Create(ctx, nil, sys); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	counts, err := repo.CountByStatus(ctx)
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}

	if counts[models.SystemStatusOperational] != 2 {
		t.Errorf("expected 2 operational, got %d", counts[models.SystemStatusOperational])
	}
	if counts[models.SystemStatusDegraded] != 1 {
		t.Errorf("expected 1 degraded, got %d", counts[models.SystemStatusDegraded])
	}
}

func TestFacilityRepository_MaintenanceRecords(t *testing.T) {
	repo, _, ctx := setupFacilityTest(t)

	system := testutil.FixtureFacilitySystem(func(s *models.FacilitySystem) {
		s.SystemCode = "TEST-SYS"
	})
	if err := repo.Create(ctx, nil, system); err != nil {
		t.Fatalf("setup: %v", err)
	}

	record := testutil.FixtureMaintenanceRecord(system.ID, func(r *models.MaintenanceRecord) {
		estimated := 4.0
		r.EstimatedHours = &estimated
	})

	t.Run("Create maintenance record", func(t *testing.T) {
		err := repo.CreateMaintenanceRecord(ctx, nil, record)
		if err != nil {
			t.Fatalf("failed to create: %v", err)
		}
	})

	t.Run("List maintenance records by system", func(t *testing.T) {
		result, err := repo.ListMaintenanceRecords(ctx,
			models.MaintenanceRecordFilter{SystemID: &system.ID},
			models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}
		if result.Total != 1 {
			t.Errorf("expected 1 record, got %d", result.Total)
		}
		if len(result.Records) > 0 {
			if result.Records[0].Description != record.Description {
				t.Errorf("expected description %q, got %q", record.Description, result.Records[0].Description)
			}
		}
	})
}

func TestFacilityRepository_GetAverageEfficiency(t *testing.T) {
	repo, _, ctx := setupFacilityTest(t)

	systems := []*models.FacilitySystem{
		testutil.FixtureFacilitySystem(func(s *models.FacilitySystem) {
			s.SystemCode = "SYS-1"
			s.EfficiencyPercent = 90.0
		}),
		testutil.FixtureFacilitySystem(func(s *models.FacilitySystem) {
			s.SystemCode = "SYS-2"
			s.EfficiencyPercent = 80.0
		}),
	}
	for _, sys := range systems {
		if err := repo.Create(ctx, nil, sys); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	avg, err := repo.GetAverageEfficiency(ctx)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if avg != 85.0 {
		t.Errorf("expected average 85.0, got %.1f", avg)
	}
}

func TestFacilityRepository_GetOverdueCount(t *testing.T) {
	repo, _, ctx := setupFacilityTest(t)

	pastDue := time.Now().UTC().AddDate(0, 0, -30)
	futureDue := time.Now().UTC().AddDate(0, 0, 30)

	systems := []*models.FacilitySystem{
		testutil.FixtureFacilitySystem(func(s *models.FacilitySystem) {
			s.SystemCode = "OVERDUE-1"
			s.NextMaintenanceDue = &pastDue
		}),
		testutil.FixtureFacilitySystem(func(s *models.FacilitySystem) {
			s.SystemCode = "OK-1"
			s.NextMaintenanceDue = &futureDue
		}),
	}
	for _, sys := range systems {
		if err := repo.Create(ctx, nil, sys); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	count, err := repo.GetOverdueCount(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 overdue, got %d", count)
	}
}
