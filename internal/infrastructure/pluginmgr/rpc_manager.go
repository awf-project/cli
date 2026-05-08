package pluginmgr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	goplugin "github.com/hashicorp/go-plugin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	infralogger "github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/pkg/plugin/sdk"
	"github.com/awf-project/cli/pkg/registry"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

// ErrNoPluginsConfigured indicates no plugin loader or directory is configured.
var ErrNoPluginsConfigured = errors.New("rpc_manager: no plugins configured")

// Default plugins directory relative to config.
const DefaultPluginsDir = "plugins"

// RPCManagerError represents an error during plugin lifecycle operations.
type RPCManagerError struct {
	Op      string // operation (load, init, shutdown)
	Plugin  string // plugin name
	Message string // error message
	Cause   error  // underlying error
}

// Error implements the error interface.
func (e *RPCManagerError) Error() string {
	if e.Plugin != "" {
		return fmt.Sprintf("%s [%s]: %s", e.Op, e.Plugin, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// Unwrap returns the underlying error.
func (e *RPCManagerError) Unwrap() error {
	return e.Cause
}

// NewRPCManagerError creates a new RPCManagerError.
func NewRPCManagerError(op, pluginName, message string) *RPCManagerError {
	return &RPCManagerError{
		Op:      op,
		Plugin:  pluginName,
		Message: message,
	}
}

// WrapRPCManagerError wraps an existing error as an RPCManagerError.
func WrapRPCManagerError(op, pluginName string, cause error) *RPCManagerError {
	return &RPCManagerError{
		Op:      op,
		Plugin:  pluginName,
		Message: cause.Error(),
		Cause:   cause,
	}
}

// pluginConnection holds an active go-plugin connection to a running plugin process.
type pluginConnection struct {
	client        *goplugin.Client
	plugin        pluginv1.PluginServiceClient
	operation     pluginv1.OperationServiceClient
	validator     pluginv1.ValidatorServiceClient
	stepType      pluginv1.StepTypeServiceClient
	event         pluginv1.EventServiceClient
	processCancel context.CancelFunc // cancels the long-lived process context on Shutdown
}

// clientPlugin implements goplugin.GRPCPlugin for the host side.
// GRPCClient() is called by go-plugin when the host calls Dispense(); it creates the gRPC stubs.
// GRPCServer() is never called on the host (only on the plugin binary side).
type clientPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
}

// grpcClientBundle holds the gRPC service clients dispensed on the host side.
type grpcClientBundle struct {
	plugin    pluginv1.PluginServiceClient
	operation pluginv1.OperationServiceClient
	validator pluginv1.ValidatorServiceClient
	stepType  pluginv1.StepTypeServiceClient
	event     pluginv1.EventServiceClient
}

// GRPCClient creates gRPC service clients from the connection established by go-plugin.
// Called by go-plugin on the host side when Dispense("awf-plugin") is invoked.
func (p *clientPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, conn *grpc.ClientConn) (interface{}, error) {
	return &grpcClientBundle{
		plugin:    pluginv1.NewPluginServiceClient(conn),
		operation: pluginv1.NewOperationServiceClient(conn),
		validator: pluginv1.NewValidatorServiceClient(conn),
		stepType:  pluginv1.NewStepTypeServiceClient(conn),
		event:     pluginv1.NewEventServiceClient(conn),
	}, nil
}

// GRPCServer is never called on the host side; provided for interface completeness.
func (p *clientPlugin) GRPCServer(_ *goplugin.GRPCBroker, _ *grpc.Server) error {
	return fmt.Errorf("GRPCServer called on host side — this is a bug")
}

var (
	_ goplugin.Plugin     = (*clientPlugin)(nil)
	_ goplugin.GRPCPlugin = (*clientPlugin)(nil)
)

// RPCPluginManager implements PluginManager using HashiCorp go-plugin for RPC.
// It manages plugin lifecycle: discovery, loading, initialization, and shutdown.
type RPCPluginManager struct {
	mu          sync.RWMutex
	plugins     map[string]*pluginmodel.PluginInfo // plugin name -> info
	connections map[string]*pluginConnection       // active connections, protected by mu (NFR-004)
	loader      *FileSystemLoader                  // for plugin discovery
	pluginsDirs []string                           // directories to discover plugins from
	hostVersion string                             // current AWF version for plugin compatibility checks
	eventBus    *EventBus                          // optional; nil means no event wiring
	stateStore  *JSONPluginStateStore              // optional; nil means no checksum verification
	zapLogger   *zap.Logger                        // optional; nil falls back to zap.NewNop()
}

// NewRPCPluginManager creates a new RPCPluginManager.
func NewRPCPluginManager(loader *FileSystemLoader) *RPCPluginManager {
	return &RPCPluginManager{
		plugins:     make(map[string]*pluginmodel.PluginInfo),
		connections: make(map[string]*pluginConnection),
		loader:      loader,
		hostVersion: "999.0.0",
	}
}

// connectWithTimeout establishes a gRPC connection to a plugin process with a 5s hard timeout (NFR-002).
// go-plugin's client.Client() is blocking and not context-aware; this wrapper enforces the deadline.
// Uses goroutine+buffered channel+select pattern for timeout enforcement (consistent with B008).
// ctx cancellation kills the client immediately (used when Init ctx times out).
func (m *RPCPluginManager) connectWithTimeout(ctx context.Context, client *goplugin.Client) (*pluginConnection, error) {
	if client == nil {
		return nil, nil
	}

	// Buffered channel for result (capacity 1 so goroutine can send without blocking)
	resultChan := make(chan interface{}, 1)

	go func() {
		// client.Client() returns the ClientProtocol; Dispense("awf-plugin") then calls GRPCClient()
		// and returns the *grpcClientBundle with the gRPC service clients.
		rpc, err := client.Client()
		if err != nil {
			resultChan <- err
			return
		}
		dispensed, err := rpc.Dispense("awf-plugin")
		if err != nil {
			resultChan <- err
			return
		}
		resultChan <- dispensed
	}()

	select {
	case result := <-resultChan:
		if err, ok := result.(error); ok {
			return nil, err
		}

		// result is *grpcClientBundle returned by clientPlugin.GRPCClient().
		conn := &pluginConnection{
			client: client,
		}
		if bundle, ok := result.(*grpcClientBundle); ok {
			conn.plugin = bundle.plugin
			conn.operation = bundle.operation
			conn.validator = bundle.validator
			conn.stepType = bundle.stepType
			conn.event = bundle.event
		}

		return conn, nil
	case <-ctx.Done():
		client.Kill()
		return nil, fmt.Errorf("plugin connection canceled: %w", ctx.Err())
	case <-time.After(5 * time.Second):
		client.Kill()
		return nil, fmt.Errorf("plugin connection timeout: exceeded 5s deadline")
	}
}

// SetPluginsDir sets the directory to discover plugins from.
// SetPluginsDir configures a single plugin directory (replaces any previous config).
func (m *RPCPluginManager) SetPluginsDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pluginsDirs = []string{dir}
}

// SetPluginsDirs configures multiple plugin directories to scan.
// Plugins are discovered from all directories; first-found wins on name conflicts.
func (m *RPCPluginManager) SetPluginsDirs(dirs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pluginsDirs = dirs
}

// Discover finds plugins in the plugins directory.
// Returns ErrNoPluginsConfigured if no loader or plugins directory is configured.
func (m *RPCPluginManager) Discover(ctx context.Context) ([]*pluginmodel.PluginInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}

	if m.loader == nil {
		return nil, ErrNoPluginsConfigured
	}

	m.mu.RLock()
	dirs := m.pluginsDirs
	m.mu.RUnlock()

	if len(dirs) == 0 {
		return nil, ErrNoPluginsConfigured
	}

	// Discover from all directories; first-found wins on manifest name conflicts.
	var allDiscovered []*pluginmodel.PluginInfo
	for _, dir := range dirs {
		discovered, err := m.loader.DiscoverPlugins(ctx, dir)
		if err != nil {
			continue // skip dirs that fail (e.g. missing directory)
		}
		allDiscovered = append(allDiscovered, discovered...)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	valid := make([]*pluginmodel.PluginInfo, 0, len(allDiscovered))
	for _, info := range allDiscovered {
		if info.Manifest == nil || info.Manifest.Name == "" {
			continue
		}
		if err := m.loader.ValidatePlugin(info); err != nil {
			continue
		}
		// First-found wins: skip if already registered by an earlier directory
		if _, exists := m.plugins[info.Manifest.Name]; exists {
			continue
		}
		m.plugins[info.Manifest.Name] = info
		valid = append(valid, info)
	}

	return valid, nil
}

// Load loads a plugin by name.
// The plugin must have been discovered first, or a pluginsDir must be configured.
func (m *RPCPluginManager) Load(ctx context.Context, name string) error {
	// Check context first
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("load: %w", err)
	}

	// Validate name
	if name == "" {
		return NewRPCManagerError("load", "", "plugin name is required")
	}

	// Check if loader is configured
	if m.loader == nil {
		return ErrNoPluginsConfigured
	}

	// Check if already loaded
	m.mu.RLock()
	existing, found := m.plugins[name]
	dirs := m.pluginsDirs
	m.mu.RUnlock()

	if found {
		// Already loaded - check status
		if existing.Status == pluginmodel.StatusLoaded ||
			existing.Status == pluginmodel.StatusRunning ||
			existing.Status == pluginmodel.StatusInitialized {
			// Already in a valid state, just return success
			return nil
		}
		// Plugin exists but in invalid state - try to reload
	}

	// Need to load from filesystem
	if len(dirs) == 0 {
		// Not fully configured
		return ErrNoPluginsConfigured
	}

	// Try to load the plugin from any configured directory
	pluginPath := ""
	for _, dir := range dirs {
		candidate := dir + "/" + name
		if _, err := os.Stat(candidate); err == nil {
			pluginPath = candidate
			break
		}
	}
	if pluginPath == "" {
		return NewRPCManagerError("load", name, "plugin directory not found in any search path")
	}
	info, err := m.loader.LoadPlugin(ctx, pluginPath)
	if err != nil {
		return WrapRPCManagerError("load", name, err)
	}

	// Validate the plugin
	if err := m.loader.ValidatePlugin(info); err != nil {
		return WrapRPCManagerError("load", name, err)
	}

	// Store the loaded plugin
	m.mu.Lock()
	m.plugins[name] = info
	m.mu.Unlock()

	return nil
}

