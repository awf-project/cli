# F024: Remote Workflows (HTTP)

## Metadata
- **Status**: backlog
- **Phase**: 4-Extensibility
- **Version**: v0.4.0
- **Priority**: low
- **Estimation**: M

## Description

Load workflow definitions from remote URLs. Support HTTP(S) sources with caching and signature verification. Enable sharing workflows via registries or direct URLs without local installation.

## Acceptance Criteria

- [ ] Load workflow from HTTP(S) URL
- [ ] Cache remote workflows locally
- [ ] Cache invalidation (TTL, force refresh)
- [ ] Signature verification for security
- [ ] Support authentication (Bearer, Basic)
- [ ] `awf run https://...` syntax
- [ ] Registry support (index of workflows)

## Dependencies

- **Blocked by**: F002
- **Unblocks**: _none_

## Impacted Files

```
internal/infrastructure/repository/remote_repository.go
internal/infrastructure/repository/cache.go
pkg/signature/verify.go
configs/settings.yaml
```

## Technical Tasks

- [ ] Implement RemoteRepository
  - [ ] Fetch workflow from URL
  - [ ] Parse response as YAML
  - [ ] Handle errors (404, timeout)
- [ ] Implement workflow cache
  - [ ] Store in ~/.awf/cache/
  - [ ] Cache key from URL hash
  - [ ] TTL-based expiration
  - [ ] Force refresh flag
- [ ] Implement signature verification
  - [ ] Ed25519 signatures
  - [ ] Public key configuration
  - [ ] Reject unsigned if required
- [ ] Support authentication
  - [ ] Bearer token
  - [ ] Basic auth
  - [ ] Token from env var
- [ ] Implement registry support
  - [ ] Registry index format
  - [ ] Search workflows
  - [ ] `awf search <query>`
- [ ] CLI support
  - [ ] `awf run https://example.com/workflow.yaml`
  - [ ] `awf run registry:analyze-code@1.0.0`
- [ ] Write tests

## Notes

Remote workflow usage:
```bash
# Direct URL
awf run https://workflows.example.com/analyze-code.yaml --file=app.py

# With authentication
AWF_REMOTE_TOKEN=xxx awf run https://private.example.com/workflow.yaml

# From registry
awf run registry:official/analyze-code@1.0.0 --file=app.py

# Force refresh
awf run https://example.com/workflow.yaml --refresh
```

Security settings:
```yaml
# settings.yaml
remote:
  enabled: true
  require_signature: true
  trusted_keys:
    - "ed25519:AAAA..."
  cache_ttl: 24h
  allowed_domains:
    - "*.example.com"
```
