# Project Brief - AI Workflow CLI

## 📋 Vue d'Ensemble

### Nom du Projet

**ai-workflow-cli** (ou `awf` pour le binaire)

### Description

Un outil CLI en Go permettant d'orchestrer des agents IA (Claude, Gemini, Codex, etc.) à travers des workflows configurables en YAML. L'outil implémente une architecture hexagonale/clean pour permettre l'ajout futur d'interfaces supplémentaires (API REST, Message Queue) tout en maintenant le même cœur métier.

### Objectifs Principaux

1. Exécuter des workflows complexes avec state machine

2. Invoquer des agents IA via leurs binaires CLI locaux

3. Gérer la persistance d'état pour reprendre les workflows interrompus

4. Supporter l'exécution parallèle d'étapes

5. Fournir des hooks pre/post pour chaque étape et au niveau workflow

6. Logger les exécutions dans un format structuré (JSON)

7. Valider les inputs et gérer les erreurs avec retry/fallback

---

## 🏗️ Architecture

### Paradigme Architectural

**Hexagonal Architecture / Clean Architecture / DDD**

```
┌─────────────────────────────────────────────────────────────┐
│                     INTERFACES LAYER                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │   CLI    │  │   API    │  │    MQ    │  │  WebUI   │   │
│  │ (actuel) │  │ (futur)  │  │ (futur)  │  │ (futur)  │   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘   │
└───────┼─────────────┼─────────────┼─────────────┼──────────┘
        │             │             │             │
┌───────┴─────────────┴─────────────┴─────────────┴──────────┐
│                   APPLICATION LAYER                          │
│  ┌────────────────────────────────────────────────────┐     │
│  │  WorkflowService, ExecutionService, StateManager   │     │
│  └────────────────────────────────────────────────────┘     │
└──────────────────────────┬───────────────────────────────────┘
                           │
┌──────────────────────────┴───────────────────────────────────┐
│                      DOMAIN LAYER                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Workflow   │  │ StateMachine │  │   Context    │       │
│  │   Entities   │  │    Logic     │  │   & State    │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
│                                                               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              PORTS (Interfaces)                      │    │
│  │  Repository | StateStore | Executor | Logger        │    │
│  └─────────────────────────────────────────────────────┘    │
└──────────────────────────┬───────────────────────────────────┘
                           │
┌──────────────────────────┴───────────────────────────────────┐
│                  INFRASTRUCTURE LAYER                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │     YAML     │  │     JSON     │  │    Shell     │       │
│  │  Repository  │  │  StateStore  │  │   Executor   │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
│  ┌──────────────┐  ┌──────────────┐                         │
│  │    SQLite    │  │     JSON     │                         │
│  │   History    │  │    Logger    │                         │
│  └──────────────┘  └──────────────┘                         │
└───────────────────────────────────────────────────────────────┘
```

### Principes de Conception

* **Dependency Inversion**: Le domain ne dépend de rien, tout dépend du domain

* **Separation of Concerns**: Chaque layer a une responsabilité claire

* **Testabilité**: Interfaces permettent le mocking facile

* **Extensibilité**: Ajout de nouveaux adapters sans modifier le core

---

## 🚨 Error Taxonomy

AWF distinguishes three error types with different handling and exit codes:

### Error Types

| Type | Exit Code | Description | Example |
|------|-----------|-------------|---------|
| `user` | 1 | User input error, bad config | Missing required input, invalid file path |
| `workflow` | 2 | Workflow definition error | Invalid state reference, cycle detected |
| `execution` | 3 | Command execution failed | Non-zero exit code, timeout |
| `system` | 4 | System/infrastructure error | IO error, permission denied |

### Error Structure

```go
// internal/errors/errors.go
type ErrorType int

const (
    ErrorTypeUser ErrorType = iota + 1  // Exit code 1
    ErrorTypeWorkflow                    // Exit code 2
    ErrorTypeExecution                   // Exit code 3
    ErrorTypeSystem                      // Exit code 4
)

type Error struct {
    Type    ErrorType
    Message string
    Cause   error
    Step    string           // Which step failed
    Context map[string]any   // Additional context
}

func (e *Error) Error() string {
    return fmt.Sprintf("[%s] %s: %s", e.Type, e.Step, e.Message)
}

func (e *Error) ExitCode() int {
    return int(e.Type)
}
```

### Error Handling in Workflows

```yaml
# Errors are available via {{error.*}} variables
hooks:
  workflow_error:
    - log: "Error type: {{error.type}}"      # user | workflow | execution | system
    - log: "Message: {{error.message}}"
    - log: "Step: {{error.step}}"
    - log: "Exit code: {{error.exit_code}}"
```

### Cancellation vs Error

| Signal | Hook | Exit Code | Description |
|--------|------|-----------|-------------|
| Error | `workflow_error` | 1-4 | Command failed, validation error |
| SIGINT (Ctrl-C) | `workflow_cancel` | 130 | User cancelled |
| SIGTERM | `workflow_cancel` | 143 | Process terminated |

---

## 🛑 Cancellation & Signal Handling

### Signal Handling Implementation

```go
// cmd/awf/main.go
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Handle signals
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

    go func() {
        sig := <-sigCh
        log.Info("Received signal", "signal", sig)
        cancel() // Propagate cancellation to all goroutines
    }()

    if err := app.Run(ctx); err != nil {
        os.Exit(exitCode(err))
    }
}
```

### Context Propagation

All operations must accept and respect `context.Context`:

```go
// internal/runner/runner.go
func (r *Runner) executeStep(ctx context.Context, step Step) error {
    // Create command with context (auto-killed on cancel)
    cmd := exec.CommandContext(ctx, "sh", "-c", step.Command)

    // Set process group for clean termination
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    if err := cmd.Start(); err != nil {
        return err
    }

    // Wait with cancellation
    done := make(chan error, 1)
    go func() { done <- cmd.Wait() }()

    select {
    case <-ctx.Done():
        // Kill entire process group
        syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
        return ctx.Err()
    case err := <-done:
        return err
    }
}
```

### Cleanup Behavior

