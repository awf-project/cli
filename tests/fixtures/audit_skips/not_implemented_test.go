package audit_skips_test

import "testing"

func TestNotImplementedFeature(t *testing.T) {
	t.Skip("not yet implemented")
}

func TestAnotherNotImplemented(t *testing.T) {
	t.Skip("not yet implemented - version parsing")
}
