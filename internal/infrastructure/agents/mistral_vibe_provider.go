package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

var _ ports.AgentProvider = (*MistralVibeProvider)(nil)

type MistralVibeProvider struct {
	base      *baseCLIProvider
	logger    ports.Logger
	executor  ports.CLIExecutor
	tokenizer ports.Tokenizer
}

func NewMistralVibeProvider() *MistralVibeProvider {
	return NewMistralVibeProviderWithOptions()
}

func NewMistralVibeProviderWithOptions(opts ...MistralVibeProviderOption) *MistralVibeProvider {
	p := &MistralVibeProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.base = p.newBase()
	return p
}

func (p *MistralVibeProvider) newBase() *baseCLIProvider {
	b := newBaseCLIProvider("mistral_vibe", "vibe", p.executor, p.logger, cliProviderHooks{
		buildExecuteArgs:      p.buildExecuteArgs,
		buildConversationArgs: p.buildConversationArgs,
		extractTextContent:    p.extractTextContent,
		extractSessionID:      p.extractSessionID,
		validateOptions:       validateMistralVibeOptions,
		mcpInjector:           p.mistralVibeMCPInjector,
	})
	if p.tokenizer != nil {
		b.tokenizer = p.tokenizer
	}
	return b
}

func (p *MistralVibeProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	result, rawOutput, err := p.base.execute(ctx, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}

	output, found := extractMistralVibeAssistantText(rawOutput)
	result.Response = nil
	if found {
		result.Response = tryParseJSONResponse(output)
	} else {
		output = extractMistralVibeTextContent(rawOutput)
	}
	if result.TokensEstimated {
		tokens, _ := p.base.tokenizer.CountTokens(output) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
		result.Tokens = tokens
	}

	result.Output = strings.TrimSpace(output)
	return result, nil
}

func (p *MistralVibeProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	result, rawOutput, err := p.base.executeConversation(ctx, state, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}

	output := strings.TrimSpace(extractMistralVibeTextContent(rawOutput))
	result.Output = output
	if result.State == nil || len(result.State.Turns) == 0 {
		return result, nil
	}

	lastTurn := &result.State.Turns[len(result.State.Turns)-1]
	if lastTurn.Role != workflow.TurnRoleAssistant {
		return result, nil
	}

	lastTurn.Content = output
	if result.TokensEstimated {
		tokens, _ := p.base.tokenizer.CountTokens(output) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
		lastTurn.Tokens = tokens
		result.TokensOutput = tokens
		result.TokensTotal = result.TokensInput + tokens
	}
	return result, nil
}

func (p *MistralVibeProvider) Name() string {
	return "mistral_vibe"
}

func (p *MistralVibeProvider) Validate() error {
	_, err := exec.LookPath("vibe")
	if err != nil {
		return errors.New("mistral_vibe provider validation failed: vibe binary not found in PATH")
	}
	return nil
}

func (p *MistralVibeProvider) buildExecuteArgs(prompt string, options map[string]any) ([]string, error) {
	return appendMistralVibeOptions([]string{"--prompt", prompt}, options)
}

func (p *MistralVibeProvider) buildConversationArgs(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
	return appendMistralVibeOptions([]string{"--prompt", mistralVibeConversationPrompt(state, prompt)}, options)
}

func mistralVibeConversationPrompt(state *workflow.ConversationState, prompt string) string {
	if state == nil || len(state.Turns) == 0 {
		return prompt
	}

	var b strings.Builder
	b.WriteString("Continue the conversation using the transcript below. Answer the final user message.\n\nTranscript:\n")
	for _, turn := range state.Turns {
		content := strings.TrimSpace(turn.Content)
		if content == "" {
			continue
		}
		b.WriteString(mistralVibeTurnRoleLabel(turn.Role))
		b.WriteString(": ")
		b.WriteString(content)
		b.WriteString("\n")
	}
	b.WriteString("User: ")
	b.WriteString(prompt)
	return b.String()
}

func mistralVibeTurnRoleLabel(role workflow.TurnRole) string {
	switch role {
	case workflow.TurnRoleSystem:
		return "System"
	case workflow.TurnRoleUser:
		return "User"
	case workflow.TurnRoleAssistant:
		return "Assistant"
	default:
		return "Turn"
	}
}

