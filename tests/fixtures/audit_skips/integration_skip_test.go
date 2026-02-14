package audit_skips_test

import "testing"

func TestIntegrationExample(t *testing.T) {
	t.Skip("skipping integration test")
}

func TestAnotherIntegration(t *testing.T) {
	t.Skip("skipping integration test")
}