// Init initializes a loaded plugin with configuration.
func (m *RPCPluginManager) Init(ctx context.Context, name string, config map[string]any) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("init: %w", err)
	}

	if name == "" {
		return NewRPCManagerError("init", "", "plugin name is required")
	}

	if m.loader == nil {
		return ErrNoPluginsConfigured
	}

	binaryPath, info, err := m.resolvePluginBinary(name)
	if err != nil {
		return err
	}

	if binaryPath == "" {
		return nil // Already running
	}

	if compatErr := m.checkVersionCompatibility(name, info); compatErr != nil {
		return compatErr
	}

	checksumBytes, err := m.verifyChecksum(name, binaryPath)
	if err != nil {
		return err
	}

	conn, _, err := m.startPluginProcess(ctx, name, binaryPath, config, checksumBytes)
	if err != nil {
		return err
	}

	// Store connection and update status
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connections[name] = conn
	if pluginInfo, found := m.plugins[name]; found {
		pluginInfo.Status = pluginmodel.StatusRunning
		pluginInfo.Operations = m.queryOperationNames(ctx, name, conn)
		pluginInfo.StepTypes = m.queryStepTypeNames(ctx, name, conn)
		m.wireEventSubscriptions(name, conn, pluginInfo)
	}

	return nil
}

