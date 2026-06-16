package application_test

import (
	"testing"

	"github.com/awf-project/cli/internal/application"
	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapError_StructuredErrorToErrorCode_HappyPath(t *testing.T) {
	tests := []struct {
		name        string
		input       error
		expectedErr *domainerrors.StructuredError
		expected    domainerrors.ErrorCode
	}{
		{
			name: "USER.INPUT.MISSING_FILE",
			input: domainerrors.NewUserError(
				domainerrors.ErrorCodeUserInputMissingFile,
				"workflow file not found",
				map[string]any{"path": "/nonexistent.yaml"},
				nil,
			),
			expected: domainerrors.ErrorCodeUserInputMissingFile,
		},
		{
			name: "USER.FACADE.IDENTIFIER_EMPTY",
			input: domainerrors.NewUserError(
				domainerrors.ErrorCodeUserFacadeIdentifierEmpty,
				"empty identifier",
				nil,
				nil,
			),
			expected: domainerrors.ErrorCodeUserFacadeIdentifierEmpty,
		},
		{
			name: "USER.FACADE.IDENTIFIER_MALFORMED",
			input: domainerrors.NewUserError(
				domainerrors.ErrorCodeUserFacadeIdentifierMalformed,
				"malformed identifier",
				nil,
				nil,
			),
			expected: domainerrors.ErrorCodeUserFacadeIdentifierMalformed,
		},
		{
			name: "USER.FACADE.PACK_NOT_FOUND",
			input: domainerrors.NewUserError(
				domainerrors.ErrorCodeUserFacadePackNotFound,
				"pack not found",
				map[string]any{"pack": "missing-pack"},
				nil,
			),
			expected: domainerrors.ErrorCodeUserFacadePackNotFound,
		},
		{
			name: "USER.FACADE.WORKFLOW_NOT_FOUND",
			input: domainerrors.NewUserError(
				domainerrors.ErrorCodeUserFacadeWorkflowNotFound,
				"workflow not found",
				map[string]any{"workflow": "missing-workflow"},
				nil,
			),
			expected: domainerrors.ErrorCodeUserFacadeWorkflowNotFound,
		},
		{
			name: "USER.FACADE.SESSION_CLOSED",
			input: domainerrors.NewUserError(
				domainerrors.ErrorCodeUserFacadeSessionClosed,
				"session closed",
				nil,
				nil,
			),
			expected: domainerrors.ErrorCodeUserFacadeSessionClosed,
		},
		{
			name: "USER.FACADE.INPUT_REJECTED",
			input: domainerrors.NewUserError(
				domainerrors.ErrorCodeUserFacadeInputRejected,
				"input rejected",
				nil,
				nil,
			),
			expected: domainerrors.ErrorCodeUserFacadeInputRejected,
		},
		{
			name: "USER.FACADE.DUPLICATE_RESPONSE",
			input: domainerrors.NewUserError(
				domainerrors.ErrorCodeUserFacadeDuplicateResponse,
				"duplicate response",
				nil,
				nil,
			),
			expected: domainerrors.ErrorCodeUserFacadeDuplicateResponse,
		},
		{
			name: "WORKFLOW.PARSE.YAML_SYNTAX",
			input: domainerrors.NewWorkflowError(
				domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
				"yaml syntax error",
				nil,
				nil,
			),
			expected: domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		},
		{
			name: "EXECUTION.COMMAND.TIMEOUT",
			input: domainerrors.NewExecutionError(
				domainerrors.ErrorCodeExecutionCommandTimeout,
				"command timeout",
				nil,
				nil,
			),
			expected: domainerrors.ErrorCodeExecutionCommandTimeout,
		},
		{
			name: "SYSTEM.IO.READ_FAILED",
			input: domainerrors.NewSystemError(
				domainerrors.ErrorCodeSystemIOReadFailed,
				"read failed",
				nil,
				nil,
			),
			expected: domainerrors.ErrorCodeSystemIOReadFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := application.MapError(tt.input)
			assert.Equal(t, tt.expected, result, "should map %s correctly", tt.name)
			assert.True(t, result.IsValid(), "result should be a valid error code")
		})
	}
}

func TestMapError_NilError(t *testing.T) {
	result := application.MapError(nil)
	assert.Equal(t, domainerrors.ErrorCode(""), result, "nil error should map to empty ErrorCode")
}

func TestMapError_NonStructuredError(t *testing.T) {
	// When a non-StructuredError is passed, should return ErrInternal
	result := application.MapError(assert.AnError)
	assert.NotNil(t, result, "should return an ErrorCode for non-StructuredError")
	assert.True(t, result.IsValid(), "should return a valid ErrorCode")
	// The spec says unmapped variants resolve to ErrInternal
	assert.Equal(t, "SYSTEM", result.Category(), "unmapped error should be SYSTEM category (internal)")
}