func (p *MistralVibeProvider) mistralVibeMCPInjector(_ context.Context, args []string, cfg *workflow.MCPProxyConfig, mcpConfigPath string, options map[string]any) (newArgs []string, newOptions map[string]any, cleanup func() error, err error) {
	if cfg == nil {
		return args, options, noopMCPCleanup, nil
	}
	if strings.TrimSpace(mcpConfigPath) == "" {
		return nil, options, noopMCPCleanup, fmt.Errorf("mistral_vibe mcp config path is empty")
	}

	vibeHome, cleanup, err := writeMistralVibeMCPHome(mcpConfigPath)
	if err != nil {
		return nil, options, noopMCPCleanup, err
	}

	newArgs = make([]string, len(args), len(args)+16)
	copy(newArgs, args)
	for _, tool := range mistralVibeMCPEnabledTools(cfg) {
		newArgs = append(newArgs, "--enabled-tools", tool)
	}

	newOpts := make(map[string]any, len(options)+1)
	for key, value := range options {
		newOpts[key] = value
	}
	env := map[string]string{"VIBE_HOME": vibeHome}
	if existing, ok := newOpts[cliProviderEnvOptionKey].(map[string]string); ok {
		env = make(map[string]string, len(existing)+1)
		for key, value := range existing {
			env[key] = value
		}
		env["VIBE_HOME"] = vibeHome
	}
	newOpts[cliProviderEnvOptionKey] = env

	return newArgs, newOpts, cleanup, nil
}

func mistralVibeMCPEnabledTools(cfg *workflow.MCPProxyConfig) []string {
	if cfg == nil {
		return nil
	}

	tools := make([]string, 0, 6+len(cfg.PluginTools))
	if cfg.InterceptBuiltins {
		for _, name := range []string{"Bash", "Edit", "Glob", "Grep", "Read", "Write"} {
			tools = append(tools, "awf-proxy_"+name)
		}
	}
	for _, plugin := range cfg.PluginTools {
		for _, op := range plugin.Expose {
			tools = append(tools, "awf-proxy_"+plugin.Plugin+"_"+op)
		}
	}
	return tools
}

func writeMistralVibeMCPHome(internalConfigPath string) (path string, cleanup func() error, err error) {
	cmd := mcpServeCommand(internalConfigPath)
	if len(cmd) == 0 {
		return "", noopMCPCleanup, fmt.Errorf("mistral_vibe mcp config: empty mcp-serve command")
	}

	tmpHome, err := os.MkdirTemp("", "awf-mistral-vibe-mcp-*")
	if err != nil {
		return "", noopMCPCleanup, fmt.Errorf("create mistral_vibe VIBE_HOME: %w", err)
	}

	if copyErr := copyMistralVibeConfig(tmpHome); copyErr != nil {
		_ = os.RemoveAll(tmpHome)
		return "", noopMCPCleanup, copyErr
	}
	if copyErr := copyMistralVibeEnvFile(tmpHome); copyErr != nil {
		_ = os.RemoveAll(tmpHome)
		return "", noopMCPCleanup, copyErr
	}

	configPath := filepath.Join(tmpHome, "config.toml")
	f, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		_ = os.RemoveAll(tmpHome)
		return "", noopMCPCleanup, fmt.Errorf("open mistral_vibe config: %w", err)
	}
	if _, err := f.WriteString(mistralVibeMCPConfigBlock(cmd)); err != nil {
		_ = f.Close()
		_ = os.RemoveAll(tmpHome)
		return "", noopMCPCleanup, fmt.Errorf("write mistral_vibe mcp config: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.RemoveAll(tmpHome)
		return "", noopMCPCleanup, fmt.Errorf("close mistral_vibe mcp config: %w", err)
	}

	var once sync.Once
	cleanup = func() error {
		var removeErr error
		once.Do(func() {
			removeErr = os.RemoveAll(tmpHome)
		})
		return removeErr
	}
	return tmpHome, cleanup, nil
}

