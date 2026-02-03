//go:build integration

package integration_test

// Feature: C043
//
// This file runs the C043 verification script as a Go test.
// It exists to ensure C043 acceptance criteria are validated as part of `make test`.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestC043_VerificationScript_Integration runs the C043 verification bash script
// which validates all acceptance criteria for the code cleanup feature.
//
// Given: The c043_verify.sh script in tests/integration
// When: Executing the script
// Then: All verification checks should pass
func TestC043_VerificationScript_Integration(t *testing.T) {
	repoRoot := getRepoRoot(t)
	scriptPath := filepath.Join(repoRoot, "tests/integration/c043_verify.sh")

	// Verify script exists
	_, err := os.Stat(scriptPath)
	require.NoError(t, err, "c043_verify.sh should exist")

	// Make script executable
	err = os.Chmod(scriptPath, 0o755)
	require.NoError(t, err, "should be able to make script executable")

	// Run verification script
	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = repoRoot

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "c043_verify.sh should pass all checks. Output:\n%s", string(output))

	t.Logf("C043 verification output:\n%s", string(output))
}
