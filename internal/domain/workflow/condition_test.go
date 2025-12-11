package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransitions_HasDefault(t *testing.T) {
	tests := []struct {
		name        string
		transitions Transitions
		want        bool
	}{
		{
			name:        "empty transitions",
			transitions: Transitions{},
			want:        false,
		},
		{
			name: "only conditional transitions",
			transitions: Transitions{
				{When: `inputs.mode == "full"`, Goto: "full"},
				{When: `inputs.mode == "summary"`, Goto: "summary"},
			},
			want: false,
		},
		{
			name: "with default transition at end",
			transitions: Transitions{
				{When: `inputs.mode == "full"`, Goto: "full"},
				{Goto: "fallback"}, // default (empty When)
			},
			want: true,
		},
		{
			name: "default transition only",
			transitions: Transitions{
				{Goto: "next"},
			},
			want: true,
		},
		{
			name: "default transition in middle",
			transitions: Transitions{
				{When: `inputs.mode == "full"`, Goto: "full"},
				{Goto: "fallback"},
				{When: `inputs.mode == "summary"`, Goto: "summary"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.transitions.HasDefault()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTransition_Validate(t *testing.T) {
	tests := []struct {
		name       string
		transition Transition
		wantErr    bool
	}{
		{
			name:       "valid conditional transition",
			transition: Transition{When: `inputs.mode == "full"`, Goto: "full_report"},
			wantErr:    false,
		},
		{
			name:       "valid default transition (no when)",
			transition: Transition{Goto: "fallback"},
			wantErr:    false,
		},
		{
			name:       "invalid - empty goto",
			transition: Transition{When: `inputs.mode == "full"`, Goto: ""},
			wantErr:    true,
		},
		{
			name:       "invalid - empty transition entirely",
			transition: Transition{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.transition.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTransitions_Validate(t *testing.T) {
	tests := []struct {
		name        string
		transitions Transitions
		wantErr     bool
	}{
		{
			name:        "empty transitions - valid",
			transitions: Transitions{},
			wantErr:     false,
		},
		{
			name: "valid transitions with default",
			transitions: Transitions{
				{When: `inputs.a == 1`, Goto: "step_a"},
				{When: `inputs.b == 2`, Goto: "step_b"},
				{Goto: "default"},
			},
			wantErr: false,
		},
		{
			name: "valid transitions without default",
			transitions: Transitions{
				{When: `inputs.a == 1`, Goto: "step_a"},
				{When: `inputs.b == 2`, Goto: "step_b"},
			},
			wantErr: false,
		},
		{
			name: "invalid - transition with empty goto",
			transitions: Transitions{
				{When: `inputs.a == 1`, Goto: ""},
			},
			wantErr: true,
		},
		{
			name: "invalid - one valid one invalid",
			transitions: Transitions{
				{When: `inputs.a == 1`, Goto: "step_a"},
				{Goto: ""}, // invalid
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.transitions.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTransitions_GetTargetStates(t *testing.T) {
	tests := []struct {
		name        string
		transitions Transitions
		want        []string
	}{
		{
			name:        "empty transitions",
			transitions: Transitions{},
			want:        []string{},
		},
		{
			name: "single transition",
			transitions: Transitions{
				{Goto: "next"},
			},
			want: []string{"next"},
		},
		{
			name: "multiple transitions",
			transitions: Transitions{
				{When: `inputs.mode == "full"`, Goto: "full_report"},
				{When: `inputs.mode == "summary"`, Goto: "summary_report"},
				{Goto: "error"},
			},
			want: []string{"full_report", "summary_report", "error"},
		},
		{
			name: "duplicate targets",
			transitions: Transitions{
				{When: `inputs.a == 1`, Goto: "same"},
				{When: `inputs.b == 2`, Goto: "same"},
			},
			want: []string{"same", "same"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.transitions.GetTargetStates()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTransitions_FirstMatchingIndex(t *testing.T) {
	// This tests the "first match wins" semantics
	// The actual evaluation will be done by the evaluator, but we test the structure
	tests := []struct {
		name        string
		transitions Transitions
		wantDefault int // index of default transition, -1 if none
	}{
		{
			name:        "empty transitions",
			transitions: Transitions{},
			wantDefault: -1,
		},
		{
			name: "default at end",
			transitions: Transitions{
				{When: `inputs.mode == "full"`, Goto: "full"},
				{Goto: "default"},
			},
			wantDefault: 1,
		},
		{
			name: "default in middle (bad practice but valid)",
			transitions: Transitions{
				{When: `inputs.a == 1`, Goto: "a"},
				{Goto: "default"},
				{When: `inputs.b == 2`, Goto: "b"},
			},
			wantDefault: 1,
		},
		{
			name: "no default",
			transitions: Transitions{
				{When: `inputs.a == 1`, Goto: "a"},
				{When: `inputs.b == 2`, Goto: "b"},
			},
			wantDefault: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.transitions.DefaultIndex()
			assert.Equal(t, tt.wantDefault, got)
		})
	}
}

// Mock evaluator for testing transition evaluation
type mockEvaluator struct {
	results map[string]bool
	err     error
}

func (m *mockEvaluator) Evaluate(expr string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	if result, ok := m.results[expr]; ok {
		return result, nil
	}
	return false, nil
}

func TestTransitions_EvaluateFirstMatch(t *testing.T) {
	tests := []struct {
		name        string
		transitions Transitions
		evaluator   *mockEvaluator
		wantGoto    string
		wantFound   bool
		wantErr     bool
	}{
		{
			name:        "empty transitions - no match",
			transitions: Transitions{},
			evaluator:   &mockEvaluator{results: map[string]bool{}},
			wantGoto:    "",
			wantFound:   false,
			wantErr:     false,
		},
		{
			name: "first condition matches",
			transitions: Transitions{
				{When: `inputs.mode == "full"`, Goto: "full"},
				{When: `inputs.mode == "summary"`, Goto: "summary"},
				{Goto: "default"},
			},
			evaluator: &mockEvaluator{results: map[string]bool{`inputs.mode == "full"`: true}},
			wantGoto:  "full",
			wantFound: true,
			wantErr:   false,
		},
		{
			name: "second condition matches",
			transitions: Transitions{
				{When: `inputs.mode == "full"`, Goto: "full"},
				{When: `inputs.mode == "summary"`, Goto: "summary"},
				{Goto: "default"},
			},
			evaluator: &mockEvaluator{results: map[string]bool{`inputs.mode == "summary"`: true}},
			wantGoto:  "summary",
			wantFound: true,
			wantErr:   false,
		},
		{
			name: "no condition matches - falls to default",
			transitions: Transitions{
				{When: `inputs.mode == "full"`, Goto: "full"},
				{When: `inputs.mode == "summary"`, Goto: "summary"},
				{Goto: "default"},
			},
			evaluator: &mockEvaluator{results: map[string]bool{}},
			wantGoto:  "default",
			wantFound: true,
			wantErr:   false,
		},
		{
			name: "no condition matches - no default",
			transitions: Transitions{
				{When: `inputs.mode == "full"`, Goto: "full"},
				{When: `inputs.mode == "summary"`, Goto: "summary"},
			},
			evaluator: &mockEvaluator{results: map[string]bool{}},
			wantGoto:  "",
			wantFound: false,
			wantErr:   false,
		},
		{
			name: "only default transition",
			transitions: Transitions{
				{Goto: "next"},
			},
			evaluator: &mockEvaluator{results: map[string]bool{}},
			wantGoto:  "next",
			wantFound: true,
			wantErr:   false,
		},
		{
			name: "evaluator returns error",
			transitions: Transitions{
				{When: `invalid expression`, Goto: "next"},
			},
			evaluator: &mockEvaluator{err: assert.AnError},
			wantGoto:  "",
			wantFound: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGoto, gotFound, err := tt.transitions.EvaluateFirstMatch(tt.evaluator.Evaluate)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantGoto, gotGoto)
			assert.Equal(t, tt.wantFound, gotFound)
		})
	}
}

func TestTransition_String(t *testing.T) {
	tests := []struct {
		name       string
		transition Transition
		want       string
	}{
		{
			name:       "conditional transition",
			transition: Transition{When: `inputs.mode == "full"`, Goto: "full_report"},
			want:       `when 'inputs.mode == "full"' goto full_report`,
		},
		{
			name:       "default transition",
			transition: Transition{Goto: "fallback"},
			want:       "goto fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.transition.String()
			assert.Equal(t, tt.want, got)
		})
	}
}