func copyMistralVibeConfig(tmpHome string) error {
	sourceHome, ok := mistralVibeSourceHome()
	if !ok {
		return nil
	}

	data, err := os.ReadFile(filepath.Join(sourceHome, "config.toml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read mistral_vibe user config.toml: %w", err)
	}
	sanitized := sanitizeMistralVibeConfig(string(data))
	if sanitized == "" {
		return nil
	}
	if err := os.WriteFile(filepath.Join(tmpHome, "config.toml"), []byte(sanitized), 0o600); err != nil {
		return fmt.Errorf("copy mistral_vibe user config.toml: %w", err)
	}
	return nil
}

func copyMistralVibeEnvFile(tmpHome string) error {
	sourceHome, ok := mistralVibeSourceHome()
	if !ok {
		return nil
	}

	data, err := os.ReadFile(filepath.Join(sourceHome, ".env"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read mistral_vibe user .env: %w", err)
	}
	//nolint:gosec // tmpHome is an os.MkdirTemp-created directory, not user-controlled input.
	if err := os.WriteFile(filepath.Join(tmpHome, ".env"), data, 0o600); err != nil {
		return fmt.Errorf("copy mistral_vibe user .env: %w", err)
	}
	return nil
}

func mistralVibeSourceHome() (string, bool) {
	sourceHome := os.Getenv("VIBE_HOME")
	if sourceHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		sourceHome = filepath.Join(home, ".vibe")
	}
	return sourceHome, true
}

func sanitizeMistralVibeConfig(config string) string {
	lines := strings.Split(config, "\n")
	out := make([]string, 0, len(lines))
	skipMCPTable := false
	skipMCPAssignment := false
	assignmentDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if skipMCPTable {
			if strings.HasPrefix(trimmed, "[") && trimmed != "[[mcp_servers]]" {
				skipMCPTable = false
			} else {
				continue
			}
		}

		if skipMCPAssignment {
			assignmentDepth += strings.Count(line, "[")
			assignmentDepth -= strings.Count(line, "]")
			if assignmentDepth <= 0 {
				skipMCPAssignment = false
			}
			continue
		}

		if trimmed == "[[mcp_servers]]" {
			skipMCPTable = true
			continue
		}
		if isMistralVibeMCPServersAssignment(trimmed) {
			assignmentDepth = strings.Count(line, "[") - strings.Count(line, "]")
			if assignmentDepth > 0 {
				skipMCPAssignment = true
			}
			continue
		}

		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func isMistralVibeMCPServersAssignment(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "mcp_servers") {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "mcp_servers"))
	return strings.HasPrefix(rest, "=")
}

func mistralVibeMCPConfigBlock(cmd []string) string {
	var b strings.Builder
	b.WriteString("\n\n[[mcp_servers]]\n")
	b.WriteString("name = \"awf-proxy\"\n")
	b.WriteString("transport = \"stdio\"\n")
	b.WriteString("command = ")
	b.WriteString(strconv.Quote(cmd[0]))
	b.WriteString("\nargs = [")
	for i, arg := range cmd[1:] {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(arg))
	}
	b.WriteString("]\nsampling_enabled = false\n")
	return b.String()
}

func appendMistralVibeOptions(args []string, options map[string]any) ([]string, error) {
	args = append(args, "--agent", mistralVibeAgentProfile(options))

	if output, ok := getStringOption(options, "output_format"); ok && output != "" {
		args = append(args, "--output", output)
	}
	if maxTurns, ok := mistralVibeIntOption(options, "max_turns"); ok {
		args = append(args, "--max-turns", strconv.FormatInt(maxTurns, 10))
	}
	if maxTokens, ok := mistralVibeIntOption(options, "max_tokens"); ok {
		args = append(args, "--max-tokens", strconv.FormatInt(maxTokens, 10))
	}
	if maxPrice, ok := mistralVibeNumberOption(options, "max_price"); ok {
		args = append(args, "--max-price", maxPrice)
	}
	if workdir, ok := getStringOption(options, "workdir"); ok {
		cleaned, err := cleanMistralVibePath(workdir)
		if err != nil {
			return nil, fmt.Errorf("mistral_vibe option workdir invalid: %w", err)
		}
		args = append(args, "--workdir", cleaned)
	}
	if enabledTools, ok := options["enabled_tools"]; ok {
		tools, err := mistralVibeStringList("enabled_tools", enabledTools)
		if err != nil {
			return nil, err
		}
		for _, tool := range tools {
			args = append(args, "--enabled-tools", tool)
		}
	}
	if addDirs, ok := options["add_dirs"]; ok {
		dirs, err := mistralVibePathList(addDirs)
		if err != nil {
			return nil, err
		}
		for _, dir := range dirs {
			args = append(args, "--add-dir", dir)
		}
	}
	if trust, ok := getBoolOption(options, "trust"); ok && trust {
		args = append(args, "--trust")
	}

	return args, nil
}