| Event | State Saved? | Cleanup Hook? | Partial Results? |
|-------|--------------|---------------|------------------|
| Normal completion | Yes | `workflow_end` | N/A |
| Step failure | Yes | `workflow_error` | Preserved |
| Ctrl-C (graceful) | Yes | `workflow_cancel` | Preserved |
| SIGKILL (forced) | Maybe | None | Maybe lost |

### Graceful Shutdown Timeout

```yaml
# settings.yaml
shutdown:
  graceful_timeout: 30s    # Time to wait for cleanup before force kill
  save_state_on_cancel: true
```

---

## 📁 Structure du Projet

```
ai-workflow-cli/
├── cmd/
│   └── awf/
│       └── main.go                          # Entry point du CLI
│
├── internal/
│   ├── domain/                              # Couche Domain (Business Logic)
│   │   ├── workflow/
│   │   │   ├── workflow.go                 # Entité Workflow
│   │   │   ├── step.go                     # Entité Step
│   │   │   ├── state.go                    # State Machine
│   │   │   ├── context.go                  # Execution Context
│   │   │   └── hooks.go                    # Hooks (pre/post)
│   │   │
│   │   ├── operation/
│   │   │   ├── operation.go                # Interface Operation
│   │   │   └── result.go                   # Operation Result
│   │   │
│   │   └── ports/                          # Interfaces (Ports)
│   │       ├── repository.go               # WorkflowRepository interface
│   │       ├── store.go                    # StateStore interface
│   │       ├── executor.go                 # CommandExecutor interface
│   │       └── logger.go                   # Logger interface
│   │
│   ├── application/                         # Couche Application (Use Cases)
│   │   ├── service.go                      # WorkflowService (orchestration)
│   │   ├── executor.go                     # ExecutionService
│   │   ├── state_manager.go                # StateManager
│   │   └── validator.go                    # Input Validator
│   │
│   ├── infrastructure/                      # Couche Infrastructure (Adapters)
│   │   ├── repository/
│   │   │   └── yaml_repository.go          # Lecture workflows YAML
│   │   │
│   │   ├── store/
│   │   │   ├── json_store.go               # Persistance état JSON
│   │   │   └── sqlite_store.go             # Historique SQLite
│   │   │
│   │   ├── executor/
│   │   │   └── shell_executor.go           # Exécution commandes shell
│   │   │
│   │   └── logger/
│   │       ├── json_logger.go              # Logger JSON structuré
│   │       └── console_logger.go           # Logger console (dev)
│   │
│   └── interfaces/                          # Couche Interfaces (UI)
│       └── cli/
│           ├── app.go                      # Application CLI setup
│           ├── commands/
│           │   ├── run.go                  # Commande: run
│           │   ├── resume.go               # Commande: resume
│           │   ├── list.go                 # Commande: list
│           │   ├── status.go               # Commande: status
│           │   ├── history.go              # Commande: history
│           │   └── validate.go             # Commande: validate
│           │
│           └── ui/
│               ├── progress.go             # Barre de progression
│               ├── formatter.go            # Formatage output
│               └── colors.go               # Couleurs terminal
│
├── pkg/                                     # Packages publics réutilisables
│   ├── interpolation/
│   │   ├── resolver.go                     # Résolution variables ${...}
│   │   └── resolver_test.go
│   │
│   ├── validation/
│   │   ├── validator.go                    # Validation inputs
│   │   └── validator_test.go
│   │
│   └── retry/
│       ├── retry.go                        # Logique retry avec backoff
│       └── retry_test.go
│
├── configs/                                 # Configuration
│   ├── workflows/                          # Workflows YAML
│   │   ├── examples/
│   │   │   ├── analyze-code.yaml
│   │   │   ├── deploy-app.yaml
│   │   │   └── data-pipeline.yaml
│   │   └── README.md
│   │
│   └── settings.yaml                       # Configuration globale
│
├── storage/                                 # Données runtime
│   ├── states/                             # États des workflows
│   ├── logs/                               # Logs JSON
│   └── history.db                          # Base SQLite historique
│
├── scripts/                                 # Scripts utilitaires
│   ├── install.sh                          # Installation
│   └── setup-dev.sh                        # Setup environnement dev
│
├── docs/                                    # Documentation
│   ├── architecture.md
│   ├── workflow-syntax.md
│   ├── examples.md
│   └── api-future.md
│
├── tests/                                   # Tests d'intégration
│   ├── integration/
│   │   └── workflow_test.go
│   └── fixtures/
│       └── test-workflows/
│
├── .gitignore
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── LICENSE
```

---

## 🔧 Technologies & Dépendances

### Langage

* **Go 1.21+**

### Dépendances Principales

```go
// go.mod
module github.com/username/ai-workflow-cli

go 1.21

require (
    // CLI Framework
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.2    // Note: Consider direct yaml.v3 for simpler config

    // YAML parsing
    gopkg.in/yaml.v3 v3.0.1

    // Logging
    go.uber.org/zap v1.26.0

    // Database (Note: Requires CGO, consider BoltDB for pure Go)
    github.com/mattn/go-sqlite3 v1.14.18

    // Terminal UI
    github.com/fatih/color v1.16.0
    github.com/schollz/progressbar/v3 v3.14.1

    // Concurrency
    golang.org/x/sync v0.6.0          // errgroup for parallel execution

    // UUID generation
    github.com/google/uuid v1.5.0

    // Testing
    github.com/stretchr/testify v1.8.4
)
```

**Note on Dependencies:**

| Dependency | Status | Alternative |
|------------|--------|-------------|
| `viper` | Keep for now | Direct yaml.v3 parsing if config stays simple |
| `sqlite3` | Keep for now | BoltDB/BadgerDB (pure Go, no CGO) |
| `validator/v10` | Removed | Custom validator using YAML rules |
| `errgroup` | Added | Required for parallel execution |

### Outils de Développement

* **golangci-lint**: Linting

* **go test**: Tests unitaires et d'intégration

* **go mod**: Gestion des dépendances

* **make**: Automatisation des tâches

---

## 📝 Format des Workflows (YAML)

### Structure Complète

