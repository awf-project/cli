package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/pkg/httpx"
)

// Compile-time interface check
var _ ports.OperationProvider = (*HTTPOperationProvider)(nil)

// HTTPOperationProvider implements ports.OperationProvider for HTTP operations.
// Dispatches http.request operation to handleHTTPRequest.
//
// The provider manages:
//   - Operation schema registry (http.request)
//   - HTTP client for request execution
//   - Input validation and request construction
//   - Response capture with body size limiting
//   - Retryable status code signaling
type HTTPOperationProvider struct {
	client *httpx.Client
	logger ports.Logger

	// operations holds the registry of HTTP operation schemas
	operations map[string]*pluginmodel.OperationSchema
}

func NewHTTPOperationProvider(client *httpx.Client, logger ports.Logger) *HTTPOperationProvider {
	// Build operation registry from schema definitions
	ops := AllOperations()
	registry := make(map[string]*pluginmodel.OperationSchema, len(ops))
	for i := range ops {
		registry[ops[i].Name] = &ops[i]
	}

	return &HTTPOperationProvider{
		client:     client,
		logger:     logger,
		operations: registry,
	}
}

func (p *HTTPOperationProvider) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	op, found := p.operations[name]
	return op, found
}

func (p *HTTPOperationProvider) ListOperations() []*pluginmodel.OperationSchema {
	result := make([]*pluginmodel.OperationSchema, 0, len(p.operations))
	for _, op := range p.operations {
		result = append(result, op)
	}
	return result
}

