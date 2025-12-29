package population

import (
	"context"
	"time"

	"github.com/vtuos/vtuos/internal/models"
)

// AgeDistribution contains population breakdown by age groups.
type AgeDistribution struct {
	Infants     int // 0-2
	Children    int // 3-12
	Adolescents int // 13-17
	YoungAdults int // 18-25
	Adults      int // 26-45
	MiddleAged  int // 46-65
	Seniors     int // 66+

	Total      int
	MedianAge  float64
	AverageAge float64
}

// GetAgeDistribution calculates the age distribution of active residents.
func (s *Service) GetAgeDistribution(ctx context.Context, asOf time.Time) (*AgeDistribution, error) {
	// Get all active residents
	filter := models.ResidentFilter{
		Status: ptr(models.ResidentStatusActive),
	}

	// Get all pages
	var allResidents []*models.Resident
	page := models.Pagination{Page: 1, PageSize: 100}

	for {
		result, err := s.residents.List(ctx, filter, page)
		if err != nil {
			return nil, err
		}
		allResidents = append(allResidents, result.Residents...)
		if page.Page >= result.TotalPages {
			break
		}
		page.Page++
	}

	dist := &AgeDistribution{}
	var totalAge float64
	var ages []int

	for _, r := range allResidents {
		age := r.Age(asOf)
		ages = append(ages, age)
		totalAge += float64(age)

		switch {
		case age <= 2:
			dist.Infants++
		case age <= 12:
			dist.Children++
		case age <= 17:
			dist.Adolescents++
		case age <= 25:
			dist.YoungAdults++
		case age <= 45:
			dist.Adults++
		case age <= 65:
			dist.MiddleAged++
		default:
			dist.Seniors++
		}
	}

	dist.Total = len(allResidents)
	if dist.Total > 0 {
		dist.AverageAge = totalAge / float64(dist.Total)
		dist.MedianAge = calculateMedian(ages)
	}

	return dist, nil
}

// SexDistribution contains population breakdown by sex.
type SexDistribution struct {
	Male      int
	Female    int
	Total     int
	MaleRatio float64
}

// GetSexDistribution calculates the sex distribution of active residents.
func (s *Service) GetSexDistribution(ctx context.Context) (*SexDistribution, error) {
	filter := models.ResidentFilter{
		Status: ptr(models.ResidentStatusActive),
	}

	var allResidents []*models.Resident
	page := models.Pagination{Page: 1, PageSize: 100}

	for {
		result, err := s.residents.List(ctx, filter, page)
		if err != nil {
			return nil, err
		}
		allResidents = append(allResidents, result.Residents...)
		if page.Page >= result.TotalPages {
			break
		}
		page.Page++
	}

	dist := &SexDistribution{}
	for _, r := range allResidents {
		switch r.Sex {
		case models.SexMale:
			dist.Male++
		case models.SexFemale:
			dist.Female++
		}
	}

	dist.Total = dist.Male + dist.Female
	if dist.Total > 0 {
		dist.MaleRatio = float64(dist.Male) / float64(dist.Total)
	}

	return dist, nil
}

// PopulationProjection contains projected population data.
type PopulationProjection struct {
	CurrentPopulation int
	Projections       []ProjectionPoint
	GrowthRate        float64 // Annual percentage
	Viability         ViabilityAssessment
}

// ProjectionPoint represents population at a point in time.
type ProjectionPoint struct {
	Year       int
	Population int
	Births     int
	Deaths     int
	NetChange  int
}

// ViabilityAssessment evaluates long-term population viability.
type ViabilityAssessment struct {
	IsViable        bool
	MinimumViable   int // Minimum viable population (MVP)
	YearsToMVP      int // Years until population drops below MVP (if negative growth)
	Concerns        []string
	Recommendations []string
}

