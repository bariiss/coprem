# Copilot Premium Observer

Go/Cobra CLI for GitHub Enterprise Copilot premium request analytics.

It targets the GitHub Enterprise Cloud billing endpoints:

- `GET /enterprises/{enterprise}/settings/billing/usage` — premium request usage
- `GET/POST/PATCH/DELETE /enterprises/{enterprise}/settings/billing/budgets` — per-user AI credit budgets

Set the target enterprise with `--enterprise` or `COPREM_ENTERPRISE`.

## Install

Install the latest version with Go:

```sh
go install github.com/bariiss/coprem@latest
```

Install a specific release:

```sh
go install github.com/bariiss/coprem@v0.1.0
```

Make sure your Go binary directory is on `PATH`:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

You can also download prebuilt binaries from GitHub Releases. Release archives
are published for macOS, Linux, and Windows on amd64 and arm64.

## Development

### Build & Test

```sh
go build ./...                 # build all packages
go build -o coprem .           # build the binary
go test ./...                  # run all tests
```

### Linting & Security

The project enforces strict code quality and security checks on both CI and locally via git hooks:

* **Linter**: `golangci-lint run` (uses `.golangci.yml` version 2 rules).
* **Security Scan**: `semgrep scan --config "p/owasp-top-ten" --config "p/gosec" --config "p/golang" --config "p/security-audit" --error`.
* **Git Pre-commit Hook**: Automatically executes `go test`, `golangci-lint`, and `semgrep` scan before allowing any commit.


## Release

Releases are created from Git tags by GitHub Actions and GoReleaser.

Create and push a semantic version tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The release workflow builds archives for:

- macOS amd64 and arm64
- Linux amd64 and arm64
- Windows amd64 and arm64

Each release includes platform archives and `checksums.txt`.

## Quick start

Run with explicit flags:

```sh
coprem premium --enterprise ENTERPRISE_SLUG
```

Or set defaults for your shell:

```sh
export COPREM_ENTERPRISE=ENTERPRISE_SLUG
export COPREM_TOKEN=github_pat_or_classic_pat

coprem premium
```

If you already use the GitHub CLI, `coprem` can read that token instead:

```sh
gh auth login
coprem premium --enterprise ENTERPRISE_SLUG --gh-user GITHUB_USER
```

## Authentication

The CLI reads a token from the first available source:

1. `--token-env <NAME>`
2. `COPREM_TOKEN`
3. `GITHUB_TOKEN`
4. `GH_TOKEN`
5. `COPILOT_PREMIUM_TOKEN`
6. `gh auth token`

The endpoint requires an enterprise administrator or billing manager token. GitHub's docs state that this premium request billing endpoint does not support GitHub App tokens or fine-grained PATs, so use a classic PAT or an authenticated `gh` profile with the right enterprise billing access.

Optional local helper:

```sh
coprem auth save-zshrc --gh-user GITHUB_USER
source ~/.zshrc
```

That writes a managed block like this to `~/.zshrc`:

```sh
# BEGIN coprem
export COPREM_TOKEN='...'
# END coprem
```

`COPREM_ENTERPRISE` can also be set in the same shell file if you do not want to
pass `--enterprise` every time:

```sh
export COPREM_ENTERPRISE=ENTERPRISE_SLUG
```

The helper stores the token in plaintext in `~/.zshrc`; using `gh auth login` is cleaner when possible.

## Examples

Current month, cumulative:

```sh
coprem premium --enterprise ENTERPRISE_SLUG
```

Use a specific logged-in GitHub CLI account without changing the active profile:

```sh
coprem premium --enterprise ENTERPRISE_SLUG --gh-user GITHUB_USER
```

Or persist that account's token for future shells:

```sh
coprem auth save-zshrc --gh-user GITHUB_USER
source ~/.zshrc
```

Last month, grouped by model:

```sh
coprem premium --enterprise ENTERPRISE_SLUG --timeframe last-month --group-by model
```

Grouped by user. This uses the Copilot seats API to discover users, then queries
premium request usage once per user:

```sh
coprem premium --enterprise ENTERPRISE_SLUG --gh-user GITHUB_USER --group-by user
```