// resolvePluginBinary validates plugin state and resolves the binary path.
func (m *RPCPluginManager) resolvePluginBinary(name string) (string, *pluginmodel.PluginInfo, error) {
	m.mu.Lock()
	info, found := m.plugins[name]
	if !found {
		m.mu.Unlock()
		return "", nil, NewRPCManagerError("init", name, "plugin not loaded")
	}

	if info.Status == pluginmodel.StatusRunning {
		m.mu.Unlock()
		return "", nil, nil // Already initialized — caller checks for empty path
	}

	if info.Status != pluginmodel.StatusLoaded && info.Status != pluginmodel.StatusDiscovered {
		m.mu.Unlock()
		return "", nil, NewRPCManagerError("init", name, fmt.Sprintf("cannot initialize plugin in state %q", info.Status))
	}

	pluginPath := info.Path
	m.mu.Unlock()

	if pluginPath == "" {
		return "", nil, NewRPCManagerError("init", name, "plugin path not set")
	}

	// Convention: binary is awf-plugin-<dirName> inside the plugin directory.
	dirName := filepath.Base(pluginPath)
	binName := "awf-plugin-" + dirName
	if strings.HasPrefix(dirName, "awf-plugin-") {
		binName = dirName
	}
	binaryPath := filepath.Join(pluginPath, binName)

	binaryInfo, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, WrapRPCManagerError("init", name, fmt.Errorf("plugin binary not found at %s", binaryPath))
		}
		return "", nil, WrapRPCManagerError("init", name, err)
	}

	if binaryInfo.IsDir() {
		return "", nil, NewRPCManagerError("init", name, fmt.Sprintf("plugin binary path is a directory, not executable: %s", binaryPath))
	}

	return binaryPath, info, nil
}

