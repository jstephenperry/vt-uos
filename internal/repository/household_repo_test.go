package repository

import (
	"context"
	"testing"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/testutil"
)

func TestHouseholdRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewHouseholdRepository(db.DB)
	ctx := context.Background()

	t.Run("Create valid household", func(t *testing.T) {
		household := testutil.FixtureHousehold()

		err := repo.Create(ctx, nil, household)
		if err != nil {
			t.Fatalf("failed to create household: %v", err)
		}

		// Verify household was created
		found, err := repo.GetByID(ctx, household.ID)
		if err != nil {
			t.Fatalf("failed to get household: %v", err)
		}

		if found.ID != household.ID {
			t.Errorf("expected ID %s, got %s", household.ID, found.ID)
		}
		if found.Designation != household.Designation {
			t.Errorf("expected designation %s, got %s", household.Designation, found.Designation)
		}
	})

	t.Run("Create with transaction", func(t *testing.T) {
		household := testutil.FixtureHousehold()

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		err = repo.Create(ctx, tx, household)
		if err != nil {
			t.Fatalf("failed to create household: %v", err)
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("failed to commit transaction: %v", err)
		}

		// Verify household was created
		found, err := repo.GetByID(ctx, household.ID)
		if err != nil {
			t.Fatalf("failed to get household: %v", err)
		}

		if found.ID != household.ID {
			t.Errorf("expected ID %s, got %s", household.ID, found.ID)
		}
	})

	t.Run("Create invalid household returns error", func(t *testing.T) {
		household := testutil.FixtureHousehold(func(h *models.Household) {
			h.ID = "" // Invalid: missing ID
		})

		err := repo.Create(ctx, nil, household)
		if err == nil {
			t.Error("expected error for invalid household, got nil")
		}
	})

	t.Run("Duplicate designation returns error", func(t *testing.T) {
		household1 := testutil.FixtureHousehold()
		err := repo.Create(ctx, nil, household1)
		if err != nil {
			t.Fatalf("failed to create first household: %v", err)
		}

		household2 := testutil.FixtureHousehold(func(h *models.Household) {
			h.Designation = household1.Designation // Duplicate
		})

		err = repo.Create(ctx, nil, household2)
		if err == nil {
			t.Error("expected error for duplicate designation, got nil")
		}
	})
}

func TestHouseholdRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewHouseholdRepository(db.DB)
	ctx := context.Background()

	t.Run("Get existing household", func(t *testing.T) {
		household := testutil.FixtureHousehold()
		err := repo.Create(ctx, nil, household)
		if err != nil {
			t.Fatalf("failed to create household: %v", err)
		}

		found, err := repo.GetByID(ctx, household.ID)
		if err != nil {
			t.Fatalf("failed to get household: %v", err)
		}

		if found.ID != household.ID {
			t.Errorf("expected ID %s, got %s", household.ID, found.ID)
		}
		if found.Designation != household.Designation {
			t.Errorf("expected designation %s, got %s", household.Designation, found.Designation)
		}
		if found.RationClass != household.RationClass {
			t.Errorf("expected ration class %s, got %s", household.RationClass, found.RationClass)
		}
	})

	t.Run("Get non-existent household returns error", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "non-existent-id")
		if err == nil {
			t.Error("expected error for non-existent household, got nil")
		}
	})
}

func TestHouseholdRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewHouseholdRepository(db.DB)
	ctx := context.Background()

	t.Run("Update household", func(t *testing.T) {
		household := testutil.FixtureHousehold()
		err := repo.Create(ctx, nil, household)
		if err != nil {
			t.Fatalf("failed to create household: %v", err)
		}

		// Update fields
		household.RationClass = models.RationClassEnhanced
		household.Status = models.HouseholdStatusActive

		err = repo.Update(ctx, nil, household)
		if err != nil {
			t.Fatalf("failed to update household: %v", err)
		}

		// Verify update
		found, err := repo.GetByID(ctx, household.ID)
		if err != nil {
			t.Fatalf("failed to get household: %v", err)
		}

		if found.RationClass != models.RationClassEnhanced {
			t.Errorf("expected ration class %s, got %s", models.RationClassEnhanced, found.RationClass)
		}
	})
}

func TestHouseholdRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewHouseholdRepository(db.DB)
	ctx := context.Background()

	t.Run("Delete household", func(t *testing.T) {
		household := testutil.FixtureHousehold()
		err := repo.Create(ctx, nil, household)
		if err != nil {
			t.Fatalf("failed to create household: %v", err)
		}

		err = repo.Delete(ctx, nil, household.ID)
		if err != nil {
			t.Fatalf("failed to delete household: %v", err)
		}

		// Verify deletion
		_, err = repo.GetByID(ctx, household.ID)
		if err == nil {
			t.Error("expected error after delete, got nil")
		}
	})
}

