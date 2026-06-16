//go:build integration

package plugins_test

import (
	"testing"

	"github.com/awf-project/cli/tests/integration/testhelpers"
)

func skipInCI(t *testing.T) {
	t.Helper()
	testhelpers.SkipInCI(t)
}

func skipIfCLIMissing(t *testing.T, cliName string) {
	t.Helper()
	testhelpers.SkipIfCLIMissing(t, cliName)
}

func getRepoRoot(t *testing.T) string {
	t.Helper()
	return testhelpers.GetRepoRoot(t)
}
