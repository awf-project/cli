package stringutil_test

import (
	"testing"

	"github.com/awf-project/cli/pkg/stringutil"
	"github.com/stretchr/testify/assert"
)

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected int
	}{
		// Happy path: basic cases
		{
			name:     "identical strings",
			s1:       "hello",
			s2:       "hello",
			expected: 0,
		},
		{
			name:     "one character substitution",
			s1:       "hello",
			s2:       "hallo",
			expected: 1,
		},
		{
			name:     "one character insertion",
			s1:       "hello",
			s2:       "helllo",
			expected: 1,
		},
		{
			name:     "one character deletion",
			s1:       "hello",
			s2:       "helo",
			expected: 1,
		},
		{
			name:     "multiple operations",
			s1:       "kitten",
			s2:       "sitting",
			expected: 3, // k->s, e->i, +g
		},
		{
			name:     "complete difference",
			s1:       "abc",
			s2:       "xyz",
			expected: 3,
		},

		// Edge cases: empty strings
		{
			name:     "both empty strings",
			s1:       "",
			s2:       "",
			expected: 0,
		},
		{
			name:     "first string empty",
			s1:       "",
			s2:       "hello",
			expected: 5,
		},
		{
			name:     "second string empty",
			s1:       "hello",
			s2:       "",
			expected: 5,
		},

		// Edge cases: single characters
		{
			name:     "single character identical",
			s1:       "a",
			s2:       "a",
			expected: 0,
		},
		{
			name:     "single character different",
			s1:       "a",
			s2:       "b",
			expected: 1,
		},

		// Edge cases: unicode and special characters
		{
			name:     "unicode characters identical",
			s1:       "日本語",
			s2:       "日本語",
			expected: 0,
		},
		{
			name:     "unicode characters one substitution",
			s1:       "日本語",
			s2:       "日本字",
			expected: 1,
		},
		{
			name:     "emoji characters",
			s1:       "hello👋",
			s2:       "hello👍",
			expected: 1,
		},
		{
			name:     "mixed unicode and ascii",
			s1:       "café",
			s2:       "cafe",
			expected: 1,
		},

		// Edge cases: case sensitivity
		{
			name:     "case difference",
			s1:       "Hello",
			s2:       "hello",
			expected: 1,
		},
		{
			name:     "all uppercase vs lowercase",
			s1:       "HELLO",
			s2:       "hello",
			expected: 5,
		},

		// Edge cases: whitespace
		{
			name:     "whitespace in middle",
			s1:       "hello world",
			s2:       "helloworld",
			expected: 1,
		},
		{
			name:     "multiple spaces",
			s1:       "hello  world",
			s2:       "hello world",
			expected: 1,
		},
		{
			name:     "tab vs space",
			s1:       "hello\tworld",
			s2:       "hello world",
			expected: 1,
		},

		// Edge cases: very different lengths
		{
			name:     "very short vs long",
			s1:       "a",
			s2:       "abcdefghij",
			expected: 9,
		},
		{
			name:     "very long vs short",
			s1:       "abcdefghij",
			s2:       "a",
			expected: 9,
		},

		// Real-world use cases from C048 spec
		{
			name:     "workflow typo missing letter",
			s1:       "my-workfow",
			s2:       "my-workflow",
			expected: 1,
		},
		{
			name:     "workflow typo transposition",
			s1:       "my-wokrflow",
			s2:       "my-workflow",
			expected: 2,
		},
		{
			name:     "state name typo",
			s1:       "fetch_dta",
			s2:       "fetch_data",
			expected: 1,
		},
		{
			name:     "input variable typo",
			s1:       "usr_name",
			s2:       "user_name",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringutil.LevenshteinDistance(tt.s1, tt.s2)
			assert.Equal(t, tt.expected, result,
				"LevenshteinDistance(%q, %q) = %d, expected %d",
				tt.s1, tt.s2, result, tt.expected)
		})
	}
}

