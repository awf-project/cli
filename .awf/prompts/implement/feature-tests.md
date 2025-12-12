You are writing functional tests for feature {{.inputs.feature_id}}.

## PHASE: Feature Functional Tests

## Feature Spec
$SPEC_CONTENT

## Implementation Plan
$PLAN_CONTENT

## Your Task
Write comprehensive **functional tests** that validate the feature works end-to-end:

### Test Categories Required
1. **Happy Path Tests** - Normal usage scenarios work correctly
2. **Edge Cases** - Boundary conditions, empty inputs, max values
3. **Error Handling** - Invalid inputs produce correct errors
4. **Integration** - Feature works with other components

### Test Structure
- File: tests/integration/{component}_test.go (match existing file if extending)
- Build tag: //go:build integration
- Naming: Test{Component}_{Action}_Integration (e.g., TestTemplateService_ExpandWorkflow_Integration)
- REQUIRED: Add file-level comment '// Feature: {{.inputs.feature_id}}'
- Use testify/require for assertions
- Table-driven tests where appropriate
- Real fixtures in tests/fixtures/

### Example Structure
```go
//go:build integration

// Feature: {{.inputs.feature_id}}
package integration

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestComponent_HappyPath_Integration(t *testing.T) {
    // Setup real dependencies
    // Execute feature end-to-end
    // Assert expected behavior
}
```

### Requirements
- Tests MUST compile and run
- Tests MUST use the actual implementation (not mocks)
- Tests MUST cover acceptance criteria from spec
- Create necessary fixtures in tests/fixtures/

## Available Agents
{{.inputs.agents}}

## Available Skills
{{.inputs.skills}}
