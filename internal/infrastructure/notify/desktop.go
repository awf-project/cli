package notify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
)

//nolint:unused // Used in integration tests and will be registered in provider.Execute() during GREEN phase
var desktopBackendCounter uint64

// desktopBackend sends OS-native desktop notifications.
// It uses platform-specific commands: notify-send (Linux) or osascript (macOS).
// This backend is intended for local development environments and will fail
// gracefully on headless servers or unsupported platforms.
//
//nolint:unused // Implements Backend interface, registered in provider.Execute() during GREEN phase
type desktopBackend struct {
	// id uniquely identifies this backend instance for testing purposes.
	// Without this field, Go would optimize empty structs to share the same memory location.
	id uint64
}

// newDesktopBackend creates a new desktop notification backend.
// No configuration is required for desktop notifications.
//
//nolint:unused // Called in integration tests and will be called in provider.Execute() during GREEN phase
func newDesktopBackend() *desktopBackend {
	return &desktopBackend{
		id: atomic.AddUint64(&desktopBackendCounter, 1),
	}
}

// NewDesktopBackend creates a new desktop notification backend (exported).
// No configuration is required for desktop notifications.
// This is the public API used for CLI wiring in run.go.
func NewDesktopBackend() Backend {
	return newDesktopBackend()
}

// Send delivers a desktop notification using platform-specific commands.
// On Linux, it uses notify-send (libnotify). On macOS, it uses osascript.
// Returns BackendResult with the command output or error on failure.
//
// In test mode (AWF_TEST_MODE=1), returns success without executing commands.
// This allows testing registration logic without requiring display servers.
//
//nolint:unused // Backend.Send interface method, tested in desktop_test.go
func (d *desktopBackend) Send(ctx context.Context, payload NotificationPayload) (*BackendResult, error) {
	// Check context before starting
	if err := ctx.Err(); err != nil {
		return &BackendResult{
			Backend:    "desktop",
			StatusCode: 1,
			Response:   "",
		}, fmt.Errorf("context error before sending notification: %w", err)
	}

	// Test mode: succeed without executing commands
	// This allows testing backend registration without display server dependencies
	if os.Getenv("AWF_TEST_MODE") == "1" {
		return &BackendResult{
			Backend:    "desktop",
			StatusCode: 0,
			Response:   "test mode: notification not sent",
		}, nil
	}

	// Default title if empty
	title := payload.Title
	if title == "" {
		title = "AWF Workflow"
	}

	// Detect platform and build command
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = d.buildLinuxCommand(ctx, title, payload.Message, payload.Priority)
	case "darwin":
		cmd = d.buildDarwinCommand(ctx, title, payload.Message)
	default:
		return &BackendResult{
			Backend:    "desktop",
			StatusCode: 1,
			Response:   "",
		}, fmt.Errorf("unsupported platform: %s (desktop notifications require linux or darwin)", runtime.GOOS)
	}

	// Execute command
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	// Build response with platform info
	platform := runtime.GOOS
	response := outputStr
	if response == "" {
		// Include platform info when command has no output
		response = fmt.Sprintf("notification sent via %s", platform)
	}

	if err != nil {
		// Check if context was cancelled during execution
		if ctxErr := ctx.Err(); ctxErr != nil {
			return &BackendResult{
				Backend:    "desktop",
				StatusCode: 1,
				Response:   outputStr,
			}, fmt.Errorf("context error during notification: %w", ctxErr)
		}

		// Command failed
		return &BackendResult{
			Backend:    "desktop",
			StatusCode: 1,
			Response:   outputStr,
		}, fmt.Errorf("desktop notification failed: %w (output: %s)", err, outputStr)
	}

	return &BackendResult{
		Backend:    "desktop",
		StatusCode: 0,
		Response:   response,
	}, nil
}

// buildLinuxCommand creates a notify-send command for Linux.
//
//nolint:unused // Private helper called by Send()
func (d *desktopBackend) buildLinuxCommand(ctx context.Context, title, message, priority string) *exec.Cmd {
	// Check if notify-send is available
	path, err := exec.LookPath("notify-send")
	if err != nil {
		// Return command that will fail with clear error
		return exec.CommandContext(ctx, "notify-send")
	}

	args := []string{}

	// Map priority to notify-send urgency levels
	switch priority {
	case "low":
		args = append(args, "-u", "low")
	case "high":
		args = append(args, "-u", "critical")
	default:
		args = append(args, "-u", "normal")
	}

	// Add title and message (notify-send handles escaping internally)
	args = append(args, title, message)

	return exec.CommandContext(ctx, path, args...)
}

// buildDarwinCommand creates an osascript command for macOS.
//
//nolint:unused // Private helper called by Send()
func (d *desktopBackend) buildDarwinCommand(ctx context.Context, title, message string) *exec.Cmd {
	// Check if osascript is available
	path, err := exec.LookPath("osascript")
	if err != nil {
		// Return command that will fail with clear error
		return exec.CommandContext(ctx, "osascript")
	}

	// Build AppleScript command
	// Escape quotes in title and message for AppleScript string literals
	escapedTitle := escapeAppleScript(title)
	escapedMessage := escapeAppleScript(message)

	// nolint:gocritic // AppleScript requires double-quoted strings, not Go-quoted
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, escapedMessage, escapedTitle)

	return exec.CommandContext(ctx, path, "-e", script)
}

// escapeAppleScript escapes special characters for AppleScript string literals.
//
//nolint:unused // Private helper called by buildDarwinCommand()
func escapeAppleScript(s string) string {
	// Replace backslashes first to avoid double-escaping
	s = strings.ReplaceAll(s, `\`, `\\`)
	// Escape double quotes
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