// checkVersionCompatibility validates manifest presence and AWF version constraints.
func (m *RPCPluginManager) checkVersionCompatibility(name string, info *pluginmodel.PluginInfo) error {
	m.mu.RLock()
	manifest := info.Manifest
	m.mu.RUnlock()

	if manifest == nil {
		return NewRPCManagerError("init", name, "plugin manifest is nil")
	}

	if manifest.Version == "" {
		return NewRPCManagerError("init", name, "plugin version is empty")
	}

	if manifest.AWFVersion == "" {
		return nil
	}

	compatible, err := registry.CheckVersionConstraint(manifest.AWFVersion, m.hostVersion)
	if err != nil {
		return WrapRPCManagerError("init", name, fmt.Errorf("version compatibility check failed: %w", err))
	}
	if !compatible {
		return NewRPCManagerError("init", name, fmt.Sprintf("plugin requires AWF version %s (host is %s)", manifest.AWFVersion, m.hostVersion))
	}

	return nil
}

// startPluginProcess creates a go-plugin client, establishes gRPC connection,
// verifies the plugin via GetInfo, and calls Init RPC.
// Returns the connection and processCancel (caller must store or invoke on error).
// checksumBytes, when non-nil, enables go-plugin SecureConfig verification (redundant layer after Init checksum check).
func (m *RPCPluginManager) startPluginProcess(ctx context.Context, name, binaryPath string, config map[string]any, checksumBytes []byte) (*pluginConnection, context.CancelFunc, error) {
	processCtx, processCancel := context.WithCancel(context.Background())

	zapLog := m.zapLogger
	if zapLog == nil {
		zapLog = zap.NewNop()
	}

	clientCfg := &goplugin.ClientConfig{
		HandshakeConfig:  sdk.Handshake,
		Plugins:          goplugin.PluginSet{"awf-plugin": &clientPlugin{}},
		Cmd:              exec.CommandContext(processCtx, binaryPath), //nolint:gosec // binaryPath is validated by resolvePluginBinary
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		AutoMTLS:         true,
		Logger:           infralogger.NewHCLogAdapter(zapLog, name),
		SyncStdout:       infralogger.NewLogWriter(zapLog, zapcore.WarnLevel),
		SyncStderr:       infralogger.NewLogWriter(zapLog, zapcore.WarnLevel),
	}
	if len(checksumBytes) > 0 {
		clientCfg.SecureConfig = &goplugin.SecureConfig{
			Checksum: checksumBytes,
			Hash:     sha256.New(),
		}
	}

	client := goplugin.NewClient(clientCfg)

	conn, err := m.connectWithTimeout(ctx, client)
	if err != nil {
		processCancel()
		return nil, nil, WrapRPCManagerError("init", name, fmt.Errorf("failed to establish gRPC connection: %w", err))
	}

	if conn == nil {
		processCancel()
		client.Kill()
		return nil, nil, NewRPCManagerError("init", name, "connection is nil after connectWithTimeout")
	}

	infoResp, err := conn.plugin.GetInfo(ctx, &pluginv1.GetInfoRequest{})
	if err != nil {
		processCancel()
		client.Kill()
		return nil, nil, WrapRPCManagerError("init", name, fmt.Errorf("GetInfo RPC failed: %w", err))
	}

	if infoResp == nil {
		processCancel()
		client.Kill()
		return nil, nil, NewRPCManagerError("init", name, "GetInfo RPC returned nil response")
	}

	if err := m.sendInitRPC(ctx, name, conn, client, processCancel, config); err != nil {
		return nil, nil, err
	}

	conn.processCancel = processCancel

	return conn, processCancel, nil
}

