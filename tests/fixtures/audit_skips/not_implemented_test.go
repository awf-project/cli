package audit_skips_test

import "testing"

// Test fixture for audit-skips.sh testing
// Contains not yet implemented skip pattern

func TestNotImplementedFeature(t *testing.T) {
	t.Skip("not yet implemented")
}

func TestAnotherNotImplemented(t *testing.T) {
	t.Skip("not yet implemented - version parsing")
}
