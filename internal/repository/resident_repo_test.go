package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/testutil"
)

func setupTestDB(t *testing.T) *testutil.TestDB {
	t.Helper()

	db := testutil.NewTestDB(t)

	// Get migrations path relative to this file
	migrationsDir := filepath.Join("..", "..", "internal", "database", "migrations")
	db.RunMigrations(t, migrationsDir)

	return db
}

func TestResidentRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewResidentRepository(db.DB)
	ctx := context.Background()

	t.Run("Create valid resident", func(t *testing.T) {
		resident := testutil.FixtureResident()

		err := repo.Create(ctx, nil, resident)
		if err != nil {
			t.Fatalf("failed to create resident: %v", err)
		}

		// Verify resident was created
		found, err := repo.GetByID(ctx, resident.ID)
		if err != nil {
			t.Fatalf("failed to get resident: %v", err)
		}

		if found.ID != resident.ID {
			t.Errorf("expected ID %s, got %s", resident.ID, found.ID)
		}
		if found.FullName() != resident.FullName() {
			t.Errorf("expected name %s, got %s", resident.FullName(), found.FullName())
		}
	})

	t.Run("Create with transaction", func(t *testing.T) {
		resident := testutil.FixtureResident()

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		err = repo.Create(ctx, tx, resident)
		if err != nil {
			t.Fatalf("failed to create resident: %v", err)
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("failed to commit transaction: %v", err)
		}

		// Verify resident was created
		found, err := repo.GetByID(ctx, resident.ID)
		if err != nil {
			t.Fatalf("failed to get resident: %v", err)
		}

		if found.ID != resident.ID {
			t.Errorf("expected ID %s, got %s", resident.ID, found.ID)
		}
	})

	t.Run("Create invalid resident returns error", func(t *testing.T) {
		resident := testutil.FixtureResident(func(r *models.Resident) {
			r.ID = "" // Invalid: missing ID
		})

		err := repo.Create(ctx, nil, resident)
		if err == nil {
			t.Error("expected error for invalid resident, got nil")
		}
	})

	t.Run("Duplicate registry number returns error", func(t *testing.T) {
		resident1 := testutil.FixtureResident()
		err := repo.Create(ctx, nil, resident1)
		if err != nil {
			t.Fatalf("failed to create first resident: %v", err)
		}

		resident2 := testutil.FixtureResident(func(r *models.Resident) {
			r.RegistryNumber = resident1.RegistryNumber // Duplicate
		})

		err = repo.Create(ctx, nil, resident2)
		if err == nil {
			t.Error("expected error for duplicate registry number, got nil")
		}
	})
}

func TestResidentRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewResidentRepository(db.DB)
	ctx := context.Background()

	t.Run("Get existing resident", func(t *testing.T) {
		resident := testutil.FixtureResident()
		err := repo.Create(ctx, nil, resident)
		if err != nil {
			t.Fatalf("failed to create resident: %v", err)
		}

		found, err := repo.GetByID(ctx, resident.ID)
		if err != nil {
			t.Fatalf("failed to get resident: %v", err)
		}

		if found.ID != resident.ID {
			t.Errorf("expected ID %s, got %s", resident.ID, found.ID)
		}
		if found.Surname != resident.Surname {
			t.Errorf("expected surname %s, got %s", resident.Surname, found.Surname)
		}
	})

	t.Run("Get non-existent resident returns error", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "non-existent-id")
		if err == nil {
			t.Error("expected error for non-existent resident, got nil")
		}
	})
}

func TestResidentRepository_GetByRegistryNumber(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewResidentRepository(db.DB)
	ctx := context.Background()

	t.Run("Get by registry number", func(t *testing.T) {
		resident := testutil.FixtureResident()
		err := repo.Create(ctx, nil, resident)
		if err != nil {
			t.Fatalf("failed to create resident: %v", err)
		}

		found, err := repo.GetByRegistryNumber(ctx, resident.RegistryNumber)
		if err != nil {
			t.Fatalf("failed to get resident: %v", err)
		}

		if found.RegistryNumber != resident.RegistryNumber {
			t.Errorf("expected registry number %s, got %s", resident.RegistryNumber, found.RegistryNumber)
		}
	})
}

func TestResidentRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewResidentRepository(db.DB)
	ctx := context.Background()

	t.Run("Update resident", func(t *testing.T) {
		resident := testutil.FixtureResident()
		err := repo.Create(ctx, nil, resident)
		if err != nil {
			t.Fatalf("failed to create resident: %v", err)
		}

		// Update fields
		resident.Surname = "Updated"
		resident.ClearanceLevel = 5
		resident.Notes = "Test notes"

		err = repo.Update(ctx, nil, resident)
		if err != nil {
			t.Fatalf("failed to update resident: %v", err)
		}

		// Verify update
		found, err := repo.GetByID(ctx, resident.ID)
		if err != nil {
			t.Fatalf("failed to get resident: %v", err)
		}

		if found.Surname != "Updated" {
			t.Errorf("expected surname 'Updated', got %s", found.Surname)
		}
		if found.ClearanceLevel != 5 {
			t.Errorf("expected clearance level 5, got %d", found.ClearanceLevel)
		}
		if found.Notes != "Test notes" {
			t.Errorf("expected notes 'Test notes', got %s", found.Notes)
		}
	})

	t.Run("Update with transaction", func(t *testing.T) {
		resident := testutil.FixtureResident()
		err := repo.Create(ctx, nil, resident)
		if err != nil {
			t.Fatalf("failed to create resident: %v", err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		resident.Surname = "TransactionUpdate"
		err = repo.Update(ctx, tx, resident)
		if err != nil {
			t.Fatalf("failed to update resident: %v", err)
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("failed to commit transaction: %v", err)
		}

		// Verify update
		found, err := repo.GetByID(ctx, resident.ID)
		if err != nil {
			t.Fatalf("failed to get resident: %v", err)
		}

		if found.Surname != "TransactionUpdate" {
			t.Errorf("expected surname 'TransactionUpdate', got %s", found.Surname)
		}
	})
}

func TestResidentRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewResidentRepository(db.DB)
	ctx := context.Background()

	t.Run("Delete resident", func(t *testing.T) {
		resident := testutil.FixtureResident()
		err := repo.Create(ctx, nil, resident)
		if err != nil {
			t.Fatalf("failed to create resident: %v", err)
		}

		err = repo.Delete(ctx, nil, resident.ID)
		if err != nil {
			t.Fatalf("failed to delete resident: %v", err)
		}

		// Verify deletion
		_, err = repo.GetByID(ctx, resident.ID)
		if err == nil {
			t.Error("expected error after delete, got nil")
		}
	})
}