// ProjectPopulation projects population for the given number of years.
func (s *Service) ProjectPopulation(ctx context.Context, asOf time.Time, years int) (*PopulationProjection, error) {
	stats, err := s.GetPopulationStats(ctx)
	if err != nil {
		return nil, err
	}

	ageDist, err := s.GetAgeDistribution(ctx, asOf)
	if err != nil {
		return nil, err
	}

	sexDist, err := s.GetSexDistribution(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate rates based on current demographics
	// Crude birth rate: assume 2.1 children per woman of childbearing age (15-44) over lifetime
	// Simplified: annual births = (women 15-44) * 0.08 (roughly 2.1/26 years)
	womenOfChildbearingAge := float64(sexDist.Female) * 0.4 // Rough estimate
	annualBirths := int(womenOfChildbearingAge * 0.08)

	// Death rate: based on age distribution
	// Simplified mortality by age
	annualDeaths := int(
		float64(ageDist.Infants)*0.01 +
			float64(ageDist.Children)*0.001 +
			float64(ageDist.Adolescents)*0.001 +
			float64(ageDist.YoungAdults)*0.002 +
			float64(ageDist.Adults)*0.003 +
			float64(ageDist.MiddleAged)*0.01 +
			float64(ageDist.Seniors)*0.05,
	)
	if annualDeaths < 1 && stats.TotalActive > 50 {
		annualDeaths = 1 // Minimum 1 death per year for realistic populations
	}

	projection := &PopulationProjection{
		CurrentPopulation: stats.TotalActive,
		GrowthRate:        float64(annualBirths-annualDeaths) / float64(stats.TotalActive) * 100,
	}

	population := stats.TotalActive
	for y := 1; y <= years; y++ {
		// Recalculate rates each year (simplified)
		births := annualBirths
		deaths := annualDeaths

		// Adjust for population changes
		if population < 100 {
			births = int(float64(births) * float64(population) / 100)
		}

		netChange := births - deaths
		population += netChange

		if population < 0 {
			population = 0
		}

		projection.Projections = append(projection.Projections, ProjectionPoint{
			Year:       asOf.Year() + y,
			Population: population,
			Births:     births,
			Deaths:     deaths,
			NetChange:  netChange,
		})
	}

	// Assess viability
	projection.Viability = assessViability(stats.TotalActive, projection, ageDist, sexDist)

	return projection, nil
}

// assessViability evaluates the long-term viability of the population.
func assessViability(current int, projection *PopulationProjection, age *AgeDistribution, sex *SexDistribution) ViabilityAssessment {
	assessment := ViabilityAssessment{
		MinimumViable: 160, // Minimum viable population for genetic diversity
	}

	// Check current population
	if current >= assessment.MinimumViable {
		assessment.IsViable = true
	}

	// Analyze projections
	for i, point := range projection.Projections {
		if point.Population < assessment.MinimumViable && assessment.YearsToMVP == 0 {
			assessment.YearsToMVP = i + 1
			assessment.IsViable = false
		}
	}

	// Check for concerns
	if projection.GrowthRate < 0 {
		assessment.Concerns = append(assessment.Concerns,
			"Negative population growth rate detected")
	}

	if age.Seniors > age.YoungAdults+age.Adults {
		assessment.Concerns = append(assessment.Concerns,
			"Aging population: seniors outnumber working-age adults")
	}

	if age.Children+age.Infants < age.Seniors {
		assessment.Concerns = append(assessment.Concerns,
			"Declining youth population")
	}

	if sex.MaleRatio > 0.6 || sex.MaleRatio < 0.4 {
		assessment.Concerns = append(assessment.Concerns,
			"Imbalanced sex ratio may affect reproduction")
	}

	// Generate recommendations
	if projection.GrowthRate < 0.5 {
		assessment.Recommendations = append(assessment.Recommendations,
			"Consider incentives for family formation")
	}

	if age.Seniors > age.Adults/2 {
		assessment.Recommendations = append(assessment.Recommendations,
			"Prepare for increased elder care needs")
	}

	if current < 300 {
		assessment.Recommendations = append(assessment.Recommendations,
			"Monitor genetic diversity and inbreeding coefficients")
	}

	if len(assessment.Concerns) == 0 {
		assessment.Concerns = append(assessment.Concerns, "No immediate concerns")
	}

	if len(assessment.Recommendations) == 0 {
		assessment.Recommendations = append(assessment.Recommendations,
			"Maintain current population policies")
	}

	return assessment
}

// WorkforceStats contains workforce-related statistics.
type WorkforceStats struct {
	WorkingAge      int     // 16-65
	TrainingAge     int     // 16-17
	FullWorkforce   int     // 18-65
	RetirementAge   int     // 66+
	DependencyRatio float64 // Non-working / working
}

// GetWorkforceStats calculates workforce statistics.
func (s *Service) GetWorkforceStats(ctx context.Context, asOf time.Time) (*WorkforceStats, error) {
	filter := models.ResidentFilter{
		Status: ptr(models.ResidentStatusActive),
	}

	var allResidents []*models.Resident
	page := models.Pagination{Page: 1, PageSize: 100}

	for {
		result, err := s.residents.List(ctx, filter, page)
		if err != nil {
			return nil, err
		}
		allResidents = append(allResidents, result.Residents...)
		if page.Page >= result.TotalPages {
			break
		}
		page.Page++
	}

	stats := &WorkforceStats{}
	var dependents, workers int

	for _, r := range allResidents {
		age := r.Age(asOf)
		switch {
		case age >= 16 && age <= 17:
			stats.TrainingAge++
			stats.WorkingAge++
			workers++
		case age >= 18 && age <= 65:
			stats.FullWorkforce++
			stats.WorkingAge++
			workers++
		case age >= 66:
			stats.RetirementAge++
			dependents++
		default:
			dependents++ // Children
		}
	}

	if workers > 0 {
		stats.DependencyRatio = float64(dependents) / float64(workers)
	}

	return stats, nil
}

// Helper functions

func ptr[T any](v T) *T {
	return &v
}

func calculateMedian(values []int) float64 {
	if len(values) == 0 {
		return 0
	}

	// Simple sort (for small datasets)
	sorted := make([]int, len(values))
	copy(sorted, values)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return float64(sorted[mid-1]+sorted[mid]) / 2
	}
	return float64(sorted[mid])
}
