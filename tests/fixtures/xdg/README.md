# XDG Test Fixtures

Test fixtures for F044 XDG Prompt Discovery feature, specifically US4 (`awf init --global`).

## Structure

```
xdg/
├── config/                     # Simulates $XDG_CONFIG_HOME
│   └── awf/
│       └── prompts/           # Pre-existing global prompts
│           ├── global-example.md   # Global-only prompt
│           └── shared.md           # For override tests
├── empty/                     # Simulates empty XDG state
│   └── awf/                   # Directory exists but no prompts
└── README.md                  # This file
```

## Usage

In tests, set `XDG_CONFIG_HOME` to point to the appropriate fixture:

```go
// Test with existing global prompts
os.Setenv("XDG_CONFIG_HOME", "tests/fixtures/xdg/config")

// Test with empty global config (init should create prompts)
os.Setenv("XDG_CONFIG_HOME", "tests/fixtures/xdg/empty")
```

## Related

- `tests/fixtures/prompts/global/` - Used by prompt discovery tests
- `tests/fixtures/prompts/local/` - Used by prompt override tests
