package layout

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/lens"
)

// OverlapDetector detects and resolves panel overlaps
type OverlapDetector interface {
	DetectOverlaps(panels []lens.PanelConfig) []OverlapError
	DetectAllConflicts(panels []lens.PanelConfig) []ConflictReport
	SuggestResolution(panels []lens.PanelConfig, conflicts []OverlapError) []ResolutionSuggestion
	HasOverlaps(panels []lens.PanelConfig) bool
}

// OverlapError represents an overlap between two panels
type OverlapError struct {
	Panel1    string
	Panel2    string
	Overlap   OverlapRegion
	Severity  OverlapSeverity
	Message   string
}

func (e OverlapError) Error() string {
	return fmt.Sprintf("panels %s and %s overlap: %s", e.Panel1, e.Panel2, e.Message)
}

// OverlapRegion represents the overlapping area
type OverlapRegion struct {
	X      int
	Y      int
	Width  int
	Height int
}

// OverlapSeverity represents how severe the overlap is
type OverlapSeverity string

const (
	SeverityMinor    OverlapSeverity = "minor"    // Small overlap
	SeverityModerate OverlapSeverity = "moderate" // Significant overlap
	SeverityCritical OverlapSeverity = "critical" // Complete overlap
)

// ConflictReport provides detailed conflict analysis
type ConflictReport struct {
	TotalOverlaps     int
	CriticalOverlaps  int
	ModerateOverlaps  int
	MinorOverlaps     int
	AffectedPanels    []string
	OverlapDetails    []OverlapError
	GridUtilization   float64
}

// ResolutionSuggestion suggests how to resolve overlaps
type ResolutionSuggestion struct {
	Type        ResolutionType
	PanelID     string
	NewPosition lens.GridPosition
	NewSize     lens.GridDimensions
	Reason      string
	Priority    int
}

// ResolutionType represents the type of resolution
type ResolutionType string

const (
	ResolutionMove   ResolutionType = "move"   // Move panel to new position
	ResolutionResize ResolutionType = "resize" // Resize panel
	ResolutionStack  ResolutionType = "stack"  // Stack panels vertically
)

// overlapDetector is the default implementation
type overlapDetector struct{}

// NewOverlapDetector creates a new overlap detector
func NewOverlapDetector() OverlapDetector {
	return &overlapDetector{}
}

// DetectOverlaps detects all overlapping panel pairs
func (od *overlapDetector) DetectOverlaps(panels []lens.PanelConfig) []OverlapError {
	var overlaps []OverlapError
	
	for i, panel1 := range panels {
		for j, panel2 := range panels {
			if i >= j {
				continue // Avoid checking the same pair twice and self-comparison
			}
			
			if overlap := od.checkPanelOverlap(panel1, panel2); overlap != nil {
				overlaps = append(overlaps, *overlap)
			}
		}
	}
	
	return overlaps
}

// checkPanelOverlap checks if two panels overlap and returns overlap details
func (od *overlapDetector) checkPanelOverlap(panel1, panel2 lens.PanelConfig) *OverlapError {
	// Calculate panel boundaries
	p1Left := panel1.Position.X
	p1Right := panel1.Position.X + panel1.Dimensions.Width
	p1Top := panel1.Position.Y
	p1Bottom := panel1.Position.Y + panel1.Dimensions.Height
	
	p2Left := panel2.Position.X
	p2Right := panel2.Position.X + panel2.Dimensions.Width
	p2Top := panel2.Position.Y
	p2Bottom := panel2.Position.Y + panel2.Dimensions.Height
	
	// Check if panels overlap
	if p1Right <= p2Left || p2Right <= p1Left || p1Bottom <= p2Top || p2Bottom <= p1Top {
		return nil // No overlap
	}
	
	// Calculate overlap region
	overlapLeft := max(p1Left, p2Left)
	overlapRight := min(p1Right, p2Right)
	overlapTop := max(p1Top, p2Top)
	overlapBottom := min(p1Bottom, p2Bottom)
	
	overlapRegion := OverlapRegion{
		X:      overlapLeft,
		Y:      overlapTop,
		Width:  overlapRight - overlapLeft,
		Height: overlapBottom - overlapTop,
	}
	
	// Calculate overlap severity
	overlapArea := overlapRegion.Width * overlapRegion.Height
	panel1Area := panel1.Dimensions.Width * panel1.Dimensions.Height
	panel2Area := panel2.Dimensions.Width * panel2.Dimensions.Height
	
	minPanelArea := min(panel1Area, panel2Area)
	overlapPercentage := float64(overlapArea) / float64(minPanelArea)
	
	severity := od.calculateSeverity(overlapPercentage)
	
	return &OverlapError{
		Panel1:   panel1.ID,
		Panel2:   panel2.ID,
		Overlap:  overlapRegion,
		Severity: severity,
		Message:  fmt.Sprintf("%s overlap of %d units", severity, overlapArea),
	}
}

// calculateSeverity determines overlap severity based on percentage
func (od *overlapDetector) calculateSeverity(overlapPercentage float64) OverlapSeverity {
	switch {
	case overlapPercentage >= 0.75:
		return SeverityCritical
	case overlapPercentage >= 0.25:
		return SeverityModerate
	default:
		return SeverityMinor
	}
}