// Execute runs an HTTP operation by name with the given inputs.
// Dispatches to handleHTTPRequest for http.request operation.
//
// Implements ports.OperationProvider.
func (p *HTTPOperationProvider) Execute(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
	// Dispatch to handler
	switch name {
	case "http.request":
		return p.handleHTTPRequest(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown operation: %s", name)
	}
}

// handleHTTPRequest executes an HTTP request with the given inputs.
// Validates method and URL, constructs request with headers and body,
// executes via httpx.Client, captures response, and signals retryable failures.
//
// Parameters:
//   - ctx: request context with timeout
//   - inputs: operation inputs (url, method, headers, body, timeout, retryable_status_codes)
//
// Returns:
//   - *pluginmodel.OperationResult: execution result with status_code, body, headers, body_truncated outputs
//   - error: nil on success, non-nil on failure
func (p *HTTPOperationProvider) handleHTTPRequest(ctx context.Context, inputs map[string]any) (*pluginmodel.OperationResult, error) {
	// Validate required inputs
	if err := validateRequiredInputs(inputs); err != nil {
		return failureResult(err.Error()), nil
	}

	// Extract validated inputs (type assertions are safe after validation)
	url, ok := inputs["url"].(string)
	if !ok {
		return failureResult("http.request: url type assertion failed"), nil
	}
	methodRaw, ok := inputs["method"].(string)
	if !ok {
		return failureResult("http.request: method type assertion failed"), nil
	}
	method := strings.ToUpper(methodRaw)

	// Extract optional inputs
	headers := extractHeaders(inputs)
	body := extractBody(inputs)
	timeout, hasTimeout := extractTimeout(inputs)
	retryableStatusCodes := extractRetryableStatusCodes(inputs)

	// Apply timeout to context if specified in inputs
	ctxWithTimeout := ctx
	if hasTimeout {
		var cancel context.CancelFunc
		ctxWithTimeout, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Execute HTTP request via httpx.Client (1MB body limit per NFR-005)
	const maxBodyBytes = 1 << 20
	resp, err := p.client.Do(ctxWithTimeout, method, url, headers, body, maxBodyBytes)
	if err != nil {
		return handleRequestError(err, timeout), nil
	}

	// Check if status code is retryable
	if isRetryableStatus(resp.StatusCode, retryableStatusCodes) {
		return &pluginmodel.OperationResult{
			Success: false,
			Outputs: buildOutputs(resp),
			Error:   fmt.Sprintf("http.request: retryable status code %d", resp.StatusCode),
		}, nil
	}

	// Success - return response data
	return &pluginmodel.OperationResult{
		Success: true,
		Outputs: buildOutputs(resp),
		Error:   "",
	}, nil
}

// validateRequiredInputs validates URL and method inputs
func validateRequiredInputs(inputs map[string]any) error {
	url, ok := inputs["url"].(string)
	if !ok || url == "" {
		return fmt.Errorf("http.request: url is required and must be a non-empty string")
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("http.request: url must start with http:// or https://")
	}

	methodRaw, ok := inputs["method"].(string)
	if !ok || methodRaw == "" {
		return fmt.Errorf("http.request: method is required and must be a non-empty string")
	}

	method := strings.ToUpper(methodRaw)
	validMethods := map[string]bool{
		http.MethodGet:    true,
		http.MethodPost:   true,
		http.MethodPut:    true,
		http.MethodDelete: true,
	}
	if !validMethods[method] {
		return fmt.Errorf("http.request: method must be one of: GET, POST, PUT, DELETE")
	}

	return nil
}

// extractHeaders extracts and converts headers from inputs
func extractHeaders(inputs map[string]any) map[string]string {
	headersRaw, exists := inputs["headers"]
	if !exists || headersRaw == nil {
		return nil
	}

	headersMap, ok := headersRaw.(map[string]any)
	if !ok {
		return nil
	}

	headers := make(map[string]string)
	for k, v := range headersMap {
		if strVal, ok := v.(string); ok {
			headers[k] = strVal
		}
	}
	return headers
}

// extractBody extracts body from inputs
func extractBody(inputs map[string]any) string {
	bodyRaw, exists := inputs["body"]
	if !exists {
		return ""
	}

	body, ok := bodyRaw.(string)
	if !ok {
		return ""
	}
	return body
}

// extractTimeout extracts timeout from inputs
func extractTimeout(inputs map[string]any) (time.Duration, bool) {
	timeoutRaw, exists := inputs["timeout"]
	if !exists {
		return 0, false
	}

	switch v := timeoutRaw.(type) {
	case int:
		if v > 0 {
			return time.Duration(v) * time.Second, true
		}
	case float64:
		if v > 0 {
			return time.Duration(v) * time.Second, true
		}
	}

	return 0, false
}

// extractRetryableStatusCodes extracts retryable status codes from inputs
func extractRetryableStatusCodes(inputs map[string]any) []int {
	retryableRaw, exists := inputs["retryable_status_codes"]
	if !exists {
		return nil
	}

	retryableArray, ok := retryableRaw.([]any)
	if !ok {
		return nil
	}

	var codes []int
	for _, item := range retryableArray {
		switch v := item.(type) {
		case int:
			codes = append(codes, v)
		case float64:
			codes = append(codes, int(v))
		}
	}
	return codes
}

// handleRequestError converts HTTP request errors to operation results.
// timeout is the user-specified timeout (0 means default was used).
func handleRequestError(err error, timeout time.Duration) *pluginmodel.OperationResult {
	if errors.Is(err, context.DeadlineExceeded) {
		if timeout > 0 {
			return failureResult("http.request: request timeout after " + timeout.String())
		}
		return failureResult("http.request: request timeout")
	}
	if errors.Is(err, context.Canceled) {
		return failureResult("http.request: request cancelled")
	}
	return failureResult("http.request: " + err.Error())
}

// isRetryableStatus checks if a status code is in the retryable list
func isRetryableStatus(statusCode int, retryableCodes []int) bool {
	return slices.Contains(retryableCodes, statusCode)
}

// failureResult creates an operation result for failures
func failureResult(errorMsg string) *pluginmodel.OperationResult {
	return &pluginmodel.OperationResult{
		Success: false,
		Outputs: make(map[string]any),
		Error:   errorMsg,
	}
}

// buildOutputs creates the outputs map from HTTP response
func buildOutputs(resp *httpx.Response) map[string]any {
	return map[string]any{
		"status_code":    resp.StatusCode,
		"body":           resp.Body,
		"headers":        resp.Headers,
		"body_truncated": resp.Truncated,
	}
}