Total per user is the default. Add a model breakdown when needed:

```sh
coprem premium --enterprise ENTERPRISE_SLUG --group-by user --users alice,bob
coprem premium --enterprise ENTERPRISE_SLUG --group-by user --breakdown model --users alice,bob
```

If the token cannot read enterprise Copilot seats, switch to an enterprise admin
account or refresh scopes:

```sh
gh auth switch
gh auth refresh -h github.com -s admin:enterprise

# or pin the enterprise admin account:
export COPREM_GH_USER=enterprise-admin
coprem budget set --all --amount 50 --yes
```

Tables use color automatically on a terminal. Override with `--color always` or
`--color never`.

April 2026, daily CSV:

```sh
coprem premium --enterprise ENTERPRISE_SLUG --year 2026 --month 4 --granularity daily --format csv
```

Filter by user, organization, model, product, or cost center:

```sh
coprem premium \
  --enterprise ENTERPRISE_SLUG \
  --organization ORG_LOGIN \
  --user octocat \
  --model GPT-5 \
  --cost-center-id none \
  --format json
```

## Per-user budgets

GitHub Enterprise lets you set monthly AI credit budgets per user with a hard stop
when the limit is reached. `coprem budget` wraps the budgets REST API.

List Copilot seat users (the same pool you pick from in the GitHub UI):

```sh
coprem budget users --enterprise ENTERPRISE_SLUG --gh-user GITHUB_USER
```

List existing user budgets:

```sh
coprem budget list --enterprise ENTERPRISE_SLUG
```

By default, `budget list` shows every Copilot seat user. Users without a budget
show `-` in the amount column. Use `--budgets-only` to list only defined budgets.

Set a $50/month limit for one user:

```sh
coprem budget set --enterprise ENTERPRISE_SLUG --user example-user --amount 50
```

Set the same limit for all Copilot seat users:

```sh
coprem budget set --enterprise ENTERPRISE_SLUG --all --amount 50 --yes
```

Set the same limit for multiple users:

```sh
coprem budget set --enterprise ENTERPRISE_SLUG --users alice,bob --amount 30 --yes
```

Interactive picker (numbered user list + amount prompt):

```sh
coprem budget set --enterprise ENTERPRISE_SLUG --interactive
```

If a user already has an `ai_credits` budget, `set` updates it via PATCH instead
of creating a duplicate.

### Keyboard-driven TUI (budget manage)

For a full-screen, keyboard-driven interface to manage budgets and monitor current-month usage (NET) per user, run:

```sh
coprem budget manage --enterprise ENTERPRISE_SLUG --gh-user GITHUB_USER
```

This launches an interactive Bubble Tea TUI where you can:

- **Browse & Navigate**: Use `↑`/`↓` keys to move through the table rows.
- **Filter**: Press `/` to search and filter users in real time.
- **Edit Inline**: Select a user, press `Enter`, input a new budget limit, and press `y` to apply.
- **Delete**: Press `d` to delete a user's budget with `[y/N]` confirmation.
- **Toggle SKU**: Press `s` to switch between `ai_credits` and `premium_requests` product SKUs.
- **Loading status**: Features a built-in async spinner during initial loading and after mutations while fetching budget updates and NET usage.

Use another enterprise or GitHub API host:

```sh
coprem premium \
  --enterprise ENTERPRISE_SLUG \
  --api-base-url https://api.github.com
```

For GHE.com dedicated subdomains, set `--api-base-url https://api.SUBDOMAIN.ghe.com`.

## Architecture

The project is structured into four layers, where dependencies flow downward only:

- **`cmd/`** — Cobra command wiring and all CLI orchestration. Subcommands: `premium.go`, `budget.go` (+ `budget_manage.go`), `auth.go`.
- **`internal/github/`** — HTTP client and all GitHub-specific API logic (Link-header pagination, token resolution, budget CRUD).
- **`internal/output/`** — Pure reporting and renderers (`WriteTable` with heatmap coloring, `WriteJSON`, `WriteCSV`).
- **`internal/tui/budgettui/`** — Bubble Tea TUI model and store interface for interactive budget management.