```yaml
# Métadonnées
name: workflow-name
description: "Description du workflow"
version: "1.0.0"
author: "username"
tags: ["ai", "analysis"]

# Variables d'entrée
inputs:
  - name: file_path
    type: string
    description: "Path to the file to analyze"
    required: true
    validation:
      pattern: "^.*\\.(py|js|go)$"

  - name: output_format
    type: string
    description: "Output format"
    required: false
    default: "markdown"
    validation:
      enum: ["markdown", "html", "json"]

  - name: max_tokens
    type: integer
    required: false
    default: 2000
    validation:
      min: 100
      max: 10000

# Variables d'environnement requises
env:
  - CLAUDE_API_KEY
  - OPENAI_API_KEY

# State Machine
states:
  # État initial
  initial: validate

  # État de validation
  validate:
    type: step
    description: "Validate input file exists"
    operation: command
    command: |
      test -f "{{inputs.file_path}}" && echo "valid"
    timeout: 5s
    on_success: extract
    on_failure: error
    hooks:
      pre:
        - log: "Validating file: {{inputs.file_path}}"
      post:
        - log: "File validated"

  # État d'extraction
  extract:
    type: step
    description: "Extract file content"
    operation: command
    command: |
      cat "{{inputs.file_path}}"
    capture:
      stdout: file_content
      stderr: extract_errors
      max_size: 10MB
      encoding: utf-8
    timeout: 10s
    on_success: analyze
    on_failure: error
    hooks:
      post:
        - log: "Extracted {{metadata.file_size}} bytes"
        - command: |
            echo "File: {{inputs.file_path}}" >> workflow.log

  # État d'analyse (avec retry)
  analyze:
    type: step
    description: "Analyze code with Claude"
    operation: command
    command: |
      claude -c "Analyze this code and provide recommendations:

      {{states.extract.output}}

      Max tokens: {{inputs.max_tokens}}"
    capture:
      stdout: analysis
      max_size: 5MB
    timeout: 120s
    retry:
      max_attempts: 3
      initial_delay: 1s
      max_delay: 30s
      backoff: exponential
      multiplier: 2
      jitter: 0.1
      retryable_exit_codes: [1, 2, 130]
    on_success: format_output
    on_failure: try_gemini
    hooks:
      pre:
        - log: "Analyzing with Claude..."
      post:
        - log: "Analysis completed"
        - command: |
            echo "{{states.analyze.output}}" > .cache/analysis.txt

  # État de fallback
  try_gemini:
    type: step
    description: "Fallback to Gemini if Claude fails"
    operation: command
    command: |
      gemini -p "Analyze this code: {{states.extract.output}}"
    capture:
      stdout: analysis
    timeout: 120s
    on_success: format_output
    on_failure: error
    hooks:
      pre:
        - log: "Claude failed, trying Gemini..."

  # État de formatage
  format_output:
    type: step
    description: "Format output"
    operation: command
    command: |
      echo "{{states.analyze.output}}" | \
      pandoc -f markdown -t {{inputs.output_format}}
    capture:
      stdout: formatted_report
    timeout: 30s
    on_success: parallel_save
    on_failure: error

  # État parallèle
  parallel_save:
    type: parallel
    description: "Save outputs in parallel"
    strategy: all_succeed    # all_succeed | any_succeed | best_effort
    max_concurrent: 3        # Limit concurrent goroutines
    parallel:
      - save_report
      - save_metadata
      - notify_user
    on_success: final
    on_failure: error

  # Sous-états parallèles
  save_report:
    type: step
    operation: command
    command: |
      echo "{{states.format_output.output}}" > \
      "report.{{inputs.output_format}}"
    timeout: 10s
    hooks:
      post:
        - log: "Report saved"

  save_metadata:
    type: step
    operation: command
    command: |
      cat > metadata.json <<EOF
      {
        "workflow": "{{workflow.name}}",
        "file": "{{inputs.file_path}}",
        "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
        "duration": {{workflow.duration}}
      }
      EOF
    timeout: 5s

  notify_user:
    type: step
    operation: command
    command: |
      notify-send "Workflow Complete" \
      "Analysis of {{inputs.file_path}} finished"
    timeout: 5s
    continue_on_error: true

  # État terminal (succès)
  final:
    type: terminal
    status: success
    message: "Workflow completed successfully"

  # État terminal (erreur)
  error:
    type: terminal
    status: failure
    message: "Workflow failed"

# Hooks globaux
hooks:
  workflow_start:
    - log: "Starting workflow: {{workflow.name}}"
    - log: "Inputs: {{inputs}}"
    - command: |
        mkdir -p .cache .workflow-states
    - command: |
        echo "$(date): Started {{workflow.name}}" >> workflow.log

  workflow_end:
    - log: "Workflow completed in {{workflow.duration}}s"
    - command: |
        echo "$(date): Completed {{workflow.name}}" >> workflow.log
    - command: |
        rm -rf .cache

  workflow_error:
    - log: "Workflow failed: {{error.message}}"
    - log: "Failed at state: {{workflow.current_state}}"
    - command: |
        echo "$(date): Failed {{workflow.name}} - {{error.message}}" >> workflow.log
    - command: |
        notify-send "Workflow Failed" "{{error.message}}"

  # NEW: Hook for cancellation (Ctrl-C)
  workflow_cancel:
    - log: "Workflow cancelled by user"
    - command: |
        echo "$(date): Cancelled {{workflow.name}}" >> workflow.log

# Configuration de persistance
persistence:
  enabled: true
  checkpoint_after_each_step: true
  state_file: ".workflow-states/{{workflow.id}}.json"
  atomic_writes: true        # Write to temp file, then rename
  cleanup_on_success: false
  cleanup_on_failure: false

# Timeouts globaux
timeouts:
  default_step: 300s
  workflow: 3600s
  # Note: Retry timeouts are INCLUDED in workflow timeout
  # Example: 3 retries x 120s = 360s counted toward workflow timeout

# Configuration de logging
logging:
  level: info
  format: json
  output: "storage/logs/{{workflow.name}}-{{workflow.id}}.log"
  console: true
```

### Variables Disponibles pour Interpolation

**Note**: AWF uses `{{var}}` syntax (Go template style) to avoid conflicts with shell variable expansion `${var}`.

