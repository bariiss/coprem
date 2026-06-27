# Design: `coprem budget manage` — interactive budget TUI

Date: 2026-06-27

## Goal

Give operators a full-screen, keyboard-driven view of Copilot seat users and
their AI credit budgets. From the table they navigate to a user's row, edit the
budget inline, and apply it after a confirmation prompt. This replaces the need
to read static `budget list` output, copy a login, and run `budget set` by hand.

## Non-goals (YAGNI)

- The existing `budget list`, `budget set`, `budget delete`, and `budget users`
  commands stay exactly as they are (scriptable, pipe-friendly).
- The current `budget set --interactive` line-based flow is left untouched.
- No multi-enterprise switching, no budget alerting configuration, no history.

## Library

`github.com/charmbracelet/bubbletea` + `github.com/charmbracelet/bubbles`
(`table`, `textinput`) + `github.com/charmbracelet/lipgloss` for styling. This
is the de facto standard for Go TUIs and matches the `gh` CLI ecosystem.

## Architecture

### New files

- `cmd/budget_manage.go` — the `budget manage` cobra command. Plumbing only:
  resolve the client, fetch users + budgets (reusing `discoverUsers`,
  `client.ListBudgets`, and `mergeUserBudgetRows`), wrap the client in a `Store`
  adapter, and run the Bubble Tea program.
- `internal/tui/budget/model.go` — the entire TUI model (state, `Update`,
  `View`). No dependency on cobra or `net/http`.
- `internal/tui/budget/model_test.go` — unit tests for `Update` transitions
  against a fake `Store`.

### Isolation boundary

The model depends on a small interface, not the GitHub client directly:

```go
type Row struct {
    User       string
    HasBudget  bool
    Amount     *int      // nil when no budget
    Consumed   *float64  // nil when no budget
    ProductSKU string
    ID         string    // budget id, empty when no budget
}

type Store interface {
    // Upsert creates or updates the user's budget; returns "Created"/"Updated".
    Upsert(ctx context.Context, user string, amount int, sku string) (action string, err error)
    Delete(ctx context.Context, budgetID string) error
    // Reload returns the full row set for the given SKU (used on SKU toggle and refresh).
    Reload(ctx context.Context, sku string) ([]Row, error)
}
```

The `cmd` layer provides a concrete adapter wrapping `*githubapi.Client` (using
`upsertUserBudget`, `client.DeleteBudget`, and the `discoverUsers` +
`ListBudgets` + `mergeUserBudgetRows` pipeline). The model is fully testable
with a fake `Store` that never touches the network.

The existing `userBudgetRow` in `cmd/budget.go` and the new `tui/budget.Row` are
kept as separate types so the TUI package has no dependency back on `cmd`; the
adapter maps between them.

## Interaction & state machine

States: `browsing → editing → confirming → applying → (browsing | error)`.

| Key            | Action                                                        |
|----------------|---------------------------------------------------------------|
| ↑/↓, j/k       | move between rows                                             |
| `enter`        | edit selected row's budget (open amount input)                |
| `d`            | delete selected budget (only when the row has a budget)       |
| `/`            | live filter by login                                          |
| `s`            | toggle SKU (ai_credits ↔ premium_requests); reloads the table |
| `q` / `esc`    | quit (in edit/filter mode, `esc` cancels that mode instead)   |

- **editing:** a `textinput` opens on the selected row; accepts a positive whole
  dollar amount only. `enter` advances to confirmation; `esc` cancels.
- **confirming ("are you sure?"):**
  - set/update: `Set $75/month for alice with hard stop? [y/N]`
  - delete: `Delete budget for bob? [y/N]`
  - `y` applies; any other key cancels back to browsing.
- **applying:** the API call runs as a `tea.Cmd` (UI never blocks); the result
  arrives as a message and the affected row is updated in place. SKU toggle and
  delete reload likewise run as async commands.

## Error handling

- API errors are shown in a bottom status line; the program does not crash and
  returns to browsing.
- Missing enterprise is caught up front by `requireEnterprise` before the
  program starts.
- Empty seat list: the command exits with the existing "no users found" error
  rather than opening an empty TUI.
- Non-TTY (piped) invocation: `budget manage` returns an error
  ("manage requires an interactive terminal"), since `list`/`set` cover
  scripting. Detected with the existing `isTerminal(os.Stdin)`.

## Testing

`internal/tui/budget` is unit-tested with a fake `Store`:

- edit → confirm (`y`) → apply calls `Store.Upsert` with the selected user and
  entered amount; the row reflects the new amount on success.
- confirm with `n`/`esc` cancels and calls nothing.
- `d` on a budgeted row → confirm → calls `Store.Delete`; `d` on a budget-less
  row is a no-op.
- invalid amount input (non-numeric, zero, negative) is rejected and stays in
  editing.
- `s` toggles the SKU and triggers `Store.Reload` with the new SKU.
- a `Store` error surfaces in the model's status field, state returns to
  browsing.

The `cmd` layer stays thin and is exercised through manual/interactive use; no
separate `cmd` test is added.
