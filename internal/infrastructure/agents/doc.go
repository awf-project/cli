// Package agents implements infrastructure adapters for AI agent integration.
//
// The agents package provides concrete implementations of the AgentProvider and AgentRegistry
// ports defined in the domain layer, enabling workflow steps to invoke AI agents (Claude, Gemini,
// Codex, OpenCode, and OpenAI-compatible endpoints) for code generation, analysis, and decision-making tasks.
// Each provider wraps a CLI executor and handles model-specific invocation patterns, streaming
// output, and error mapping.
//
// # Architecture Role
//
// In the hexagonal architecture:
//   - Implements domain/ports.AgentProvider (per-provider adapters)
//   - Implements domain/ports.AgentRegistry (provider registration and lookup)
//   - Implements domain/ports.CLIExecutor (CLI command execution)
//   - Application layer orchestrates agent steps via these port interfaces
//   - Domain layer defines agent contracts without implementation coupling
//
// All agent providers delegate CLI execution to an injected CLIExecutor, allowing test
// isolation via mock executors. The registry supports runtime provider registration and
// enables workflow steps to reference agents by name (e.g., "claude", "gemini").
//
// # Agent Providers
//
// ## ClaudeProvider (claude_provider.go)
//
// Anthropic Claude provider:
//   - Execute: Single-shot prompt execution via Claude CLI
//   - ExecuteConversation: Multi-turn conversation with context preservation
//   - Name: Returns "claude" for registry lookup
//   - Validate: Checks required options (model, temperature)
//
// Supported models: claude-3-opus, claude-3-sonnet, claude-3-haiku, claude-2.1, claude-2
//
// ## GeminiProvider (gemini_provider.go)
//
// Google Gemini provider:
//   - Execute: Single-shot prompt execution via Gemini CLI
//   - ExecuteConversation: Multi-turn conversation support
//   - Name: Returns "gemini"
//   - Validate: Checks model and API key configuration
//
// ## CodexProvider (codex_provider.go)
//
// OpenAI Codex provider (code-focused GPT models):
//   - Execute: Single-shot code generation via OpenAI CLI
//   - ExecuteConversation: Not supported (returns error)
//   - Name: Returns "codex"
//   - Validate: Checks API key and model configuration
//
// ## OpenCodeProvider (opencode_provider.go)
//
// Open-source code generation models (StarCoder, CodeLlama, etc.):
//   - Execute: Single-shot execution via custom CLI wrapper
//   - ExecuteConversation: Limited support (model-dependent)
//   - Name: Returns "opencode"
//   - Validate: Checks CLI tool availability
//
// ## OpenAICompatibleProvider (openai_compatible_provider.go)
//
// HTTP adapter for any OpenAI-compatible API endpoint (Ollama, LM Studio, vLLM, etc.):
//   - Execute: Single-shot prompt via Chat Completions API
//   - ExecuteConversation: Multi-turn conversation with history
//   - Name: Returns "openai_compatible" for registry lookup
//   - Validate: Checks base_url and model configuration
//
// Configuration via agent options: base_url, api_key, model, temperature.
// Falls back to OPENAI_BASE_URL / OPENAI_MODEL / OPENAI_API_KEY env vars.
//
// # Registry and Discovery
//
// ## AgentRegistry (registry.go)
//
// Provider registration and lookup:
//   - Register: Add provider by name (thread-safe)
//   - Get: Retrieve provider by name
//   - List: Enumerate registered provider names
//   - Has: Check if provider exists
//   - RegisterDefaults: Pre-populate with built-in providers
//
// # CLI Execution
//
// ## ExecCLIExecutor (cli_executor.go)
//
// External binary execution:
//   - Run: Execute command with streaming stdout/stderr, context cancellation
//   - Process cleanup: Kills descendant processes on cancellation
//   - Signal propagation: Forwards SIGINT/SIGTERM to child processes
//
// Used by all agent providers to invoke CLI tools (claude, gemini, gpt, etc.).
//
// # Provider Options
//
// ## Functional Options Pattern (options.go)
//
// Provider configuration via option functions:
//   - WithClaudeExecutor: Inject custom executor for Claude provider
//   - WithGeminiExecutor: Inject custom executor for Gemini provider
//   - WithCodexExecutor: Inject custom executor for Codex provider
//   - WithOpenCodeExecutor: Inject custom executor for OpenCode provider
//   - WithHTTPClient: Inject custom HTTP client for OpenAICompatible provider
//
// Each provider accepts zero or more options at construction time for dependency injection.
package agents
