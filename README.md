# Copilot Premium Observer

Go/Cobra CLI for GitHub Enterprise Copilot premium request analytics.

It targets the GitHub Enterprise Cloud billing endpoint:

`GET /enterprises/{enterprise}/settings/billing/premium_request/usage`

Set the target enterprise with `--enterprise` or `COPREM_ENTERPRISE`.

## Build

```sh
go build ./...
go build -o coprem .
```

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

If the token cannot read enterprise Copilot seats, refresh the GitHub CLI token
or pass a user list explicitly:

```sh
gh auth refresh -h github.com -u GITHUB_USER -s admin:enterprise

coprem premium \
  --enterprise ENTERPRISE_SLUG \
  --gh-user GITHUB_USER \
  --group-by user \
  --users alice,bob
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

Use another enterprise or GitHub API host:

```sh
coprem premium \
  --enterprise ENTERPRISE_SLUG \
  --api-base-url https://api.github.com
```

For GHE.com dedicated subdomains, set `--api-base-url https://api.SUBDOMAIN.ghe.com`.
