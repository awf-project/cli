package diagram

import (
	"testing"
)

func TestDirection_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		direction Direction
		want      bool
	}{
		{"TB is valid", DirectionTB, true},
		{"LR is valid", DirectionLR, true},
		{"BT is valid", DirectionBT, true},
		{"RL is valid", DirectionRL, true},
		{"empty is invalid", Direction(""), false},
		{"lowercase tb is invalid", Direction("tb"), false},
		{"unknown direction is invalid", Direction("XX"), false},
		{"partial match is invalid", Direction("T"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.direction.IsValid(); got != tt.want {
				t.Errorf("Direction(%q).IsValid() = %v, want %v", tt.direction, got, tt.want)
			}
		})
	}
}

func TestDirection_String(t *testing.T) {
	tests := []struct {
		direction Direction
		want      string
	}{
		{DirectionTB, "TB"},
		{DirectionLR, "LR"},
		{DirectionBT, "BT"},
		{DirectionRL, "RL"},
		{Direction(""), ""},
		{Direction("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.direction.String(); got != tt.want {
				t.Errorf("Direction.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDirection_Constants(t *testing.T) {
	// Verify constant values match DOT rankdir expectations
	if DirectionTB != "TB" {
		t.Errorf("DirectionTB = %q, want %q", DirectionTB, "TB")
	}
	if DirectionLR != "LR" {
		t.Errorf("DirectionLR = %q, want %q", DirectionLR, "LR")
	}
	if DirectionBT != "BT" {
		t.Errorf("DirectionBT = %q, want %q", DirectionBT, "BT")
	}
	if DirectionRL != "RL" {
		t.Errorf("DirectionRL = %q, want %q", DirectionRL, "RL")
	}
}

func TestNodeStyle_ZeroValue(t *testing.T) {
	style := NodeStyle{}

	if style.Shape != "" {
		t.Errorf("NodeStyle.Shape zero value = %q, want empty", style.Shape)
	}
	if style.Color != "" {
		t.Errorf("NodeStyle.Color zero value = %q, want empty", style.Color)
	}
	if style.FillColor != "" {
		t.Errorf("NodeStyle.FillColor zero value = %q, want empty", style.FillColor)
	}
	if style.Style != "" {
		t.Errorf("NodeStyle.Style zero value = %q, want empty", style.Style)
	}
}

func TestNodeStyle_WithValues(t *testing.T) {
	style := NodeStyle{
		Shape:     "box",
		Color:     "black",
		FillColor: "lightblue",
		Style:     "filled",
	}

	if style.Shape != "box" {
		t.Errorf("NodeStyle.Shape = %q, want %q", style.Shape, "box")
	}
	if style.Color != "black" {
		t.Errorf("NodeStyle.Color = %q, want %q", style.Color, "black")
	}
	if style.FillColor != "lightblue" {
		t.Errorf("NodeStyle.FillColor = %q, want %q", style.FillColor, "lightblue")
	}
	if style.Style != "filled" {
		t.Errorf("NodeStyle.Style = %q, want %q", style.Style, "filled")
	}
}

func TestDiagramConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *DiagramConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config with TB direction",
			config:  &DiagramConfig{Direction: DirectionTB},
			wantErr: false,
		},
		{
			name:    "valid config with LR direction",
			config:  &DiagramConfig{Direction: DirectionLR},
			wantErr: false,
		},
		{
			name:    "valid config with BT direction",
			config:  &DiagramConfig{Direction: DirectionBT},
			wantErr: false,
		},
		{
			name:    "valid config with RL direction",
			config:  &DiagramConfig{Direction: DirectionRL},
			wantErr: false,
		},
		{
			name:    "valid config with empty direction",
			config:  &DiagramConfig{Direction: ""},
			wantErr: false,
		},
		{
			name:    "invalid direction lowercase",
			config:  &DiagramConfig{Direction: Direction("tb")},
			wantErr: true,
			errMsg:  `invalid direction "tb"`,
		},
		{
			name:    "invalid direction unknown",
			config:  &DiagramConfig{Direction: Direction("INVALID")},
			wantErr: true,
			errMsg:  `invalid direction "INVALID"`,
		},
		{
			name: "valid config with all fields",
			config: &DiagramConfig{
				Direction:  DirectionLR,
				OutputPath: "/tmp/workflow.png",
				Highlight:  "step_1",
				ShowLabels: true,
			},
			wantErr: false,
		},
		{
			name: "valid config with output path only",
			config: &DiagramConfig{
				OutputPath: "output.svg",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DiagramConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, should contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestDiagramConfig_ZeroValue(t *testing.T) {
	cfg := &DiagramConfig{}

	if cfg.Direction != "" {
		t.Errorf("DiagramConfig.Direction zero value = %q, want empty", cfg.Direction)
	}
	if cfg.OutputPath != "" {
		t.Errorf("DiagramConfig.OutputPath zero value = %q, want empty", cfg.OutputPath)
	}
	if cfg.Highlight != "" {
		t.Errorf("DiagramConfig.Highlight zero value = %q, want empty", cfg.Highlight)
	}
	if cfg.ShowLabels != false {
		t.Errorf("DiagramConfig.ShowLabels zero value = %v, want false", cfg.ShowLabels)
	}
}

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg == nil {
		t.Fatal("NewDefaultConfig() returned nil")
	}

	if cfg.Direction != DirectionTB {
		t.Errorf("default Direction = %q, want %q", cfg.Direction, DirectionTB)
	}
	if cfg.OutputPath != "" {
		t.Errorf("default OutputPath = %q, want empty", cfg.OutputPath)
	}
	if cfg.Highlight != "" {
		t.Errorf("default Highlight = %q, want empty", cfg.Highlight)
	}
	if cfg.ShowLabels != true {
		t.Errorf("default ShowLabels = %v, want true", cfg.ShowLabels)
	}
}

func TestNewDefaultConfig_IsValid(t *testing.T) {
	cfg := NewDefaultConfig()

	err := cfg.Validate()
	if err != nil {
		t.Errorf("NewDefaultConfig().Validate() = %v, want nil", err)
	}
}

func TestNewDefaultConfig_ReturnsNewInstance(t *testing.T) {
	cfg1 := NewDefaultConfig()
	cfg2 := NewDefaultConfig()

	if cfg1 == cfg2 {
		t.Error("NewDefaultConfig() should return new instance each time")
	}

	// Modify cfg1 and verify cfg2 is unchanged
	cfg1.Direction = DirectionLR
	cfg1.Highlight = "modified"

	if cfg2.Direction != DirectionTB {
		t.Error("modifying one config affected another")
	}
}

func TestDiagramConfig_OutputPathVariations(t *testing.T) {
	tests := []struct {
		name       string
		outputPath string
	}{
		{"empty path", ""},
		{"png extension", "diagram.png"},
		{"svg extension", "diagram.svg"},
		{"pdf extension", "diagram.pdf"},
		{"dot extension", "diagram.dot"},
		{"absolute path", "/tmp/output/workflow.png"},
		{"relative path", "./output/workflow.svg"},
		{"path with spaces", "/tmp/my workflow/diagram.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &DiagramConfig{OutputPath: tt.outputPath}
			err := cfg.Validate()
			if err != nil {
				t.Errorf("OutputPath %q should be valid, got error: %v", tt.outputPath, err)
			}
		})
	}
}

func TestDiagramConfig_HighlightVariations(t *testing.T) {
	tests := []struct {
		name      string
		highlight string
	}{
		{"empty highlight", ""},
		{"simple step name", "step_1"},
		{"step with dashes", "my-step-name"},
		{"step with numbers", "step123"},
		{"complex name", "process_data_step"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &DiagramConfig{Highlight: tt.highlight}
			err := cfg.Validate()
			if err != nil {
				t.Errorf("Highlight %q should be valid, got error: %v", tt.highlight, err)
			}
		})
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
