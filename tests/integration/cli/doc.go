//go:build integration

package cli_test

// Package cli_test contains integration tests for the AWF CLI commands.
// These tests execute full command flows including workflow parsing,
// state management, and command execution.
//
// To run these tests:
//
//	make test-integration
//	go test -tags=integration ./tests/integration/cli/...
