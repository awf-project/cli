package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/awf-project/cli/internal/application"
	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/operation"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/analyzer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type validationOperationProvider struct {
	operations map[string]*pluginmodel.OperationSchema
	executed   bool
}

func (p *validationOperationProvider) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	schema, ok := p.operations[name]
	return schema, ok
}

func (p *validationOperationProvider) ListOperations() []*pluginmodel.OperationSchema {
	operations := make([]*pluginmodel.OperationSchema, 0, len(p.operations))
	for _, schema := range p.operations {
		operations = append(operations, schema)
	}
	return operations
}

func (p *validationOperationProvider) Execute(context.Context, string, map[string]any) (*pluginmodel.OperationResult, error) {
	p.executed = true
	return nil, errors.New("validation must not execute operations")
}

func TestValidateLoadedWorkflowRejectsInvalidOperationInputs(t *testing.T) {
	t.Run("accumulates absent operations and invalid required fields", checkOperationValidationAccumulatesAbsentOperationsAndInvalidRequiredFields)
	t.Run("rejects unresolved required input when run inputs are supplied", checkOperationValidationRejectsUnresolvedRequiredInputWhenRunInputsAreSupplied)
	t.Run("resolves operation names from defaults and supplied run inputs", checkOperationValidationResolvesOperationNamesFromDefaultsAndSuppliedRunInputs)
	t.Run("rejects a missing supplied input referenced by an operation name", checkOperationValidationRejectsAMissingSuppliedInputReferencedByAnOperationName)
	t.Run("reports missing inputs in operation name and operation fields", checkOperationValidationReportsMissingInputsInOperationNameAndOperationFields)
	t.Run("applies the static-resolution table without skipping independent fields", checkOperationValidationAppliesTheStaticResolutionTableWithoutSkippingIndependentFields)
	t.Run("defers state-derived fields and accepts valid built-in inputs without mutation", checkOperationValidationDefersStateDerivedFieldsAndAcceptsValidBuiltInInputsWithoutMutation)
	t.Run("does not mutate nested workflow defaults, supplied inputs, or operation schema defaults", checkOperationValidationDoesNotMutateNestedWorkflowDefaultsSuppliedInputsOrOperationSchemaDefaults)
	t.Run("requires run inputs referenced by agent executable fields", checkOperationValidationRequiresRunInputsReferencedByAgentExecutableFields)
	t.Run("accumulates template and operation validation failures", checkOperationValidationAccumulatesTemplateAndOperationValidationFailures)
	t.Run("rejects unavailable inputs in expressions before runtime deferral", checkOperationValidationRejectsUnavailableInputsInExpressionsBeforeRuntimeDeferral)
	t.Run("quotes unusual operation schema keys in diagnostic paths", checkOperationValidationQuotesUnusualOperationSchemaKeysInDiagnosticPaths)
	t.Run("returns typed validation error for nil operation schema", checkOperationValidationReturnsTypedValidationErrorForNilOperationSchema)
	t.Run("does not normalize the caller workflow during validation", checkOperationValidationDoesNotNormalizeTheCallerWorkflowDuringValidation)
}

func checkOperationValidationAccumulatesAbsentOperationsAndInvalidRequiredFields(t *testing.T) {
	provider, service := newOperationValidationFixture()
	wf := operationValidationWorkflow(map[string]*workflow.Step{
		"missing_operation": operationValidationStep("missing_operation", "unknown.operation", map[string]any{}),
		"github_missing":    operationValidationStep("github_missing", "github.create_issue", map[string]any{}),
		"http_nil":          operationValidationStep("http_nil", "http.request", map[string]any{"url": nil}),
		"notification_empty": operationValidationStep("notification_empty", "notification.send", map[string]any{
			"backend": "slack",
			"message": "",
		}),
	})

	err := service.ValidateLoadedWorkflow(context.Background(), wf, "invalid-operations", ports.ValidateOptions{})

	require.Error(t, err)
	structured := collectStructuredErrors(err)
	require.Len(t, structured, 4)
	assertStructuredOperationFailure(t, structured[0], "github.create_issue", "github_missing", []string{
		"steps.github_missing.inputs.repository",
		"steps.github_missing.inputs.title",
	}, operation.ErrInvalidInputs)
	assertStructuredOperationFailure(t, structured[1], "http.request", "http_nil", []string{
		"steps.http_nil.inputs.url",
	}, operation.ErrInvalidInputs)
	assertStructuredOperationFailure(t, structured[2], "unknown.operation", "missing_operation", []string{
		"steps.missing_operation.operation",
	}, operation.ErrOperationNotFound)
	assertStructuredOperationFailure(t, structured[3], "notification.send", "notification_empty", []string{
		"steps.notification_empty.inputs.message",
	}, operation.ErrInvalidInputs)
	assert.False(t, provider.executed)
}