```yaml
# Inputs
{{inputs.variable_name}}

# Outputs d'états précédents
{{states.state_name.output}}
{{states.state_name.exit_code}}
{{states.state_name.metadata.key}}

# Métadonnées workflow
{{workflow.id}}
{{workflow.name}}
{{workflow.version}}
{{workflow.current_state}}
{{workflow.duration}}
{{workflow.started_at}}

# Contexte
{{context.working_dir}}
{{context.user}}
{{context.hostname}}

# Environnement
{{env.VARIABLE_NAME}}

# Erreurs
{{error.message}}
{{error.state}}
{{error.exit_code}}
{{error.type}}              # user | workflow | system

# Métadonnées système
{{metadata.timestamp}}
{{metadata.file_size}}
```

### Escaping

To use a literal `{{` in commands, escape with backslash:
```yaml
command: |
  echo "AWF var: {{inputs.name}}"
  echo "Literal braces: \{\{ not interpolated \}\}"
```

---

## 🎯 Fonctionnalités Détaillées

### 1\. Exécution de Workflows

#### Commande: `run`

```bash
# Exécution basique
awf run analyze-code --file-path=app.py

# Avec options
awf run analyze-code \
  --file-path=app.py \
  --output-format=html \
  --max-tokens=5000 \
  --verbose

# Mode dry-run (simulation)
awf run analyze-code --file-path=app.py --dry-run

# Avec variables d'environnement
CLAUDE_API_KEY=xxx awf run analyze-code --file-path=app.py
```

**Comportement:**

* Charge le workflow depuis `configs/workflows/`

* Valide les inputs

* Crée un contexte d'exécution avec ID unique

* Exécute la state machine

* Persiste l'état après chaque step (si configuré)

* Log toutes les opérations

* Affiche une barre de progression

### 2\. Reprise de Workflows

#### Commande: `resume`

```bash
# Reprendre un workflow interrompu
awf resume <workflow-id>

# Lister les workflows en pause
awf resume --list

# Reprendre avec override d'inputs
awf resume <workflow-id> --file-path=new-file.py
```

**Comportement:**

* Charge l'état depuis le state store

* Reprend à l'état actuel

* Réutilise les outputs des états complétés

### 3\. Gestion d'État

#### Format JSON de l'État

```json
{
  "workflow_id": "analyze-code-20231209-143022-a1b2c3",
  "workflow_name": "analyze-code",
  "workflow_version": "1.0.0",
  "status": "running",
  "current_state": "analyze",
  "started_at": "2023-12-09T14:30:22Z",
  "updated_at": "2023-12-09T14:30:45Z",
  "completed_at": null,
  
  "inputs": {
    "file_path": "app.py",
    "output_format": "markdown",
    "max_tokens": 2000
  },
  
  "states": {
    "validate": {
      "status": "completed",
      "started_at": "2023-12-09T14:30:22Z",
      "completed_at": "2023-12-09T14:30:23Z",
      "duration": 1.2,
      "output": "valid",
      "exit_code": 0,
      "attempt": 1,
      "metadata": {}
    },
    "extract": {
      "status": "completed",
      "started_at": "2023-12-09T14:30:23Z",
      "completed_at": "2023-12-09T14:30:24Z",
      "duration": 0.8,
      "output": "def hello():\n    print('Hello')",
      "exit_code": 0,
      "attempt": 1,
      "metadata": {
        "file_size": 35
      }
    },
    "analyze": {
      "status": "running",
      "started_at": "2023-12-09T14:30:24Z",
      "completed_at": null,
      "attempt": 1,
      "metadata": {}
    }
  },
  
  "context": {
    "working_dir": "/home/user/project",
    "user": "user",
    "hostname": "laptop",
    "env": {
      "CLAUDE_API_KEY": "***"
    }
  },
  
  "metadata": {
    "total_steps": 8,
    "completed_steps": 2,
    "failed_steps": 0
  }
}
```

#### State Atomicity & Recovery

**Problem**: What happens if AWF crashes while writing state?

**Solution**: Atomic writes via temp file + rename

```go
// internal/state/store.go
func (s *JSONStore) Save(state *State) error {
    data, _ := json.MarshalIndent(state, "", "  ")

    // Write to temp file first
    tmpFile := s.stateFile + ".tmp"
    if err := os.WriteFile(tmpFile, data, 0600); err != nil {
        return err
    }

    // Atomic rename (POSIX guarantees atomicity)
    return os.Rename(tmpFile, s.stateFile)
}
```

**Concurrent Access Protection**:

```go
// Use file locks for exclusivity
func (s *JSONStore) Lock() error {
    f, _ := os.OpenFile(s.lockFile, os.O_CREATE|os.O_RDWR, 0600)
    return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}
```

**Recovery on Startup**:

```go
func (s *JSONStore) Load() (*State, error) {
    // Clean up any leftover temp files
    os.Remove(s.stateFile + ".tmp")

    // Load main state file
    data, err := os.ReadFile(s.stateFile)
    if err != nil {
        return nil, err
    }
    // ...
}
```

### 4\. Exécution Parallèle

```yaml
# Définition d'un état parallèle
parallel_processing:
  type: parallel
  strategy: all_succeed       # all_succeed | any_succeed | best_effort
  max_concurrent: 5           # Limit concurrent goroutines (default: 10)
  parallel:
    - process_images
    - process_videos
    - process_audio
  on_success: merge_results
  on_failure: error

# Les 3 états s'exécutent en parallèle
process_images:
  type: step
  command: "process-images.sh"

process_videos:
  type: step
  command: "process-videos.sh"

process_audio:
  type: step
  command: "process-audio.sh"
```

**Parallel Strategies:**

| Strategy | Behavior |
|----------|----------|
| `all_succeed` | All steps must succeed. First failure cancels remaining steps. |
| `any_succeed` | At least one step must succeed. Others can fail. |
| `best_effort` | Run all steps, collect results. `on_success` if any succeeded. |

**Implementation (Go):**

```go
// Uses errgroup for coordinated goroutines
import "golang.org/x/sync/errgroup"

func (r *Runner) executeParallel(ctx context.Context, steps []Step) error {
    g, ctx := errgroup.WithContext(ctx)
    sem := make(chan struct{}, r.maxConcurrent) // Semaphore

    for _, step := range steps {
        step := step // Capture loop var
        g.Go(func() error {
            sem <- struct{}{}        // Acquire
            defer func() { <-sem }() // Release
            return r.executeStep(ctx, step)
        })
    }
    return g.Wait()
}
```

