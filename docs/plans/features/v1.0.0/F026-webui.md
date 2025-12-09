# F026: WebUI

## Metadata
- **Statut**: backlog
- **Phase**: 5-Interfaces
- **Version**: v1.0.0
- **Priorité**: medium
- **Estimation**: XL

## Description

Build a web-based user interface for AWF. Provide visual workflow management, execution monitoring, and log viewing. Enable non-technical users to interact with workflows through a browser.

## Critères d'Acceptance

- [ ] Dashboard with execution overview
- [ ] Workflow list with search/filter
- [ ] Workflow detail view
- [ ] Start workflow with form inputs
- [ ] Real-time execution progress
- [ ] Log viewer with streaming
- [ ] Execution history with filters
- [ ] Basic workflow editor (optional)

## Dépendances

- **Bloqué par**: F025
- **Débloque**: _none_

## Fichiers Impactés

```
web/
├── src/
│   ├── components/
│   ├── pages/
│   ├── api/
│   └── App.tsx
├── package.json
└── vite.config.ts
internal/interfaces/api/static.go
```

## Tâches Techniques

- [ ] Choose frontend stack
  - [ ] React + TypeScript
  - [ ] Vite for bundling
  - [ ] TailwindCSS for styling
- [ ] Set up project structure
  - [ ] Components library
  - [ ] API client
  - [ ] State management
- [ ] Implement pages
  - [ ] Dashboard
  - [ ] Workflow list
  - [ ] Workflow detail
  - [ ] Execution detail
  - [ ] History
  - [ ] Settings
- [ ] Implement components
  - [ ] WorkflowCard
  - [ ] ExecutionProgress
  - [ ] LogViewer
  - [ ] InputForm
  - [ ] StateGraph visualization
- [ ] Real-time updates
  - [ ] SSE for log streaming
  - [ ] Polling or WebSocket for status
- [ ] Embed in Go binary
  - [ ] go:embed for static files
  - [ ] Serve from API server
- [ ] Write E2E tests

## Notes

WebUI pages:
```
/                     - Dashboard
/workflows            - Workflow list
/workflows/:name      - Workflow detail, run form
/executions           - Execution history
/executions/:id       - Execution detail, logs
/settings             - Configuration
```

Dashboard wireframe:
```
┌─────────────────────────────────────────────────────┐
│  AWF Dashboard                              [user]  │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐  │
│  │ Running │ │ Success │ │ Failed  │ │  Total  │  │
│  │    3    │ │   127   │ │   12    │ │   142   │  │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘  │
│                                                     │
│  Recent Executions                                  │
│  ┌─────────────────────────────────────────────┐   │
│  │ analyze-code-xxx  [running] ████░░░░ 50%   │   │
│  │ deploy-app-xxx    [success] ✓ 2m ago       │   │
│  │ data-pipe-xxx     [failed]  ✗ 5m ago       │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
│  Popular Workflows                                  │
│  ┌──────────────┐ ┌──────────────┐                │
│  │ analyze-code │ │  deploy-app  │ ...           │
│  │   ▶ Run      │ │   ▶ Run      │                │
│  └──────────────┘ └──────────────┘                │
└─────────────────────────────────────────────────────┘
```

Use go:embed to bundle frontend into single binary.