func validateMistralVibeOptions(options map[string]any) error {
	if err := validateMistralVibeApprovalOptions(options); err != nil {
		return err
	}
	if err := validateMistralVibeOutputFormat(options); err != nil {
		return err
	}
	if err := validateMistralVibePositiveIntOption(options, "max_turns"); err != nil {
		return err
	}
	if err := validateMistralVibePositiveIntOption(options, "max_tokens"); err != nil {
		return err
	}
	if err := validateMistralVibeNonNegativeNumberOption(options, "max_price"); err != nil {
		return err
	}
	if err := validateMistralVibeWorkdir(options); err != nil {
		return err
	}
	if err := validateMistralVibeListOptions(options); err != nil {
		return err
	}
	return validateMistralVibeTrust(options)
}

func validateMistralVibeApprovalOptions(options map[string]any) error {
	skip, ok := getBoolOption(options, "dangerously_skip_permissions")
	if !ok || !skip {
		return nil
	}
	profile, hasProfile := getStringOption(options, "agent_profile")
	if hasProfile && profile != "auto-approve" {
		return fmt.Errorf("mistral_vibe option agent_profile conflicts with dangerously_skip_permissions: expected auto-approve, got %s", profile)
	}
	return nil
}

func validateMistralVibeOutputFormat(options map[string]any) error {
	output, ok := getStringOption(options, "output_format")
	if !ok || output == "" {
		return nil
	}
	switch output {
	case "text", "json", "streaming":
		return nil
	default:
		return fmt.Errorf("mistral_vibe option output_format invalid: expected one of text, json, streaming, got %s", output)
	}
}

func validateMistralVibePositiveIntOption(options map[string]any, name string) error {
	value, ok := options[name]
	if !ok {
		return nil
	}
	intValue, valid := mistralVibeIntValue(value)
	if !valid {
		return fmt.Errorf("mistral_vibe option %s invalid: expected positive integer, got %s", name, mistralVibeOptionTypeName(value))
	}
	if intValue <= 0 {
		return fmt.Errorf("mistral_vibe option %s invalid: expected positive integer, got %d", name, intValue)
	}
	return nil
}

func validateMistralVibeNonNegativeNumberOption(options map[string]any, name string) error {
	value, ok := options[name]
	if !ok {
		return nil
	}
	number, valid := mistralVibeNumberValue(value)
	if !valid || math.IsNaN(number) || math.IsInf(number, 0) {
		return fmt.Errorf("mistral_vibe option %s invalid: expected non-negative number, got %s", name, mistralVibeOptionTypeName(value))
	}
	if number < 0 {
		return fmt.Errorf("mistral_vibe option %s invalid: expected non-negative number, got %s", name, mistralVibeNumberString(value))
	}
	return nil
}

func validateMistralVibeWorkdir(options map[string]any) error {
	workdir, ok := getStringOption(options, "workdir")
	if !ok {
		return nil
	}
	if _, err := cleanMistralVibePath(workdir); err != nil {
		return fmt.Errorf("mistral_vibe option workdir invalid: %w", err)
	}
	return nil
}

func validateMistralVibeListOptions(options map[string]any) error {
	if enabledTools, ok := options["enabled_tools"]; ok {
		if _, err := mistralVibeStringList("enabled_tools", enabledTools); err != nil {
			return err
		}
	}
	if addDirs, ok := options["add_dirs"]; ok {
		if _, err := mistralVibePathList(addDirs); err != nil {
			return err
		}
	}
	return nil
}

func validateMistralVibeTrust(options map[string]any) error {
	value, ok := options["trust"]
	if !ok {
		return nil
	}
	if _, valid := value.(bool); !valid {
		return fmt.Errorf("mistral_vibe option trust invalid: expected bool, got %s", mistralVibeOptionTypeName(value))
	}
	return nil
}

