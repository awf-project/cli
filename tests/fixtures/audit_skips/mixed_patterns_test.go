package audit_skips_test

import "testing"

func TestIntegration(t *testing.T) {
	t.Skip("skipping integration test")
}

func TestPending(t *testing.T) {
	t.Skip("pending design decision")
}

func TestFixture(t *testing.T) {
	t.Skip("fixture directory not created")
}

func TestRootUser(t *testing.T) {
	t.Skip("requires root permission")
}

func TestStub(t *testing.T) {
	t.Skip("stub implementation - negative test")
}

func TestOther(t *testing.T) {
	t.Skip("some other reason")
}
