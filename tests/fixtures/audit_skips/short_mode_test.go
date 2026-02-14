package audit_skips_test

import "testing"

func TestSlowOperation(t *testing.T) {
	t.Skip("slow test - resource intensive")
}