func mistralVibeAgentProfile(options map[string]any) string {
	if skip, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skip {
		return "auto-approve"
	}
	if profile, ok := getStringOption(options, "agent_profile"); ok && profile != "" {
		return profile
	}
	return "default"
}

func mistralVibeIntOption(options map[string]any, name string) (int64, bool) {
	value, ok := options[name]
	if !ok {
		return 0, false
	}
	intValue, valid := mistralVibeIntValue(value)
	return intValue, valid
}

func mistralVibeNumberOption(options map[string]any, name string) (string, bool) {
	value, ok := options[name]
	if !ok {
		return "", false
	}
	if _, valid := mistralVibeNumberValue(value); !valid {
		return "", false
	}
	return mistralVibeNumberString(value), true
}

func mistralVibeIntValue(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		if uint64(v) > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		if v > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(v), true
	default:
		return 0, false
	}
}

func mistralVibeNumberValue(value any) (float64, bool) {
	switch v := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		intValue, ok := mistralVibeIntValue(v)
		return float64(intValue), ok
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}

func mistralVibeNumberString(value any) string {
	switch v := value.(type) {
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func mistralVibeStringList(name string, value any) ([]string, error) {
	switch list := value.(type) {
	case []string:
		return list, nil
	case []any:
		values := make([]string, 0, len(list))
		for idx, item := range list {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("mistral_vibe option %s invalid: expected string at index %d, got %s", name, idx, mistralVibeOptionTypeName(item))
			}
			values = append(values, str)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("mistral_vibe option %s invalid: expected list, got %s", name, mistralVibeOptionTypeName(value))
	}
}

func mistralVibePathList(value any) ([]string, error) {
	paths, err := mistralVibeStringList("add_dirs", value)
	if err != nil {
		return nil, err
	}
	cleanedPaths := make([]string, 0, len(paths))
	for idx, path := range paths {
		cleaned, cleanErr := cleanMistralVibePath(path)
		if cleanErr != nil {
			return nil, fmt.Errorf("mistral_vibe option add_dirs invalid at index %d: %w", idx, cleanErr)
		}
		cleanedPaths = append(cleanedPaths, cleaned)
	}
	return cleanedPaths, nil
}

func cleanMistralVibePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path cannot be empty")
	}
	cleaned := filepath.Clean(path)
	if hasMistralVibeTraversalSegment(path) || hasMistralVibeTraversalSegment(cleaned) {
		return "", errors.New("path must not contain traversal segments")
	}
	return cleaned, nil
}

func hasMistralVibeTraversalSegment(path string) bool {
	for _, segment := range strings.Split(path, string(filepath.Separator)) {
		if segment == ".." {
			return true
		}
	}
	return false
}

func mistralVibeOptionTypeName(value any) string {
	if value == nil {
		return "nil"
	}
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "bool"
	case int, int8, int16, int32, int64:
		return "int"
	case uint, uint8, uint16, uint32, uint64:
		return "uint"
	case float32, float64:
		return "float"
	case []string, []any:
		return "list"
	}
	if reflect.TypeOf(value).Kind() == reflect.Map {
		return "map"
	}
	return fmt.Sprintf("%T", value)
}

func (p *MistralVibeProvider) extractTextContent(output string) string {
	return extractMistralVibeTextContent(output)
}

func extractMistralVibeTextContent(output string) string {
	if text, found := extractMistralVibeAssistantText(output); found {
		return text
	}
	if isMistralVibeEnvelopeLike(output) {
		return ""
	}
	return output
}

func extractMistralVibeAssistantText(output string) (string, bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "", false
	}

	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		text, found := extractMistralVibeAssistantTextFromJSONDocument(trimmed)
		if found || !isMistralVibeNDJSONLike(trimmed) {
			return text, found
		}
		return extractMistralVibeAssistantTextFromNDJSON(trimmed)
	}

	if isMistralVibeNDJSONLike(trimmed) {
		return extractMistralVibeAssistantTextFromNDJSON(trimmed)
	}

	return "", false
}