func checkOperationValidationRejectsUnresolvedRequiredInputWhenRunInputsAreSupplied(t *testing.T) {
	_, service := newOperationValidationFixture()
	wf := operationValidationWorkflow(map[string]*workflow.Step{
		"create": operationValidationStep("create", "github.create_issue", map[string]any{
			"repository": "awf-project/cli",
			"title":      "{{inputs.issue_title}}",
		}),
	})
	wf.Inputs = []workflow.Input{{Name: "issue_title", Type: "string", Required: true}}

	err := service.ValidateLoadedWorkflowWithInputs(
		context.Background(), wf, "missing-run-input", ports.ValidateOptions{}, map[string]any{},
	)

	require.Error(t, err)
	structured := collectStructuredErrors(err)
	require.Len(t, structured, 1)
	assertStructuredOperationFailure(t, structured[0], "github.create_issue", "create", []string{
		"steps.create.inputs.title",
	}, operation.ErrInvalidInputs)
}

func checkOperationValidationResolvesOperationNamesFromDefaultsAndSuppliedRunInputs(t *testing.T) {
	_, service := newOperationValidationFixture()
	tests := []struct {
		name       string
		inputs     []workflow.Input
		runInputs  map[string]any
		withInputs bool
	}{
		{
			name: "declared default",
			inputs: []workflow.Input{
				{Name: "operation_name", Type: "string", Default: "github.create_issue"},
			},
		},
		{
			name: "supplied run input",
			inputs: []workflow.Input{
				{Name: "operation_name", Type: "string", Required: true},
			},
			runInputs:  map[string]any{"operation_name": "github.create_issue"},
			withInputs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := operationValidationWorkflow(map[string]*workflow.Step{
				"create": operationValidationStep("create", "{{inputs.operation_name}}", map[string]any{
					"repository": "awf-project/cli",
					"title":      "release",
				}),
			})
			wf.Inputs = tt.inputs

			var err error
			if tt.withInputs {
				err = service.ValidateLoadedWorkflowWithInputs(
					context.Background(), wf, tt.name, ports.ValidateOptions{}, tt.runInputs,
				)
			} else {
				err = service.ValidateLoadedWorkflow(context.Background(), wf, tt.name, ports.ValidateOptions{})
			}

			require.NoError(t, err)
		})
	}
}

func checkOperationValidationRejectsAMissingSuppliedInputReferencedByAnOperationName(t *testing.T) {
	_, service := newOperationValidationFixture()
	wf := operationValidationWorkflow(map[string]*workflow.Step{
		"create": operationValidationStep("create", "{{inputs.operation_name}}", map[string]any{
			"repository": "awf-project/cli",
			"title":      "release",
		}),
	})
	wf.Inputs = []workflow.Input{{Name: "operation_name", Type: "string", Required: true}}

	err := service.ValidateLoadedWorkflowWithInputs(
		context.Background(), wf, "missing-operation-name", ports.ValidateOptions{}, map[string]any{},
	)

	var validationErr workflow.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, workflow.ErrUndefinedInput, validationErr.Code)
	assert.Equal(t, "steps.create.operation", validationErr.Path)
	assert.Equal(t, `referenced input "operation_name" is missing`, validationErr.Message)
}

