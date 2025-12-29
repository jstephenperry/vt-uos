package population

import (
	"context"
	"fmt"

	"github.com/vtuos/vtuos/internal/models"
)

// FamilyTree represents a family tree structure.
type FamilyTree struct {
	Root        *models.Resident
	Ancestors   map[string]*FamilyTreeNode
	Descendants map[string]*FamilyTreeNode
}

// FamilyTreeNode represents a node in the family tree.
type FamilyTreeNode struct {
	Resident   *models.Resident
	Generation int
	Parent1    *FamilyTreeNode
	Parent2    *FamilyTreeNode
	Children   []*FamilyTreeNode
}

// CalculateCOI calculates the Coefficient of Inbreeding for potential offspring
// of two parents using Wright's path coefficient method.
//
// COI = Σ (0.5)^(n1+n2+1) × (1 + FA)
//
// Where:
// - n1 = generations from individual to common ancestor through parent 1
// - n2 = generations from individual to common ancestor through parent 2
// - FA = COI of the common ancestor
//
// A COI > 0.0625 (first cousin level) is flagged as high risk.
func (s *Service) CalculateCOI(ctx context.Context, parent1ID, parent2ID string) (float64, error) {
	// Get ancestors for both parents (up to 5 generations)
	ancestors1, err := s.getAncestorMap(ctx, parent1ID, 5)
	if err != nil {
		return 0, fmt.Errorf("getting ancestors for parent 1: %w", err)
	}

	ancestors2, err := s.getAncestorMap(ctx, parent2ID, 5)
	if err != nil {
		return 0, fmt.Errorf("getting ancestors for parent 2: %w", err)
	}

	// Find common ancestors
	commonAncestors := make(map[string]struct {
		gen1 int
		gen2 int
	})

	for id, gen1 := range ancestors1 {
		if gen2, exists := ancestors2[id]; exists {
			commonAncestors[id] = struct {
				gen1 int
				gen2 int
			}{gen1, gen2}
		}
	}

	if len(commonAncestors) == 0 {
		return 0, nil // No common ancestors, COI = 0
	}

	// Calculate COI using Wright's formula
	var coi float64
	for id, gens := range commonAncestors {
		// Get the COI of the common ancestor (recursive, but cached)
		ancestorCOI := 0.0
		ancestor, err := s.residents.GetByID(ctx, id)
		if err == nil && ancestor.BiologicalParent1ID != nil && ancestor.BiologicalParent2ID != nil {
			// Recursively calculate ancestor's COI (simplified - just one level)
			ancestorCOI, _ = s.calculateSimpleCOI(ctx, *ancestor.BiologicalParent1ID, *ancestor.BiologicalParent2ID, 3)
		}

		// Path coefficient contribution
		// n1 = generations from parent1 to common ancestor
		// n2 = generations from parent2 to common ancestor
		// The offspring is one more generation away, so we add 1 to each
		pathCoef := pow(0.5, gens.gen1+gens.gen2+1) * (1 + ancestorCOI)
		coi += pathCoef
	}

	return coi, nil
}

// calculateSimpleCOI is a simplified COI calculation with depth limit.
func (s *Service) calculateSimpleCOI(ctx context.Context, parent1ID, parent2ID string, maxDepth int) (float64, error) {
	if maxDepth <= 0 {
		return 0, nil
	}

	ancestors1, err := s.getAncestorMap(ctx, parent1ID, maxDepth)
	if err != nil {
		return 0, err
	}

	ancestors2, err := s.getAncestorMap(ctx, parent2ID, maxDepth)
	if err != nil {
		return 0, err
	}

	var coi float64
	for id, gen1 := range ancestors1 {
		if gen2, exists := ancestors2[id]; exists {
			coi += pow(0.5, gen1+gen2+1)
			_ = id // suppress unused warning
		}
	}

	return coi, nil
}

// getAncestorMap builds a map of ancestor ID -> generation distance.
func (s *Service) getAncestorMap(ctx context.Context, residentID string, maxGenerations int) (map[string]int, error) {
	ancestors := make(map[string]int)
	visited := make(map[string]bool)

	var traverse func(id string, generation int) error
	traverse = func(id string, generation int) error {
		if generation > maxGenerations {
			return nil
		}
		if visited[id] {
			return nil
		}
		visited[id] = true

		resident, err := s.residents.GetByID(ctx, id)
		if err != nil {
			return nil // Ancestor not in database, stop traversal
		}

		// Record this ancestor if we haven't seen them at a closer generation
		if existing, exists := ancestors[id]; !exists || generation < existing {
			ancestors[id] = generation
		}

		// Traverse to parents
		if resident.BiologicalParent1ID != nil {
			if err := traverse(*resident.BiologicalParent1ID, generation+1); err != nil {
				return err
			}
		}
		if resident.BiologicalParent2ID != nil {
			if err := traverse(*resident.BiologicalParent2ID, generation+1); err != nil {
				return err
			}
		}

		return nil
	}

	resident, err := s.residents.GetByID(ctx, residentID)
	if err != nil {
		return nil, err
	}

	// Start traversal from parents
	if resident.BiologicalParent1ID != nil {
		if err := traverse(*resident.BiologicalParent1ID, 1); err != nil {
			return nil, err
		}
	}
	if resident.BiologicalParent2ID != nil {
		if err := traverse(*resident.BiologicalParent2ID, 1); err != nil {
			return nil, err
		}
	}

	return ancestors, nil
}

