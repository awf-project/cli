package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/pkg/registry"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    registry.Version
		wantErr bool
	}{
		{
			name:  "simple semantic version",
			input: "1.2.3",
			want:  registry.Version{Major: 1, Minor: 2, Patch: 3, Prerelease: ""},
		},
		{
			name:  "version with prerelease",
			input: "1.2.3-alpha.1",
			want:  registry.Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha.1"},
		},
		{
			name:  "zero version",
			input: "0.0.0",
			want:  registry.Version{Major: 0, Minor: 0, Patch: 0, Prerelease: ""},
		},
		{
			name:  "complex prerelease",
			input: "2.0.0-rc.1+build.123",
			want:  registry.Version{Major: 2, Minor: 0, Patch: 0, Prerelease: "rc.1+build.123"},
		},
		{
			name:    "invalid format - missing patch",
			input:   "1.2",
			wantErr: true,
		},
		{
			name:    "invalid format - non-numeric major",
			input:   "a.2.3",
			wantErr: true,
		},
		{
			name:    "invalid format - non-numeric minor",
			input:   "1.b.3",
			wantErr: true,
		},
		{
			name:    "invalid format - non-numeric patch",
			input:   "1.2.c",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.ParseVersion(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "expected error for input: %s", tt.input)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.Major, got.Major)
			assert.Equal(t, tt.want.Minor, got.Minor)
			assert.Equal(t, tt.want.Patch, got.Patch)
			assert.Equal(t, tt.want.Prerelease, got.Prerelease)
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		name string
		v    registry.Version
		want string
	}{
		{
			name: "simple version",
			v:    registry.Version{Major: 1, Minor: 2, Patch: 3},
			want: "1.2.3",
		},
		{
			name: "version with prerelease",
			v:    registry.Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha"},
			want: "1.2.3-alpha",
		},
		{
			name: "zero version",
			v:    registry.Version{Major: 0, Minor: 0, Patch: 0},
			want: "0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		name string
		v1   registry.Version
		v2   registry.Version
		want int
	}{
		{
			name: "equal versions",
			v1:   registry.Version{Major: 1, Minor: 2, Patch: 3},
			v2:   registry.Version{Major: 1, Minor: 2, Patch: 3},
			want: 0,
		},
		{
			name: "v1 greater by major",
			v1:   registry.Version{Major: 2, Minor: 0, Patch: 0},
			v2:   registry.Version{Major: 1, Minor: 9, Patch: 9},
			want: 1,
		},
		{
			name: "v1 less by major",
			v1:   registry.Version{Major: 1, Minor: 9, Patch: 9},
			v2:   registry.Version{Major: 2, Minor: 0, Patch: 0},
			want: -1,
		},
		{
			name: "v1 greater by minor",
			v1:   registry.Version{Major: 1, Minor: 3, Patch: 0},
			v2:   registry.Version{Major: 1, Minor: 2, Patch: 9},
			want: 1,
		},
		{
			name: "v1 less by minor",
			v1:   registry.Version{Major: 1, Minor: 2, Patch: 9},
			v2:   registry.Version{Major: 1, Minor: 3, Patch: 0},
			want: -1,
		},
		{
			name: "v1 greater by patch",
			v1:   registry.Version{Major: 1, Minor: 2, Patch: 4},
			v2:   registry.Version{Major: 1, Minor: 2, Patch: 3},
			want: 1,
		},
		{
			name: "v1 less by patch",
			v1:   registry.Version{Major: 1, Minor: 2, Patch: 3},
			v2:   registry.Version{Major: 1, Minor: 2, Patch: 4},
			want: -1,
		},
		{
			name: "release vs prerelease - release greater",
			v1:   registry.Version{Major: 1, Minor: 2, Patch: 3, Prerelease: ""},
			v2:   registry.Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha"},
			want: 1,
		},
		{
			name: "prerelease vs release - prerelease less",
			v1:   registry.Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha"},
			v2:   registry.Version{Major: 1, Minor: 2, Patch: 3, Prerelease: ""},
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v1.Compare(tt.v2)
			assert.Equal(t, tt.want, got, "compare %s vs %s", tt.v1.String(), tt.v2.String())
		})
	}
}

