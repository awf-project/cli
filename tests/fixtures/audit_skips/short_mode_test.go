package audit_skips_test

import "testing"

// Test fixture for audit-skips.sh testing
// Contains short mode skip patterns

func TestSlowOperation(t *testing.T) {
	t.Skip("slow test - resource intensive")
}