func checkOperationValidationReportsMissingInputsInOperationNameAndOperationFields(t *testing.T) {
	_, service := newOperationValidationFixture()
	wf := operationValidationWorkflow(map[string]*workflow.Step{
		"create": operationValidationStep("create", "{{inputs.operation_name}}", map[string]any{
			"body":       "{{inputs.typo}}",
			"repository": "awf-project/cli",
			"title":      "{{inputs.issue_title}}",
		}),
	})
	wf.Inputs = []workflow.Input{
		{Name: "operation_name", Type: "string", Required: true},
		{Name: "issue_title", Type: "string", Required: true},
	}

	err := service.ValidateLoadedWorkflowWithInputs(
		context.Background(), wf, "missing-operation-and-field-inputs", ports.ValidateOptions{}, map[string]any{},
	)

	require.Error(t, err)
	assert.Equal(t, []string{
		"steps.create.operation",
		"steps.create.inputs.body",
		"steps.create.inputs.title",
	}, collectValidationErrorPaths(err))
}

func checkOperationValidationAppliesTheStaticResolutionTableWithoutSkippingIndependentFields(t *testing.T) {
	_, service := newOperationValidationFixture()
	tests := []struct {
		name           string
		workflowInputs []workflow.Input
		step           *workflow.Step
		runInputs      map[string]any
		withRunInputs  bool
		wantFields     []string
		wantContains   string
	}{
		{
			name: "document validation resolves declared defaults",
			workflowInputs: []workflow.Input{
				{Name: "issue_title", Type: "string", Default: "default title"},
			},
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "release {{inputs.issue_title}}",
			}),
		},
		{
			name: "validation with run inputs resolves supplied declared input",
			workflowInputs: []workflow.Input{
				{Name: "issue_title", Type: "string", Required: true},
			},
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "release {{inputs.issue_title}}",
			}),
			withRunInputs: true,
			runInputs:     map[string]any{"issue_title": "v1.2.3"},
		},
		{
			name: "document validation defers declared input without default",
			workflowInputs: []workflow.Input{
				{Name: "issue_title", Type: "string", Required: true},
			},
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "{{inputs.issue_title}}",
			}),
		},
		{
			name: "validation with run inputs rejects missing declared input",
			workflowInputs: []workflow.Input{
				{Name: "issue_title", Type: "string", Required: true},
			},
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "{{inputs.issue_title}}",
			}),
			withRunInputs: true,
			runInputs:     map[string]any{},
			wantFields:    []string{"steps.create.inputs.title"},
		},
		{
			name: "undeclared input is rejected",
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "{{inputs.missing_title}}",
			}),
			wantContains: "undefined input",
		},
		{
			name: "state derived value is deferred",
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "{{states.prepare.output}}",
			}),
		},
		{
			name: "mixed literal and default input is resolved",
			workflowInputs: []workflow.Input{
				{Name: "issue_title", Type: "string", Default: "release"},
			},
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "prefix {{inputs.issue_title}} suffix",
			}),
		},
		{
			name: "mixed literal and supplied input is resolved",
			workflowInputs: []workflow.Input{
				{Name: "issue_title", Type: "string", Required: true},
			},
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "prefix {{inputs.issue_title}} suffix",
			}),
			withRunInputs: true,
			runInputs:     map[string]any{"issue_title": "v1.2.3"},
		},
		{
			name: "mixed literal and unavailable input is deferred",
			workflowInputs: []workflow.Input{
				{Name: "issue_title", Type: "string", Required: true},
			},
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "prefix {{inputs.issue_title}} suffix",
			}),
		},
		{
			name: "mixed literal and missing run input is rejected",
			workflowInputs: []workflow.Input{
				{Name: "issue_title", Type: "string", Required: true},
			},
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "prefix {{inputs.issue_title}} suffix",
			}),
			withRunInputs: true,
			runInputs:     map[string]any{},
			wantFields:    []string{"steps.create.inputs.title"},
		},
		{
			name: "mixed state and input checks the input before deferral",
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "{{states.prepare.output}} {{inputs.missing_title}}",
			}),
			wantContains: "undefined input",
		},
		{
			name: "mixed state and missing run input is rejected before deferral",
			workflowInputs: []workflow.Input{
				{Name: "issue_title", Type: "string", Required: true},
			},
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "{{states.prepare.output}} {{inputs.issue_title}}",
			}),
			withRunInputs: true,
			runInputs:     map[string]any{},
			wantFields:    []string{"steps.create.inputs.title"},
		},
		{
			name: "mixed state and supplied input remains deferred",
			workflowInputs: []workflow.Input{
				{Name: "issue_title", Type: "string", Required: true},
			},
			step: operationValidationStep("create", "github.create_issue", map[string]any{
				"repository": "awf-project/cli",
				"title":      "{{states.prepare.output}} {{inputs.issue_title}}",
			}),
			withRunInputs: true,
			runInputs:     map[string]any{"issue_title": "v1.2.3"},
		},
		{
			name: "invalid decidable field is reported beside deferred field",
			step: operationValidationStep("compound", "compound.request", map[string]any{
				"deferred": "{{states.prepare.output}}",
				"url":      "not a URL",
			}),
			wantFields: []string{"steps.compound.inputs.url"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := operationValidationWorkflow(map[string]*workflow.Step{tt.step.Name: tt.step})
			wf.Inputs = tt.workflowInputs

			var err error
			if tt.withRunInputs {
				err = service.ValidateLoadedWorkflowWithInputs(
					context.Background(), wf, tt.name, ports.ValidateOptions{}, tt.runInputs,
				)
			} else {
				err = service.ValidateLoadedWorkflow(context.Background(), wf, tt.name, ports.ValidateOptions{})
			}

			if tt.wantContains != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantContains)
				return
			}
			if tt.wantFields == nil {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			structured := collectStructuredErrors(err)
			require.Len(t, structured, 1)
			assertStructuredOperationFailure(
				t, structured[0], tt.step.Operation, tt.step.Name, tt.wantFields, operation.ErrInvalidInputs,
			)
		})
	}
}