// sendInitRPC encodes config and calls the Init RPC. Cleans up on failure.
func (m *RPCPluginManager) sendInitRPC(ctx context.Context, name string, conn *pluginConnection, client *goplugin.Client, processCancel context.CancelFunc, config map[string]any) error {
	initRequest := &pluginv1.InitRequest{
		Config: make(map[string][]byte),
	}

	for key, val := range config {
		encoded, encErr := json.Marshal(val)
		if encErr != nil {
			processCancel()
			client.Kill()
			return WrapRPCManagerError("init", name, fmt.Errorf("failed to encode config: %w", encErr))
		}
		initRequest.Config[key] = encoded
	}

	_, err := conn.plugin.Init(ctx, initRequest)
	if err != nil {
		processCancel()
		client.Kill()
		return WrapRPCManagerError("init", name, fmt.Errorf("Init RPC failed: %w", err))
	}

	return nil
}

// queryOperationNames lists operation names from a connected plugin via gRPC.
// Returns nil on failure (non-fatal — operations are optional metadata).
func (m *RPCPluginManager) queryOperationNames(ctx context.Context, pluginID string, conn *pluginConnection) []string {
	if conn.operation == nil {
		return nil
	}

	opCtx, opCancel := context.WithTimeout(ctx, 5*time.Second)
	defer opCancel()

	resp, err := conn.operation.ListOperations(opCtx, &pluginv1.ListOperationsRequest{})
	if err != nil || resp == nil {
		return nil
	}

	names := make([]string, 0, len(resp.Operations))
	for _, schema := range resp.Operations {
		if schema != nil && schema.Name != "" {
			names = append(names, pluginID+"."+schema.Name)
		}
	}

	return names
}

// queryStepTypeNames lists step type names from a connected plugin via gRPC.
// Returns nil on failure (non-fatal — step types are optional metadata).
func (m *RPCPluginManager) queryStepTypeNames(ctx context.Context, pluginID string, conn *pluginConnection) []string {
	if conn.stepType == nil {
		return nil
	}

	stCtx, stCancel := context.WithTimeout(ctx, 5*time.Second)
	defer stCancel()

	resp, err := conn.stepType.ListStepTypes(stCtx, &pluginv1.ListStepTypesRequest{})
	if err != nil || resp == nil {
		return nil
	}

	names := make([]string, 0, len(resp.StepTypes))
	for _, st := range resp.StepTypes {
		if st != nil && st.Name != "" {
			names = append(names, pluginID+"."+st.Name)
		}
	}

	return names
}

