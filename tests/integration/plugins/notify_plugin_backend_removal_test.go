//go:build integration

// C058 Component T010: Integration Test Cleanup Verification
//
// This component involves DELETING test functions, not creating new ones.
// The actual test work for T010 is:
// 1. Delete TestNotifyNtfy_Success, TestNotifyNtfy_MissingURL
// 2. Delete TestNotifySlack_Success, TestNotifySlack_MissingWebhookURL
// 3. Remove ntfy/slack registration from setupNotifyTestWorkflowService()
//
// Since the component is about test cleanup, minimal verification tests are added
// to ensure the setup function doesn't regress after cleanup.

package plugins_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/awf-project/awf/internal/infrastructure/notify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotifyConfig_StructHasOnlySupportedBackendFields verifies that NotifyConfig
// only contains fields for supported backends (desktop, webhook).
//
// Will FAIL in RED: NtfyURL and SlackWebhookURL fields still exist
// Will PASS in GREEN: After fields are removed
func TestNotifyConfig_StructHasOnlySupportedBackendFields(t *testing.T) {
	config := notify.NotifyConfig{
		DefaultBackend: "desktop",
	}

	_ = config

	configType := reflect.TypeOf(config)
	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		assert.NotEqual(t, "NtfyURL", field.Name, "NtfyURL field should not exist after C058")
		assert.NotEqual(t, "SlackWebhookURL", field.Name, "SlackWebhookURL field should not exist after C058")
	}
}

// TestSetupNotifyTestWorkflowService_ConfigWithoutNtfySlackFields verifies that
// setupNotifyTestWorkflowService can be called with a NotifyConfig that has no
// ntfy or slack configuration.
//
// Will FAIL in RED: Function tries to access config.NtfyURL and config.SlackWebhookURL
// Will PASS in GREEN: After those config field accesses are removed
func TestSetupNotifyTestWorkflowService_ConfigWithoutNtfySlackFields(t *testing.T) {
	skipInCI(t)

	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	config := notify.NotifyConfig{
		DefaultBackend: "desktop",
	}

	execSvc, stateStore := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, config)

	require.NotNil(t, execSvc, "ExecutionService should be created successfully")
	require.NotNil(t, stateStore, "StateStore should be created successfully")
}
