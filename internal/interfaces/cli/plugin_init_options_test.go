package cli

import (
	"errors"
	"path/filepath"
	"testing"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginInitOptions_ParsePluginInitOptionsReturnsDistributionNameRuntimeIDKindOutputDirectoryAndForceFlagForAwfPluginExample(t *testing.T) {
	got, err := parsePluginInitOptions([]string{"awf-plugin-example"}, pluginInitFlags{
		kind:   []string{"operation"},
		output: "./plugins/example",
		force:  true,
	})

	require.NoError(t, err)
	assert.Equal(t, "awf-plugin-example", got.distributionName)
	assert.Equal(t, "example", got.runtimeID)
	assert.Equal(t, "operation", got.kind)
	assert.Equal(t, "plugins/example", got.outputDir)
	assert.True(t, got.force)
}

func TestPluginInitOptions_ParsePluginInitOptionsDefaultsOmittedKindToOperation(t *testing.T) {
	got, err := parsePluginInitOptions([]string{"awf-plugin-example"}, pluginInitFlags{})

	require.NoError(t, err)
	assert.Equal(t, "operation", got.kind)
}

func TestPluginInitName_ParsePluginInitOptionsRejectsInvalidNamesBeforeAnyOutputPathIsUsed(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		value string
	}{
		{name: "missing name", args: nil, value: ""},
		{name: "empty name", args: []string{""}, value: ""},
		{name: "uppercase", args: []string{"awf-plugin-Example"}, value: "awf-plugin-Example"},
		{name: "unicode", args: []string{"awf-plugin-cafeé"}, value: "awf-plugin-cafeé"},
		{name: "whitespace", args: []string{"awf-plugin-example name"}, value: "awf-plugin-example name"},
		{name: "shell metacharacter", args: []string{"awf-plugin-example;rm"}, value: "awf-plugin-example;rm"},
		{name: "dot dot", args: []string{"awf-plugin-.."}, value: "awf-plugin-.."},
		{name: "slash", args: []string{"awf-plugin-example/name"}, value: "awf-plugin-example/name"},
		{name: "backslash", args: []string{`awf-plugin-example\name`}, value: `awf-plugin-example\name`},
		{name: "leading dot", args: []string{"awf-plugin-.example"}, value: "awf-plugin-.example"},
		{name: "missing prefix", args: []string{"example"}, value: "example"},
		{name: "trailing hyphen", args: []string{"awf-plugin-example-"}, value: "awf-plugin-example-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePluginInitOptions(tt.args, pluginInitFlags{output: "../must-not-be-normalized"})

			structErr := requirePluginInitValidationError(t, err, "name", tt.value)
			assert.NotEmpty(t, structErr.Details["rule"])
			assert.Contains(t, structErr.Error(), "awf-plugin-example")
		})
	}
}

func TestPluginInitName_InvalidNameErrorsIncludeInvalidValueViolatedRuleAndAwfPluginExample(t *testing.T) {
	_, err := parsePluginInitOptions([]string{"bad name"}, pluginInitFlags{})

	structErr := requirePluginInitValidationError(t, err, "name", "bad name")
	assert.Contains(t, structErr.Error(), "bad name")
	assert.Contains(t, structErr.Error(), "awf-plugin-example")
	assert.NotEmpty(t, structErr.Details["rule"])
}

func TestPluginInitName_ParsePluginInitOptionsRejectsExtraPositionalArguments(t *testing.T) {
	args := []string{"awf-plugin-example", "awf-plugin-other"}
	_, err := parsePluginInitOptions(args, pluginInitFlags{})

	structErr := requirePluginInitValidationError(t, err, "name", args)
	assert.Equal(t, "single-name", structErr.Details["rule"])
	assert.Contains(t, structErr.Error(), "exactly one <name>")
	assert.Contains(t, structErr.Error(), "awf-plugin-example")
}

func TestPluginInitOptions_DerivePluginRuntimeIDAwfPluginSecurityValidatorReturnsSecurityValidator(t *testing.T) {
	got, err := derivePluginRuntimeID("awf-plugin-security-validator")

	require.NoError(t, err)
	assert.Equal(t, "security-validator", got)
}

