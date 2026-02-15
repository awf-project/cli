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

AWF executes shell commands defined in workflow YAML files. Be aware of:

### Command Execution

- AWF uses `/bin/sh -c` for command execution
- Only run workflows from trusted sources
- Review workflow files before execution

### Secret Handling

- Variables prefixed with `SECRET_`, `API_KEY`, or `PASSWORD` are masked in logs
- Use environment variables for sensitive values
- Never commit secrets to workflow files

### File System Access

- AWF reads/writes state files to `.awf/storage/`
- Atomic writes prevent corruption (temp file + rename)
- File locking prevents concurrent access issues

### Best Practices

When using AWF:
- Review workflow YAML files before running
- Use `awf validate` to check workflow syntax
- Use `awf run --dry-run` to preview execution
- Keep AWF updated to latest version
- Use environment variables for secrets
- Restrict file permissions on `.awf/` directory

## Security Updates

Subscribe to security advisories:
- Watch this repository (Releases only)
- Check [GitHub Security Advisories](https://github.com/awf-project/awf/security/advisories)
