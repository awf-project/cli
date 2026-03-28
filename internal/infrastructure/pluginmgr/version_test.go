package pluginmgr

import (
	"testing"
)

// --- ParseVersion Tests ---

func TestParseVersion_Valid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Version
		wantErr bool
	}{
		{
			name:  "simple version",
			input: "1.0.0",
			want:  Version{Major: 1, Minor: 0, Patch: 0},
		},
		{
			name:  "with minor and patch",
			input: "2.3.4",
			want:  Version{Major: 2, Minor: 3, Patch: 4},
		},
		{
			name:  "zero version",
			input: "0.0.0",
			want:  Version{Major: 0, Minor: 0, Patch: 0},
		},
		{
			name:  "large numbers",
			input: "100.200.300",
			want:  Version{Major: 100, Minor: 200, Patch: 300},
		},
		{
			name:  "with prerelease",
			input: "1.0.0-alpha",
			want:  Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
		},
		{
			name:  "with numeric prerelease",
			input: "1.0.0-alpha.1",
			want:  Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"},
		},
		{
			name:  "with beta prerelease",
			input: "2.0.0-beta.2",
			want:  Version{Major: 2, Minor: 0, Patch: 0, Prerelease: "beta.2"},
		},
		{
			name:  "with rc prerelease",
			input: "3.0.0-rc.1",
			want:  Version{Major: 3, Minor: 0, Patch: 0, Prerelease: "rc.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseVersion(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseVersion_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "single number", input: "1"},
		{name: "two numbers", input: "1.0"},
		{name: "four numbers", input: "1.0.0.0"},
		{name: "letters in version", input: "a.b.c"},
		{name: "leading zeros", input: "01.0.0"},
		{name: "negative numbers", input: "-1.0.0"},
		{name: "spaces", input: "1 . 0 . 0"},
		{name: "leading v", input: "v1.0.0"},
		{name: "trailing text", input: "1.0.0foo"},
		{name: "empty prerelease", input: "1.0.0-"},
		{name: "double dash", input: "1.0.0--alpha"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseVersion(tt.input)
			if err == nil {
				t.Errorf("ParseVersion(%q) error = nil, want error", tt.input)
			}
		})
	}
}

// --- Version.String Tests ---