func checkOperationValidationDefersStateDerivedFieldsAndAcceptsValidBuiltInInputsWithoutMutation(t *testing.T) {
	provider, service := newOperationValidationFixture()
	wf := operationValidationWorkflow(map[string]*workflow.Step{
		"github": operationValidationStep("github", "github.create_issue", map[string]any{
			"repository": "awf-project/cli",
			"title":      "{{states.prepare.output}}",
		}),
		"http": operationValidationStep("http", "http.request", map[string]any{
			"url": "https://example.com/hooks",
		}),
		"notification": operationValidationStep("notification", "notification.send", map[string]any{
			"backend": "slack",
			"message": "completed",
		}),
	})
	before := fmt.Sprintf("%#v", wf)

	err := service.ValidateLoadedWorkflow(context.Background(), wf, "valid-operations", ports.ValidateOptions{})

	require.NoError(t, err)
	assert.Equal(t, before, fmt.Sprintf("%#v", wf))
	assert.False(t, provider.executed)
}

func checkOperationValidationDoesNotMutateNestedWorkflowDefaultsSuppliedInputsOrOperationSchemaDefaults(t *testing.T) {
	provider, service := newOperationValidationFixture()
	schemaDefault := map[string]any{
		"metadata": map[string]any{"source": "schema"},
		"tags":     []any{"schema", map[string]any{"nested": true}},
	}
	workflowDefault := map[string]any{
		"metadata": map[string]any{"source": "workflow"},
		"tags":     []any{"workflow", map[string]any{"nested": true}},
	}
	suppliedInputs := map[string]any{
		"run_payload": map[string]any{
			"metadata": map[string]any{"source": "run"},
			"tags":     []any{"run", map[string]any{"nested": true}},
		},
	}
	wantSchemaDefault := map[string]any{
		"metadata": map[string]any{"source": "schema"},
		"tags":     []any{"schema", map[string]any{"nested": true}},
	}
	wantWorkflowDefault := map[string]any{
		"metadata": map[string]any{"source": "workflow"},
		"tags":     []any{"workflow", map[string]any{"nested": true}},
	}
	wantSuppliedInputs := map[string]any{
		"run_payload": map[string]any{
			"metadata": map[string]any{"source": "run"},
			"tags":     []any{"run", map[string]any{"nested": true}},
		},
	}
	provider.operations["nested.defaults"] = &pluginmodel.OperationSchema{
		Name: "nested.defaults",
		Inputs: map[string]pluginmodel.InputSchema{
			"payload": {Type: pluginmodel.InputTypeObject, Default: schemaDefault},
			"title":   {Type: pluginmodel.InputTypeString, Required: true},
		},
	}
	wf := operationValidationWorkflow(map[string]*workflow.Step{
		"create": operationValidationStep("create", "nested.defaults", map[string]any{"title": "release"}),
	})
	wf.Inputs = []workflow.Input{
		{Name: "workflow_payload", Type: "string", Default: workflowDefault},
		{Name: "run_payload", Type: "string", Required: true},
	}

	require.NoError(t, service.ValidateLoadedWorkflow(context.Background(), wf, "nested-defaults", ports.ValidateOptions{}))
	require.NoError(t, service.ValidateLoadedWorkflowWithInputs(
		context.Background(), wf, "nested-supplied", ports.ValidateOptions{}, suppliedInputs,
	))

	assert.Equal(t, wantSchemaDefault, provider.operations["nested.defaults"].Inputs["payload"].Default)
	assert.Equal(t, wantWorkflowDefault, wf.Inputs[0].Default)
	assert.Equal(t, wantSuppliedInputs, suppliedInputs)
}