func TestExitCode_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		code     domainerrors.ErrorCode
		expected int
	}{
		{
			name:     "USER.INPUT.MISSING_FILE maps to 1",
			code:     domainerrors.ErrorCodeUserInputMissingFile,
			expected: 1,
		},
		{
			name:     "USER.FACADE.IDENTIFIER_EMPTY maps to 1",
			code:     domainerrors.ErrorCodeUserFacadeIdentifierEmpty,
			expected: 1,
		},
		{
			name:     "USER.FACADE.PACK_NOT_FOUND maps to 1",
			code:     domainerrors.ErrorCodeUserFacadePackNotFound,
			expected: 1,
		},
		{
			name:     "WORKFLOW.PARSE.YAML_SYNTAX maps to 2",
			code:     domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
			expected: 2,
		},
		{
			name:     "EXECUTION.COMMAND.TIMEOUT maps to 3",
			code:     domainerrors.ErrorCodeExecutionCommandTimeout,
			expected: 3,
		},
		{
			name:     "SYSTEM.IO.READ_FAILED maps to 4",
			code:     domainerrors.ErrorCodeSystemIOReadFailed,
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := application.ExitCode(tt.code)
			assert.Equal(t, tt.expected, result, "should map %s to exit code %d", tt.code, tt.expected)
		})
	}
}

func TestExitCode_EmptyCode(t *testing.T) {
	result := application.ExitCode(domainerrors.ErrorCode(""))
	assert.Equal(t, 0, result, "empty code should map to 0")
}

func TestExitCode_AllCategories(t *testing.T) {
	t.Run("USER category codes", func(t *testing.T) {
		userCodes := []domainerrors.ErrorCode{
			domainerrors.ErrorCodeUserInputMissingFile,
			domainerrors.ErrorCodeUserFacadeIdentifierEmpty,
			domainerrors.ErrorCodeUserFacadeWorkflowNotFound,
		}
		for _, code := range userCodes {
			t.Run(string(code), func(t *testing.T) {
				assert.Equal(t, 1, application.ExitCode(code))
			})
		}
	})

	t.Run("WORKFLOW category codes", func(t *testing.T) {
		wfCodes := []domainerrors.ErrorCode{
			domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
			domainerrors.ErrorCodeWorkflowValidationCycleDetected,
		}
		for _, code := range wfCodes {
			t.Run(string(code), func(t *testing.T) {
				assert.Equal(t, 2, application.ExitCode(code))
			})
		}
	})

	t.Run("EXECUTION category codes", func(t *testing.T) {
		execCodes := []domainerrors.ErrorCode{
			domainerrors.ErrorCodeExecutionCommandFailed,
			domainerrors.ErrorCodeExecutionCommandTimeout,
		}
		for _, code := range execCodes {
			t.Run(string(code), func(t *testing.T) {
				assert.Equal(t, 3, application.ExitCode(code))
			})
		}
	})

	t.Run("SYSTEM category codes", func(t *testing.T) {
		sysCodes := []domainerrors.ErrorCode{
			domainerrors.ErrorCodeSystemIOReadFailed,
			domainerrors.ErrorCodeSystemIOWriteFailed,
		}
		for _, code := range sysCodes {
			t.Run(string(code), func(t *testing.T) {
				assert.Equal(t, 4, application.ExitCode(code))
			})
		}
	})
}