**Behavior:**

* Uses `errgroup` with semaphore for controlled parallelism
* Context propagation for cancellation
* First error cancels remaining steps (for `all_succeed` strategy)
* Outputs available individually via `{{states.step_name.output}}`
* **Warning**: Parallel steps MUST NOT write to same files (no locking)

### 5\. Retry & Fallback

```yaml
# Retry avec backoff exponentiel
risky_operation:
  type: step
  command: "curl https://api.example.com/data"
  retry:
    max_attempts: 5
    initial_delay: 1s
    max_delay: 30s                    # Cap exponential growth
    backoff: exponential              # linear | exponential | constant
    multiplier: 2
    jitter: 0.1                       # ±10% randomization
    retryable_exit_codes: [1, 2, 130] # Only retry these codes
  on_success: next_step
  on_failure: fallback_operation

# Fallback vers une alternative
fallback_operation:
  type: step
  command: "curl https://backup-api.example.com/data"
  on_success: next_step
  on_failure: error
```

**Retry Configuration:**

| Field | Description | Default |
|-------|-------------|---------|
| `max_attempts` | Total attempts (including first) | 3 |
| `initial_delay` | Delay before first retry | 1s |
| `max_delay` | Maximum delay cap | 60s |
| `backoff` | `constant`, `linear`, `exponential` | exponential |
| `multiplier` | Backoff multiplier | 2 |
| `jitter` | Random variation (0.0-1.0) | 0 |
| `retryable_exit_codes` | Exit codes that trigger retry | [1] |

**Retry Timing Example (exponential, multiplier=2, jitter=0.1):**

```
Attempt 1: immediate
Attempt 2: after 1s   (±0.1s jitter)
Attempt 3: after 2s   (±0.2s jitter)
Attempt 4: after 4s   (±0.4s jitter)
Attempt 5: after 8s   (±0.8s jitter)
           --------
Total max: ~15.5s of delays + 5 × step_timeout
```

**Timeout Interaction:**

```yaml
analyze:
  timeout: 120s
  retry:
    max_attempts: 3
# Total possible time: 3 × 120s + delays ≈ 366s
# This counts toward workflow timeout!
```

### 6\. Hooks Pre/Post

```yaml
# Hooks au niveau step
analyze:
  type: step
  command: "claude -c '{{prompt}}'"
  hooks:
    pre:
      - log: "Starting analysis..."
      - command: "echo 'Pre-analysis' >> log.txt"
      - command: "mkdir -p output"
    post:
      - log: "Analysis completed"
      - command: "echo 'Post-analysis' >> log.txt"
      - command: "cp result.txt output/"

# Hooks au niveau workflow
hooks:
  workflow_start:
    - log: "Workflow starting"
    - command: "setup-environment.sh"

  workflow_end:
    - log: "Workflow completed"
    - command: "cleanup.sh"

  workflow_error:
    - log: "Workflow failed: {{error.message}}"
    - command: "send-alert.sh"

  # NEW: Cancellation hook (Ctrl-C / SIGTERM)
  workflow_cancel:
    - log: "Workflow cancelled"
    - command: "cleanup-partial.sh"
```

**Hook Execution Order:**

```
workflow_start → step.pre → step.command → step.post → ... → workflow_end
                                    ↓ (on error)
                             workflow_error
                                    ↓ (on Ctrl-C)
                             workflow_cancel
```

### 7\. Validation des Inputs

```yaml
inputs:
  - name: email
    type: string
    required: true
    validation:
      pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"

  - name: age
    type: integer
    required: true
    validation:
      min: 18
      max: 120

  - name: country
    type: string
    required: true
    validation:
      enum: ["FR", "US", "UK", "DE"]

  - name: file_path
    type: string
    required: true
    validation:
      file_exists: true
      file_extension: [".py", ".js", ".go"]
```

### 8\. Workflow Validation (`awf validate`)

The `awf validate` command performs comprehensive static analysis:

#### Parse-Time Validation

| Check | Description |
|-------|-------------|
| YAML syntax | Valid YAML structure |
| Required fields | `name`, `states.initial`, at least one state |
| State references | All `on_success`/`on_failure` targets exist |
| No orphan states | All states reachable from initial |
| Terminal states | At least one `type: terminal` state |
| Cycle detection | No infinite loops without explicit limit |
| Variable references | All `{{var}}` references are valid paths |
| Input types | Input types match usage in commands |

#### Runtime Validation (at workflow start)

| Check | Description |
|-------|-------------|
| Required inputs | All required inputs provided |
| Input validation | Values match patterns, enums, ranges |
| Environment vars | Required env vars exist |
| File existence | Files with `file_exists: true` exist |
| Executables | Programs in PATH (optional, `--check-executables`) |

#### Validation Output

```bash
$ awf validate analyze-code

Validating: analyze-code.yaml

[OK] YAML syntax valid
[OK] Required fields present
[OK] State graph valid (8 states, 1 terminal)
[OK] No unreachable states
[OK] No infinite cycles detected
[OK] Variable references valid
[WARN] State 'notify_user' has continue_on_error but no fallback

Validation passed with 1 warning.
```

```bash
$ awf validate broken-workflow

Validating: broken-workflow.yaml

[FAIL] State 'process' references undefined state 'next_step'
[FAIL] No terminal state defined
[FAIL] State 'orphan' is unreachable from initial state
[WARN] Input 'count' has no validation rules

Validation failed with 3 errors and 1 warning.
```

### 9\. Logging Structuré (JSON)