// Shutdown stops a running plugin gracefully.
// Full implementation: gRPC Shutdown call, client.Kill(), connection cleanup from m.connections.
func (m *RPCPluginManager) Shutdown(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	if name == "" {
		return NewRPCManagerError("shutdown", "", "plugin name is required")
	}

	if m.loader == nil {
		return ErrNoPluginsConfigured
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	info, found := m.plugins[name]
	if !found {
		return nil
	}

	if info.Status == pluginmodel.StatusStopped || info.Status == pluginmodel.StatusDisabled {
		return nil
	}

	conn := m.connections[name]
	if conn != nil {
		if conn.plugin != nil {
			conn.plugin.Shutdown(ctx, &pluginv1.ShutdownRequest{}) //nolint:gosec,errcheck // Best effort shutdown, don't fail if RPC fails
		}
		if conn.client != nil {
			conn.client.Kill()
		}
		if conn.processCancel != nil {
			conn.processCancel()
		}
		delete(m.connections, name)
	}

	info.Status = pluginmodel.StatusStopped

	return nil
}

// ShutdownAll stops all running plugins with a 5s per-plugin deadline.
// Errors are accumulated via errors.Join() so all plugins are attempted even on partial failure.
func (m *RPCPluginManager) ShutdownAll(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("shutdown all: %w", err)
	}

	if m.loader == nil {
		return ErrNoPluginsConfigured
	}

	m.mu.Lock()
	names := make([]string, 0, len(m.plugins))
	for name, info := range m.plugins {
		if info.Status == pluginmodel.StatusRunning || info.Status == pluginmodel.StatusInitialized {
			names = append(names, name)
		}
	}
	m.mu.Unlock()

	var errs []error
	for _, name := range names {
		pluginCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := m.Shutdown(pluginCtx, name)
		cancel()
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// Get returns plugin info by name.
// Returns (nil, false) if plugin not found.
func (m *RPCPluginManager) Get(name string) (*pluginmodel.PluginInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.plugins[name]
	return info, ok
}

// List returns all known plugins.
func (m *RPCPluginManager) List() []*pluginmodel.PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*pluginmodel.PluginInfo, 0, len(m.plugins))
	for _, info := range m.plugins {
		result = append(result, info)
	}
	return result
}

// connectionsSnapshot returns a shallow copy of m.connections under RLock.
// Callers must not hold the lock when making gRPC calls (which can block for seconds).
func (m *RPCPluginManager) connectionsSnapshot() map[string]*pluginConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conns := make(map[string]*pluginConnection, len(m.connections))
	for k, v := range m.connections {
		conns[k] = v
	}
	return conns
}

// GetOperation returns an operation schema by name, searching all connected plugins.
// Name format is "pluginName.operationName" (consistent with built-in providers).
// Uses an internal 5s timeout per call because the port interface does not accept ctx.
func (m *RPCPluginManager) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	pluginID, opName := splitOperationName(name)
	conns := m.connectionsSnapshot()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// If prefixed, route directly to the target plugin.
	if pluginID != "" {
		conn, found := conns[pluginID]
		if !found || conn.operation == nil {
			return nil, false
		}
		resp, err := conn.operation.GetOperation(ctx, &pluginv1.GetOperationRequest{Name: opName})
		if err != nil || resp == nil || resp.Operation == nil {
			return nil, false
		}
		return convertOperationSchema(pluginID, resp.Operation), true
	}

	// Unprefixed: search all plugins (fallback).
	for pid, conn := range conns {
		if conn.operation == nil {
			continue
		}
		resp, err := conn.operation.GetOperation(ctx, &pluginv1.GetOperationRequest{Name: name})
		if err != nil || resp == nil || resp.Operation == nil {
			continue
		}
		return convertOperationSchema(pid, resp.Operation), true
	}
	return nil, false
}