// GetAncestry returns the family tree of ancestors for a resident.
func (s *Service) GetAncestry(ctx context.Context, residentID string, generations int) (*FamilyTree, error) {
	resident, err := s.residents.GetByID(ctx, residentID)
	if err != nil {
		return nil, err
	}

	tree := &FamilyTree{
		Root:      resident,
		Ancestors: make(map[string]*FamilyTreeNode),
	}

	rootNode := &FamilyTreeNode{
		Resident:   resident,
		Generation: 0,
	}
	tree.Ancestors[resident.ID] = rootNode

	// Build ancestor tree
	if err := s.buildAncestorTree(ctx, rootNode, tree, generations); err != nil {
		return nil, err
	}

	return tree, nil
}

func (s *Service) buildAncestorTree(ctx context.Context, node *FamilyTreeNode, tree *FamilyTree, maxGen int) error {
	if node.Generation >= maxGen {
		return nil
	}

	resident := node.Resident

	// Get parent 1
	if resident.BiologicalParent1ID != nil {
		parent1, err := s.residents.GetByID(ctx, *resident.BiologicalParent1ID)
		if err == nil {
			parentNode := &FamilyTreeNode{
				Resident:   parent1,
				Generation: node.Generation + 1,
			}
			node.Parent1 = parentNode
			tree.Ancestors[parent1.ID] = parentNode

			if err := s.buildAncestorTree(ctx, parentNode, tree, maxGen); err != nil {
				return err
			}
		}
	}

	// Get parent 2
	if resident.BiologicalParent2ID != nil {
		parent2, err := s.residents.GetByID(ctx, *resident.BiologicalParent2ID)
		if err == nil {
			parentNode := &FamilyTreeNode{
				Resident:   parent2,
				Generation: node.Generation + 1,
			}
			node.Parent2 = parentNode
			tree.Ancestors[parent2.ID] = parentNode

			if err := s.buildAncestorTree(ctx, parentNode, tree, maxGen); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetDescendants returns the family tree of descendants for a resident.
func (s *Service) GetDescendants(ctx context.Context, residentID string, generations int) (*FamilyTree, error) {
	resident, err := s.residents.GetByID(ctx, residentID)
	if err != nil {
		return nil, err
	}

	tree := &FamilyTree{
		Root:        resident,
		Descendants: make(map[string]*FamilyTreeNode),
	}

	rootNode := &FamilyTreeNode{
		Resident:   resident,
		Generation: 0,
	}
	tree.Descendants[resident.ID] = rootNode

	// Build descendant tree
	if err := s.buildDescendantTree(ctx, rootNode, tree, generations); err != nil {
		return nil, err
	}

	return tree, nil
}

func (s *Service) buildDescendantTree(ctx context.Context, node *FamilyTreeNode, tree *FamilyTree, maxGen int) error {
	if node.Generation >= maxGen {
		return nil
	}

	children, err := s.residents.GetChildren(ctx, node.Resident.ID)
	if err != nil {
		return err
	}

	for _, child := range children {
		childNode := &FamilyTreeNode{
			Resident:   child,
			Generation: node.Generation + 1,
		}
		node.Children = append(node.Children, childNode)
		tree.Descendants[child.ID] = childNode

		if err := s.buildDescendantTree(ctx, childNode, tree, maxGen); err != nil {
			return err
		}
	}

	return nil
}

// FindCommonAncestors finds common ancestors between two residents.
func (s *Service) FindCommonAncestors(ctx context.Context, resident1ID, resident2ID string) ([]*models.Resident, error) {
	ancestors1, err := s.getAncestorMap(ctx, resident1ID, 5)
	if err != nil {
		return nil, err
	}

	ancestors2, err := s.getAncestorMap(ctx, resident2ID, 5)
	if err != nil {
		return nil, err
	}

	var common []*models.Resident
	for id := range ancestors1 {
		if _, exists := ancestors2[id]; exists {
			ancestor, err := s.residents.GetByID(ctx, id)
			if err == nil {
				common = append(common, ancestor)
			}
		}
	}

	return common, nil
}

// COIRiskLevel categorizes the risk level of a COI value.
type COIRiskLevel string

const (
	COIRiskNone     COIRiskLevel = "NONE"
	COIRiskLow      COIRiskLevel = "LOW"
	COIRiskModerate COIRiskLevel = "MODERATE"
	COIRiskHigh     COIRiskLevel = "HIGH"
	COIRiskCritical COIRiskLevel = "CRITICAL"
)

// AssessCOIRisk categorizes a COI value into risk levels.
func AssessCOIRisk(coi float64) COIRiskLevel {
	switch {
	case coi <= 0:
		return COIRiskNone
	case coi <= 0.0156: // Second cousin level
		return COIRiskLow
	case coi <= 0.0625: // First cousin level
		return COIRiskModerate
	case coi <= 0.125: // Half-sibling level
		return COIRiskHigh
	default: // Full sibling or closer
		return COIRiskCritical
	}
}

// pow calculates x^n for float64.
func pow(x float64, n int) float64 {
	result := 1.0
	for i := 0; i < n; i++ {
		result *= x
	}
	return result
}
