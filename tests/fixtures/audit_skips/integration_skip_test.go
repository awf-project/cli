package audit_skips_test

import "testing"

// Test fixture for audit-skips.sh testing
// Contains integration test skip pattern

func TestIntegrationExample(t *testing.T) {
	t.Skip("skipping integration test")
}

func TestAnotherIntegration(t *testing.T) {
	t.Skip("skipping integration test")
}