// ListOperations returns all operation schemas from all connected plugins.
// Calls gRPC ListOperations on each connection; skips plugins that fail.
// Uses an internal 5s timeout per call because the port interface does not accept ctx.
func (m *RPCPluginManager) ListOperations() []*pluginmodel.OperationSchema {
	conns := m.connectionsSnapshot()

	result := make([]*pluginmodel.OperationSchema, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for pluginID, conn := range conns {
		if conn.operation == nil {
			continue
		}
		resp, err := conn.operation.ListOperations(ctx, &pluginv1.ListOperationsRequest{})
		if err != nil || resp == nil {
			continue
		}
		for _, schema := range resp.Operations {
			if schema != nil {
				result = append(result, convertOperationSchema(pluginID, schema))
			}
		}
	}
	return result
}

// Execute delegates an operation call to the correct connected plugin via gRPC.
// Name format is "pluginName.operationName" (consistent with built-in providers).
// If unprefixed, iterates all connections as fallback.
func (m *RPCPluginManager) Execute(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	conns := m.connectionsSnapshot()

	if len(conns) == 0 {
		return nil, NewRPCManagerError("execute", name, "operation not found: no plugins connected")
	}

	pluginID, opName := splitOperationName(name)

	encInputs := make(map[string][]byte, len(inputs))
	for key, val := range inputs {
		data, err := json.Marshal(val)
		if err != nil {
			return nil, WrapRPCManagerError("execute", name, fmt.Errorf("failed to encode input %q: %w", key, err))
		}
		encInputs[key] = data
	}

	// If prefixed, route directly to the target plugin.
	if pluginID != "" {
		conn, found := conns[pluginID]
		if !found || conn.operation == nil {
			return nil, NewRPCManagerError("execute", name, "plugin not connected")
		}
		req := &pluginv1.ExecuteRequest{Operation: opName, Inputs: encInputs}
		resp, err := conn.operation.Execute(ctx, req)
		if err != nil {
			return nil, WrapRPCManagerError("execute", name, err)
		}
		return convertExecuteResponse(pluginID, resp), nil
	}

	// Unprefixed: search all plugins (fallback).
	req := &pluginv1.ExecuteRequest{Operation: name, Inputs: encInputs}
	var lastErr error
	for pid, conn := range conns {
		if conn.operation == nil {
			continue
		}
		resp, err := conn.operation.Execute(ctx, req)
		if err != nil {
			lastErr = WrapRPCManagerError("execute", name, err)
			continue
		}
		return convertExecuteResponse(pid, resp), nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, NewRPCManagerError("execute", name, "operation not found")
}

// validatorClients returns validator adapters for all connected plugins that declare the validators capability.
// Intended for use by WorkflowService to run plugin-provided validation rules.
func (m *RPCPluginManager) validatorClients(timeout time.Duration) []*grpcValidatorAdapter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	adapters := make([]*grpcValidatorAdapter, 0)

	for pluginName, conn := range m.connections {
		if conn.validator == nil {
			continue
		}

		info, found := m.plugins[pluginName]
		if !found || info.Manifest == nil {
			continue
		}

		// Check if plugin has validators capability
		hasCapability := false
		for _, cap := range info.Manifest.Capabilities {
			if cap == pluginmodel.CapabilityValidators {
				hasCapability = true
				break
			}
		}

		if !hasCapability {
			continue
		}

		adapter := newGRPCValidatorAdapter(conn.validator, pluginName, timeout)
		adapters = append(adapters, adapter)
	}

	return adapters
}

// stepTypeClient returns step type adapters for all connected plugins that declare the step_types capability.
// Intended for use by ExecutionService to dispatch unknown step type executions to plugins.
func (m *RPCPluginManager) stepTypeClient(logger ports.Logger) []*grpcStepTypeAdapter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	adapters := make([]*grpcStepTypeAdapter, 0)

	for pluginName, conn := range m.connections {
		if conn.stepType == nil {
			continue
		}

		info, found := m.plugins[pluginName]
		if !found || info.Manifest == nil {
			continue
		}

		// Check if plugin has step_types capability
		hasCapability := false
		for _, cap := range info.Manifest.Capabilities {
			if cap == pluginmodel.CapabilityStepTypes {
				hasCapability = true
				break
			}
		}

		if !hasCapability {
			continue
		}

		adapter := newGRPCStepTypeAdapter(conn.stepType, pluginName, 0, logger)
		adapters = append(adapters, adapter)
	}

	return adapters
}

// ValidatorProvider returns a WorkflowValidatorProvider wrapping all validator-capable plugins.
// Returns nil when no plugins have declared the validators capability.
func (m *RPCPluginManager) ValidatorProvider(timeout time.Duration) ports.WorkflowValidatorProvider {
	adapters := m.validatorClients(timeout)
	if len(adapters) == 0 {
		return nil
	}
	return &compositeValidatorProvider{adapters: adapters}
}