func TestResidentRepository_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewResidentRepository(db.DB)
	ctx := context.Background()

	// Create test residents
	residents := []*models.Resident{
		testutil.FixtureResident(func(r *models.Resident) {
			r.Surname = "Alpha"
			r.Status = models.ResidentStatusActive
		}),
		testutil.FixtureResident(func(r *models.Resident) {
			r.Surname = "Beta"
			r.Status = models.ResidentStatusActive
		}),
		testutil.FixtureFemaleResident(func(r *models.Resident) {
			r.Surname = "Gamma"
			r.Status = models.ResidentStatusActive
		}),
		testutil.FixtureDeceasedResident(func(r *models.Resident) {
			r.Surname = "Delta"
		}),
	}

	for _, r := range residents {
		if err := repo.Create(ctx, nil, r); err != nil {
			t.Fatalf("failed to create resident: %v", err)
		}
	}

	t.Run("List all residents", func(t *testing.T) {
		result, err := repo.List(ctx, models.ResidentFilter{}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list residents: %v", err)
		}

		if result.Total != 4 {
			t.Errorf("expected total 4, got %d", result.Total)
		}
		if len(result.Residents) != 4 {
			t.Errorf("expected 4 residents, got %d", len(result.Residents))
		}
	})

	t.Run("Filter by status", func(t *testing.T) {
		status := models.ResidentStatusActive
		result, err := repo.List(ctx, models.ResidentFilter{Status: &status}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list residents: %v", err)
		}

		if result.Total != 3 {
			t.Errorf("expected total 3 active residents, got %d", result.Total)
		}
	})

	t.Run("Filter by sex", func(t *testing.T) {
		sex := models.SexFemale
		result, err := repo.List(ctx, models.ResidentFilter{Sex: &sex}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list residents: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("expected total 1 female resident, got %d", result.Total)
		}
	})

	t.Run("Search by name", func(t *testing.T) {
		result, err := repo.List(ctx, models.ResidentFilter{SearchTerm: "Alpha"}, models.Pagination{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("failed to list residents: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("expected total 1 resident matching 'Alpha', got %d", result.Total)
		}
		if len(result.Residents) > 0 && result.Residents[0].Surname != "Alpha" {
			t.Errorf("expected surname 'Alpha', got %s", result.Residents[0].Surname)
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		// Get first page (2 items)
		result, err := repo.List(ctx, models.ResidentFilter{}, models.Pagination{Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("failed to list residents: %v", err)
		}

		if result.Total != 4 {
			t.Errorf("expected total 4, got %d", result.Total)
		}
		if len(result.Residents) != 2 {
			t.Errorf("expected 2 residents on first page, got %d", len(result.Residents))
		}
		if result.TotalPages != 2 {
			t.Errorf("expected 2 total pages, got %d", result.TotalPages)
		}

		// Get second page
		result, err = repo.List(ctx, models.ResidentFilter{}, models.Pagination{Page: 2, PageSize: 2})
		if err != nil {
			t.Fatalf("failed to list residents: %v", err)
		}

		if len(result.Residents) != 2 {
			t.Errorf("expected 2 residents on second page, got %d", len(result.Residents))
		}
	})
}

func TestResidentRepository_CountByStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewResidentRepository(db.DB)
	ctx := context.Background()

	// Create test residents
	residents := []*models.Resident{
		testutil.FixtureResident(),
		testutil.FixtureResident(),
		testutil.FixtureDeceasedResident(),
	}

	for _, r := range residents {
		if err := repo.Create(ctx, nil, r); err != nil {
			t.Fatalf("failed to create resident: %v", err)
		}
	}

	t.Run("Count by status", func(t *testing.T) {
		counts, err := repo.CountByStatus(ctx)
		if err != nil {
			t.Fatalf("failed to count by status: %v", err)
		}

		if counts[models.ResidentStatusActive] != 2 {
			t.Errorf("expected 2 active residents, got %d", counts[models.ResidentStatusActive])
		}
		if counts[models.ResidentStatusDeceased] != 1 {
			t.Errorf("expected 1 deceased resident, got %d", counts[models.ResidentStatusDeceased])
		}
	})
}

func TestResidentRepository_VaultBornWithParents(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewResidentRepository(db.DB)
	ctx := context.Background()

	// Create parent residents
	parent1 := testutil.FixtureResident()
	parent2 := testutil.FixtureFemaleResident()

	if err := repo.Create(ctx, nil, parent1); err != nil {
		t.Fatalf("failed to create parent1: %v", err)
	}
	if err := repo.Create(ctx, nil, parent2); err != nil {
		t.Fatalf("failed to create parent2: %v", err)
	}

	// Create vault-born child
	child := testutil.FixtureVaultBornResident(parent1.ID, parent2.ID)
	if err := repo.Create(ctx, nil, child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Verify child has parents
	found, err := repo.GetByID(ctx, child.ID)
	if err != nil {
		t.Fatalf("failed to get child: %v", err)
	}

	if found.BiologicalParent1ID == nil || *found.BiologicalParent1ID != parent1.ID {
		t.Errorf("expected parent1 ID %s, got %v", parent1.ID, found.BiologicalParent1ID)
	}
	if found.BiologicalParent2ID == nil || *found.BiologicalParent2ID != parent2.ID {
		t.Errorf("expected parent2 ID %s, got %v", parent2.ID, found.BiologicalParent2ID)
	}
}

func TestResidentRepository_AgeCalculations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close(t)

	repo := NewResidentRepository(db.DB)
	ctx := context.Background()

	now := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	// Create residents of various ages
	residents := []*models.Resident{
		testutil.FixtureResident(func(r *models.Resident) {
			r.DateOfBirth = now.AddDate(-15, 0, 0) // 15 years old
		}),
		testutil.FixtureResident(func(r *models.Resident) {
			r.DateOfBirth = now.AddDate(-25, 0, 0) // 25 years old
		}),
		testutil.FixtureResident(func(r *models.Resident) {
			r.DateOfBirth = now.AddDate(-70, 0, 0) // 70 years old
		}),
	}

	for _, r := range residents {
		if err := repo.Create(ctx, nil, r); err != nil {
			t.Fatalf("failed to create resident: %v", err)
		}

		// Verify age calculation
		if r.Age(now) < 0 {
			t.Errorf("age should not be negative, got %d", r.Age(now))
		}
	}

	// Test age-related methods
	if !residents[0].IsAdult(now) && residents[0].Age(now) >= 18 {
		t.Error("resident should be adult")
	}
	if !residents[1].IsWorkingAge(now) {
		t.Error("25 year old should be working age")
	}
	if residents[2].IsWorkingAge(now) {
		t.Error("70 year old should not be working age")
	}
}