```json
{
  "timestamp": "2023-12-09T14:30:22Z",
  "level": "info",
  "message": "step_started",
  "workflow_id": "analyze-code-20231209-143022",
  "workflow_name": "analyze-code",
  "step": "analyze",
  "context": {
    "user": "user",
    "hostname": "laptop"
  }
}

{
  "timestamp": "2023-12-09T14:30:45Z",
  "level": "info",
  "message": "step_completed",
  "workflow_id": "analyze-code-20231209-143022",
  "workflow_name": "analyze-code",
  "step": "analyze",
  "duration": 23.4,
  "exit_code": 0,
  "metadata": {
    "output_size": 1024
  }
}

{
  "timestamp": "2023-12-09T14:31:02Z",
  "level": "error",
  "message": "step_failed",
  "workflow_id": "analyze-code-20231209-143022",
  "workflow_name": "analyze-code",
  "step": "format_output",
  "error": "command not found: pandoc",
  "exit_code": 127,
  "attempt": 1
}
```

---

## 🎨 Interface CLI

### Commandes Disponibles

```bash
# Gestion des workflows
awf list                              # Liste tous les workflows disponibles
awf validate <workflow-name>          # Valide la syntaxe d'un workflow
awf describe <workflow-name>          # Affiche les détails d'un workflow

# Exécution
awf run <workflow-name> [flags]       # Exécute un workflow
awf resume <workflow-id>              # Reprend un workflow
awf stop <workflow-id>                # Arrête un workflow en cours

# Monitoring
awf status <workflow-id>              # Affiche le statut d'un workflow
awf logs <workflow-id>                # Affiche les logs d'un workflow
awf history [flags]                   # Affiche l'historique des exécutions

# Utilitaires
awf init <workflow-name>              # Crée un template de workflow
awf version                           # Affiche la version
awf help                              # Affiche l'aide
```

### Flags Globaux

```bash
--config string       # Chemin vers le dossier de config (default: ./configs)
--storage string      # Chemin vers le dossier de storage (default: ./storage)
--verbose, -v         # Mode verbose
--quiet, -q           # Mode silencieux
--no-color            # Désactive les couleurs
--log-level string    # Niveau de log (debug, info, warn, error)
```

### Exemples d'Output

#### Exécution Normale

```bash
$ awf run analyze-code --file-path=app.py

Starting workflow: analyze-code
ID: analyze-code-20231209-143022

[1/8] validate
  ├─ pre: Validating file: app.py
  ├─ Executing: test -f "app.py" && echo "valid"
  ├─ Duration: 1.2s
  └─ post: File validated [OK]

[2/8] extract
  ├─ Executing: cat "app.py"
  ├─ Duration: 0.8s
  └─ post: Extracted 1024 bytes [OK]

[3/8] analyze
  ├─ pre: Analyzing with Claude...
  ├─ Executing: claude -c "Analyze this code..."
  ├─ Duration: 23.4s
  └─ post: Analysis completed [OK]

[4/8] format_output
  ├─ Executing: echo "..." | pandoc -f markdown -t markdown
  ├─ Duration: 2.1s
  └─ [OK]

[5/8] parallel_save (3 parallel tasks, strategy: all_succeed)
  ├─ [1/3] save_report [OK] (1.2s)
  ├─ [2/3] save_metadata [OK] (0.5s)
  └─ [3/3] notify_user [OK] (0.3s)

Workflow completed successfully in 29.5s

Output files:
  - report.markdown
  - metadata.json

Logs: storage/logs/analyze-code-20231209-143022.log
State: storage/states/analyze-code-20231209-143022.json
```

#### Exécution avec Erreur

```bash
$ awf run analyze-code --file-path=missing.py

Starting workflow: analyze-code
ID: analyze-code-20231209-143500

[1/8] validate
  ├─ pre: Validating file: missing.py
  ├─ Executing: test -f "missing.py" && echo "valid"
  ├─ Exit code: 1
  └─ [FAILED]

Workflow failed at state: validate
Error type: user
Error: File not found: missing.py

Duration: 1.2s
Exit code: 1
Logs: storage/logs/analyze-code-20231209-143500.log
State: storage/states/analyze-code-20231209-143500.json

To resume: awf resume analyze-code-20231209-143500
```

#### Mode Verbose

```bash
$ awf run analyze-code --file-path=app.py --verbose

Starting workflow: analyze-code
ID: analyze-code-20231209-143022
Config: configs/workflows/analyze-code.yaml
Storage: storage/states/analyze-code-20231209-143022.json

Inputs:
  - file_path: app.py
  - output_format: markdown (default)
  - max_tokens: 2000 (default)

[1/8] validate
  State: validate
  Type: step
  Operation: command
  ├─ pre: Validating file: app.py
  ├─ Command: test -f "app.py" && echo "valid"
  ├─ Timeout: 5s
  ├─ Executing...
  ├─ Stdout: valid
  ├─ Stderr:
  ├─ Exit code: 0
  ├─ Duration: 1.234s
  ├─ post: File validated
  └─ Next state: extract [OK]

[2/8] extract
  State: extract
  Type: step
  Operation: command
  ├─ Command: cat "app.py"
  ├─ Timeout: 10s
  ├─ Executing...
  ├─ Stdout: def hello():\n    print('Hello')
  ├─ Output saved to: states.extract.output (35 bytes)
  ├─ Exit code: 0
  ├─ Duration: 0.856s
  ├─ post: Extracted 35 bytes
  └─ Next state: analyze [OK]

...
```

#### Status d'un Workflow

```bash
$ awf status analyze-code-20231209-143022

Workflow: analyze-code
ID: analyze-code-20231209-143022
Status: running
Current State: analyze (3/8)

Started: 2023-12-09 14:30:22
Duration: 23s

Progress: [========--------] 37.5%

States:
  [OK] validate      (1.2s)
  [OK] extract       (0.8s)
  [..] analyze       (running, attempt 1/3)
  [  ] format_output
  [  ] parallel_save
  [  ] save_report
  [  ] save_metadata
  [  ] notify_user

Logs: storage/logs/analyze-code-20231209-143022.log
```

#### Historique

```bash
$ awf history --limit=5

Recent Workflows:

ID                                  Workflow        Status    Exit  Duration  Started
analyze-code-20231209-143022       analyze-code    success   0     29.5s     2023-12-09 14:30:22
deploy-app-20231209-140015         deploy-app      failed    3     12.3s     2023-12-09 14:00:15
data-pipeline-20231209-135500      data-pipeline   success   0     145.2s    2023-12-09 13:55:00
analyze-code-20231209-134000       analyze-code    success   0     31.2s     2023-12-09 13:40:00
deploy-app-20231209-133000         deploy-app      success   0     89.5s     2023-12-09 13:30:00

Total: 127 workflows
Success: 98 (77%)
Failed: 29 (23%)
```

