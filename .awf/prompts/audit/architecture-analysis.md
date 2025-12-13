You are a Go expert performing deep architecture analysis on the awf project.

## Architecture Goal
Hexagonal/Clean Architecture with strict dependency inversion:
- Domain layer MUST have ZERO external dependencies (only stdlib)
- Application layer orchestrates domain via ports
- Infrastructure implements ports
- Interfaces (CLI) wire everything together

## Code Inventory
$INVENTORY

## Domain Layer Imports
$DOMAIN_IMPORTS

## Port Definitions
$PORTS

## Past Architecture Issues (from project history)
$ARCH_VIOLATIONS

Check if any of these past issues have reappeared or if similar patterns exist.

## Your Task
Perform deep architecture analysis:

1. **Domain Purity**
   - Check if domain imports ONLY standard library
   - Allowed: fmt, errors, time, context, strings, strconv, sort, sync, regexp
   - NOT allowed: any third-party packages, any internal/* outside domain

2. **Port/Adapter Pattern**
   - Are all external dependencies behind ports?
   - Is CommandExecutor properly abstracted?
   - Is StateStore properly abstracted?
   - Is WorkflowRepository properly abstracted?

3. **Dependency Direction**
   - Does domain depend on nothing external?
   - Does application only depend on domain?
   - Does infrastructure implement domain ports?

4. **SOLID Principles**
   - Single Responsibility: Are services focused?
   - Open/Closed: Can we extend without modifying?
   - Liskov Substitution: Can adapters be swapped?
   - Interface Segregation: Are ports minimal?
   - Dependency Inversion: Do we depend on abstractions?

## Output Format
- ARCHITECTURE_SCORE: X/10
- DOMAIN_PURITY: PASS/FAIL with details
- VIOLATIONS: List any architecture violations
- PATTERNS_USED: Correctly implemented patterns
- IMPROVEMENTS: Suggested architectural improvements

Use mcp__plugin_common_serena__find_symbol and mcp__plugin_common_serena__get_symbols_overview to explore the code structure.
