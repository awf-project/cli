//go:build integration

// Component: T003
// Feature: C024

package integration_test

import (
	"os"
	"testing"
)

// =============================================================================
// skipInCI Helper Function Tests
// Tests validate that skipInCI correctly detects CI environments and skips
// tests appropriately.
//
// Testing approach: Since skipInCI uses *testing.T which cannot be easily mocked,
// we test the function's behavior by running sub-tests with different environment
// configurations and checking if they execute or are skipped.
// =============================================================================

// TestSkipInCI_HappyPath_NotInCIEnvironment verifies that skipInCI does not
// skip tests when neither CI nor GITHUB_ACTIONS environment variables are set.
func TestSkipInCI_HappyPath_NotInCIEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Given: No CI environment variables set
	// Save original values and restore after test
	originalCI := os.Getenv("CI")
	originalGHA := os.Getenv("GITHUB_ACTIONS")
	defer func() {
		if originalCI != "" {
			os.Setenv("CI", originalCI)
		} else {
			os.Unsetenv("CI")
		}
		if originalGHA != "" {
			os.Setenv("GITHUB_ACTIONS", originalGHA)
		} else {
			os.Unsetenv("GITHUB_ACTIONS")
		}
	}()

	// Clear both environment variables
	os.Unsetenv("CI")
	os.Unsetenv("GITHUB_ACTIONS")

	// When: skipInCI is called in a sub-test
	executed := false
	t.Run("SubTest", func(t *testing.T) {
		skipInCI(t)
		// Then: Test should NOT be skipped (i.e., this code executes)
		executed = true
	})

	// Assert: Sub-test executed successfully
	if !executed {
		t.Error("skipInCI should not skip when CI environment variables are not set")
	}
}

// TestSkipInCI_EdgeCase_CIVariableSet verifies that skipInCI correctly
// skips tests when the CI environment variable is set.
func TestSkipInCI_EdgeCase_CIVariableSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Given: CI environment variable is set
	originalCI := os.Getenv("CI")
	originalGHA := os.Getenv("GITHUB_ACTIONS")
	defer func() {
		if originalCI != "" {
			os.Setenv("CI", originalCI)
		} else {
			os.Unsetenv("CI")
		}
		if originalGHA != "" {
			os.Setenv("GITHUB_ACTIONS", originalGHA)
		} else {
			os.Unsetenv("GITHUB_ACTIONS")
		}
	}()

	os.Setenv("CI", "true")
	os.Unsetenv("GITHUB_ACTIONS")

	// When: skipInCI is called in a sub-test
	executed := false
	skipped := false
	t.Run("SubTest", func(t *testing.T) {
		// Capture skip by checking if test continues after skipInCI
		skipInCI(t)
		executed = true
	})

	// Then: If the sub-test was skipped, executed will be false
	// We determine if it was skipped by checking the test result
	// In Go, when t.Skip() is called, the function returns immediately
	// so executed will remain false
	skipped = !executed

	// Assert: Test should be skipped in CI environment
	if !skipped {
		t.Error("skipInCI should skip when CI environment variable is set")
	}
}

// TestSkipInCI_EdgeCase_GithubActionsVariableSet verifies that skipInCI
// correctly skips tests when the GITHUB_ACTIONS environment variable is set.
func TestSkipInCI_EdgeCase_GithubActionsVariableSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Given: GITHUB_ACTIONS environment variable is set
	originalCI := os.Getenv("CI")
	originalGHA := os.Getenv("GITHUB_ACTIONS")
	defer func() {
		if originalCI != "" {
			os.Setenv("CI", originalCI)
		} else {
			os.Unsetenv("CI")
		}
		if originalGHA != "" {
			os.Setenv("GITHUB_ACTIONS", originalGHA)
		} else {
			os.Unsetenv("GITHUB_ACTIONS")
		}
	}()

	os.Unsetenv("CI")
	os.Setenv("GITHUB_ACTIONS", "true")

	// When: skipInCI is called in a sub-test
	executed := false
	skipped := false
	t.Run("SubTest", func(t *testing.T) {
		skipInCI(t)
		executed = true
	})

	// Then: Verify test was skipped
	skipped = !executed

	// Assert: Test should be skipped when GITHUB_ACTIONS is set
	if !skipped {
		t.Error("skipInCI should skip when GITHUB_ACTIONS environment variable is set")
	}
}

// TestSkipInCI_EdgeCase_BothVariablesSet verifies that skipInCI correctly
// skips tests when both CI environment variables are set.
func TestSkipInCI_EdgeCase_BothVariablesSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Given: Both CI and GITHUB_ACTIONS environment variables are set
	originalCI := os.Getenv("CI")
	originalGHA := os.Getenv("GITHUB_ACTIONS")
	defer func() {
		if originalCI != "" {
			os.Setenv("CI", originalCI)
		} else {
			os.Unsetenv("CI")
		}
		if originalGHA != "" {
			os.Setenv("GITHUB_ACTIONS", originalGHA)
		} else {
			os.Unsetenv("GITHUB_ACTIONS")
		}
	}()

	os.Setenv("CI", "true")
	os.Setenv("GITHUB_ACTIONS", "true")

	// When: skipInCI is called in a sub-test
	executed := false
	skipped := false
	t.Run("SubTest", func(t *testing.T) {
		skipInCI(t)
		executed = true
	})

	// Then: Verify test was skipped
	skipped = !executed

	// Assert: Test should be skipped when both variables are set
	if !skipped {
		t.Error("skipInCI should skip when both CI environment variables are set")
	}
}