func checkOperationValidationRequiresRunInputsReferencedByAgentExecutableFields(t *testing.T) {
	_, service := newOperationValidationFixture()
	wf := operationValidationWorkflow(map[string]*workflow.Step{
		"agent": {
			Name: "agent", Type: workflow.StepTypeAgent,
			Agent: &workflow.AgentConfig{
				Provider: "{{inputs.provider}}", Prompt: "{{inputs.prompt}}",
				Options: map[string]any{"model": "{{inputs.model}}"},
			},
		},
	})
	wf.Inputs = []workflow.Input{
		{Name: "provider", Type: "string", Required: true},
		{Name: "model", Type: "string", Required: true},
		{Name: "prompt", Type: "string", Required: true},
	}

	err := service.ValidateLoadedWorkflowWithInputs(
		context.Background(), wf, "missing-agent-inputs", ports.ValidateOptions{}, map[string]any{},
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, "steps.agent.agent.provider")
	assert.ErrorContains(t, err, "steps.agent.agent.prompt")
	assert.ErrorContains(t, err, "steps.agent.agent.options.model")
}

func checkOperationValidationAccumulatesTemplateAndOperationValidationFailures(t *testing.T) {
	provider, _ := newOperationValidationFixture()
	analyzingService := application.NewWorkflowService(nil, nil, nil, nil, nil)
	analyzingService.SetPluginOperationProvider(provider)
	analyzingService.SetTemplateAnalyzer(analyzer.NewInterpolationAnalyzer())
	wf := operationValidationWorkflow(map[string]*workflow.Step{
		"create": operationValidationStep("create", "github.create_issue", map[string]any{
			"repository": "awf-project/cli",
			"title":      "",
		}),
		"notify": {
			Name: "notify", Type: workflow.StepTypeAgent,
			Agent: &workflow.AgentConfig{Provider: "claude", Prompt: "{{inputs.undeclared}}"},
		},
	})

	err := analyzingService.ValidateLoadedWorkflow(context.Background(), wf, "mixed-errors", ports.ValidateOptions{})

	require.Error(t, err)
	assert.ErrorContains(t, err, "undefined input")
	structured := collectStructuredErrors(err)
	require.Len(t, structured, 1)
	assertStructuredOperationFailure(t, structured[0], "github.create_issue", "create", []string{
		"steps.create.inputs.title",
	}, operation.ErrInvalidInputs)
}

func checkOperationValidationRejectsUnavailableInputsInExpressionsBeforeRuntimeDeferral(t *testing.T) {
	provider, _ := newOperationValidationFixture()
	analyzingService := application.NewWorkflowService(nil, nil, nil, nil, nil)
	analyzingService.SetPluginOperationProvider(provider)
	analyzingService.SetTemplateAnalyzer(analyzer.NewInterpolationAnalyzer())
	wf := operationValidationWorkflow(map[string]*workflow.Step{
		"compound": operationValidationStep("compound", "compound.request", map[string]any{
			"deferred": "{{states.prepare.output}} {{inputs.count + 1}}",
			"url":      "https://example.com",
		}),
	})
	wf.Inputs = []workflow.Input{{Name: "count", Type: "integer", Required: true}}

	err := analyzingService.ValidateLoadedWorkflowWithInputs(
		context.Background(), wf, "missing-expression-input", ports.ValidateOptions{}, map[string]any{},
	)

	require.Error(t, err)
	structured := collectStructuredErrors(err)
	require.Len(t, structured, 1)
	assertStructuredOperationFailure(t, structured[0], "compound.request", "compound", []string{
		"steps.compound.inputs.deferred",
	}, operation.ErrInvalidInputs)
}