func TestVersion_String(t *testing.T) {
	tests := []struct {
		name    string
		version Version
		want    string
	}{
		{
			name:    "simple version",
			version: Version{Major: 1, Minor: 0, Patch: 0},
			want:    "1.0.0",
		},
		{
			name:    "with all parts",
			version: Version{Major: 2, Minor: 3, Patch: 4},
			want:    "2.3.4",
		},
		{
			name:    "with prerelease",
			version: Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"},
			want:    "1.0.0-alpha.1",
		},
		{
			name:    "zero version",
			version: Version{Major: 0, Minor: 0, Patch: 0},
			want:    "0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.version.String()
			if got != tt.want {
				t.Errorf("Version.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Version.Compare Tests ---

func TestVersion_Compare(t *testing.T) {
	tests := []struct {
		name string
		v1   Version
		v2   Version
		want int // -1: v1 < v2, 0: v1 == v2, 1: v1 > v2
	}{
		// Equal versions
		{
			name: "equal simple",
			v1:   Version{Major: 1, Minor: 0, Patch: 0},
			v2:   Version{Major: 1, Minor: 0, Patch: 0},
			want: 0,
		},
		{
			name: "equal with prerelease",
			v1:   Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			v2:   Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want: 0,
		},

		// Major differences
		{
			name: "major greater",
			v1:   Version{Major: 2, Minor: 0, Patch: 0},
			v2:   Version{Major: 1, Minor: 0, Patch: 0},
			want: 1,
		},
		{
			name: "major less",
			v1:   Version{Major: 1, Minor: 0, Patch: 0},
			v2:   Version{Major: 2, Minor: 0, Patch: 0},
			want: -1,
		},

		// Minor differences
		{
			name: "minor greater",
			v1:   Version{Major: 1, Minor: 2, Patch: 0},
			v2:   Version{Major: 1, Minor: 1, Patch: 0},
			want: 1,
		},
		{
			name: "minor less",
			v1:   Version{Major: 1, Minor: 1, Patch: 0},
			v2:   Version{Major: 1, Minor: 2, Patch: 0},
			want: -1,
		},

		// Patch differences
		{
			name: "patch greater",
			v1:   Version{Major: 1, Minor: 0, Patch: 2},
			v2:   Version{Major: 1, Minor: 0, Patch: 1},
			want: 1,
		},
		{
			name: "patch less",
			v1:   Version{Major: 1, Minor: 0, Patch: 1},
			v2:   Version{Major: 1, Minor: 0, Patch: 2},
			want: -1,
		},

		// Prerelease comparisons (prerelease < release)
		{
			name: "prerelease less than release",
			v1:   Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			v2:   Version{Major: 1, Minor: 0, Patch: 0},
			want: -1,
		},
		{
			name: "release greater than prerelease",
			v1:   Version{Major: 1, Minor: 0, Patch: 0},
			v2:   Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want: 1,
		},

		// Prerelease ordering
		{
			name: "alpha less than beta",
			v1:   Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			v2:   Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta"},
			want: -1,
		},
		{
			name: "alpha.1 less than alpha.2",
			v1:   Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"},
			v2:   Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.2"},
			want: -1,
		},

		// Complex comparisons
		{
			name: "0.4.0 less than 1.0.0",
			v1:   Version{Major: 0, Minor: 4, Patch: 0},
			v2:   Version{Major: 1, Minor: 0, Patch: 0},
			want: -1,
		},
		{
			name: "0.5.0 greater than 0.4.0",
			v1:   Version{Major: 0, Minor: 5, Patch: 0},
			v2:   Version{Major: 0, Minor: 4, Patch: 0},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v1.Compare(tt.v2)
			if got != tt.want {
				t.Errorf("Version{%v}.Compare(Version{%v}) = %d, want %d", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

// --- ParseConstraint Tests ---

func TestParseConstraint_Valid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOp  string
		wantVer Version
	}{
		{
			name:    "equal",
			input:   "==1.0.0",
			wantOp:  OpEqual,
			wantVer: Version{Major: 1, Minor: 0, Patch: 0},
		},
		{
			name:    "equal with space",
			input:   "== 1.0.0",
			wantOp:  OpEqual,
			wantVer: Version{Major: 1, Minor: 0, Patch: 0},
		},
		{
			name:    "not equal",
			input:   "!=1.0.0",
			wantOp:  OpNotEqual,
			wantVer: Version{Major: 1, Minor: 0, Patch: 0},
		},
		{
			name:    "greater than",
			input:   ">1.0.0",
			wantOp:  OpGreater,
			wantVer: Version{Major: 1, Minor: 0, Patch: 0},
		},
		{
			name:    "greater or equal",
			input:   ">=0.4.0",
			wantOp:  OpGreaterOrEqual,
			wantVer: Version{Major: 0, Minor: 4, Patch: 0},
		},
		{
			name:    "less than",
			input:   "<2.0.0",
			wantOp:  OpLess,
			wantVer: Version{Major: 2, Minor: 0, Patch: 0},
		},
		{
			name:    "less or equal",
			input:   "<=1.5.0",
			wantOp:  OpLessOrEqual,
			wantVer: Version{Major: 1, Minor: 5, Patch: 0},
		},
		{
			name:    "tilde",
			input:   "~1.2.0",
			wantOp:  OpTilde,
			wantVer: Version{Major: 1, Minor: 2, Patch: 0},
		},
		{
			name:    "caret",
			input:   "^2.0.0",
			wantOp:  OpCaret,
			wantVer: Version{Major: 2, Minor: 0, Patch: 0},
		},
		{
			name:    "bare version implies equal",
			input:   "1.0.0",
			wantOp:  OpEqual,
			wantVer: Version{Major: 1, Minor: 0, Patch: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConstraint(tt.input)
			if err != nil {
				t.Fatalf("ParseConstraint(%q) error = %v", tt.input, err)
			}
			if got.Operator != tt.wantOp {
				t.Errorf("ParseConstraint(%q).Operator = %q, want %q", tt.input, got.Operator, tt.wantOp)
			}
			if got.Version != tt.wantVer {
				t.Errorf("ParseConstraint(%q).Version = %+v, want %+v", tt.input, got.Version, tt.wantVer)
			}
		})
	}
}

func TestParseConstraint_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "only operator", input: ">="},
		{name: "invalid operator", input: "<>1.0.0"},
		{name: "triple equals", input: "===1.0.0"},
		{name: "invalid version", input: ">=abc"},
		{name: "incomplete version", input: ">=1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConstraint(tt.input)
			if err == nil {
				t.Errorf("ParseConstraint(%q) error = nil, want error", tt.input)
			}
		})
	}
}

// --- ParseConstraints Tests ---

func TestParseConstraints_Valid(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
	}{
		{
			name:      "single constraint",
			input:     ">=0.4.0",
			wantCount: 1,
		},
		{
			name:      "two constraints with space",
			input:     ">=0.4.0 <1.0.0",
			wantCount: 2,
		},
		{
			name:      "two constraints with comma",
			input:     ">=0.4.0, <1.0.0",
			wantCount: 2,
		},
		{
			name:      "three constraints",
			input:     ">=0.4.0 <1.0.0 !=0.5.0",
			wantCount: 3,
		},
		{
			name:      "mixed delimiters",
			input:     ">=0.4.0, <1.0.0 !=0.5.0",
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConstraints(tt.input)
			if err != nil {
				t.Fatalf("ParseConstraints(%q) error = %v", tt.input, err)
			}
			if len(got) != tt.wantCount {
				t.Errorf("ParseConstraints(%q) returned %d constraints, want %d", tt.input, len(got), tt.wantCount)
			}
		})
	}
}

func TestParseConstraints_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "only spaces", input: "   "},
		{name: "only commas", input: ",,,"},
		{name: "invalid constraint in list", input: ">=0.4.0 invalid <1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConstraints(tt.input)
			if err == nil {
				t.Errorf("ParseConstraints(%q) error = nil, want error", tt.input)
			}
		})
	}
}

