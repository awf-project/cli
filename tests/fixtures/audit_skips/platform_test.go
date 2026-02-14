package audit_skips_test

import "testing"

func TestPlatformSpecific(t *testing.T) {
	t.Skip("test requires linux platform")
}

func TestSymlinkSupport(t *testing.T) {
	t.Skip("symlink not supported on windows")
}
