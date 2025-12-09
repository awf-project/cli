# F018: Variables d'Environnement Chiffrées

## Metadata
- **Statut**: backlog
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priorité**: medium
- **Estimation**: M

## Description

Support encrypted environment variables and secrets. Allow storing sensitive values in encrypted form in configuration. Decrypt at runtime using a master key. Prevent accidental exposure in logs and outputs.

## Critères d'Acceptance

- [ ] `awf encrypt` command to encrypt values
- [ ] Store encrypted values in settings
- [ ] Decrypt at runtime with master key
- [ ] Support multiple encryption backends
- [ ] Mask decrypted values in logs
- [ ] Clear error if decryption fails
- [ ] Support key rotation

## Dépendances

- **Bloqué par**: F006
- **Débloque**: _none_

## Fichiers Impactés

```
pkg/crypto/encrypt.go
pkg/crypto/decrypt.go
internal/interfaces/cli/commands/encrypt.go
internal/application/secret_manager.go
configs/secrets.yaml.enc
```

## Tâches Techniques

- [ ] Implement encryption module
  - [ ] AES-256-GCM encryption
  - [ ] Key derivation (PBKDF2 or Argon2)
  - [ ] Salt and nonce handling
- [ ] Implement `encrypt` command
  - [ ] Read value from stdin or --value
  - [ ] Output encrypted string
  - [ ] Support --key-file or env var
- [ ] Implement SecretManager
  - [ ] Load encrypted secrets
  - [ ] Decrypt on demand
  - [ ] Cache decrypted values
  - [ ] Clear cache on workflow end
- [ ] Define encrypted value format
  - [ ] `ENC[version:salt:nonce:ciphertext]`
- [ ] Extend variable interpolation
  - [ ] `{{secrets.API_KEY}}`
  - [ ] Decrypt before interpolation
- [ ] Enhanced log masking
  - [ ] Mask all decrypted values
- [ ] Support key sources
  - [ ] Environment variable
  - [ ] Key file
  - [ ] System keyring (future)
- [ ] Write unit tests
- [ ] Write security tests

## Notes

Usage flow:
```bash
# Encrypt a secret
echo -n "my-api-key" | awf encrypt --key-file ~/.awf/master.key
# Output: ENC[1:abc123:xyz789:encrypted_data_here]

# Store in secrets.yaml
secrets:
  API_KEY: "ENC[1:abc123:xyz789:encrypted_data_here]"

# Use in workflow
command: "curl -H 'Authorization: {{secrets.API_KEY}}' api.example.com"
```

Master key should be at least 32 bytes. Never store in repository.