func extractMistralVibeAssistantTextFromJSONDocument(output string) (string, bool) {
	var value any
	if err := json.Unmarshal([]byte(output), &value); err != nil {
		return "", false
	}
	return extractMistralVibeAssistantTextFromValue(value)
}

func extractMistralVibeAssistantTextFromNDJSON(output string) (string, bool) {
	var lastText string
	found := false
	for _, line := range strings.Split(output, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		if !strings.HasPrefix(trimmedLine, "{") {
			continue
		}

		var value any
		if err := json.Unmarshal([]byte(trimmedLine), &value); err != nil {
			return "", false
		}
		if text, ok := extractMistralVibeAssistantTextFromValue(value); ok {
			lastText = text
			found = true
		}
	}
	return lastText, found
}

func extractMistralVibeAssistantTextFromValue(value any) (string, bool) {
	switch typed := value.(type) {
	case []any:
		var lastText string
		found := false
		for _, item := range typed {
			if text, ok := extractMistralVibeAssistantTextFromValue(item); ok {
				lastText = text
				found = true
			}
		}
		return lastText, found
	case map[string]any:
		return extractMistralVibeAssistantTextFromObject(typed)
	default:
		return "", false
	}
}

func extractMistralVibeAssistantTextFromObject(object map[string]any) (string, bool) {
	message := mistralVibeObjectValue(object["message"])
	if !isMistralVibeAssistantObject(object) && !isMistralVibeAssistantObject(message) {
		return "", false
	}

	candidates := []any{
		object["content"],
		object["text"],
		message["content"],
		message["text"],
	}
	if delta, ok := object["delta"].(map[string]any); ok {
		candidates = append(candidates, delta["text"])
	}
	if messageDelta, ok := message["delta"].(map[string]any); ok {
		candidates = append(candidates, messageDelta["text"])
	}

	for _, candidate := range candidates {
		if text := mistralVibeTextFromContent(candidate); text != "" {
			return text, true
		}
	}
	return "", false
}

func mistralVibeObjectValue(value any) map[string]any {
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return object
}

func isMistralVibeAssistantObject(object map[string]any) bool {
	if object == nil {
		return false
	}
	role, ok := object["role"].(string)
	return ok && role == "assistant"
}

func mistralVibeTextFromContent(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		var builder strings.Builder
		for _, item := range typed {
			object, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blockType, hasBlockType := object["type"].(string)
			text, hasText := object["text"].(string)
			if hasBlockType && hasText && blockType == "text" && text != "" {
				builder.WriteString(text)
			}
		}
		return builder.String()
	default:
		return ""
	}
}

func isMistralVibeEnvelopeLike(output string) bool {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var value any
		if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
			return hasMistralVibeEnvelopeMarkers(trimmed)
		}
		return hasMistralVibeEnvelopeValue(value)
	}
	return isMistralVibeNDJSONLike(trimmed)
}

func hasMistralVibeEnvelopeValue(value any) bool {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if hasMistralVibeEnvelopeValue(item) {
				return true
			}
		}
	case map[string]any:
		if _, ok := typed["role"]; ok {
			return true
		}
		if _, ok := typed["message"]; ok {
			return true
		}
		if _, ok := typed["delta"]; ok {
			return true
		}
		if eventType, ok := typed["type"].(string); ok && eventType == "message" {
			return true
		}
	}
	return false
}

func hasMistralVibeEnvelopeMarkers(output string) bool {
	return strings.Contains(output, `"role"`) ||
		strings.Contains(output, `"message"`) ||
		strings.Contains(output, `"delta"`) ||
		strings.Contains(output, `"type"`)
}

func isMistralVibeNDJSONLike(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || !strings.HasPrefix(trimmedLine, "{") {
			continue
		}
		if strings.Contains(trimmedLine, `"role"`) ||
			strings.Contains(trimmedLine, `"message"`) ||
			strings.Contains(trimmedLine, `"content"`) ||
			strings.Contains(trimmedLine, `"delta"`) ||
			strings.Contains(trimmedLine, `"type"`) {
			return true
		}
	}
	return false
}

func (p *MistralVibeProvider) extractSessionID(_ string) (string, error) {
	return "", errors.New("mistral_vibe session ID extraction not supported")
}