// TestSkipInCI_EdgeCase_EmptyStringVariables verifies that skipInCI treats
// empty string values as "not set" for environment variables.
func TestSkipInCI_EdgeCase_EmptyStringVariables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Given: Environment variables are set to empty strings
	originalCI := os.Getenv("CI")
	originalGHA := os.Getenv("GITHUB_ACTIONS")
	defer func() {
		if originalCI != "" {
			os.Setenv("CI", originalCI)
		} else {
			os.Unsetenv("CI")
		}
		if originalGHA != "" {
			os.Setenv("GITHUB_ACTIONS", originalGHA)
		} else {
			os.Unsetenv("GITHUB_ACTIONS")
		}
	}()

	// Note: The function checks for != "", so empty string means "not in CI"
	os.Setenv("CI", "")
	os.Setenv("GITHUB_ACTIONS", "")

	// When: skipInCI is called in a sub-test
	executed := false
	t.Run("SubTest", func(t *testing.T) {
		skipInCI(t)
		// Then: Test should NOT be skipped (empty string == not set for the != "" check)
		executed = true
	})

	// Assert: Test should execute when variables are empty strings
	if !executed {
		t.Error("skipInCI should not skip when environment variables are empty strings")
	}
}

// TestSkipInCI_EdgeCase_VariousNonEmptyValues verifies that skipInCI skips
// tests when CI variables have any non-empty value (not just "true").
func TestSkipInCI_EdgeCase_VariousNonEmptyValues(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	testCases := []struct {
		name     string
		ciValue  string
		ghaValue string
	}{
		{"CI=1", "1", ""},
		{"CI=false", "false", ""},
		{"GITHUB_ACTIONS=yes", "", "yes"},
		{"CI=anything", "anything", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Given: CI variable has a non-empty value (even non-boolean)
			originalCI := os.Getenv("CI")
			originalGHA := os.Getenv("GITHUB_ACTIONS")
			defer func() {
				if originalCI != "" {
					os.Setenv("CI", originalCI)
				} else {
					os.Unsetenv("CI")
				}
				if originalGHA != "" {
					os.Setenv("GITHUB_ACTIONS", originalGHA)
				} else {
					os.Unsetenv("GITHUB_ACTIONS")
				}
			}()

			if tc.ciValue != "" {
				os.Setenv("CI", tc.ciValue)
			} else {
				os.Unsetenv("CI")
			}

			if tc.ghaValue != "" {
				os.Setenv("GITHUB_ACTIONS", tc.ghaValue)
			} else {
				os.Unsetenv("GITHUB_ACTIONS")
			}

			// When: skipInCI is called in a sub-test
			executed := false
			skipped := false
			t.Run("SubTest", func(t *testing.T) {
				skipInCI(t)
				executed = true
			})

			// Then: Verify test was skipped
			skipped = !executed

			// Assert: Test should be skipped for any non-empty value
			if !skipped {
				t.Errorf("skipInCI should skip when CI variable has value '%s' or GITHUB_ACTIONS has value '%s'",
					tc.ciValue, tc.ghaValue)
			}
		})
	}
}

// TestSkipInCI_ErrorHandling_HelperUsage verifies that skipInCI is used
// correctly as a test helper function (no direct test for t.Helper() call,
// but we verify the function works correctly in helper context).
func TestSkipInCI_ErrorHandling_HelperUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Given: skipInCI is called from within a helper function
	// When: The helper calls skipInCI
	// Then: Test behavior should be consistent

	helperFunction := func(t *testing.T) {
		t.Helper()
		skipInCI(t)
		// This line should execute if not in CI
	}

	// Clear CI variables to ensure test executes
	originalCI := os.Getenv("CI")
	originalGHA := os.Getenv("GITHUB_ACTIONS")
	defer func() {
		if originalCI != "" {
			os.Setenv("CI", originalCI)
		} else {
			os.Unsetenv("CI")
		}
		if originalGHA != "" {
			os.Setenv("GITHUB_ACTIONS", originalGHA)
		} else {
			os.Unsetenv("GITHUB_ACTIONS")
		}
	}()

	os.Unsetenv("CI")
	os.Unsetenv("GITHUB_ACTIONS")

	executed := false
	t.Run("SubTest", func(t *testing.T) {
		helperFunction(t)
		executed = true
	})

	// Assert: Test executed successfully when not in CI
	if !executed {
		t.Error("skipInCI should not skip when called from helper and not in CI")
	}
}
