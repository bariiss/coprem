# AGENTS.md

This file provides guidance to AI agents when working with code in this repository.

## What this is

`coprem` (Copilot Premium Observer) is a Go/Cobra CLI for GitHub Enterprise Cloud
Copilot premium-request analytics and per-user AI-credit budget management. It calls
the enterprise billing endpoints:

- `GET /enterprises/{enterprise}/settings/billing/ai_credit/usage` — premium request usage
- `GET/POST/PATCH/DELETE /enterprises/{enterprise}/settings/billing/budgets` — per-user budgets
- `GET /enterprises/{enterprise}/copilot/billing/seats` — seat discovery (auto-paginated)

These endpoints require an **enterprise admin / billing manager token with `admin:enterprise`
scope**. GitHub App tokens and fine-grained PATs are not accepted — use a classic PAT or an
authenticated `gh` profile.

## Commands

```sh
go build ./...                 # build all packages
go build -o coprem .           # build the binary
go test ./...                  # run all tests
go test ./internal/tui/budgettui -run TestName   # run a single test
golangci-lint run              # lint (also runs as the pre-commit hook)
```

CI (`.github/workflows/ci.yml`) runs golangci-lint **v2.12.2**, `go test ./...`, and
`go build ./...`. The local pre-commit hook (`.git/hooks/pre-commit`) is just `golangci-lint run`,
so keep the installed binary at the same version. Releases are tag-driven via GoReleaser
(`.goreleaser.yaml`) — push a `vX.Y.Z` tag.

## Architecture

Four layers, each its own package; dependencies flow downward only:

- **`cmd/`** — Cobra command wiring and all CLI orchestration. `root.go` holds the global
  `opts` (enterprise, API base/version, token source, format, color) as persistent flags.
  `common.go` has shared helpers (token/client construction, user discovery, interactive
  prompts). Subcommands: `premium.go`, `budget.go` (+ `budget_manage.go`), `auth.go`.
- **`internal/github/`** — the HTTP client and all GitHub specifics. `client.go` is the
  request plumbing (`get`/`getWithNext` with Link-header pagination, `doJSON` for writes,
  `ResolveToken`, error formatting). `budget.go` holds budget types + CRUD. `auth_hint.go`
  parses `gh auth status` to pick an enterprise-scoped account and build actionable auth hints.
- **`internal/output/`** — pure reporting: `Report`/`Row` types, `GroupReport` (client-side
  grouping), `SortRows`, and the three renderers `WriteTable` / `WriteJSON` / `WriteCSV`.
  This package owns the ANSI table formatting and heatmap coloring.
- **`internal/tui/budgettui/`** — the Bubble Tea TUI behind `budget manage`. `model.go` is the
  `tea.Model` state machine (browse / filter / edit / confirm / apply modes). `store.go`
  defines the `Store` interface — the model's *only* dependency on the outside world.
  `cmd/budget_manage.go` provides `budgetStore`, the adapter from `*github.Client` to `Store`.

### Key flows

- **Token resolution** (`github.ResolveToken`): tries `gh auth token` first when a gh user is
  set or the active gh account has enterprise scope, otherwise falls through env vars
  `--token-env` → `COPREM_TOKEN` → `GITHUB_TOKEN` → `GH_TOKEN` → `COPILOT_PREMIUM_TOKEN`, then
  `gh auth token` as a last resort. Enterprise is set via `--enterprise` / `COPREM_ENTERPRISE`.
- **`premium`**: resolves a time `period` from `--timeframe`/`--year`/`--month`/`--day`/`--from`/`--to`,
  fetches cumulative or per-day usage, then `GroupReport` → `SortRows`. The usage endpoint only
  attributes line items to a user when queried *with* a user filter, so `--group-by user` (and
  the TUI's NET column) fetch **once per user** and aggregate. When grouping by user, budgets are
  merged in via `mergeUserBudgets`.
- **`budget manage`** opens the TUI immediately with `nil` rows and loads data (budgets + per-user
  NET usage) in `Init`, showing a spinner — because the per-user NET fetch takes several seconds.
  NET is fetched lazily once and reused across SKU toggles and post-mutation reloads; it is
  best-effort (column shows `-` on failure).

User-scoped budgets are always created with `prevent_further_usage=true` (a hard monthly stop)
and `budget_type=BundlePricing`. Two SKUs: `ai_credits` (default) and `premium_requests`.

## Conventions

- Output respects `--format table|json|csv` and `--color auto|always|never` globally; new
  commands should honor both rather than printing ad hoc.
- `cmd/` is exempt from the `forbidigo` ban on `fmt.Print*` (it's the CLI's output channel);
  elsewhere that lint is active. `_test.go` files are exempt from `dupl`.
- The linter config (`.golangci.yml`) is strict (v2 format, ~50 linters incl. `funlen` 120 lines,
  `gocyclo` 30, `gocognit` 30). Keep functions small; existing `//nolint:` directives document
  the deliberate exceptions (CLI error paths, user-supplied file reads).
- Go-symbol search/edit in this repo is backed by the gograph MCP server — prefer
  `gograph_query`/`gograph_context`/`gograph_plan` over grep for Go symbols (see global AGENTS.md).
