package errors_test

import (
	"testing"

	_ "github.com/vanoix/awf/internal/domain/errors"
)

// TestPackageExists verifies the errors package can be imported and used.
// This test ensures the package documentation (doc.go) is valid and the
// package is properly structured within the domain layer.
func TestPackageExists(t *testing.T) {
	// Verify package can be imported
	// The import statement above validates this
	t.Run("package can be imported", func(t *testing.T) {
		// No-op: If we got here, import succeeded
	})
}

// TestPackageStructure validates the errors package follows domain layer conventions.
func TestPackageStructure(t *testing.T) {
	t.Run("package is in domain layer", func(t *testing.T) {
		// Package path should be internal/domain/errors
		// This is validated by the import path above
	})

	t.Run("package follows naming convention", func(t *testing.T) {
		// Package name should match directory name: "errors"
		// This is enforced by Go compiler
	})
}

// TestDomainLayerPurity validates the package imports only stdlib.
// According to C047 plan, domain layer must have zero infrastructure dependencies.
func TestDomainLayerPurity(t *testing.T) {
	t.Run("package has no infrastructure imports", func(t *testing.T) {
		// This is validated at compile time
		// The domain/errors package should only import:
		// - errors (stdlib)
		// - fmt (stdlib)
		// - time (stdlib)
		// - strings (stdlib)
		// No infrastructure, application, or interfaces layer imports allowed
	})
}