// DetectAllConflicts provides comprehensive conflict analysis
func (od *overlapDetector) DetectAllConflicts(panels []lens.PanelConfig) []ConflictReport {
	overlaps := od.DetectOverlaps(panels)
	
	if len(overlaps) == 0 {
		return []ConflictReport{{
			TotalOverlaps:   0,
			GridUtilization: od.calculateGridUtilization(panels),
		}}
	}
	
	// Count overlaps by severity
	var critical, moderate, minor int
	affectedPanels := make(map[string]bool)
	
	for _, overlap := range overlaps {
		affectedPanels[overlap.Panel1] = true
		affectedPanels[overlap.Panel2] = true
		
		switch overlap.Severity {
		case SeverityCritical:
			critical++
		case SeverityModerate:
			moderate++
		case SeverityMinor:
			minor++
		}
	}
	
	// Convert affected panels map to slice
	var affected []string
	for panelID := range affectedPanels {
		affected = append(affected, panelID)
	}
	
	report := ConflictReport{
		TotalOverlaps:     len(overlaps),
		CriticalOverlaps:  critical,
		ModerateOverlaps:  moderate,
		MinorOverlaps:     minor,
		AffectedPanels:    affected,
		OverlapDetails:    overlaps,
		GridUtilization:   od.calculateGridUtilization(panels),
	}
	
	return []ConflictReport{report}
}

// calculateGridUtilization calculates how much of the grid is utilized
func (od *overlapDetector) calculateGridUtilization(panels []lens.PanelConfig) float64 {
	if len(panels) == 0 {
		return 0.0
	}
	
	// Find grid bounds
	maxX, maxY := 0, 0
	totalPanelArea := 0
	
	for _, panel := range panels {
		right := panel.Position.X + panel.Dimensions.Width
		bottom := panel.Position.Y + panel.Dimensions.Height
		
		if right > maxX {
			maxX = right
		}
		if bottom > maxY {
			maxY = bottom
		}
		
		totalPanelArea += panel.Dimensions.Width * panel.Dimensions.Height
	}
	
	if maxX == 0 || maxY == 0 {
		return 0.0
	}
	
	gridArea := maxX * maxY
	return float64(totalPanelArea) / float64(gridArea)
}

// SuggestResolution suggests how to resolve overlaps
func (od *overlapDetector) SuggestResolution(panels []lens.PanelConfig, conflicts []OverlapError) []ResolutionSuggestion {
	var suggestions []ResolutionSuggestion
	
	// Group conflicts by panel
	panelConflicts := make(map[string][]OverlapError)
	for _, conflict := range conflicts {
		panelConflicts[conflict.Panel1] = append(panelConflicts[conflict.Panel1], conflict)
		panelConflicts[conflict.Panel2] = append(panelConflicts[conflict.Panel2], conflict)
	}
	
	// Generate suggestions for each conflicted panel
	for panelID, panelConflictsList := range panelConflicts {
		panel := od.findPanelByID(panels, panelID)
		if panel == nil {
			continue
		}
		
		suggestion := od.generateSuggestionForPanel(*panel, panelConflictsList)
		if suggestion != nil {
			suggestions = append(suggestions, *suggestion)
		}
	}
	
	return suggestions
}

// generateSuggestionForPanel generates a resolution suggestion for a specific panel
func (od *overlapDetector) generateSuggestionForPanel(panel lens.PanelConfig, conflicts []OverlapError) *ResolutionSuggestion {
	// For critical overlaps, suggest moving the panel
	for _, conflict := range conflicts {
		if conflict.Severity == SeverityCritical {
			// Find the next available position
			newPos := od.findNextAvailablePosition(panel)
			
			return &ResolutionSuggestion{
				Type:        ResolutionMove,
				PanelID:     panel.ID,
				NewPosition: newPos,
				NewSize:     panel.Dimensions,
				Reason:      fmt.Sprintf("Move to avoid critical overlap with panel %s", conflict.Panel2),
				Priority:    1,
			}
		}
	}
	
	// For moderate overlaps, suggest resizing
	for _, conflict := range conflicts {
		if conflict.Severity == SeverityModerate {
			// Suggest reducing width to avoid overlap
			newWidth := max(1, panel.Dimensions.Width-conflict.Overlap.Width)
			
			return &ResolutionSuggestion{
				Type:    ResolutionResize,
				PanelID: panel.ID,
				NewSize: lens.GridDimensions{
					Width:  newWidth,
					Height: panel.Dimensions.Height,
				},
				Reason:   fmt.Sprintf("Reduce width to avoid overlap with panel %s", conflict.Panel2),
				Priority: 2,
			}
		}
	}
	
	return nil
}

// findNextAvailablePosition finds the next available position for a panel
func (od *overlapDetector) findNextAvailablePosition(panel lens.PanelConfig) lens.GridPosition {
	// Simple strategy: move down by panel height
	return lens.GridPosition{
		X: panel.Position.X,
		Y: panel.Position.Y + panel.Dimensions.Height + 1,
	}
}

// findPanelByID finds a panel by its ID
func (od *overlapDetector) findPanelByID(panels []lens.PanelConfig, id string) *lens.PanelConfig {
	for _, panel := range panels {
		if panel.ID == id {
			return &panel
		}
	}
	return nil
}

// HasOverlaps returns true if any panels overlap
func (od *overlapDetector) HasOverlaps(panels []lens.PanelConfig) bool {
	overlaps := od.DetectOverlaps(panels)
	return len(overlaps) > 0
}