---

## 🧪 Tests

### Structure des Tests

```
tests/
├── unit/
│   ├── domain/
│   │   ├── workflow_test.go
│   │   └── state_machine_test.go
│   ├── application/
│   │   └── service_test.go
│   └── pkg/
│       ├── interpolation_test.go
│       └── validation_test.go
│
├── integration/
│   ├── workflow_execution_test.go
│   ├── state_persistence_test.go
│   └── parallel_execution_test.go
│
└── fixtures/
    ├── workflows/
    │   ├── simple.yaml
    │   ├── parallel.yaml
    │   └── retry.yaml
    └── states/
        └── example-state.json
```

### Exemples de Tests

```go
// tests/unit/domain/workflow_test.go
func TestWorkflowValidation(t *testing.T) {
    tests := []struct {
        name    string
        workflow *workflow.Workflow
        wantErr bool
    }{
        {
            name: "valid workflow",
            workflow: &workflow.Workflow{
                Name: "test",
                States: map[string]workflow.State{
                    "initial": {Type: workflow.StateTypeStep},
                },
            },
            wantErr: false,
        },
        // ...
    }
    // ...
}

// tests/integration/workflow_execution_test.go
func TestWorkflowExecution(t *testing.T) {
    // Setup
    repo := repository.NewYAMLRepository("fixtures/workflows")
    store := store.NewJSONStore("fixtures/states")
    executor := executor.NewShellExecutor()
    logger := logger.NewConsoleLogger()
    service := application.NewWorkflowService(repo, store, executor, logger)
    
    // Execute
    ctx := context.Background()
    inputs := map[string]interface{}{
        "file_path": "test.txt",
    }
    err := service.Run(ctx, "simple", inputs)
    
    // Assert
    assert.NoError(t, err)
}
```

### Commandes de Test

```bash
# Tests unitaires
make test-unit

# Tests d'intégration
make test-integration

# Tous les tests
make test

# Coverage
make test-coverage

# Tests avec race detector
make test-race
```

---

## 🚀 Installation & Déploiement

### Installation depuis les Sources

```bash
# Cloner le repo
git clone https://github.com/username/ai-workflow-cli.git
cd ai-workflow-cli

# Installer les dépendances
go mod download

# Build
make build

# Installer globalement
make install

# Vérifier l'installation
awf version
```

### Installation via Script

```bash
# Linux/macOS
curl -sSL https://raw.githubusercontent.com/username/ai-workflow-cli/main/scripts/install.sh | bash

# Ou avec wget
wget -qO- https://raw.githubusercontent.com/username/ai-workflow-cli/main/scripts/install.sh | bash
```

### Installation via Go

```bash
go install github.com/username/ai-workflow-cli/cmd/awf@latest
```

### Configuration Initiale

```bash
# Créer les dossiers nécessaires
awf init

# Structure créée:
# ~/.awf/
# ├── configs/
# │   └── workflows/
# ├── storage/
# │   ├── states/
# │   └── logs/
# └── settings.yaml
```

### Configuration Globale (settings.yaml)

```yaml
# ~/.awf/settings.yaml

# Chemins
paths:
  workflows: ~/.awf/configs/workflows
  storage: ~/.awf/storage
  logs: ~/.awf/storage/logs

# Logging
logging:
  level: info
  format: json
  console: true

# Exécution
execution:
  default_timeout: 300
  max_parallel: 10
  shell: /bin/bash

# Persistance
persistence:
  enabled: true
  auto_cleanup: true
  retention_days: 30

# Notifications
notifications:
  enabled: false
  on_success: false
  on_failure: true
```

---

## 📊 Roadmap

### Phase 1: MVP (v0.1.0) ✅

* \[x\] Architecture hexagonale de base

* \[x\] Parsing YAML des workflows

* \[x\] Exécution linéaire de steps

* \[x\] Persistance d'état JSON

* \[x\] CLI basique (run, list, status)

* \[x\] Logging JSON structuré

* \[x\] Interpolation de variables

* \[x\] Hooks pre/post

### Phase 2: Core Features (v0.2.0)

* \[ \] State machine avec transitions

* \[ \] Exécution parallèle

* \[ \] Retry avec backoff

* \[ \] Validation des inputs

* \[ \] Commande resume

* \[ \] Historique SQLite

* \[ \] Tests unitaires complets

### Phase 3: Advanced Features (v0.3.0)

* \[ \] Conditions complexes (if/else)

* \[ \] Boucles (for/while)

* \[ \] Templates de workflows

* \[ \] Variables d'environnement chiffrées

* \[ \] Dry-run mode

* \[ \] Interactive mode

### Phase 4: Extensibilité (v0.4.0)

* \[ \] Plugin system

* \[ \] Custom operations

* \[ \] Workflow composition (sous-workflows)

* \[ \] Remote workflows (HTTP)

* \[ \] Workflow marketplace

### Phase 5: Interfaces Additionnelles (v1.0.0)

* \[ \] API REST

* \[ \] WebUI

* \[ \] Message Queue support

* \[ \] Webhooks

* \[ \] Monitoring dashboard

---

## 🔐 Sécurité

### Validation des Inputs

* Validation stricte des types

* Patterns regex pour strings

* Ranges pour integers

* Enums pour choix limités

* Vérification d'existence de fichiers

### Command Injection Prevention

**Problem**: Interpolated variables can contain shell metacharacters.

```yaml
# DANGEROUS - shell injection possible
command: |
  process-file "{{inputs.filename}}"
# If filename = '"; rm -rf /' → arbitrary code execution
```

**Solution 1**: Use argument arrays (preferred)

```yaml
# SAFE - no shell interpretation
command:
  program: process-file
  args:
    - "{{inputs.filename}}"
# Executed via exec.CommandContext(ctx, program, args...)
```

**Solution 2**: Automatic escaping

AWF automatically escapes interpolated values when used in shell commands:
- Single quotes are escaped: `'` → `'\''`
- Special characters are quoted