func checkOperationValidationQuotesUnusualOperationSchemaKeysInDiagnosticPaths(t *testing.T) {
	provider, service := newOperationValidationFixture()
	provider.operations["unusual.request"] = &pluginmodel.OperationSchema{
		Name: "unusual.request",
		Inputs: map[string]pluginmodel.InputSchema{
			"api-key": {Type: pluginmodel.InputTypeString, Required: true},
		},
	}
	wf := operationValidationWorkflow(map[string]*workflow.Step{
		"request": operationValidationStep("request", "unusual.request", map[string]any{}),
	})

	err := service.ValidateLoadedWorkflow(context.Background(), wf, "unusual-key", ports.ValidateOptions{})

	require.Error(t, err)
	structured := collectStructuredErrors(err)
	require.Len(t, structured, 1)
	assertStructuredOperationFailure(t, structured[0], "unusual.request", "request", []string{
		`steps.request.inputs["api-key"]`,
	}, operation.ErrInvalidInputs)
}

func checkOperationValidationReturnsTypedValidationErrorForNilOperationSchema(t *testing.T) {
	provider, service := newOperationValidationFixture()
	provider.operations["broken.operation"] = nil
	wf := operationValidationWorkflow(map[string]*workflow.Step{
		"broken": operationValidationStep("broken", "broken.operation", map[string]any{}),
	})

	var err error
	assert.NotPanics(t, func() {
		err = service.ValidateLoadedWorkflow(context.Background(), wf, "nil-schema", ports.ValidateOptions{})
	})
	require.Error(t, err)
	structured := collectStructuredErrors(err)
	require.Len(t, structured, 1)
	assertStructuredOperationFailure(t, structured[0], "broken.operation", "broken", []string{
		"steps.broken.inputs",
	}, operation.ErrInvalidInputs)
}

