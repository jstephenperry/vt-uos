package simulation

import (
	"context"
	"fmt"
	"time"

	"github.com/vtuos/vtuos/internal/models"
	"github.com/vtuos/vtuos/internal/protocol"
	"github.com/vtuos/vtuos/internal/services/population"
)

const (
	// annualBirthRate and annualDeathRate are crude vault-wide vital rates used
	// to derive daily probabilities. They are intentionally conservative.
	annualBirthRate = 0.014
	annualDeathRate = 0.009
)

var newbornGivenNames = []string{
	"Alex", "Sam", "Jordan", "Casey", "Riley", "Morgan", "Quinn", "Avery",
	"Reese", "Drew", "Parker", "Rowan", "Sage", "Emerson", "Hollis", "Marlow",
}

var deathCauses = []string{
	"Natural causes", "Cardiac event", "Respiratory failure",
	"Complications of chronic illness", "Industrial accident",
}

// processDemographics applies one day of vital statistics: a small daily
// probability of a birth and of a death, each routed through the population
// service so registries, lineage and counts stay consistent.
func (e *Engine) processDemographics(ctx context.Context, asOf time.Time) {
	var active int
	if err := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM residents WHERE status = 'ACTIVE'`).Scan(&active); err != nil || active <= 0 {
		return
	}

	if e.rng.Float64() < float64(active)*annualDeathRate/365.0 {
		e.registerNaturalDeath(ctx, asOf)
	}
	if e.rng.Float64() < float64(active)*annualBirthRate/365.0 {
		e.registerVaultBirth(ctx, asOf)
	}
}

// registerNaturalDeath records the death of a randomly selected active resident.
func (e *Engine) registerNaturalDeath(ctx context.Context, asOf time.Time) {
	var id, name string
	const q = `SELECT id, surname || ', ' || given_names FROM residents
		WHERE status = 'ACTIVE' LIMIT 1 OFFSET ?`
	offset := e.randomOffset(ctx, "SELECT COUNT(*) FROM residents WHERE status='ACTIVE'")
	if offset < 0 {
		return
	}
	if err := e.db.QueryRowContext(ctx, q, offset).Scan(&id, &name); err != nil {
		return
	}
	cause := deathCauses[e.rng.Intn(len(deathCauses))]
	if err := e.pop.RegisterDeath(ctx, id, population.DeathRegistration{
		DateOfDeath: asOf,
		Cause:       cause,
	}); err != nil {
		e.log.Debug("registering death failed", "error", err)
		return
	}
	e.mu.Lock()
	e.deaths++
	e.mu.Unlock()
	e.emitEvent(ctx, asOf, catPopulation, "DEATH", protocol.AlertInfo,
		fmt.Sprintf("Death recorded: %s", name),
		fmt.Sprintf("%s passed away (%s). Vital record filed.", name, cause))
}

// registerVaultBirth records a birth when a viable parental pairing exists.
func (e *Engine) registerVaultBirth(ctx context.Context, asOf time.Time) {
	// date_of_birth is stored as a DATE ('YYYY-MM-DD'); compare against the same
	// format so SQLite's comparison and any date functions behave correctly.
	adultCutoff := asOf.AddDate(-18, 0, 0).Format(time.DateOnly)
	fertileFloor := asOf.AddDate(-45, 0, 0).Format(time.DateOnly)

	// Choose an active adult mother of child-bearing age who shares a household.
	var motherID, householdID, surname string
	const motherQ = `SELECT id, household_id, surname FROM residents
		WHERE status = 'ACTIVE' AND sex = 'F' AND household_id IS NOT NULL
		  AND date_of_birth <= ? AND date_of_birth >= ?
		LIMIT 1 OFFSET ?`
	const countQ = `SELECT COUNT(*) FROM residents
		WHERE status='ACTIVE' AND sex='F' AND household_id IS NOT NULL
		  AND date_of_birth <= ? AND date_of_birth >= ?`
	offset := e.randomOffset(ctx, countQ, adultCutoff, fertileFloor)
	if offset < 0 {
		return
	}
	if err := e.db.QueryRowContext(ctx, motherQ, adultCutoff, fertileFloor, offset).
		Scan(&motherID, &householdID, &surname); err != nil {
		return
	}

	// Require an adult father in the same household.
	var fatherID string
	const fatherQ = `SELECT id FROM residents
		WHERE status = 'ACTIVE' AND sex = 'M' AND household_id = ? AND date_of_birth <= ?
		LIMIT 1`
	if err := e.db.QueryRowContext(ctx, fatherQ, householdID, adultCutoff).Scan(&fatherID); err != nil {
		return // no eligible pairing; skip this birth opportunity
	}

	sex := models.SexFemale
	if e.rng.Float64() < 0.5 {
		sex = models.SexMale
	}
	given := newbornGivenNames[e.rng.Intn(len(newbornGivenNames))]

	child, err := e.pop.RegisterBirth(ctx, population.BirthRegistration{
		Surname:     surname,
		GivenNames:  given,
		DateOfBirth: asOf,
		Sex:         sex,
		Parent1ID:   fatherID,
		Parent2ID:   motherID,
		HouseholdID: householdID,
		Notes:       "Registered by simulation control core",
	})
	if err != nil {
		e.log.Debug("registering birth failed", "error", err)
		return
	}
	e.mu.Lock()
	e.births++
	e.mu.Unlock()
	e.emitEvent(ctx, asOf, catPopulation, "BIRTH", protocol.AlertInfo,
		fmt.Sprintf("Birth registered: %s, %s", surname, given),
		fmt.Sprintf("A child (%s) was born to household %s. Registry number %s assigned.",
			sex.String(), householdID, child.RegistryNumber))
}