func TestLevenshteinDistance_Symmetric(t *testing.T) {
	// Distance should be symmetric: d(a,b) = d(b,a)
	tests := []struct {
		name string
		s1   string
		s2   string
	}{
		{name: "simple case", s1: "hello", s2: "world"},
		{name: "with unicode", s1: "日本", s2: "中国"},
		{name: "empty and non-empty", s1: "", s2: "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d1 := stringutil.LevenshteinDistance(tt.s1, tt.s2)
			d2 := stringutil.LevenshteinDistance(tt.s2, tt.s1)
			assert.Equal(t, d1, d2,
				"LevenshteinDistance should be symmetric: d(%q,%q)=%d but d(%q,%q)=%d",
				tt.s1, tt.s2, d1, tt.s2, tt.s1, d2)
		})
	}
}

func TestClosestMatch(t *testing.T) {
	tests := []struct {
		name             string
		target           string
		candidates       []string
		threshold        int
		expectedMatch    string
		expectedDistance int
	}{
		// Happy path: exact match
		{
			name:             "exact match exists",
			target:           "hello",
			candidates:       []string{"world", "hello", "foo"},
			threshold:        3,
			expectedMatch:    "hello",
			expectedDistance: 0,
		},

		// Happy path: close match below threshold
		{
			name:             "close match within threshold",
			target:           "hello",
			candidates:       []string{"hallo", "world", "foo"},
			threshold:        2,
			expectedMatch:    "hallo",
			expectedDistance: 1,
		},
		{
			name:             "multiple candidates pick closest",
			target:           "hello",
			candidates:       []string{"hallo", "helo", "world"},
			threshold:        3,
			expectedMatch:    "hallo", // distance 1 vs helo's distance 1, picks first
			expectedDistance: 1,
		},

		// Happy path: no threshold (threshold = -1)
		{
			name:             "no threshold returns closest",
			target:           "hello",
			candidates:       []string{"xyz", "world", "foo"},
			threshold:        -1,
			expectedMatch:    "world", // Even though far, it's closest
			expectedDistance: 4,
		},

		// Edge cases: no match above threshold
		{
			name:             "no match within threshold",
			target:           "hello",
			candidates:       []string{"xyz", "abc", "def"},
			threshold:        2,
			expectedMatch:    "",
			expectedDistance: -1,
		},
		{
			name:             "all candidates exceed threshold",
			target:           "hello",
			candidates:       []string{"completely", "different", "words"},
			threshold:        3,
			expectedMatch:    "",
			expectedDistance: -1,
		},

		// Edge cases: empty inputs
		{
			name:             "empty candidates",
			target:           "hello",
			candidates:       []string{},
			threshold:        3,
			expectedMatch:    "",
			expectedDistance: -1,
		},
		{
			name:             "nil candidates",
			target:           "hello",
			candidates:       nil,
			threshold:        3,
			expectedMatch:    "",
			expectedDistance: -1,
		},
		{
			name:             "empty target",
			target:           "",
			candidates:       []string{"hello", "world"},
			threshold:        5,
			expectedMatch:    "hello",
			expectedDistance: 5,
		},
		{
			name:             "empty target and empty candidate",
			target:           "",
			candidates:       []string{"", "hello"},
			threshold:        5,
			expectedMatch:    "",
			expectedDistance: 0,
		},

		// Edge cases: threshold edge values
		{
			name:             "threshold zero allows only exact match",
			target:           "hello",
			candidates:       []string{"hello", "hallo"},
			threshold:        0,
			expectedMatch:    "hello",
			expectedDistance: 0,
		},
		{
			name:             "threshold zero no exact match",
			target:           "hello",
			candidates:       []string{"hallo", "world"},
			threshold:        0,
			expectedMatch:    "",
			expectedDistance: -1,
		},
		{
			name:             "threshold equal to distance",
			target:           "hello",
			candidates:       []string{"hallo"},
			threshold:        1,
			expectedMatch:    "hallo",
			expectedDistance: 1,
		},
		{
			name:             "threshold one less than distance",
			target:           "hello",
			candidates:       []string{"hxllo"},
			threshold:        0,
			expectedMatch:    "",
			expectedDistance: -1,
		},

		// Edge cases: candidates with duplicates
		{
			name:             "duplicate candidates",
			target:           "hello",
			candidates:       []string{"hallo", "hallo", "world"},
			threshold:        2,
			expectedMatch:    "hallo", // First occurrence
			expectedDistance: 1,
		},

		// Edge cases: unicode
		{
			name:             "unicode target and candidates",
			target:           "日本",
			candidates:       []string{"中国", "日本", "韓国"},
			threshold:        2,
			expectedMatch:    "日本",
			expectedDistance: 0,
		},
		{
			name:             "unicode close match",
			target:           "café",
			candidates:       []string{"cafe", "coffee"},
			threshold:        2,
			expectedMatch:    "cafe",
			expectedDistance: 1,
		},

		// Real-world use cases from C048 spec
		{
			name:             "workflow file suggestion",
			target:           "my-workfow.yaml",
			candidates:       []string{"my-workflow.yaml", "other-workflow.yaml", "test.yaml"},
			threshold:        3,
			expectedMatch:    "my-workflow.yaml",
			expectedDistance: 1,
		},
		{
			name:             "state name suggestion",
			target:           "fetch_dta",
			candidates:       []string{"fetch_data", "fetch_meta", "process_data"},
			threshold:        2,
			expectedMatch:    "fetch_data",
			expectedDistance: 1,
		},
		{
			name:             "input variable suggestion",
			target:           "usr_name",
			candidates:       []string{"user_name", "username", "user_id"},
			threshold:        3,
			expectedMatch:    "user_name",
			expectedDistance: 1,
		},
		{
			name:             "command name suggestion",
			target:           "git-statsu",
			candidates:       []string{"git-status", "git-stash", "git-stats"},
			threshold:        2,
			expectedMatch:    "git-stats", // git-stats has distance 1, git-status has distance 2
			expectedDistance: 1,
		},

		// Error handling: large threshold
		{
			name:             "very large threshold",
			target:           "a",
			candidates:       []string{"z"},
			threshold:        1000,
			expectedMatch:    "z",
			expectedDistance: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, distance := stringutil.ClosestMatch(tt.target, tt.candidates, tt.threshold)
			assert.Equal(t, tt.expectedMatch, match,
				"ClosestMatch(%q, %v, %d) match = %q, expected %q",
				tt.target, tt.candidates, tt.threshold, match, tt.expectedMatch)
			assert.Equal(t, tt.expectedDistance, distance,
				"ClosestMatch(%q, %v, %d) distance = %d, expected %d",
				tt.target, tt.candidates, tt.threshold, distance, tt.expectedDistance)
		})
	}
}

func TestClosestMatch_FirstMatchWins(t *testing.T) {
	// When multiple candidates have the same minimal distance, first one should win
	target := "hello"
	candidates := []string{"hallo", "hxllo", "hzllo"} // All distance 1
	threshold := 2

	match, distance := stringutil.ClosestMatch(target, candidates, threshold)
	assert.Equal(t, "hallo", match, "should return first candidate with minimal distance")
	assert.Equal(t, 1, distance)
}

func TestClosestMatch_CaseSensitive(t *testing.T) {
	// ClosestMatch should be case-sensitive (matching LevenshteinDistance behavior)
	target := "Hello"
	candidates := []string{"hello", "HELLO", "HeLLo"}
	threshold := 5

	match, distance := stringutil.ClosestMatch(target, candidates, threshold)
	// "hello" has distance 1 (one case change)
	// "HELLO" has distance 5 (five case changes)
	// "HeLLo" has distance 2 (two case changes)
	assert.Equal(t, "hello", match, "should find closest case-insensitive match")
	assert.Equal(t, 1, distance)
}
