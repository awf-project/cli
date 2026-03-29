package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/awf-project/cli/pkg/plugin/sdk"
)

// SecurityValidatorPlugin implements sdk.Plugin and sdk.Validator.
// It performs security checks on workflows including:
// - Detection of hardcoded secrets (API keys, passwords)
// - Detection of command injection vulnerabilities
// - Enforcement of timeout requirements
type SecurityValidatorPlugin struct {
	sdk.BasePlugin
	secretPatterns []*regexp.Regexp
	dangerousWords []string
}

func (p *SecurityValidatorPlugin) Init(ctx context.Context, config map[string]any) error {
	// Compile regex patterns for secret detection
	p.secretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*['"]?[a-zA-Z0-9_-]+['"]?`),
		regexp.MustCompile(`(?i)(password|passwd)\s*[:=]\s*['"]?[a-zA-Z0-9_-]+['"]?`),
		regexp.MustCompile(`(?i)(token|auth)\s*[:=]\s*['"]?[a-zA-Z0-9_-]+['"]?`),
		regexp.MustCompile(`(?i)secret\s*[:=]\s*['"]?[a-zA-Z0-9_-]+['"]?`),
	}

	// List of shell commands that need careful scrutiny
	p.dangerousWords = []string{
		"rm", "dd", "mkfs", "format", "fdisk",
		"kill", "killall", "pkill",
	}

	return nil
}

func (p *SecurityValidatorPlugin) Shutdown(ctx context.Context) error {
	return nil
}

// ValidateWorkflow performs security checks at the workflow level.
// Parameter passed by value (required by sdk.Validator interface); struct size is acceptable.
func (p *SecurityValidatorPlugin) ValidateWorkflow(ctx context.Context, workflow sdk.WorkflowDefinition) ([]sdk.ValidationIssue, error) { //nolint:gocritic
	var issues []sdk.ValidationIssue

	// Workflow-level security checks
	// Currently relies on per-step validation
	_ = workflow

	return issues, nil
}

// ValidateStep performs security checks for a specific step.
// Parameter passed by value (required by sdk.Validator interface); struct size is acceptable.
func (p *SecurityValidatorPlugin) ValidateStep(ctx context.Context, workflow sdk.WorkflowDefinition, stepName string) ([]sdk.ValidationIssue, error) { //nolint:gocritic
	var issues []sdk.ValidationIssue

	step, ok := workflow.Steps[stepName]
	if !ok {
		return issues, nil
	}

	// Check for hardcoded secrets in command
	if step.Command != "" {
		if issue := p.checkForSecrets(stepName, "command", step.Command); issue != nil {
			issues = append(issues, *issue)
		}

		if issue := p.checkForDangerousCommands(stepName, step.Command); issue != nil {
			issues = append(issues, *issue)
		}

		if issue := p.checkForCommandInjection(stepName, step.Command); issue != nil {
			issues = append(issues, *issue)
		}
	}

	// Check for secrets in description
	if step.Description != "" {
		if issue := p.checkForSecrets(stepName, "description", step.Description); issue != nil {
			issues = append(issues, *issue)
		}
	}

	// Check for missing timeout on commands
	if step.Type == "command" && step.Timeout == 0 {
		issues = append(issues, sdk.ValidationIssue{
			Severity: sdk.SeverityWarning,
			Message:  "Command step has no timeout; consider adding timeout to prevent runaway processes",
			Step:     stepName,
			Field:    "timeout",
		})
	}

	return issues, nil
}

func (p *SecurityValidatorPlugin) checkForSecrets(stepName, field, content string) *sdk.ValidationIssue {
	for _, pattern := range p.secretPatterns {
		if pattern.MatchString(content) {
			return &sdk.ValidationIssue{
				Severity: sdk.SeverityError,
				Message:  fmt.Sprintf("Potential hardcoded secret detected in %s", field),
				Step:     stepName,
				Field:    field,
			}
		}
	}
	return nil
}

func (p *SecurityValidatorPlugin) checkForDangerousCommands(stepName, command string) *sdk.ValidationIssue {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil
	}

	baseName := strings.TrimPrefix(parts[0], "./")
	baseName = strings.TrimPrefix(baseName, "/bin/")
	baseName = strings.TrimPrefix(baseName, "/usr/bin/")

	for _, dangerous := range p.dangerousWords {
		if strings.Contains(baseName, dangerous) || strings.HasSuffix(baseName, dangerous) {
			return &sdk.ValidationIssue{
				Severity: sdk.SeverityWarning,
				Message:  fmt.Sprintf("Command uses potentially dangerous operation: %s", dangerous),
				Step:     stepName,
				Field:    "command",
			}
		}
	}

	return nil
}

func (p *SecurityValidatorPlugin) checkForCommandInjection(stepName, command string) *sdk.ValidationIssue {
	// Check for unquoted variable interpolation patterns that could enable injection
	if strings.Contains(command, "$") && !strings.Contains(command, "${{") {
		// Allow {{...}} interpolation syntax, warn about shell variable expansion
		if strings.Count(command, "$") > strings.Count(command, "{{") {
			return &sdk.ValidationIssue{
				Severity: sdk.SeverityWarning,
				Message:  "Command contains unquoted variable references; ensure values are properly escaped",
				Step:     stepName,
				Field:    "command",
			}
		}
	}

	return nil
}

func main() {
	sdk.Serve(&SecurityValidatorPlugin{
		BasePlugin: sdk.BasePlugin{
			PluginName:    "security-validator",
			PluginVersion: "1.0.0",
		},
	})
}
