//go:build integration

// Feature: C049
//
// Architecture verification tests for C049 - Refactor InteractivePrompt Interface Following ISP.
// Validates that InteractivePrompt has been split into focused interfaces (StepPresenter,
// StatusPresenter, UserInteraction) with a composite InteractivePrompt embedding all three,
// ensuring Interface Segregation Principle compliance and hexagonal architecture.

package integration_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInteractivePromptInterfaceCount verifies that interactive.go contains
// exactly 4 interface definitions: 3 focused interfaces + 1 composite interface.
func TestInteractivePromptInterfaceCount(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Domain ports interactive.go file
	portsFile := filepath.Join(repoRoot, "internal", "domain", "ports", "interactive.go")
	require.FileExists(t, portsFile, "interactive.go should exist")

	// When: Parsing the file with AST
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, portsFile, nil, 0)
	require.NoError(t, err, "should parse interactive.go")

	// Then: Count interface type declarations
	interfaces := make(map[string]int) // name -> method count
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok {
				interfaces[ts.Name.Name] = len(iface.Methods.List)
			}
		}
		return true
	})

	// Then: Should have exactly 4 interfaces
	assert.Len(t, interfaces, 4, "should have exactly 4 interfaces (3 focused + 1 composite)")
}

// TestFocusedInterfaceMethodCounts verifies that each focused interface
// has ≤4 methods, satisfying the ISP ≤4 method constraint from acceptance criteria.
func TestFocusedInterfaceMethodCounts(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Domain ports interactive.go file
	portsFile := filepath.Join(repoRoot, "internal", "domain", "ports", "interactive.go")
	require.FileExists(t, portsFile, "interactive.go should exist")

	// When: Parsing the file with AST
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, portsFile, nil, 0)
	require.NoError(t, err, "should parse interactive.go")

	// Extract method counts for each interface
	interfaces := make(map[string]int)
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok {
				interfaces[ts.Name.Name] = len(iface.Methods.List)
			}
		}
		return true
	})

	// Then: StepPresenter should have ≤4 methods
	assert.LessOrEqual(t, interfaces["StepPresenter"], 4,
		"StepPresenter should have ≤4 methods")

	// Then: StatusPresenter should have ≤4 methods
	assert.LessOrEqual(t, interfaces["StatusPresenter"], 4,
		"StatusPresenter should have ≤4 methods")

	// Then: UserInteraction should have ≤4 methods
	assert.LessOrEqual(t, interfaces["UserInteraction"], 4,
		"UserInteraction should have ≤4 methods")
}

// TestCompositeInterfaceEmbedding verifies that InteractivePrompt
// embeds exactly 3 interfaces (StepPresenter, StatusPresenter, UserInteraction)
// and has no direct methods of its own.
func TestCompositeInterfaceEmbedding(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: Domain ports interactive.go file
	portsFile := filepath.Join(repoRoot, "internal", "domain", "ports", "interactive.go")
	require.FileExists(t, portsFile, "interactive.go should exist")

	// When: Parsing the file with AST
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, portsFile, nil, 0)
	require.NoError(t, err, "should parse interactive.go")

	// Find InteractivePrompt interface and analyze its structure
	var embedCount int
	var hasDirectMethods bool
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if ts.Name.Name == "InteractivePrompt" {
				if iface, ok := ts.Type.(*ast.InterfaceType); ok {
					for _, field := range iface.Methods.List {
						// Embedded interfaces have no Names (field.Names is nil/empty)
						if len(field.Names) == 0 {
							embedCount++
						} else {
							hasDirectMethods = true
						}
					}
				}
			}
		}
		return true
	})

	// Then: InteractivePrompt should embed exactly 3 interfaces
	assert.Equal(t, 3, embedCount,
		"InteractivePrompt should embed exactly 3 interfaces")

	// Then: InteractivePrompt should have no direct methods
	assert.False(t, hasDirectMethods,
		"InteractivePrompt should have no direct methods (composition only)")
}

// TestCLIPromptCompileTimeChecks verifies that the CLIPrompt implementation
// has compile-time interface satisfaction checks for all 4 interfaces.
func TestCLIPromptCompileTimeChecks(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	// Given: CLI prompt implementation file
	promptFile := filepath.Join(repoRoot, "internal", "interfaces", "cli", "ui", "interactive_prompt.go")
	require.FileExists(t, promptFile, "interactive_prompt.go should exist")

	// Then: Should have compile-time checks for all 4 interfaces
	interfaceChecks := []string{
		"ports.InteractivePrompt",
		"ports.StepPresenter",
		"ports.StatusPresenter",
		"ports.UserInteraction",
	}

	for _, interfaceName := range interfaceChecks {
		hasCheck := hasCompileTimeInterfaceCheckFlexible(t, promptFile, interfaceName, "CLIPrompt")
		assert.True(t, hasCheck,
			"CLIPrompt should have compile-time check for %s", interfaceName)
	}
}

// Note: findRepoRoot() and hasCompileTimeInterfaceCheckFlexible() helpers
// are imported from c042_dip_compliance_test.go (shared in same package integration_test)
