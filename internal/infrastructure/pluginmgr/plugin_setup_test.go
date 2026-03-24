package pluginmgr_test

import (
	"os"
	"path/filepath"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// TestT001_ProtoDependenciesImportable verifies that T001 dependencies can be imported.
// This is a compile-time check that the go-plugin, gRPC, and protobuf packages are available.
func TestT001_ProtoDependenciesImportable(t *testing.T) {
	// Verify google.golang.org/grpc is importable and usable
	require.NotNil(t, grpc.NewServer, "grpc.NewServer should be available")

	// Verify google.golang.org/protobuf is importable and usable
	require.NotNil(t, proto.Equal, "proto.Equal should be available")
}

// TestT001_ProtoDirectoryExists verifies that proto/plugin/v1 directory exists.
func TestT001_ProtoDirectoryExists(t *testing.T) {
	projRoot := findProjectRoot(t)
	protoDirPath := filepath.Join(projRoot, "proto", "plugin", "v1")

	info, err := os.Stat(protoDirPath)
	require.NoError(t, err, "proto/plugin/v1 directory should exist")
	require.True(t, info.IsDir(), "proto/plugin/v1 should be a directory")
}

// TestT001_GoModTidySucceeds verifies that go.mod is in a valid state after tidying.
// This test checks that the required dependencies are listed in go.mod.
func TestT001_GoModTidySucceeds(t *testing.T) {
	projRoot := findProjectRoot(t)
	goModPath := filepath.Join(projRoot, "go.mod")

	content, err := os.ReadFile(goModPath)
	require.NoError(t, err, "go.mod should be readable")

	goModContent := string(content)

	// Verify required dependencies are present
	assert.Contains(t, goModContent, "github.com/hashicorp/go-plugin", "go-plugin dependency should be present")
	assert.Contains(t, goModContent, "google.golang.org/grpc", "grpc dependency should be present")
	assert.Contains(t, goModContent, "google.golang.org/protobuf", "protobuf dependency should be present")
}

// TestT001_GoPluginPackageUsable verifies go-plugin can be used from code.
func TestT001_GoPluginPackageUsable(t *testing.T) {
	// Verify we can instantiate a go-plugin HandshakeConfig (proves package is available)
	handshake := goplugin.HandshakeConfig{
		MagicCookieKey:   "TEST_PLUGIN",
		MagicCookieValue: "test-v1",
		ProtocolVersion:  1,
	}
	assert.Equal(t, "TEST_PLUGIN", handshake.MagicCookieKey)
	assert.Equal(t, uint(1), handshake.ProtocolVersion)
}

// findProjectRoot locates the project root by looking for go.mod
func findProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from current working directory
	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Walk up directory tree until we find go.mod
	dir := cwd
	for {
		goModPath := filepath.Join(dir, "go.mod")
		_, err := os.Stat(goModPath)
		if err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("Could not find project root (go.mod)")
		}
		dir = parent
	}
}