**Implementation:**

```go
// Use exec.CommandContext instead of shell string
cmd := exec.CommandContext(ctx, program, args...)
// NOT: exec.CommandContext(ctx, "sh", "-c", shellString)
```

### Exécution de Commandes

* Pas de sandboxing (exécution directe)

* Timeout obligatoire sur chaque commande

* Logging de toutes les commandes exécutées

* Variables d'environnement isolées par workflow

* **Process groups**: Kill entire command tree on timeout/cancel

### Secrets

* Variables d'environnement pour secrets

* Pas de stockage en clair dans les workflows

* Masquage dans les logs (automatic for env vars starting with `SECRET_`, `API_KEY`, `PASSWORD`)

### Permissions

* Exécution avec les permissions de l'utilisateur

* Pas d'élévation de privilèges

* Validation des chemins de fichiers

### Resource Limits

```yaml
# Global limits (settings.yaml)
limits:
  max_output_size: 100MB      # Per-step output limit
  max_parallel: 10            # Max concurrent goroutines
  max_workflow_duration: 24h  # Hard limit
```

---

## 📚 Documentation

### Documentation Utilisateur

* **[README.md](http://README.md)**: Introduction et quick start

* **docs/installation.md**: Guide d'installation détaillé

* **docs/workflow-syntax.md**: Référence complète de la syntaxe YAML

* **docs/examples.md**: Exemples de workflows

* **docs/cli-reference.md**: Référence des commandes CLI

* **docs/troubleshooting.md**: Guide de dépannage

### Documentation Développeur

* **docs/architecture.md**: Architecture détaillée

* **docs/contributing.md**: Guide de contribution

* **docs/development.md**: Setup environnement de dev

* **docs/testing.md**: Guide des tests

* **docs/api-future.md**: Spécification API future

### Exemples de Workflows

* **analyze-code.yaml**: Analyse de code avec IA

* **deploy-app.yaml**: Déploiement d'application

* **data-pipeline.yaml**: Pipeline de traitement de données

* **backup-restore.yaml**: Sauvegarde et restauration

* **ci-cd.yaml**: Intégration et déploiement continu

---

## 🛠️ Makefile

```makefile
# Makefile

.PHONY: help build install test clean

# Variables
BINARY_NAME=awf
MAIN_PATH=./cmd/awf
BUILD_DIR=./bin
INSTALL_PATH=/usr/local/bin

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  install        - Install the binary"
	@echo "  test           - Run all tests"
	@echo "  test-unit      - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  lint           - Run linter"
	@echo "  clean          - Clean build artifacts"
	@echo "  run            - Run the application"

# Build
build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Install
install: build
	@echo "Installing to $(INSTALL_PATH)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/
	@echo "Installation complete"

# Tests
test:
	@echo "Running all tests..."
	@go test -v ./...

test-unit:
	@echo "Running unit tests..."
	@go test -v ./internal/... ./pkg/...

test-integration:
	@echo "Running integration tests..."
	@go test -v ./tests/integration/...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-race:
	@echo "Running tests with race detector..."
	@go test -race -v ./...

# Lint
lint:
	@echo "Running linter..."
	@golangci-lint run

# Clean
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# Run
run: build
	@$(BUILD_DIR)/$(BINARY_NAME)

# Development
dev:
	@go run $(MAIN_PATH)

# Dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Format
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Vet
vet:
	@echo "Vetting code..."
	@go vet ./...
```

---

## 📦 Livrables

### Code Source

* Repository Git complet

* Architecture hexagonale implémentée

* Tests unitaires et d'intégration

* Documentation inline (godoc)

### Binaires

* Linux (amd64, arm64)

* macOS (amd64, arm64)

* Windows (amd64)

### Documentation

* [README.md](http://README.md) complet

* Guide d'installation

* Référence de la syntaxe YAML

* Exemples de workflows

* Documentation d'architecture

### Exemples

* 5+ workflows d'exemple

* Cas d'usage variés

* Templates réutilisables

---

## 🎓 Ressources & Références

### Architecture

* [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)

* [Clean Architecture (Uncle Bob)](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)

* [Domain-Driven Design](https://martinfowler.com/bliki/DomainDrivenDesign.html)

### Go Best Practices

* [Effective Go](https://go.dev/doc/effective_go)

* [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

* [Standard Go Project Layout](https://github.com/golang-standards/project-layout)

### CLI Design

* [Command Line Interface Guidelines](https://clig.dev/)

* [12 Factor CLI Apps](https://medium.com/@jdxcode/12-factor-cli-apps-dd3c227a0e46)

### State Machines

* [State Pattern](https://refactoring.guru/design-patterns/state)

* [Finite State Machines](https://en.wikipedia.org/wiki/Finite-state_machine)

---

## 👥 Contributeurs

### Mainteneur Principal

* **Nom**: \[À définir\]

* **Email**: \[À définir\]

* **GitHub**: \[À définir\]

### Comment Contribuer

1. Fork le projet

2. Créer une branche feature (`git checkout -b feature/amazing-feature`)

3. Commit les changements (`git commit -m 'Add amazing feature'`)

4. Push vers la branche (`git push origin feature/amazing-feature`)

5. Ouvrir une Pull Request

### Guidelines de Contribution

* Suivre l'architecture hexagonale

* Écrire des tests pour toute nouvelle feature

* Documenter le code (godoc)

* Respecter le style Go (gofmt)

* Mettre à jour la documentation

---

## 📄 License

MIT License - voir le fichier LICENSE pour plus de détails

---

## 🔄 Changelog

### v0.1.0 (À venir)

* Initial release

* Core workflow engine

* CLI basique

* Persistance d'état

* Logging structuré

---

## 📞 Support

### Issues

Ouvrir une issue sur GitHub: [github.com/username/ai-workflow-cli/issues](https://github.com/username/ai-workflow-cli/issues)

### Discussions

Rejoindre les discussions: [github.com/username/ai-workflow-cli/discussions](https://github.com/username/ai-workflow-cli/discussions)

### Email

Contact: [email@example.com](mailto:email@example.com)

---

**Date de création**: 2023-12-09\
**Dernière mise à jour**: 2023-12-09\
**Version du document**: 1.0.0