func TestPluginInitOptions_DerivePluginRuntimeIDRejectsAwfPluginWithEmptyRuntimeID(t *testing.T) {
	_, err := derivePluginRuntimeID("awf-plugin-")

	requirePluginInitValidationError(t, err, "name", "awf-plugin-")
}

func TestPluginInitKind_ValidatePluginInitKindOperationSucceeds(t *testing.T) {
	got, err := validatePluginInitKind("operation")

	require.NoError(t, err)
	assert.Equal(t, "operation", got)
}

func TestPluginInitKind_ValidatePluginInitKindEmptyResolvesToOperation(t *testing.T) {
	got, err := validatePluginInitKind("")

	require.NoError(t, err)
	assert.Equal(t, "operation", got)
}

func TestPluginInitKind_ValidatePluginInitKindValidatorReturnsUnsupportedKindErrorNamingOperationAsSupportedKind(t *testing.T) {
	_, err := validatePluginInitKind("validator")

	structErr := requirePluginInitValidationError(t, err, "kind", "validator")
	assert.Contains(t, structErr.Error(), "unsupported")
	assert.Contains(t, structErr.Error(), `supported kind is "operation"`)
}

func TestPluginInitKind_ValidatePluginInitKindOperationValidatorRejectsCommaSeparatedKindsWithSingleKindGuidance(t *testing.T) {
	_, err := validatePluginInitKind("operation,validator")

	structErr := requirePluginInitValidationError(t, err, "kind", "operation,validator")
	assert.Contains(t, structErr.Error(), "choose exactly one --kind value")
	assert.Contains(t, structErr.Error(), `supported kind is "operation"`)
	assert.NotContains(t, structErr.Error(), "future")
	assert.NotContains(t, structErr.Error(), "MVP")
}

func TestPluginInitKind_RepeatedKindValuesAreRejectedAsSingleKindViolation(t *testing.T) {
	_, err := parsePluginInitOptions([]string{"awf-plugin-example"}, pluginInitFlags{
		kind: []string{"operation", "validator"},
	})

	structErr := requirePluginInitValidationError(t, err, "kind", []string{"operation", "validator"})
	assert.Contains(t, structErr.Error(), "single")
	assert.Contains(t, structErr.Error(), "kind")
}

func TestPluginInitOptions_NormalizePluginInitOutputDefaultsAndCleansWithoutAcceptingTraversalThroughPluginName(t *testing.T) {
	gotDefault, err := normalizePluginInitOutput("awf-plugin-example", "")
	require.NoError(t, err)
	assert.Equal(t, "./awf-plugin-example", gotDefault)

	gotExplicit, err := normalizePluginInitOutput("awf-plugin-example", "./plugins/../generated/example")
	require.NoError(t, err)
	assert.Equal(t, "generated/example", gotExplicit)

	_, err = normalizePluginInitOutput("../awf-plugin-example", "")
	requirePluginInitValidationError(t, err, "output", "../awf-plugin-example")

	absoluteOutput := filepath.Join(string(filepath.Separator), "tmp", "awf-plugin-example")
	gotAbsolute, err := normalizePluginInitOutput("awf-plugin-example", absoluteOutput)
	require.NoError(t, err)
	assert.Equal(t, absoluteOutput, gotAbsolute)
}

func requirePluginInitValidationError(t *testing.T, err error, field string, value any) *domerrors.StructuredError {
	t.Helper()

	require.Error(t, err)

	var structErr *domerrors.StructuredError
	require.True(t, errors.As(err, &structErr), "error must be a *domerrors.StructuredError")
	assert.Equal(t, domerrors.ErrorCodeUserInputValidationFailed, structErr.Code)
	assert.Equal(t, ExitUser, categorizeError(err))
	assert.Contains(t, structErr.Error(), string(domerrors.ErrorCodeUserInputValidationFailed))
	assert.Equal(t, field, structErr.Details["field"])
	assert.Equal(t, value, structErr.Details["value"])

	return structErr
}
