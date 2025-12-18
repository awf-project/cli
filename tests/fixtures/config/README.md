# Config Test Fixtures

Test fixtures for F036 Project Configuration File feature.

## Structure

```
config/
├── valid.yaml              # Valid config with inputs section
├── invalid-syntax.yaml     # Malformed YAML for error tests
├── unknown-keys.yaml       # Valid YAML with unknown keys (warning tests)
└── README.md               # This file
```

## Usage

In tests, copy fixtures to a temp `.awf/` directory:

```go
// Setup temp project directory
tmpDir := t.TempDir()
awfDir := filepath.Join(tmpDir, ".awf")
os.MkdirAll(awfDir, 0755)

// Copy fixture
src, _ := os.ReadFile("tests/fixtures/config/valid.yaml")
os.WriteFile(filepath.Join(awfDir, "config.yaml"), src, 0644)

// Test config loading from tmpDir
```

## Related

- `internal/infrastructure/config/` - ConfigLoader implementation
- `.awf/config.yaml` - Default project config path