func TestVersionIsPrerelease(t *testing.T) {
	tests := []struct {
		name string
		v    registry.Version
		want bool
	}{
		{
			name: "version without prerelease",
			v:    registry.Version{Major: 1, Minor: 2, Patch: 3, Prerelease: ""},
			want: false,
		},
		{
			name: "version with prerelease",
			v:    registry.Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha"},
			want: true,
		},
		{
			name: "version with rc prerelease",
			v:    registry.Version{Major: 2, Minor: 0, Patch: 0, Prerelease: "rc.1"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.IsPrerelease()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseConstraint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    registry.Constraint
		wantErr bool
	}{
		{
			name:  "exact match (==)",
			input: "==1.2.3",
			want:  registry.Constraint{Operator: "==", Version: registry.Version{Major: 1, Minor: 2, Patch: 3}},
		},
		{
			name:  "greater than or equal",
			input: ">=0.4.0",
			want:  registry.Constraint{Operator: ">=", Version: registry.Version{Major: 0, Minor: 4, Patch: 0}},
		},
		{
			name:  "less than",
			input: "<2.0.0",
			want:  registry.Constraint{Operator: "<", Version: registry.Version{Major: 2, Minor: 0, Patch: 0}},
		},
		{
			name:  "tilde constraint",
			input: "~1.2.3",
			want:  registry.Constraint{Operator: "~", Version: registry.Version{Major: 1, Minor: 2, Patch: 3}},
		},
		{
			name:  "caret constraint",
			input: "^2.0.0",
			want:  registry.Constraint{Operator: "^", Version: registry.Version{Major: 2, Minor: 0, Patch: 0}},
		},
		{
			name:  "implicit equality",
			input: "1.0.0",
			want:  registry.Constraint{Operator: "==", Version: registry.Version{Major: 1, Minor: 0, Patch: 0}},
		},
		{
			name:    "invalid operator",
			input:   ">>1.0.0",
			wantErr: true,
		},
		{
			name:    "invalid version",
			input:   ">=invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.ParseConstraint(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "expected error for input: %s", tt.input)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.Operator, got.Operator)
			assert.Equal(t, tt.want.Version.Major, got.Version.Major)
			assert.Equal(t, tt.want.Version.Minor, got.Version.Minor)
			assert.Equal(t, tt.want.Version.Patch, got.Version.Patch)
		})
	}
}

func TestConstraintCheck(t *testing.T) {
	tests := []struct {
		name       string
		constraint registry.Constraint
		version    registry.Version
		want       bool
	}{
		{
			name:       "== satisfied",
			constraint: registry.Constraint{Operator: "==", Version: registry.Version{Major: 1, Minor: 2, Patch: 3}},
			version:    registry.Version{Major: 1, Minor: 2, Patch: 3},
			want:       true,
		},
		{
			name:       "== not satisfied",
			constraint: registry.Constraint{Operator: "==", Version: registry.Version{Major: 1, Minor: 2, Patch: 3}},
			version:    registry.Version{Major: 1, Minor: 2, Patch: 4},
			want:       false,
		},
		{
			name:       ">= satisfied",
			constraint: registry.Constraint{Operator: ">=", Version: registry.Version{Major: 1, Minor: 0, Patch: 0}},
			version:    registry.Version{Major: 1, Minor: 2, Patch: 3},
			want:       true,
		},
		{
			name:       ">= not satisfied",
			constraint: registry.Constraint{Operator: ">=", Version: registry.Version{Major: 2, Minor: 0, Patch: 0}},
			version:    registry.Version{Major: 1, Minor: 9, Patch: 9},
			want:       false,
		},
		{
			name:       "< satisfied",
			constraint: registry.Constraint{Operator: "<", Version: registry.Version{Major: 2, Minor: 0, Patch: 0}},
			version:    registry.Version{Major: 1, Minor: 9, Patch: 9},
			want:       true,
		},
		{
			name:       "< not satisfied",
			constraint: registry.Constraint{Operator: "<", Version: registry.Version{Major: 1, Minor: 0, Patch: 0}},
			version:    registry.Version{Major: 1, Minor: 0, Patch: 0},
			want:       false,
		},
		{
			name:       "~ allows patch bumps",
			constraint: registry.Constraint{Operator: "~", Version: registry.Version{Major: 1, Minor: 2, Patch: 0}},
			version:    registry.Version{Major: 1, Minor: 2, Patch: 5},
			want:       true,
		},
		{
			name:       "~ blocks minor bumps",
			constraint: registry.Constraint{Operator: "~", Version: registry.Version{Major: 1, Minor: 2, Patch: 0}},
			version:    registry.Version{Major: 1, Minor: 3, Patch: 0},
			want:       false,
		},
		{
			name:       "^ allows minor/patch bumps",
			constraint: registry.Constraint{Operator: "^", Version: registry.Version{Major: 1, Minor: 2, Patch: 3}},
			version:    registry.Version{Major: 1, Minor: 3, Patch: 0},
			want:       true,
		},
		{
			name:       "^ blocks major bumps",
			constraint: registry.Constraint{Operator: "^", Version: registry.Version{Major: 1, Minor: 2, Patch: 3}},
			version:    registry.Version{Major: 2, Minor: 0, Patch: 0},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.constraint.Check(tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseConstraints(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{
			name:    "single constraint",
			input:   ">=1.0.0",
			wantLen: 1,
		},
		{
			name:    "space separated",
			input:   ">=0.4.0 <1.0.0",
			wantLen: 2,
		},
		{
			name:    "comma separated",
			input:   ">=0.4.0,<1.0.0",
			wantLen: 2,
		},
		{
			name:    "mixed separators",
			input:   ">=0.4.0, <1.0.0",
			wantLen: 2,
		},
		{
			name:    "multiple constraints",
			input:   ">=1.0.0 <2.0.0 !=1.5.0",
			wantLen: 3,
		},
		{
			name:    "invalid constraint in group",
			input:   ">=1.0.0 invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.ParseConstraints(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, got, tt.wantLen)
		})
	}
}

func TestConstraintsCheck(t *testing.T) {
	tests := []struct {
		name        string
		constraints registry.Constraints
		version     registry.Version
		want        bool
	}{
		{
			name: "single constraint pass",
			constraints: registry.Constraints{
				registry.Constraint{Operator: ">=", Version: registry.Version{Major: 1, Minor: 0, Patch: 0}},
			},
			version: registry.Version{Major: 1, Minor: 2, Patch: 3},
			want:    true,
		},
		{
			name: "single constraint fail",
			constraints: registry.Constraints{
				registry.Constraint{Operator: ">=", Version: registry.Version{Major: 2, Minor: 0, Patch: 0}},
			},
			version: registry.Version{Major: 1, Minor: 2, Patch: 3},
			want:    false,
		},
		{
			name: "all constraints pass",
			constraints: registry.Constraints{
				registry.Constraint{Operator: ">=", Version: registry.Version{Major: 1, Minor: 0, Patch: 0}},
				registry.Constraint{Operator: "<", Version: registry.Version{Major: 2, Minor: 0, Patch: 0}},
			},
			version: registry.Version{Major: 1, Minor: 2, Patch: 3},
			want:    true,
		},
		{
			name: "one constraint fails",
			constraints: registry.Constraints{
				registry.Constraint{Operator: ">=", Version: registry.Version{Major: 1, Minor: 0, Patch: 0}},
				registry.Constraint{Operator: "<", Version: registry.Version{Major: 2, Minor: 0, Patch: 0}},
				registry.Constraint{Operator: "!=", Version: registry.Version{Major: 1, Minor: 5, Patch: 0}},
			},
			version: registry.Version{Major: 1, Minor: 5, Patch: 0},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.constraints.Check(tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConstraintsString(t *testing.T) {
	tests := []struct {
		name        string
		constraints registry.Constraints
		want        string
	}{
		{
			name: "single constraint",
			constraints: registry.Constraints{
				registry.Constraint{Operator: ">=", Version: registry.Version{Major: 1, Minor: 0, Patch: 0}},
			},
			want: ">=1.0.0",
		},
		{
			name: "multiple constraints",
			constraints: registry.Constraints{
				registry.Constraint{Operator: ">=", Version: registry.Version{Major: 1, Minor: 0, Patch: 0}},
				registry.Constraint{Operator: "<", Version: registry.Version{Major: 2, Minor: 0, Patch: 0}},
			},
			want: ">=1.0.0 <2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.constraints.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckVersionConstraint(t *testing.T) {
	tests := []struct {
		name          string
		constraintStr string
		versionStr    string
		want          bool
		wantErr       bool
	}{
		{
			name:          "exact match passes",
			constraintStr: "==1.2.3",
			versionStr:    "1.2.3",
			want:          true,
		},
		{
			name:          "version too old",
			constraintStr: ">=2.0.0",
			versionStr:    "1.2.3",
			want:          false,
		},
		{
			name:          "version in range",
			constraintStr: ">=1.0.0 <2.0.0",
			versionStr:    "1.5.0",
			want:          true,
		},
		{
			name:          "version out of range",
			constraintStr: ">=1.0.0 <2.0.0",
			versionStr:    "2.0.0",
			want:          false,
		},
		{
			name:          "implicit equality",
			constraintStr: "1.2.3",
			versionStr:    "1.2.3",
			want:          true,
		},
		{
			name:          "invalid constraint",
			constraintStr: ">>1.0.0",
			versionStr:    "1.2.3",
			wantErr:       true,
		},
		{
			name:          "invalid version",
			constraintStr: ">=1.0.0",
			versionStr:    "invalid",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.CheckVersionConstraint(tt.constraintStr, tt.versionStr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
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
			name: "removes leading v",
			tag:  "v1.2.3",
			want: "1.2.3",
		},
		{
			name: "no leading v",
			tag:  "1.2.3",
			want: "1.2.3",
		},
		{
			name: "multiple v prefixes",
			tag:  "vv1.2.3",
			want: "v1.2.3",
		},
		{
			name: "only v",
			tag:  "v",
			want: "",
		},
		{
			name: "empty string",
			tag:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registry.NormalizeTag(tt.tag)
			assert.Equal(t, tt.want, got)
		})
	}
}
