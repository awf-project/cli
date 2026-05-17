package api

import (
	"context"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypes_RunWorkflowOutput_OpenAPISchemaContainsExecutionID(t *testing.T) {
	_, api := humatest.New(t)

	// Register a minimal operation that returns RunWorkflowOutput to inspect the schema.
	huma.Register(api, huma.Operation{
		Method:      "POST",
		Path:        "/test",
		OperationID: "test-output",
	}, func(ctx context.Context, _ *struct{}) (*RunWorkflowOutput, error) {
		return nil, nil
	})

	spec := api.OpenAPI()
	require.NotNil(t, spec)
	require.NotNil(t, spec.Paths)

	testPath := spec.Paths["/test"]
	require.NotNil(t, testPath)
	require.NotNil(t, testPath.Post)

	response := testPath.Post.Responses["200"]
	require.NotNil(t, response)

	schema := response.Content["application/json"].Schema
	require.NotNil(t, schema)

	// The schema should have Body property with execution_id field.
	bodySchema := schema.Properties["body"]
	require.NotNil(t, bodySchema)

	// execution_id field must exist in the Body schema.
	executionIDSchema := bodySchema.Properties["execution_id"]
	assert.NotNil(t, executionIDSchema, "OpenAPI schema must contain execution_id field in RunWorkflowOutput.Body")
}
