// Package logger provides infrastructure adapters for structured logging.
//
// This package implements the Logger port from the domain layer,
// providing multiple logging backends with secret masking:
//   - ConsoleLogger: Human-readable colored output for CLI environments
//   - JSONLogger: Structured JSON logging for production and log aggregation
//   - MultiLogger: Broadcast logs to multiple loggers simultaneously
//
// # Architecture
//
//   - Domain defines: Logger port interface with Debug/Info/Warn/Error/WithContext methods
//   - Infrastructure provides: Three concrete logger implementations with secret masking
//   - Application injects: Logger via dependency injection
//
// # Example Usage
//
// ConsoleLogger:
//
//	logger := logger.NewConsoleLogger(os.Stdout, "INFO")
//	logger.Info("workflow started", "id", "wf-123", "name", "deploy")
//	// Output: [INFO] workflow started id=wf-123 name=deploy
//
// JSONLogger:
//
//	logger := logger.NewJSONLogger(os.Stdout, "DEBUG")
//	logger.Error("command failed", "exit_code", 1, "stderr", "not found")
//	// Output: {"level":"ERROR","msg":"command failed","exit_code":1,"stderr":"not found"}
//
// MultiLogger:
//
//	console := logger.NewConsoleLogger(os.Stdout, "INFO")
//	jsonFile := logger.NewJSONLogger(file, "DEBUG")
//	multi := logger.NewMultiLogger(console, jsonFile)
//	multi.Info("event", "key", "value") // Logged to both outputs
//
// # Secret Masking
//
//   - Automatically masks values for keys matching: SECRET_*, API_KEY*, PASSWORD*, *_TOKEN
//   - Masked values appear as "***MASKED***" in logs
//   - Prevents accidental credential leaks in log files and console output
//   - Applied by all logger implementations (ConsoleLogger, JSONLogger, MultiLogger)
//
// Component: C056 Infrastructure Package Documentation
// Layer: Infrastructure
package logger