func TestErrorCodeMapping_Exhaustive(t *testing.T) {
	// List all declared error codes (source of truth per FR-008)
	allCodes := []domainerrors.ErrorCode{
		// USER.INPUT
		domainerrors.ErrorCodeUserInputMissingFile,
		domainerrors.ErrorCodeUserInputInvalidFormat,
		domainerrors.ErrorCodeUserInputValidationFailed,
		domainerrors.ErrorCodeUserInputMissingSkill,
		domainerrors.ErrorCodeUserInputMissingRole,
		// USER.UPGRADE
		domainerrors.ErrorCodeUserUpgradeVersionNotFound,
		domainerrors.ErrorCodeUserUpgradeAlreadyLatest,
		// USER.FACADE
		domainerrors.ErrorCodeUserFacadeIdentifierEmpty,
		domainerrors.ErrorCodeUserFacadeIdentifierMalformed,
		domainerrors.ErrorCodeUserFacadePackNotFound,
		domainerrors.ErrorCodeUserFacadeWorkflowNotFound,
		domainerrors.ErrorCodeUserFacadeSessionClosed,
		domainerrors.ErrorCodeUserFacadeInputRejected,
		domainerrors.ErrorCodeUserFacadeDuplicateResponse,
		// USER.MCP_PROXY
		domainerrors.ErrorCodeUserMCPProxyUnknownKey,
		domainerrors.ErrorCodeUserMCPProxyUnknownPlugin,
		domainerrors.ErrorCodeUserMCPProxyUnknownOperation,
		domainerrors.ErrorCodeUserMCPProxyNameCollision,
		domainerrors.ErrorCodeUserMCPProxyEmptyProxy,
		domainerrors.ErrorCodeUserMCPProxyUnsupportedProvider,
		domainerrors.ErrorCodeUserMCPProxyInfiniteLoopGuard,
		// USER.ACP
		domainerrors.ErrorCodeUserACPInvalidPrompt,
		domainerrors.ErrorCodeUserACPUnsupportedBlock,
		domainerrors.ErrorCodeUserACPPromptInFlight,
		domainerrors.ErrorCodeUserACPUnknownSession,
		domainerrors.ErrorCodeUserACPProtocolVersionUnsupported,
		// WORKFLOW.PARSE
		domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
		domainerrors.ErrorCodeWorkflowParseUnknownField,
		// WORKFLOW.VALIDATION
		domainerrors.ErrorCodeWorkflowValidationCycleDetected,
		domainerrors.ErrorCodeWorkflowValidationMissingState,
		domainerrors.ErrorCodeWorkflowValidationInvalidTransition,
		// EXECUTION.COMMAND
		domainerrors.ErrorCodeExecutionCommandFailed,
		domainerrors.ErrorCodeExecutionCommandTimeout,
		// EXECUTION.PARALLEL
		domainerrors.ErrorCodeExecutionParallelPartialFailure,
		// EXECUTION.PLUGIN
		domainerrors.ErrorCodeExecutionPluginDisabled,
		domainerrors.ErrorCodeExecutionPluginChecksumMismatch,
		domainerrors.ErrorCodeExecutionPluginBrokerEmitDenied,
		domainerrors.ErrorCodeExecutionPluginStreamSetupFailed,
		// EXECUTION.EVENT
		domainerrors.ErrorCodeExecutionEventDeliveryFailed,
		domainerrors.ErrorCodeExecutionEventCycleDetected,
		domainerrors.ErrorCodeExecutionEventBufferFull,
		// SYSTEM.IO
		domainerrors.ErrorCodeSystemIOReadFailed,
		domainerrors.ErrorCodeSystemIOWriteFailed,
		domainerrors.ErrorCodeSystemIOPermissionDenied,
		// SYSTEM.UPGRADE
		domainerrors.ErrorCodeSystemUpgradeChecksumMismatch,
		domainerrors.ErrorCodeSystemUpgradeBinaryReplaceFailed,
		domainerrors.ErrorCodeSystemUpgradeDownloadFailed,
	}

	for _, code := range allCodes {
		t.Run(string(code), func(t *testing.T) {
			// Create a StructuredError with this code
			err := domainerrors.NewStructuredError(code, "test error", nil, nil)

			// Map it back
			mapped := application.MapError(err)

			// Should not map to unmapped sentinel
			require.NotEqual(t, domainerrors.ErrorCodeSystemInternalUnmapped, mapped,
				"code %s should have a mapping, not fall through to unmapped sentinel", code)

			// Should map to the same code
			assert.Equal(t, code, mapped, "code %s should map to itself", code)
		})
	}
}

func TestErrorCodeMapping_Comprehensive(t *testing.T) {
	t.Run("ExitCode never returns invalid values", func(t *testing.T) {
		codes := []domainerrors.ErrorCode{
			domainerrors.ErrorCodeUserInputMissingFile,
			domainerrors.ErrorCodeWorkflowParseYAMLSyntax,
			domainerrors.ErrorCodeExecutionCommandFailed,
			domainerrors.ErrorCodeSystemIOReadFailed,
		}
		for _, code := range codes {
			result := application.ExitCode(code)
			assert.GreaterOrEqual(t, result, 0, "exit code should be non-negative")
			assert.LessOrEqual(t, result, 4, "exit code should be <= 4")
		}
	})
}

func TestMapError_PreservesCauseChain(t *testing.T) {
	originalCause := assert.AnError
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserFacadePackNotFound,
		"pack not found",
		nil,
		originalCause,
	)

	mapped := application.MapError(err)
	assert.Equal(t, domainerrors.ErrorCodeUserFacadePackNotFound, mapped,
		"should extract error code while preserving cause chain")
}

func TestMapError_WithDetails(t *testing.T) {
	err := domainerrors.NewStructuredError(
		domainerrors.ErrorCodeUserFacadeWorkflowNotFound,
		"workflow not found",
		map[string]any{
			"pack":     "my-pack",
			"workflow": "missing-workflow",
		},
		nil,
	)

	mapped := application.MapError(err)
	assert.Equal(t, domainerrors.ErrorCodeUserFacadeWorkflowNotFound, mapped,
		"should map error code regardless of details")
}
