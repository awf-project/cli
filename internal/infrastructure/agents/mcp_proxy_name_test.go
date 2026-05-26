package agents

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRandShortID_Length asserts output is exactly 16 hex chars for n=8.
// randShortID(8) generates 8 bytes → 16 hex characters.
func TestRandShortID_Length(t *testing.T) {
	got := randShortID(8)
	assert.Len(t, got, 16, "randShortID(8) must return exactly 16 hex characters")
}

// TestRandShortID_Uniqueness asserts 100 consecutive calls produce 100 distinct strings.
func TestRandShortID_Uniqueness(t *testing.T) {
	const iterations = 100
	seen := make(map[string]struct{}, iterations)
	for i := range iterations {
		id := randShortID(8)
		_, duplicate := seen[id]
		assert.Falsef(t, duplicate, "iteration %d produced duplicate ID %q", i, id)
		seen[id] = struct{}{}
	}
}

// TestRandShortID_OnlyHex asserts output matches ^[0-9a-f]+$.
func TestRandShortID_OnlyHex(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]+$`)
	for range 20 {
		got := randShortID(8)
		assert.Regexp(t, re, got, "randShortID must return only lowercase hex characters")
	}
}