// StepTypeProvider returns a StepTypeProvider wrapping all step-type-capable plugins.
// Returns nil when no plugins have declared the step_types capability.
func (m *RPCPluginManager) StepTypeProvider(logger ports.Logger) ports.StepTypeProvider {
	adapters := m.stepTypeClient(logger)
	if len(adapters) == 0 {
		return nil
	}
	return &compositeStepTypeProvider{adapters: adapters}
}

// SetHostVersion overrides the default host version used for plugin compatibility checks.
func (m *RPCPluginManager) SetHostVersion(v string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hostVersion = v
}

// SetEventBus injects an EventBus for event subscription wiring after plugin Init.
// Must be called before any Init() calls to enable event routing.
func (m *RPCPluginManager) SetEventBus(bus *EventBus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventBus = bus
}

// wireEventSubscriptions registers the plugin's event subscriptions if the manifest declares the events capability.
func (m *RPCPluginManager) wireEventSubscriptions(pluginName string, conn *pluginConnection, info *pluginmodel.PluginInfo) {
	if m.eventBus == nil || conn.event == nil || info.Manifest == nil {
		return
	}

	if !slices.Contains(info.Manifest.Capabilities, pluginmodel.CapabilityEvents) {
		return
	}

	adapter := newGRPCEventAdapter(conn.event, pluginName)
	m.eventBus.Subscribe(pluginName, info.Manifest.Events.Subscribe, adapter)
}

// SetStateStore injects a JSONPluginStateStore for launch-time checksum verification.
// Must be called before any Init() calls to enable checksum enforcement.
func (m *RPCPluginManager) SetStateStore(store *JSONPluginStateStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateStore = store
}

// SetZapLogger injects a zap.Logger used for the hclog adapter and stdout/stderr capture.
// When not set, startPluginProcess falls back to zap.NewNop().
func (m *RPCPluginManager) SetZapLogger(zapLogger *zap.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.zapLogger = zapLogger
}

// verifyChecksum reads the plugin binary, computes SHA-256, and compares with the stored hash.
// Returns (nil, nil) when no state store is configured or no checksum is stored for the plugin.
// Returns (checksumBytes, nil) on a hash match; returns (nil, error) on mismatch.
func (m *RPCPluginManager) verifyChecksum(pluginName, binaryPath string) ([]byte, error) {
	if m.stateStore == nil {
		return nil, nil
	}

	hexHash, _, exists := m.stateStore.GetChecksum(pluginName)
	if !exists {
		return nil, nil
	}

	realPath, err := filepath.EvalSymlinks(binaryPath)
	if err != nil {
		realPath = binaryPath
	}

	data, err := os.ReadFile(realPath) //nolint:gosec // path validated by resolvePluginBinary
	if err != nil {
		return nil, WrapRPCManagerError("init", pluginName, fmt.Errorf("failed to read plugin binary for checksum: %w", err))
	}

	actualSum := sha256.Sum256(data)
	actualHex := hex.EncodeToString(actualSum[:])

	if actualHex != hexHash {
		return nil, domainerrors.NewStructuredError(
			domainerrors.ErrorCodeExecutionPluginChecksumMismatch,
			fmt.Sprintf("CHECKSUM_MISMATCH: plugin %q binary hash mismatch (expected %s, got %s)", pluginName, hexHash, actualHex),
			map[string]any{
				"plugin":   pluginName,
				"expected": hexHash,
				"actual":   actualHex,
			},
			nil,
		)
	}

	decoded, err := hex.DecodeString(hexHash)
	if err != nil {
		return nil, WrapRPCManagerError("init", pluginName, fmt.Errorf("invalid stored checksum hex: %w", err))
	}

	return decoded, nil
}

// splitOperationName splits "pluginName.opName" into (pluginName, opName).
// Returns ("", name) if no prefix is found.
func splitOperationName(name string) (pluginName, opName string) {
	idx := strings.IndexByte(name, '.')
	if idx < 0 {
		return "", name
	}
	return name[:idx], name[idx+1:]
}

// compile-time checks that RPCPluginManager implements PluginManager and OperationProvider
var (
	_ ports.PluginManager     = (*RPCPluginManager)(nil)
	_ ports.OperationProvider = (*RPCPluginManager)(nil)
)
