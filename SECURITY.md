# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them to security@provops.com

Include:
- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Impact assessment (if known)

## Response Timeline

- **Acknowledgment**: within 48 hours
- **Triage and initial assessment**: within 5 business days
- **Fix timeline**: depends on severity (Critical: 7 days, High: 30 days, Medium/Low: next release)

## Scope

In scope:
- Server API (Go backend)
- Web UI (React frontend)
- Helm chart and deployment manifests
- Authentication and authorization (OIDC, RBAC)

Out of scope:
- Third-party dependencies (report upstream)
- Infrastructure or hosting issues
- Social engineering

## Disclosure Policy

We follow coordinated disclosure. We ask reporters to allow us reasonable
time to fix vulnerabilities before public disclosure.
