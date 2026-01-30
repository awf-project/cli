package audit_skips_test

import "testing"

// Test fixture for audit-skips.sh testing
// Contains platform-specific skip patterns

func TestPlatformSpecific(t *testing.T) {
	t.Skip("test requires linux platform")
}

func TestSymlinkSupport(t *testing.T) {
	t.Skip("symlink not supported on windows")
}
