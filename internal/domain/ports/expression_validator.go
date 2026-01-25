package ports

// ExpressionValidator defines the contract for validating expression syntax.
// This port abstracts expression compilation to maintain domain layer purity
// by avoiding direct dependencies on external expression libraries.
type ExpressionValidator interface {
	// Compile validates the syntax of an expression string.
	// Returns nil if the expression is syntactically valid, error otherwise.
	// This method does NOT evaluate the expression, only checks if it can be compiled.
	Compile(expression string) error
}