// --- Constraint.Check Tests ---

func TestConstraint_Check(t *testing.T) {
	tests := []struct {
		name       string
		constraint Constraint
		version    Version
		want       bool
	}{
		// Equal operator
		{
			name:       "equal matches",
			constraint: Constraint{Operator: OpEqual, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 1, Minor: 0, Patch: 0},
			want:       true,
		},
		{
			name:       "equal not matches",
			constraint: Constraint{Operator: OpEqual, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 1, Minor: 0, Patch: 1},
			want:       false,
		},

		// Not equal operator
		{
			name:       "not equal matches",
			constraint: Constraint{Operator: OpNotEqual, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 1, Minor: 0, Patch: 1},
			want:       true,
		},
		{
			name:       "not equal not matches",
			constraint: Constraint{Operator: OpNotEqual, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 1, Minor: 0, Patch: 0},
			want:       false,
		},

		// Greater than operator
		{
			name:       "greater than matches",
			constraint: Constraint{Operator: OpGreater, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 1, Minor: 0, Patch: 1},
			want:       true,
		},
		{
			name:       "greater than not matches equal",
			constraint: Constraint{Operator: OpGreater, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 1, Minor: 0, Patch: 0},
			want:       false,
		},
		{
			name:       "greater than not matches less",
			constraint: Constraint{Operator: OpGreater, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 0, Minor: 9, Patch: 0},
			want:       false,
		},

		// Greater or equal operator
		{
			name:       "greater or equal matches greater",
			constraint: Constraint{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
			version:    Version{Major: 0, Minor: 5, Patch: 0},
			want:       true,
		},
		{
			name:       "greater or equal matches equal",
			constraint: Constraint{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
			version:    Version{Major: 0, Minor: 4, Patch: 0},
			want:       true,
		},
		{
			name:       "greater or equal not matches",
			constraint: Constraint{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
			version:    Version{Major: 0, Minor: 3, Patch: 0},
			want:       false,
		},

		// Less than operator
		{
			name:       "less than matches",
			constraint: Constraint{Operator: OpLess, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 0, Minor: 9, Patch: 0},
			want:       true,
		},
		{
			name:       "less than not matches equal",
			constraint: Constraint{Operator: OpLess, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 1, Minor: 0, Patch: 0},
			want:       false,
		},
		{
			name:       "less than not matches greater",
			constraint: Constraint{Operator: OpLess, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 1, Minor: 0, Patch: 1},
			want:       false,
		},

		// Less or equal operator
		{
			name:       "less or equal matches less",
			constraint: Constraint{Operator: OpLessOrEqual, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 0, Minor: 9, Patch: 0},
			want:       true,
		},
		{
			name:       "less or equal matches equal",
			constraint: Constraint{Operator: OpLessOrEqual, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 1, Minor: 0, Patch: 0},
			want:       true,
		},
		{
			name:       "less or equal not matches",
			constraint: Constraint{Operator: OpLessOrEqual, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			version:    Version{Major: 1, Minor: 0, Patch: 1},
			want:       false,
		},

		// Tilde operator (~1.2.0 allows 1.2.x but not 1.3.0)
		{
			name:       "tilde matches same",
			constraint: Constraint{Operator: OpTilde, Version: Version{Major: 1, Minor: 2, Patch: 0}},
			version:    Version{Major: 1, Minor: 2, Patch: 0},
			want:       true,
		},
		{
			name:       "tilde matches patch update",
			constraint: Constraint{Operator: OpTilde, Version: Version{Major: 1, Minor: 2, Patch: 0}},
			version:    Version{Major: 1, Minor: 2, Patch: 5},
			want:       true,
		},
		{
			name:       "tilde not matches minor update",
			constraint: Constraint{Operator: OpTilde, Version: Version{Major: 1, Minor: 2, Patch: 0}},
			version:    Version{Major: 1, Minor: 3, Patch: 0},
			want:       false,
		},
		{
			name:       "tilde not matches major update",
			constraint: Constraint{Operator: OpTilde, Version: Version{Major: 1, Minor: 2, Patch: 0}},
			version:    Version{Major: 2, Minor: 0, Patch: 0},
			want:       false,
		},
		{
			name:       "tilde not matches lower",
			constraint: Constraint{Operator: OpTilde, Version: Version{Major: 1, Minor: 2, Patch: 0}},
			version:    Version{Major: 1, Minor: 1, Patch: 9},
			want:       false,
		},

		// Caret operator (^1.2.0 allows 1.x.x but not 2.0.0)
		{
			name:       "caret matches same",
			constraint: Constraint{Operator: OpCaret, Version: Version{Major: 1, Minor: 2, Patch: 0}},
			version:    Version{Major: 1, Minor: 2, Patch: 0},
			want:       true,
		},
		{
			name:       "caret matches patch update",
			constraint: Constraint{Operator: OpCaret, Version: Version{Major: 1, Minor: 2, Patch: 0}},
			version:    Version{Major: 1, Minor: 2, Patch: 5},
			want:       true,
		},
		{
			name:       "caret matches minor update",
			constraint: Constraint{Operator: OpCaret, Version: Version{Major: 1, Minor: 2, Patch: 0}},
			version:    Version{Major: 1, Minor: 5, Patch: 0},
			want:       true,
		},
		{
			name:       "caret not matches major update",
			constraint: Constraint{Operator: OpCaret, Version: Version{Major: 1, Minor: 2, Patch: 0}},
			version:    Version{Major: 2, Minor: 0, Patch: 0},
			want:       false,
		},
		{
			name:       "caret not matches lower minor",
			constraint: Constraint{Operator: OpCaret, Version: Version{Major: 1, Minor: 2, Patch: 0}},
			version:    Version{Major: 1, Minor: 1, Patch: 0},
			want:       false,
		},

		// Special case: caret with 0.x versions (^0.2.0 allows 0.2.x only)
		{
			name:       "caret 0.x matches patch",
			constraint: Constraint{Operator: OpCaret, Version: Version{Major: 0, Minor: 2, Patch: 0}},
			version:    Version{Major: 0, Minor: 2, Patch: 5},
			want:       true,
		},
		{
			name:       "caret 0.x not matches minor",
			constraint: Constraint{Operator: OpCaret, Version: Version{Major: 0, Minor: 2, Patch: 0}},
			version:    Version{Major: 0, Minor: 3, Patch: 0},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.constraint.Check(tt.version)
			if got != tt.want {
				t.Errorf("Constraint{%s %v}.Check(%v) = %v, want %v",
					tt.constraint.Operator, tt.constraint.Version, tt.version, got, tt.want)
			}
		})
	}
}

// --- Constraints.Check Tests ---

func TestConstraints_Check(t *testing.T) {
	tests := []struct {
		name        string
		constraints Constraints
		version     Version
		want        bool
	}{
		{
			name: "single constraint satisfied",
			constraints: Constraints{
				{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
			},
			version: Version{Major: 0, Minor: 5, Patch: 0},
			want:    true,
		},
		{
			name: "single constraint not satisfied",
			constraints: Constraints{
				{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
			},
			version: Version{Major: 0, Minor: 3, Patch: 0},
			want:    false,
		},
		{
			name: "range constraint satisfied",
			constraints: Constraints{
				{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
				{Operator: OpLess, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			},
			version: Version{Major: 0, Minor: 5, Patch: 0},
			want:    true,
		},
		{
			name: "range constraint not satisfied - too low",
			constraints: Constraints{
				{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
				{Operator: OpLess, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			},
			version: Version{Major: 0, Minor: 3, Patch: 0},
			want:    false,
		},
		{
			name: "range constraint not satisfied - too high",
			constraints: Constraints{
				{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
				{Operator: OpLess, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			},
			version: Version{Major: 1, Minor: 0, Patch: 0},
			want:    false,
		},
		{
			name: "three constraints all satisfied",
			constraints: Constraints{
				{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
				{Operator: OpLess, Version: Version{Major: 1, Minor: 0, Patch: 0}},
				{Operator: OpNotEqual, Version: Version{Major: 0, Minor: 6, Patch: 0}},
			},
			version: Version{Major: 0, Minor: 5, Patch: 0},
			want:    true,
		},
		{
			name: "three constraints one not satisfied",
			constraints: Constraints{
				{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
				{Operator: OpLess, Version: Version{Major: 1, Minor: 0, Patch: 0}},
				{Operator: OpNotEqual, Version: Version{Major: 0, Minor: 5, Patch: 0}},
			},
			version: Version{Major: 0, Minor: 5, Patch: 0},
			want:    false,
		},
		{
			name:        "empty constraints always satisfied",
			constraints: Constraints{},
			version:     Version{Major: 999, Minor: 999, Patch: 999},
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.constraints.Check(tt.version)
			if got != tt.want {
				t.Errorf("Constraints.Check(%v) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

// --- CheckVersionConstraint Tests ---

func TestCheckVersionConstraint(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		version    string
		want       bool
		wantErr    bool
	}{
		// Simple constraints
		{
			name:       "exact match",
			constraint: "1.0.0",
			version:    "1.0.0",
			want:       true,
		},
		{
			name:       "exact match fails",
			constraint: "1.0.0",
			version:    "1.0.1",
			want:       false,
		},
		{
			name:       "greater or equal satisfied",
			constraint: ">=0.4.0",
			version:    "0.5.0",
			want:       true,
		},
		{
			name:       "greater or equal at boundary",
			constraint: ">=0.4.0",
			version:    "0.4.0",
			want:       true,
		},
		{
			name:       "greater or equal not satisfied",
			constraint: ">=0.4.0",
			version:    "0.3.0",
			want:       false,
		},

		// Range constraints
		{
			name:       "range satisfied",
			constraint: ">=0.4.0 <1.0.0",
			version:    "0.5.0",
			want:       true,
		},
		{
			name:       "range at lower boundary",
			constraint: ">=0.4.0 <1.0.0",
			version:    "0.4.0",
			want:       true,
		},
		{
			name:       "range below lower boundary",
			constraint: ">=0.4.0 <1.0.0",
			version:    "0.3.9",
			want:       false,
		},
		{
			name:       "range at upper boundary fails",
			constraint: ">=0.4.0 <1.0.0",
			version:    "1.0.0",
			want:       false,
		},

		// Tilde constraints
		{
			name:       "tilde allows patch",
			constraint: "~1.2.0",
			version:    "1.2.5",
			want:       true,
		},
		{
			name:       "tilde rejects minor",
			constraint: "~1.2.0",
			version:    "1.3.0",
			want:       false,
		},

		// Caret constraints
		{
			name:       "caret allows minor",
			constraint: "^1.2.0",
			version:    "1.5.0",
			want:       true,
		},
		{
			name:       "caret rejects major",
			constraint: "^1.2.0",
			version:    "2.0.0",
			want:       false,
		},

		// Error cases
		{
			name:       "invalid constraint",
			constraint: "invalid",
			version:    "1.0.0",
			wantErr:    true,
		},
		{
			name:       "invalid version",
			constraint: ">=1.0.0",
			version:    "invalid",
			wantErr:    true,
		},
		{
			name:       "empty constraint",
			constraint: "",
			version:    "1.0.0",
			wantErr:    true,
		},
		{
			name:       "empty version",
			constraint: ">=1.0.0",
			version:    "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckVersionConstraint(tt.constraint, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckVersionConstraint(%q, %q) error = %v, wantErr %v",
					tt.constraint, tt.version, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("CheckVersionConstraint(%q, %q) = %v, want %v",
					tt.constraint, tt.version, got, tt.want)
			}
		})
	}
}

// --- IsCompatible Tests ---

func TestIsCompatible(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		awfVersion string
		want       bool
		wantErr    bool
	}{
		{
			name:       "compatible with current version",
			constraint: ">=0.4.0",
			awfVersion: "0.5.0",
			want:       true,
		},
		{
			name:       "incompatible with current version",
			constraint: ">=1.0.0",
			awfVersion: "0.5.0",
			want:       false,
		},
		{
			name:       "compatible with range",
			constraint: ">=0.4.0 <1.0.0",
			awfVersion: "0.6.0",
			want:       true,
		},
		{
			name:       "incompatible outside range",
			constraint: ">=0.4.0 <0.5.0",
			awfVersion: "0.6.0",
			want:       false,
		},
		{
			name:       "dev version special handling",
			constraint: ">=0.4.0",
			awfVersion: "dev",
			wantErr:    true, // or special handling for dev versions
		},
		{
			name:       "invalid constraint",
			constraint: "not-a-constraint",
			awfVersion: "0.5.0",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsCompatible(tt.constraint, tt.awfVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsCompatible(%q, %q) error = %v, wantErr %v",
					tt.constraint, tt.awfVersion, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("IsCompatible(%q, %q) = %v, want %v",
					tt.constraint, tt.awfVersion, got, tt.want)
			}
		})
	}
}

// --- Real-World Scenario Tests ---

func TestVersionConstraint_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		version    string
		want       bool
	}{
		// AWF plugin manifest scenarios
		{
			name:       "plugin requires AWF 0.4.0+, running 0.4.0",
			constraint: ">=0.4.0",
			version:    "0.4.0",
			want:       true,
		},
		{
			name:       "plugin requires AWF 0.4.0+, running 0.5.0",
			constraint: ">=0.4.0",
			version:    "0.5.0",
			want:       true,
		},
		{
			name:       "plugin requires AWF 0.4.0+, running 0.3.0",
			constraint: ">=0.4.0",
			version:    "0.3.0",
			want:       false,
		},
		{
			name:       "plugin requires AWF 0.4.x to 0.x, running 0.9.0",
			constraint: ">=0.4.0 <1.0.0",
			version:    "0.9.0",
			want:       true,
		},
		{
			name:       "plugin requires AWF 0.4.x to 0.x, running 1.0.0",
			constraint: ">=0.4.0 <1.0.0",
			version:    "1.0.0",
			want:       false,
		},
		{
			name:       "plugin pinned to exact AWF version",
			constraint: "0.4.0",
			version:    "0.4.0",
			want:       true,
		},
		{
			name:       "plugin pinned to exact AWF version mismatch",
			constraint: "0.4.0",
			version:    "0.4.1",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckVersionConstraint(tt.constraint, tt.version)
			if err != nil {
				t.Fatalf("CheckVersionConstraint(%q, %q) error = %v", tt.constraint, tt.version, err)
			}
			if got != tt.want {
				t.Errorf("CheckVersionConstraint(%q, %q) = %v, want %v",
					tt.constraint, tt.version, got, tt.want)
			}
		})
	}
}

// --- Constraints.String Tests ---

func TestConstraints_String(t *testing.T) {
	tests := []struct {
		name        string
		constraints Constraints
		want        string
	}{
		{
			name:        "empty constraints",
			constraints: Constraints{},
			want:        "",
		},
		{
			name: "single constraint",
			constraints: Constraints{
				{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
			},
			want: ">=0.4.0",
		},
		{
			name: "range constraints",
			constraints: Constraints{
				{Operator: OpGreaterOrEqual, Version: Version{Major: 0, Minor: 4, Patch: 0}},
				{Operator: OpLess, Version: Version{Major: 1, Minor: 0, Patch: 0}},
			},
			want: ">=0.4.0 <1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.constraints.String()
			if got != tt.want {
				t.Errorf("Constraints.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{
			name: "tag with v prefix",
			tag:  "v1.0.0",
			want: "1.0.0",
		},
		{
			name: "tag without v prefix",
			tag:  "1.0.0",
			want: "1.0.0",
		},
		{
			name: "empty string",
			tag:  "",
			want: "",
		},
		{
			name: "only v",
			tag:  "v",
			want: "",
		},
		{
			name: "multiple v prefixes",
			tag:  "vv1.0.0",
			want: "v1.0.0",
		},
		{
			name: "v with prerelease",
			tag:  "v1.0.0-alpha.1",
			want: "1.0.0-alpha.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeTag(tt.tag)
			if got != tt.want {
				t.Errorf("NormalizeTag(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}

func TestVersion_IsPrerelease(t *testing.T) {
	tests := []struct {
		name    string
		version Version
		want    bool
	}{
		{
			name:    "version with prerelease",
			version: Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want:    true,
		},
		{
			name:    "version with prerelease and dot notation",
			version: Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"},
			want:    true,
		},
		{
			name:    "version without prerelease",
			version: Version{Major: 1, Minor: 0, Patch: 0, Prerelease: ""},
			want:    false,
		},
		{
			name:    "zero version without prerelease",
			version: Version{Major: 0, Minor: 0, Patch: 0, Prerelease: ""},
			want:    false,
		},
		{
			name:    "beta prerelease",
			version: Version{Major: 2, Minor: 1, Patch: 3, Prerelease: "beta.2"},
			want:    true,
		},
		{
			name:    "rc prerelease",
			version: Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.version.IsPrerelease()
			if got != tt.want {
				t.Errorf("Version.IsPrerelease() = %v, want %v", got, tt.want)
			}
		})
	}
}