func checkOperationValidationDoesNotNormalizeTheCallerWorkflowDuringValidation(t *testing.T) {
	validatorService := application.NewWorkflowService(nil, nil, nil, nil, &noopValidator{})
	wf := &workflow.Workflow{
		Name: "immutable", Initial: "agent",
		Steps: map[string]*workflow.Step{
			"agent": {
				Name: "agent", Type: workflow.StepTypeAgent, OnSuccess: "done",
				Agent: &workflow.AgentConfig{Provider: "  claude  ", Prompt: "hello"},
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	err := validatorService.ValidateLoadedWorkflow(context.Background(), wf, "immutable", ports.ValidateOptions{})

	require.NoError(t, err)
	assert.Equal(t, "  claude  ", wf.Steps["agent"].Agent.Provider)
	assert.Empty(t, wf.Steps["agent"].Agent.Mode)
}

func newOperationValidationFixture() (*validationOperationProvider, *application.WorkflowService) {
	provider := &validationOperationProvider{operations: validationOperationSchemas()}
	service := application.NewWorkflowService(nil, nil, nil, nil, nil)
	service.SetPluginOperationProvider(provider)
	return provider, service
}

func TestOperationValidationNestedDiagnosticsAreDeterministic(t *testing.T) {
	t.Run("missing and undefined references retain nested diagnostic paths", func(t *testing.T) {
		_, service := newOperationValidationFixture()
		wf := operationValidationWorkflow(map[string]*workflow.Step{
			"nested.step": operationValidationStep("nested.step", "{{inputs.operation}}", map[string]any{
				"z":   "{{inputs.missing}}",
				"a.b": []any{map[string]any{"x": "{{inputs.typo}}"}, "{{inputs.missing}}", 42},
			}),
		})
		wf.Inputs = []workflow.Input{{Name: "operation", Type: "string"}, {Name: "missing", Type: "string"}}
		err := service.ValidateLoadedWorkflowWithInputs(context.Background(), wf, "nested", ports.ValidateOptions{}, nil)
		require.Error(t, err)
		assert.Equal(t, []string{
			`steps["nested.step"].operation`,
			`steps["nested.step"].inputs["a.b"][0].x`,
			`steps["nested.step"].inputs["a.b"][1]`,
			`steps["nested.step"].inputs.z`,
		}, collectValidationErrorPaths(err))
	})

	t.Run("static resolution orders errors across nested maps and preserves the input", func(t *testing.T) {
		_, service := newOperationValidationFixture()
		values := map[string]any{
			"title": map[string]any{
				"z": "{{inputs.zeta}}",
				"a": []any{"{{inputs.alpha}}", map[string]any{"x": "{{inputs.beta}}"}},
			},
			"body": "{{inputs.first}}",
		}
		wf := operationValidationWorkflow(map[string]*workflow.Step{
			"create": operationValidationStep("create", "github.create_issue", values),
		})
		const want = "[error] undefined input \"first\"\n[error] undefined input \"alpha\"\n[error] undefined input \"beta\"\n[error] undefined input \"zeta\""
		for range 20 {
			err := service.ValidateLoadedWorkflow(context.Background(), wf, "nested", ports.ValidateOptions{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), want)
		}
		assert.Equal(t, "{{inputs.zeta}}", values["title"].(map[string]any)["z"])
	})
}

func validationOperationSchemas() map[string]*pluginmodel.OperationSchema {
	return map[string]*pluginmodel.OperationSchema{
		"github.create_issue": {
			Name: "github.create_issue",
			Inputs: map[string]pluginmodel.InputSchema{
				"repository": {Type: pluginmodel.InputTypeString, Required: true},
				"title":      {Type: pluginmodel.InputTypeString, Required: true},
			},
		},
		"http.request": {
			Name: "http.request",
			Inputs: map[string]pluginmodel.InputSchema{
				"url": {Type: pluginmodel.InputTypeString, Required: true, Validation: "url"},
			},
		},
		"notification.send": {
			Name: "notification.send",
			Inputs: map[string]pluginmodel.InputSchema{
				"backend": {Type: pluginmodel.InputTypeString, Required: true},
				"message": {Type: pluginmodel.InputTypeString, Required: true},
			},
		},
		"compound.request": {
			Name: "compound.request",
			Inputs: map[string]pluginmodel.InputSchema{
				"deferred": {Type: pluginmodel.InputTypeString, Required: true},
				"url":      {Type: pluginmodel.InputTypeString, Required: true, Validation: "url"},
			},
		},
	}
}

func operationValidationWorkflow(steps map[string]*workflow.Step) *workflow.Workflow {
	return &workflow.Workflow{Name: "operation-validation", Initial: "unused", Steps: steps}
}

func operationValidationStep(name, operationName string, inputs map[string]any) *workflow.Step {
	return &workflow.Step{
		Name:            name,
		Type:            workflow.StepTypeOperation,
		Operation:       operationName,
		OperationInputs: inputs,
	}
}

func collectStructuredErrors(err error) []*domainerrors.StructuredError {
	var collected []*domainerrors.StructuredError
	var visit func(error)
	visit = func(current error) {
		if current == nil {
			return
		}
		if joined, ok := current.(interface{ Unwrap() []error }); ok {
			for _, child := range joined.Unwrap() {
				visit(child)
			}
			return
		}
		var structured *domainerrors.StructuredError
		if errors.As(current, &structured) {
			collected = append(collected, structured)
		}
	}
	visit(err)
	return collected
}

func collectValidationErrorPaths(err error) []string {
	var paths []string
	var visit func(error)
	visit = func(current error) {
		if current == nil {
			return
		}
		if joined, ok := current.(interface{ Unwrap() []error }); ok {
			for _, child := range joined.Unwrap() {
				visit(child)
			}
			return
		}
		var validationErr workflow.ValidationError
		if errors.As(current, &validationErr) {
			paths = append(paths, validationErr.Path)
		}
	}
	visit(err)
	return paths
}

func assertStructuredOperationFailure(
	t *testing.T,
	err *domainerrors.StructuredError,
	operationName string,
	stepName string,
	fieldPaths []string,
	cause error,
) {
	t.Helper()
	assert.Equal(t, domainerrors.ErrorCodeUserInputValidationFailed, err.Code)
	assert.Equal(t, operationName, err.Details["operation"])
	assert.Equal(t, stepName, err.Details["step"])
	assert.Equal(t, fieldPaths, err.Details["field_paths"])
	assert.ErrorIs(t, err, cause)
}
