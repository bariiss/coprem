# Contributing to Copilot Premium Observer

First off, thank you for considering contributing to `coprem`! Contributions from the community help make this tool better for everyone.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md). Please report any unacceptable behavior to **<baris.dogu@icloud.com>**.

## Development Setup

### Prerequisites

- **Go**: Version 1.22 or later.
- **golangci-lint**: Installed locally to match the project's quality configuration.
- **Semgrep**: Installed locally for security scans.

### Git Workflow

1. Fork the repository and create your branch from `main`.
2. Ensure you have the local pre-commit hook installed to automatically test and lint your changes:

   ```sh
   cp .git/hooks/pre-commit.sample .git/hooks/pre-commit # if you want to set it up manually
   chmod +x .git/hooks/pre-commit
   ```

### Quality & Security Checklist

Before pushing any changes or opening a Pull Request, please ensure that:

1. **Tests pass successfully**:

   ```sh
   go test ./...
   ```

2. **Linter returns no issues**:

   ```sh
   golangci-lint run
   ```

3. **Semgrep security scan returns zero findings**:

   ```sh
   semgrep scan --config "p/owasp-top-ten" --config "p/gosec" --config "p/golang" --config "p/security-audit" --error
   ```

## Pull Request Guidelines

- Ensure your commit messages are descriptive and follow standard conventions (e.g., `feat: add filter to budget command`, `fix: handle response bounds`).
- Keep pull requests focused on a single change or fix.
- Make sure all CI checks pass on the GitHub Actions interface before requesting a review.
