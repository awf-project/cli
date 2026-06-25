package cli

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
)

const (
	pluginInitDistributionPrefix = "awf-plugin-"
	pluginInitExampleName        = "awf-plugin-example"
	pluginInitKindOperation      = "operation"
)

var pluginInitDistributionNamePattern = regexp.MustCompile(`^awf-plugin-[a-z0-9]+(?:-[a-z0-9]+)*$`)

type pluginInitOptions struct {
	distributionName string
	runtimeID        string
	kind             string
	outputDir        string
	force            bool
}

type pluginInitFlags struct {
	kind   []string
	output string
	force  bool
}

func parsePluginInitOptions(args []string, flags pluginInitFlags) (pluginInitOptions, error) {
	if len(args) != 1 {
		value := any(args)
		if len(args) == 0 {
			value = ""
		}

		return pluginInitOptions{}, newPluginInitValidationError(
			"name",
			value,
			"single-name",
			"provide exactly one <name> argument such as awf-plugin-example",
		)
	}

	distributionName := args[0]
	runtimeID, err := derivePluginRuntimeID(distributionName)
	if err != nil {
		return pluginInitOptions{}, err
	}

	kindValue := ""
	if len(flags.kind) > 1 {
		return pluginInitOptions{}, newPluginInitValidationError(
			"kind",
			flags.kind,
			"single-kind",
			"choose exactly one --kind value; awf plugin init supports a single scaffold kind",
		)
	}
	if len(flags.kind) == 1 {
		kindValue = flags.kind[0]
	}

	kind, err := validatePluginInitKind(kindValue)
	if err != nil {
		return pluginInitOptions{}, err
	}

	outputDir, err := normalizePluginInitOutput(distributionName, flags.output)
	if err != nil {
		return pluginInitOptions{}, err
	}

	return pluginInitOptions{
		distributionName: distributionName,
		runtimeID:        runtimeID,
		kind:             kind,
		outputDir:        outputDir,
		force:            flags.force,
	}, nil
}

func derivePluginRuntimeID(distributionName string) (runtimeID string, err error) {
	if err := validatePluginDistributionName(distributionName); err != nil {
		return runtimeID, err
	}

	return strings.TrimPrefix(distributionName, pluginInitDistributionPrefix), nil
}

func validatePluginInitKind(rawValue string) (kind string, err error) {
	if rawValue == "" {
		return pluginInitKindOperation, nil
	}
	if strings.Contains(rawValue, ",") {
		return kind, newPluginInitValidationError(
			"kind",
			rawValue,
			"single-kind",
			fmt.Sprintf("choose exactly one --kind value; supported kind is %q", pluginInitKindOperation),
		)
	}
	if rawValue != pluginInitKindOperation {
		return kind, newPluginInitValidationError(
			"kind",
			rawValue,
			"unsupported-kind",
			fmt.Sprintf("unsupported plugin init kind %q; supported kind is %q", rawValue, pluginInitKindOperation),
		)
	}

	return pluginInitKindOperation, nil
}

func normalizePluginInitOutput(distributionName, output string) (outputDir string, err error) {
	if err := validatePluginDistributionName(distributionName); err != nil {
		return outputDir, newPluginInitValidationError(
			"output",
			distributionName,
			"safe-distribution-name",
			fmt.Sprintf("output defaults require a safe distribution name such as %s", pluginInitExampleName),
		)
	}

	if output == "" {
		return "." + string(filepath.Separator) + distributionName, nil
	}

	cleaned := filepath.Clean(output)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return outputDir, newPluginInitValidationError(
			"output",
			output,
			"safe-output-path",
			"choose a relative output path inside the current directory",
		)
	}

	return strings.TrimPrefix(cleaned, "."+string(filepath.Separator)), nil
}

func validatePluginDistributionName(distributionName string) error {
	if !strings.HasPrefix(distributionName, pluginInitDistributionPrefix) {
		return newPluginInitValidationError(
			"name",
			distributionName,
			"required-prefix",
			fmt.Sprintf("plugin distribution name must start with %q, for example %s", pluginInitDistributionPrefix, pluginInitExampleName),
		)
	}
	if !pluginInitDistributionNamePattern.MatchString(distributionName) {
		return newPluginInitValidationError(
			"name",
			distributionName,
			"lowercase-ascii-filesystem-safe",
			fmt.Sprintf("plugin distribution name must be lowercase ASCII, filesystem-safe, and shaped like %s", pluginInitExampleName),
		)
	}

	return nil
}

func newPluginInitValidationError(field string, value any, rule, guidance string) *domerrors.StructuredError {
	return domerrors.NewUserError(
		domerrors.ErrorCodeUserInputValidationFailed,
		fmt.Sprintf("%s: invalid plugin init %s %q violates %s: %s", domerrors.ErrorCodeUserInputValidationFailed, field, value, rule, guidance),
		map[string]any{
			"field": field,
			"value": value,
			"rule":  rule,
		},
		nil,
	)
}
