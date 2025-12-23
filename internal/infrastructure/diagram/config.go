// Package diagram provides DOT format generation for workflow visualization.
package diagram

import "fmt"

// Direction controls the graph layout direction.
type Direction string

const (
	DirectionTB Direction = "TB" // Top to Bottom (default)
	DirectionLR Direction = "LR" // Left to Right
	DirectionBT Direction = "BT" // Bottom to Top
	DirectionRL Direction = "RL" // Right to Left
)

// validDirections defines allowed direction values.
var validDirections = map[Direction]bool{
	DirectionTB: true,
	DirectionLR: true,
	DirectionBT: true,
	DirectionRL: true,
}

// IsValid checks if the direction is a valid DOT rankdir value.
func (d Direction) IsValid() bool {
	return validDirections[d]
}

// String returns the direction as a string.
func (d Direction) String() string {
	return string(d)
}

// NodeStyle defines visual style for step nodes in the diagram.
type NodeStyle struct {
	Shape     string // DOT shape: box, diamond, oval, hexagon, box3d, folder
	Color     string // border color
	FillColor string // background color
	Style     string // DOT style: filled, dashed, bold, etc.
}

// DiagramConfig holds configuration for diagram generation.
type DiagramConfig struct {
	Direction  Direction // graph layout direction
	OutputPath string    // file path for image export (empty = stdout)
	Highlight  string    // step name to highlight
	ShowLabels bool      // show transition labels
}

// Validate checks if the configuration is valid.
func (c *DiagramConfig) Validate() error {
	if c.Direction != "" && !c.Direction.IsValid() {
		return fmt.Errorf("invalid direction %q: must be one of TB, LR, BT, RL", c.Direction)
	}
	return nil
}

// NewDefaultConfig creates a DiagramConfig with sensible defaults.
func NewDefaultConfig() *DiagramConfig {
	return &DiagramConfig{
		Direction:  DirectionTB,
		ShowLabels: true,
	}
}