func TestHouseholdRepository_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewHouseholdRepository(db.DB)
	ctx := context.Background()

	// Create test households
	households := []*models.Household{
		testutil.FixtureHousehold(func(h *models.Household) {
			h.Designation = "Family-Alpha"
			h.HouseholdType = models.HouseholdTypeFamily
			h.RationClass = models.RationClassStandard
		}),
		testutil.FixtureIndividualHousehold(func(h *models.Household) {
			h.Designation = "Individual-Beta"
			h.RationClass = models.RationClassMinimal
		}),
		testutil.FixtureDissolvedHousehold(func(h *models.Household) {
			h.Designation = "Family-Gamma"
			h.RationClass = models.RationClassMedical
		}),
	}

	for _, h := range households {
		if err := repo.Create(ctx, nil, h); err != nil {
			t.Fatalf("failed to create household: %v", err)
		}
	}

	t.Run("List all households", func(t *testing.T) {
		result, err := repo.List(ctx, models.HouseholdFilter{}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list households: %v", err)
		}

		if result.Total != 3 {
			t.Errorf("expected total 3, got %d", result.Total)
		}
		if len(result.Households) != 3 {
			t.Errorf("expected 3 households, got %d", len(result.Households))
		}
	})

	t.Run("Filter by status", func(t *testing.T) {
		status := models.HouseholdStatusActive
		result, err := repo.List(ctx, models.HouseholdFilter{Status: &status}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list households: %v", err)
		}

		if result.Total != 2 {
			t.Errorf("expected total 2 active households, got %d", result.Total)
		}
	})

	t.Run("Filter by household type", func(t *testing.T) {
		householdType := models.HouseholdTypeIndividual
		result, err := repo.List(ctx, models.HouseholdFilter{HouseholdType: &householdType}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list households: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("expected total 1 individual household, got %d", result.Total)
		}
	})

	t.Run("Filter by ration class", func(t *testing.T) {
		rationClass := models.RationClassStandard
		result, err := repo.List(ctx, models.HouseholdFilter{RationClass: &rationClass}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list households: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("expected total 1 household with standard rations, got %d", result.Total)
		}
	})

	t.Run("Search by designation", func(t *testing.T) {
		result, err := repo.List(ctx, models.HouseholdFilter{SearchTerm: "Alpha"}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list households: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("expected total 1 household matching 'Alpha', got %d", result.Total)
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		// Get first page (2 items)
		result, err := repo.List(ctx, models.HouseholdFilter{}, models.Pagination{Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("failed to list households: %v", err)
		}

		if result.Total != 3 {
			t.Errorf("expected total 3, got %d", result.Total)
		}
		if len(result.Households) != 2 {
			t.Errorf("expected 2 households on first page, got %d", len(result.Households))
		}
		if result.TotalPages != 2 {
			t.Errorf("expected 2 total pages, got %d", result.TotalPages)
		}
	})
}

func TestHouseholdRepository_GetMemberCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	householdRepo := NewHouseholdRepository(db.DB)
	residentRepo := NewResidentRepository(db.DB)
	ctx := context.Background()

	// Create household
	household := testutil.FixtureHousehold()
	if err := householdRepo.Create(ctx, nil, household); err != nil {
		t.Fatalf("failed to create household: %v", err)
	}

	// Create residents in household
	for i := 0; i < 3; i++ {
		resident := testutil.FixtureResident(func(r *models.Resident) {
			r.HouseholdID = &household.ID
		})
		if err := residentRepo.Create(ctx, nil, resident); err != nil {
			t.Fatalf("failed to create resident: %v", err)
		}
	}

	t.Run("Get member count", func(t *testing.T) {
		count, err := householdRepo.GetMemberCount(ctx, household.ID)
		if err != nil {
			t.Fatalf("failed to get member count: %v", err)
		}

		if count != 3 {
			t.Errorf("expected member count 3, got %d", count)
		}
	})
}

func TestHouseholdRepository_CountByStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewHouseholdRepository(db.DB)
	ctx := context.Background()

	// Create test households
	households := []*models.Household{
		testutil.FixtureHousehold(),
		testutil.FixtureHousehold(),
		testutil.FixtureDissolvedHousehold(),
	}

	for _, h := range households {
		if err := repo.Create(ctx, nil, h); err != nil {
			t.Fatalf("failed to create household: %v", err)
		}
	}

	t.Run("Count by status", func(t *testing.T) {
		counts, err := repo.CountByStatus(ctx)
		if err != nil {
			t.Fatalf("failed to count by status: %v", err)
		}

		if counts[models.HouseholdStatusActive] != 2 {
			t.Errorf("expected 2 active households, got %d", counts[models.HouseholdStatusActive])
		}
		if counts[models.HouseholdStatusDissolved] != 1 {
			t.Errorf("expected 1 dissolved household, got %d", counts[models.HouseholdStatusDissolved])
		}
	})
}

func TestHouseholdRepository_RationClassOperations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewHouseholdRepository(db.DB)
	ctx := context.Background()

	// Test all ration classes
	rationClasses := []models.RationClass{
		models.RationClassMinimal,
		models.RationClassStandard,
		models.RationClassEnhanced,
		models.RationClassMedical,
		models.RationClassLaborIntensive,
	}

	for _, rationClass := range rationClasses {
		t.Run("Create household with "+string(rationClass), func(t *testing.T) {
			household := testutil.FixtureHousehold(func(h *models.Household) {
				h.RationClass = rationClass
			})

			err := repo.Create(ctx, nil, household)
			if err != nil {
				t.Fatalf("failed to create household with ration class %s: %v", rationClass, err)
			}

			// Verify ration class was saved correctly
			found, err := repo.GetByID(ctx, household.ID)
			if err != nil {
				t.Fatalf("failed to get household: %v", err)
			}

			if found.RationClass != rationClass {
				t.Errorf("expected ration class %s, got %s", rationClass, found.RationClass)
			}

			// Verify calorie and water targets
			if rationClass.CalorieTarget() <= 0 {
				t.Errorf("calorie target should be positive, got %d", rationClass.CalorieTarget())
			}
			if rationClass.WaterTarget() <= 0 {
				t.Errorf("water target should be positive, got %f", rationClass.WaterTarget())
			}
		})
	}
}
