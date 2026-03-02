# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.x.x   | :white_check_mark: |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to: alexandre@vanoix.com

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial Assessment**: Within 1 week
- **Resolution**: Depends on severity
  - Critical: Within 1 week
  - High: Within 2 weeks
  - Medium: Within 1 month
  - Low: Next release

### Disclosure Policy

- We follow coordinated disclosure
- Credit will be given in release notes (unless you prefer anonymity)
- We may request a CVE for critical vulnerabilities

## Security Considerations

AWF is a powerful orchestration tool that combines AI agents and shell execution. This combination introduces specific security risks:

### ⚠️ Operational Risks

- **Arbitrary Code Execution:** AWF is designed to execute shell commands defined in YAML workflows using the user's preferred shell (detected via `$SHELL`, falling back to `/bin/sh`). It runs with the same permissions as the user executing the CLI.
- **AI Non-Determinism (Hallucinations):** AI agents (LLMs) are probabilistic models. They can produce unexpected, incorrect, or destructive output ("hallucinations"). A prompt that seems safe can generate a harmful command in certain contexts.
- **Untrusted Workflows:** Treat workflow files (`.yaml`) and prompt files (`.md`) as executable scripts. Never run a workflow from an untrusted source without a thorough manual audit.

### Data Security

- **Prompt Injection:** Be aware that prompts processed by agents could theoretically contain malicious instructions if they include untrusted input from previous steps or external files.
- **Secret Handling:** Variables prefixed with `SECRET_`, `API_KEY`, or `PASSWORD` are masked in logs. Always use environment variables for sensitive values and never commit secrets to workflow files.
- **Local Context:** AWF is intended for local or controlled environments. Do not expose the CLI as a public service or through a network gateway without extreme caution.

### File System Access

- **Atomic Writes:** AWF uses a temp file + rename pattern for all state file writes to prevent corruption.
- **File Locking:** AWF uses `flock` to prevent concurrent access issues when writing to the JSON state store.

### Safe Usage Best Practices

To minimize risk while using AWF:

1. **Audit First:** Manually review all workflow YAML and prompt files before execution.
2. **Validate Syntax:** Use `awf validate` to check workflow syntax.
3. **Dry-Run Mode:** Use `awf run --dry-run` to preview the execution plan.
4. **Interactive Mode:** Use `awf run --interactive` to approve each command step-by-step.
5. **Sandboxing:** Run AWF in isolated environments (Docker, VM, DevContainer) when executing workflows that modify the system or come from external sources.
6. **Least Privilege:** Run AWF with the minimum necessary permissions. Avoid running as root.
7. **Keep Updated:** Keep AWF and its dependencies updated to the latest version.

## Security Updates

Subscribe to security advisories:
- Watch this repository (Releases only)
- Check [GitHub Security Advisories](https://github.com/awf-project/cli/security/advisories)
