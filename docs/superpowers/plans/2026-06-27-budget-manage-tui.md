# Budget Manage TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `coprem budget manage` command that opens a full-screen, keyboard-driven table of Copilot seat users and their AI credit budgets, where the operator navigates to a row, edits the budget inline, and applies it after a `[y/N]` confirmation.

**Architecture:** A Bubble Tea model in `internal/tui/budgettui` holds all interaction state and depends only on a small `Store` interface (no cobra, no `net/http`). The `cmd` layer provides a concrete `Store` adapter over `*githubapi.Client`, fetches the initial rows, and runs the program. After every successful mutation the model reloads rows from the store so consumed amounts and budget IDs stay correct.

**Tech Stack:** Go 1.24, cobra, charmbracelet/bubbletea + bubbles (`table`, `textinput`) + lipgloss.

## Global Constraints

- Module path: `github.com/bariiss/coprem`. New package import: `github.com/bariiss/coprem/internal/tui/budgettui`.
- Go version floor: `go 1.24` (from `go.mod`).
- SKU values come from `internal/github`: `ai_credits` / `premium_requests`. The TUI mirrors them as local `SKUAICredits` / `SKUPremiumRequests` constants with identical string values.
- Budgets are user-scoped with `prevent_further_usage=true` (hard stop) — already handled by `upsertUserBudget` / `NewUserBudgetRequest`. The TUI does not change that.
- Existing commands (`budget list/set/delete/users`) and `budget set --interactive` are left untouched.
- Run all Go commands from repo root `/Users/baris.dogu/src/coprem`.

---

## File Structure

- Create `internal/tui/budgettui/store.go` — `Row`, `Store`, SKU constants, pure helpers (`parseAmount`, `budgetCell`, `consumedCell`, `toggleSKU`, `filterRows`).
- Create `internal/tui/budgettui/model.go` — Bubble Tea `Model`: `New`, `Init`, `Update`, `View`, per-mode handlers, async commands, message types.
- Create `internal/tui/budgettui/model_test.go` — white-box (`package budgettui`) unit tests with a fake `Store`.
- Create `cmd/budget_manage.go` — the `budget manage` cobra command + `Store` adapter + TTY guard + registration.

---

## Task 1: Package types, helpers, and dependencies

**Files:**
- Create: `internal/tui/budgettui/store.go`
- Create: `internal/tui/budgettui/store_test.go`
- Modify: `go.mod`, `go.sum` (via `go get`)

**Interfaces:**
- Produces: `Row` struct; `Store` interface; constants `SKUAICredits`, `SKUPremiumRequests`; helpers `parseAmount(string) (int, error)`, `budgetCell(*int) string`, `consumedCell(*float64) string`, `toggleSKU(string) string`, `filterRows([]Row, string) []Row`.

- [ ] **Step 1: Add dependencies**

```bash
cd /Users/baris.dogu/src/coprem
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
```
Expected: `go.mod` gains the three `require` lines; `go.sum` updated.

- [ ] **Step 2: Write `store.go`**

```go
package budgettui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// SKU values mirror internal/github budget product SKUs.
const (
	SKUAICredits       = "ai_credits"
	SKUPremiumRequests = "premium_requests"
)

// Row is one user line in the budget table.
type Row struct {
	User       string
	HasBudget  bool
	Amount     *int     // nil when the user has no budget
	Consumed   *float64 // nil when the user has no budget
	ProductSKU string
	ID         string // budget id; empty when the user has no budget
}

// Store is the model's only dependency on the outside world.
type Store interface {
	// Upsert creates or updates the user's budget; action is "Created" or "Updated".
	Upsert(ctx context.Context, user string, amount int, sku string) (action string, err error)
	Delete(ctx context.Context, budgetID string) error
	// Reload returns the full row set for the given SKU.
	Reload(ctx context.Context, sku string) ([]Row, error)
}

func parseAmount(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid amount %q; enter a positive whole dollar amount", s)
	}
	return n, nil
}

func budgetCell(amount *int) string {
	if amount == nil {
		return "-"
	}
	return fmt.Sprintf("$%d", *amount)
}

func consumedCell(consumed *float64) string {
	if consumed == nil {
		return "-"
	}
	return fmt.Sprintf("$%.2f", *consumed)
}

func toggleSKU(sku string) string {
	if sku == SKUPremiumRequests {
		return SKUAICredits
	}
	return SKUPremiumRequests
}

func filterRows(all []Row, query string) []Row {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return all
	}
	out := make([]Row, 0, len(all))
	for _, r := range all {
		if strings.Contains(strings.ToLower(r.User), query) {
			out = append(out, r)
		}
	}
	return out
}
```

- [ ] **Step 3: Write `store_test.go`**

```go
package budgettui

import "testing"

func TestParseAmount(t *testing.T) {
	if n, err := parseAmount(" 50 "); err != nil || n != 50 {
		t.Fatalf("got (%d, %v), want (50, nil)", n, err)
	}
	for _, bad := range []string{"", "0", "-3", "abc", "1.5"} {
		if _, err := parseAmount(bad); err == nil {
			t.Errorf("parseAmount(%q) = nil error, want error", bad)
		}
	}
}

func TestToggleSKU(t *testing.T) {
	if got := toggleSKU(SKUAICredits); got != SKUPremiumRequests {
		t.Errorf("toggle ai_credits = %q, want %q", got, SKUPremiumRequests)
	}
	if got := toggleSKU(SKUPremiumRequests); got != SKUAICredits {
		t.Errorf("toggle premium_requests = %q, want %q", got, SKUAICredits)
	}
}

func TestFilterRows(t *testing.T) {
	all := []Row{{User: "alice"}, {User: "bob"}, {User: "ALBERT"}}
	got := filterRows(all, "al")
	if len(got) != 2 {
		t.Fatalf("filter 'al' returned %d rows, want 2", len(got))
	}
	if len(filterRows(all, "")) != 3 {
		t.Error("empty filter should return all rows")
	}
}

func TestCells(t *testing.T) {
	if budgetCell(nil) != "-" {
		t.Error("nil amount should render '-'")
	}
	n := 50
	if budgetCell(&n) != "$50" {
		t.Error("amount should render '$50'")
	}
	if consumedCell(nil) != "-" {
		t.Error("nil consumed should render '-'")
	}
	c := 12.4
	if consumedCell(&c) != "$12.40" {
		t.Error("consumed should render '$12.40'")
	}
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/baris.dogu/src/coprem && go test ./internal/tui/budgettui/`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/tui/budgettui/store.go internal/tui/budgettui/store_test.go
git commit -m "feat: add budgettui store types and helpers"
```

---

## Task 2: Model foundation — browse, filter, SKU toggle, reload

**Files:**
- Create: `internal/tui/budgettui/model.go`
- Create: `internal/tui/budgettui/model_test.go`

**Interfaces:**
- Consumes: `Row`, `Store`, SKU constants, and the helpers from Task 1.
- Produces: `Model` (implements `tea.Model`); `New(ctx context.Context, store Store, sku, enterprise string, rows []Row) Model`; message types `rowsLoadedMsg`, `appliedMsg`, `deletedMsg`; command builders `reloadCmd`, `upsertCmd`, `deleteCmd`; mode/confirm constants. (Editing/confirming/delete key wiring is added in Task 3.)

- [ ] **Step 1: Write the failing test `model_test.go`**

```go
package budgettui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type upsertCall struct {
	user   string
	amount int
	sku    string
}

type fakeStore struct {
	upsertCalls []upsertCall
	deleteCalls []string
	reloadCalls []string
	reloadRows  []Row
	upsertErr   error
	deleteErr   error
}

func (f *fakeStore) Upsert(_ context.Context, user string, amount int, sku string) (string, error) {
	f.upsertCalls = append(f.upsertCalls, upsertCall{user, amount, sku})
	if f.upsertErr != nil {
		return "", f.upsertErr
	}
	return "Updated", nil
}

func (f *fakeStore) Delete(_ context.Context, id string) error {
	f.deleteCalls = append(f.deleteCalls, id)
	return f.deleteErr
}

func (f *fakeStore) Reload(_ context.Context, sku string) ([]Row, error) {
	f.reloadCalls = append(f.reloadCalls, sku)
	return f.reloadRows, nil
}

func newTestModel(store Store, rows []Row) Model {
	return New(context.Background(), store, SKUAICredits, "ent", rows)
}

func step(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
func enter() tea.KeyMsg         { return tea.KeyMsg{Type: tea.KeyEnter} }
func esc() tea.KeyMsg           { return tea.KeyMsg{Type: tea.KeyEsc} }

func TestNewShowsRows(t *testing.T) {
	m := newTestModel(&fakeStore{}, []Row{{User: "alice"}, {User: "bob"}})
	if len(m.rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(m.rows))
	}
	if m.mode != modeBrowsing {
		t.Errorf("mode = %v, want browsing", m.mode)
	}
}

func TestFilter(t *testing.T) {
	m := newTestModel(&fakeStore{}, []Row{{User: "alice"}, {User: "bob"}})
	m, _ = step(m, runes("/")) // enter filter mode
	if m.mode != modeFiltering {
		t.Fatalf("mode = %v, want filtering", m.mode)
	}
	m, _ = step(m, runes("b"))
	if len(m.rows) != 1 || m.rows[0].User != "bob" {
		t.Fatalf("filtered rows = %v, want [bob]", m.rows)
	}
	m, _ = step(m, esc()) // esc clears filter
	if m.mode != modeBrowsing || len(m.rows) != 2 {
		t.Fatalf("after esc: mode=%v rows=%d, want browsing/2", m.mode, len(m.rows))
	}
}

func TestSKUToggleTriggersReload(t *testing.T) {
	fs := &fakeStore{reloadRows: []Row{{User: "carol", ProductSKU: SKUPremiumRequests}}}
	m := newTestModel(fs, []Row{{User: "alice"}})
	m, cmd := step(m, runes("s"))
	if m.sku != SKUPremiumRequests {
		t.Fatalf("sku = %q, want premium_requests", m.sku)
	}
	if cmd == nil {
		t.Fatal("expected reload command")
	}
	msg := cmd()
	loaded, ok := msg.(rowsLoadedMsg)
	if !ok {
		t.Fatalf("msg type = %T, want rowsLoadedMsg", msg)
	}
	if len(fs.reloadCalls) != 1 || fs.reloadCalls[0] != SKUPremiumRequests {
		t.Fatalf("reload calls = %v, want [premium_requests]", fs.reloadCalls)
	}
	m, _ = step(m, loaded) // feed result back
	if len(m.rows) != 1 || m.rows[0].User != "carol" {
		t.Fatalf("rows after reload = %v, want [carol]", m.rows)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/baris.dogu/src/coprem && go test ./internal/tui/budgettui/ -run 'TestNew|TestFilter|TestSKU'`
Expected: FAIL — `undefined: New`, `Model`, `modeBrowsing`, etc.

- [ ] **Step 3: Write `model.go`**

```go
package budgettui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeBrowsing mode = iota
	modeFiltering
	modeEditing
	modeConfirming
	modeApplying
)

type confirmKind int

const (
	confirmSet confirmKind = iota
	confirmDelete
)

type rowsLoadedMsg struct {
	rows []Row
	err  error
}

type appliedMsg struct {
	user   string
	action string
	amount int
	err    error
}

type deletedMsg struct {
	user string
	err  error
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	helpStyle   = lipgloss.NewStyle().Faint(true)
	statusStyle = lipgloss.NewStyle().Bold(true)
)

// Model is the budget manage TUI state. It implements tea.Model.
type Model struct {
	ctx        context.Context
	store      Store
	sku        string
	enterprise string

	allRows []Row
	rows    []Row
	filter  string

	table table.Model
	input textinput.Model

	mode    mode
	confirm confirmKind

	pendingUser   string
	pendingAmount int
	pendingID     string

	status string
}

// New builds a Model seeded with rows already fetched by the caller.
func New(ctx context.Context, store Store, sku, enterprise string, rows []Row) Model {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "USER", Width: 28},
			{Title: "AMOUNT", Width: 10},
			{Title: "CONSUMED", Width: 12},
			{Title: "SKU", Width: 18},
		}),
		table.WithFocused(true),
		table.WithHeight(12),
	)
	st := table.DefaultStyles()
	st.Selected = st.Selected.Bold(true).Reverse(true)
	t.SetStyles(st)

	ti := textinput.New()
	ti.Prompt = ""

	m := Model{
		ctx:        ctx,
		store:      store,
		sku:        sku,
		enterprise: enterprise,
		allRows:    rows,
		rows:       rows,
		table:      t,
		input:      ti,
		mode:       modeBrowsing,
	}
	m.applyFilter()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m *Model) applyFilter() {
	m.rows = filterRows(m.allRows, m.filter)
	tableRows := make([]table.Row, 0, len(m.rows))
	for _, r := range m.rows {
		tableRows = append(tableRows, table.Row{r.User, budgetCell(r.Amount), consumedCell(r.Consumed), r.ProductSKU})
	}
	m.table.SetRows(tableRows)
}

func (m Model) selected() (Row, bool) {
	i := m.table.Cursor()
	if i < 0 || i >= len(m.rows) {
		return Row{}, false
	}
	return m.rows[i], true
}

func (m Model) reloadCmd(sku string) tea.Cmd {
	store, ctx := m.store, m.ctx
	return func() tea.Msg {
		rows, err := store.Reload(ctx, sku)
		return rowsLoadedMsg{rows: rows, err: err}
	}
}

func (m Model) upsertCmd(user string, amount int, sku string) tea.Cmd {
	store, ctx := m.store, m.ctx
	return func() tea.Msg {
		action, err := store.Upsert(ctx, user, amount, sku)
		return appliedMsg{user: user, action: action, amount: amount, err: err}
	}
}

func (m Model) deleteCmd(user, id string) tea.Cmd {
	store, ctx := m.store, m.ctx
	return func() tea.Msg {
		err := store.Delete(ctx, id)
		return deletedMsg{user: user, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.table.SetHeight(max(3, msg.Height-6))
		return m, nil
	case rowsLoadedMsg:
		nm, cmd := m.onRowsLoaded(msg)
		return nm, cmd
	case appliedMsg:
		nm, cmd := m.onApplied(msg)
		return nm, cmd
	case deletedMsg:
		nm, cmd := m.onDeleted(msg)
		return nm, cmd
	case tea.KeyMsg:
		switch m.mode {
		case modeFiltering:
			nm, cmd := m.updateFiltering(msg)
			return nm, cmd
		case modeApplying:
			return m, nil
		default:
			nm, cmd := m.updateBrowsing(msg)
			return nm, cmd
		}
	}
	return m, nil
}

func (m Model) updateBrowsing(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "/":
		m.input.SetValue(m.filter)
		m.input.Focus()
		m.mode = modeFiltering
		return m, nil
	case "s":
		m.sku = toggleSKU(m.sku)
		m.status = "loading…"
		m.mode = modeApplying
		return m, m.reloadCmd(m.sku)
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) updateFiltering(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.input.Blur()
		m.mode = modeBrowsing
		return m, nil
	case tea.KeyEsc:
		m.input.Blur()
		m.filter = ""
		m.applyFilter()
		m.mode = modeBrowsing
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.filter = m.input.Value()
	m.applyFilter()
	return m, cmd
}

func (m Model) onRowsLoaded(msg rowsLoadedMsg) (Model, tea.Cmd) {
	m.mode = modeBrowsing
	if msg.err != nil {
		m.status = msg.err.Error()
		return m, nil
	}
	m.allRows = msg.rows
	m.applyFilter()
	m.status = ""
	return m, nil
}

func (m Model) onApplied(msg appliedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.mode = modeBrowsing
		m.status = msg.err.Error()
		return m, nil
	}
	m.status = fmt.Sprintf("%s budget for %s: $%d/month", msg.action, msg.user, msg.amount)
	return m, m.reloadCmd(m.sku)
}

func (m Model) onDeleted(msg deletedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.mode = modeBrowsing
		m.status = msg.err.Error()
		return m, nil
	}
	m.status = fmt.Sprintf("deleted budget for %s", msg.user)
	return m, m.reloadCmd(m.sku)
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("coprem · %s · %s", m.enterprise, m.sku)))
	b.WriteString("\n")
	b.WriteString(m.table.View())
	b.WriteString("\n")
	switch m.mode {
	case modeFiltering:
		b.WriteString("filter: " + m.input.View())
	case modeEditing:
		b.WriteString(fmt.Sprintf("New monthly budget for %s ($): %s", m.pendingUser, m.input.View()))
	case modeConfirming:
		if m.confirm == confirmDelete {
			b.WriteString(fmt.Sprintf("Delete budget for %s? [y/N]", m.pendingUser))
		} else {
			b.WriteString(fmt.Sprintf("Set $%d/month for %s with hard stop? [y/N]", m.pendingAmount, m.pendingUser))
		}
	case modeApplying:
		b.WriteString("working…")
	default:
		b.WriteString(helpStyle.Render("↑/↓ move · enter edit · d delete · / filter · s toggle sku · q quit"))
	}
	if m.status != "" {
		b.WriteString("\n" + statusStyle.Render(m.status))
	}
	// Reference strconv so editing-mode code in Task 3 keeps the import; harmless here.
	_ = strconv.Itoa
	return b.String()
}
```

> Note: the `_ = strconv.Itoa` line is a placeholder to keep `strconv` imported for Task 3. If you prefer, omit the `strconv` import now and add it in Task 3 — either is fine, but do not leave an unused import.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/baris.dogu/src/coprem && go test ./internal/tui/budgettui/`
Expected: PASS (Task 1 tests + TestNewShowsRows, TestFilter, TestSKUToggleTriggersReload).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/budgettui/model.go internal/tui/budgettui/model_test.go
git commit -m "feat: budgettui model with browse, filter, and sku toggle"
```

---

## Task 3: Mutations — edit, confirm, set, delete, errors

**Files:**
- Modify: `internal/tui/budgettui/model.go` (extend `updateBrowsing`, add `updateEditing`, `updateConfirming`, wire `modeEditing`/`modeConfirming` into `Update`)
- Modify: `internal/tui/budgettui/model_test.go` (add mutation tests)

**Interfaces:**
- Consumes: everything from Task 2 plus `parseAmount` from Task 1.
- Produces: editing/confirming behavior reachable via `enter` (set) and `d` (delete) in browsing.

- [ ] **Step 1: Write the failing tests (append to `model_test.go`)**

```go
func TestEditConfirmApplyUpsert(t *testing.T) {
	fs := &fakeStore{reloadRows: []Row{{User: "alice", HasBudget: true, Amount: ptrInt(50)}}}
	m := newTestModel(fs, []Row{{User: "alice", ProductSKU: SKUAICredits}})

	m, _ = step(m, enter()) // browsing -> editing
	if m.mode != modeEditing {
		t.Fatalf("mode = %v, want editing", m.mode)
	}
	m, _ = step(m, runes("5"))
	m, _ = step(m, runes("0"))
	m, _ = step(m, enter()) // editing -> confirming
	if m.mode != modeConfirming || m.confirm != confirmSet {
		t.Fatalf("mode/confirm = %v/%v, want confirming/set", m.mode, m.confirm)
	}
	m, cmd := step(m, runes("y")) // confirm -> applying
	if m.mode != modeApplying {
		t.Fatalf("mode = %v, want applying", m.mode)
	}
	if cmd == nil {
		t.Fatal("expected upsert command")
	}
	msg := cmd()
	if len(fs.upsertCalls) != 1 || fs.upsertCalls[0] != (upsertCall{"alice", 50, SKUAICredits}) {
		t.Fatalf("upsert calls = %v, want one {alice 50 ai_credits}", fs.upsertCalls)
	}
	m, reload := step(m, msg.(appliedMsg)) // success -> reload
	if reload == nil {
		t.Fatal("expected reload after successful upsert")
	}
	if m.mode != modeApplying { // still applying until reload returns
		t.Fatalf("mode = %v, want applying", m.mode)
	}
	m, _ = step(m, reload().(rowsLoadedMsg))
	if m.mode != modeBrowsing {
		t.Fatalf("mode = %v, want browsing", m.mode)
	}
}

func TestEditCancelDoesNothing(t *testing.T) {
	fs := &fakeStore{}
	m := newTestModel(fs, []Row{{User: "alice"}})
	m, _ = step(m, enter())
	m, _ = step(m, runes("50"))
	m, _ = step(m, enter())       // confirming
	m, _ = step(m, runes("n"))    // anything but y cancels
	if m.mode != modeBrowsing {
		t.Fatalf("mode = %v, want browsing", m.mode)
	}
	if len(fs.upsertCalls) != 0 {
		t.Fatalf("upsert calls = %v, want none", fs.upsertCalls)
	}
}

func TestEditInvalidAmountStaysEditing(t *testing.T) {
	m := newTestModel(&fakeStore{}, []Row{{User: "alice"}})
	m, _ = step(m, enter())
	m, _ = step(m, runes("abc"))
	m, _ = step(m, enter())
	if m.mode != modeEditing {
		t.Fatalf("mode = %v, want editing (invalid amount)", m.mode)
	}
	if m.status == "" {
		t.Error("expected an error status for invalid amount")
	}
}

func TestDeleteFlow(t *testing.T) {
	fs := &fakeStore{reloadRows: []Row{{User: "alice"}}}
	m := newTestModel(fs, []Row{{User: "alice", HasBudget: true, Amount: ptrInt(30), ID: "bgt_1"}})
	m, _ = step(m, runes("d")) // browsing -> confirming(delete)
	if m.mode != modeConfirming || m.confirm != confirmDelete {
		t.Fatalf("mode/confirm = %v/%v, want confirming/delete", m.mode, m.confirm)
	}
	m, cmd := step(m, runes("y"))
	if cmd == nil {
		t.Fatal("expected delete command")
	}
	msg := cmd()
	if len(fs.deleteCalls) != 1 || fs.deleteCalls[0] != "bgt_1" {
		t.Fatalf("delete calls = %v, want [bgt_1]", fs.deleteCalls)
	}
	m, reload := step(m, msg.(deletedMsg))
	if reload == nil {
		t.Fatal("expected reload after delete")
	}
}

func TestDeleteOnRowWithoutBudgetIsNoop(t *testing.T) {
	fs := &fakeStore{}
	m := newTestModel(fs, []Row{{User: "alice", HasBudget: false}})
	m, _ = step(m, runes("d"))
	if m.mode != modeBrowsing {
		t.Fatalf("mode = %v, want browsing", m.mode)
	}
	if len(fs.deleteCalls) != 0 {
		t.Fatalf("delete calls = %v, want none", fs.deleteCalls)
	}
}

func TestUpsertErrorSurfaces(t *testing.T) {
	fs := &fakeStore{upsertErr: errFake}
	m := newTestModel(fs, []Row{{User: "alice"}})
	m, _ = step(m, enter())
	m, _ = step(m, runes("50"))
	m, _ = step(m, enter())
	_, cmd := step(m, runes("y"))
	applied := cmd().(appliedMsg)
	m, reload := step(m, applied)
	if m.mode != modeBrowsing {
		t.Fatalf("mode = %v, want browsing after error", m.mode)
	}
	if reload != nil {
		t.Error("no reload should run after a failed upsert")
	}
	if m.status == "" {
		t.Error("expected error status")
	}
}

func ptrInt(n int) *int { return &n }

var errFake = fmtError("boom")

type fmtError string

func (e fmtError) Error() string { return string(e) }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/baris.dogu/src/coprem && go test ./internal/tui/budgettui/ -run 'TestEdit|TestDelete|TestUpsertError'`
Expected: FAIL — `enter` does not enter editing; `d` does not confirm; no `modeEditing`/`modeConfirming` dispatch.

- [ ] **Step 3: Extend `updateBrowsing` (add `enter` and `d` cases before the table fallthrough)**

In `model.go`, replace the `updateBrowsing` switch body so it includes the new cases (full function shown):

```go
func (m Model) updateBrowsing(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "/":
		m.input.SetValue(m.filter)
		m.input.Focus()
		m.mode = modeFiltering
		return m, nil
	case "s":
		m.sku = toggleSKU(m.sku)
		m.status = "loading…"
		m.mode = modeApplying
		return m, m.reloadCmd(m.sku)
	case "enter":
		r, ok := m.selected()
		if !ok {
			return m, nil
		}
		m.pendingUser = r.User
		if r.HasBudget && r.Amount != nil {
			m.input.SetValue(strconv.Itoa(*r.Amount))
		} else {
			m.input.SetValue("")
		}
		m.input.Focus()
		m.status = ""
		m.mode = modeEditing
		return m, nil
	case "d":
		r, ok := m.selected()
		if !ok || !r.HasBudget {
			m.status = "no budget to delete"
			return m, nil
		}
		m.pendingUser = r.User
		m.pendingID = r.ID
		m.confirm = confirmDelete
		m.mode = modeConfirming
		return m, nil
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}
```

- [ ] **Step 4: Add `updateEditing` and `updateConfirming` (new functions in `model.go`)**

```go
func (m Model) updateEditing(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.input.Blur()
		m.status = ""
		m.mode = modeBrowsing
		return m, nil
	case tea.KeyEnter:
		amount, err := parseAmount(m.input.Value())
		if err != nil {
			m.status = err.Error()
			return m, nil // stay in editing
		}
		m.pendingAmount = amount
		m.confirm = confirmSet
		m.status = ""
		m.mode = modeConfirming
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateConfirming(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && strings.EqualFold(string(msg.Runes), "y") {
		m.input.Blur()
		m.mode = modeApplying
		if m.confirm == confirmDelete {
			return m, m.deleteCmd(m.pendingUser, m.pendingID)
		}
		return m, m.upsertCmd(m.pendingUser, m.pendingAmount, m.sku)
	}
	m.input.Blur()
	m.status = "cancelled"
	m.mode = modeBrowsing
	return m, nil
}
```

- [ ] **Step 5: Wire `modeEditing` and `modeConfirming` into `Update`**

In `model.go`, in the `tea.KeyMsg` branch of `Update`, replace the inner `switch m.mode` so it routes editing and confirming:

```go
	case tea.KeyMsg:
		switch m.mode {
		case modeFiltering:
			nm, cmd := m.updateFiltering(msg)
			return nm, cmd
		case modeEditing:
			nm, cmd := m.updateEditing(msg)
			return nm, cmd
		case modeConfirming:
			nm, cmd := m.updateConfirming(msg)
			return nm, cmd
		case modeApplying:
			return m, nil
		default:
			nm, cmd := m.updateBrowsing(msg)
			return nm, cmd
		}
```

If you used the `_ = strconv.Itoa` placeholder in Task 2, remove it now (the real `strconv.Itoa` call in `updateBrowsing` keeps the import live).

- [ ] **Step 6: Run all package tests to verify they pass**

Run: `cd /Users/baris.dogu/src/coprem && go test ./internal/tui/budgettui/`
Expected: PASS (all Task 1–3 tests).

- [ ] **Step 7: Commit**

```bash
git add internal/tui/budgettui/model.go internal/tui/budgettui/model_test.go
git commit -m "feat: budgettui edit, delete, and confirmation flows"
```

---

## Task 4: `budget manage` command and Store adapter

**Files:**
- Create: `cmd/budget_manage.go`
- Modify: `cmd/budget.go:91-110` (register `budgetManageCmd` and its `--product-sku` flag in `init`)

**Interfaces:**
- Consumes: `budgettui.New`, `budgettui.Row`, `budgettui.Store`, SKU constants; existing `discoverUsers`, `mergeUserBudgetRows`, `filterBudgetsBySKU`, `upsertUserBudget`, `requireEnterprise`, `validateBudgetProductSKU`, `newGitHubClient`, `isTerminal`; `githubapi.Client` methods `ListBudgets`, `DeleteBudget`.
- Produces: `budgetManageCmd` registered under `budgetCmd`.

- [ ] **Step 1: Write `cmd/budget_manage.go`**

```go
package cmd

import (
	"context"
	"errors"
	"os"

	githubapi "github.com/bariiss/coprem/internal/github"
	budgettui "github.com/bariiss/coprem/internal/tui/budgettui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var budgetManageCmd = &cobra.Command{
	Use:   "manage",
	Short: "Interactively view and edit per-user budgets in a table UI",
	RunE:  runBudgetManage,
}

// budgetStore adapts *githubapi.Client to budgettui.Store.
type budgetStore struct {
	client *githubapi.Client
}

func (s budgetStore) Upsert(ctx context.Context, user string, amount int, sku string) (string, error) {
	_, action, err := upsertUserBudget(ctx, s.client, user, amount, sku)
	return action, err
}

func (s budgetStore) Delete(ctx context.Context, budgetID string) error {
	return s.client.DeleteBudget(ctx, opts.Enterprise, budgetID)
}

func (s budgetStore) Reload(ctx context.Context, sku string) ([]budgettui.Row, error) {
	users, err := discoverUsers(ctx, s.client, "", "")
	if err != nil {
		return nil, err
	}
	budgets, err := s.client.ListBudgets(ctx, opts.Enterprise, githubapi.BudgetListQuery{
		Scope: githubapi.BudgetScopeUser,
	})
	if err != nil {
		return nil, err
	}
	budgets = filterBudgetsBySKU(budgets, sku)
	rows := mergeUserBudgetRows(users, budgets, sku)
	return toTUIRows(rows), nil
}

func toTUIRows(rows []userBudgetRow) []budgettui.Row {
	out := make([]budgettui.Row, 0, len(rows))
	for _, r := range rows {
		out = append(out, budgettui.Row{
			User:       r.User,
			HasBudget:  r.HasBudget,
			Amount:     r.Amount,
			Consumed:   r.Consumed,
			ProductSKU: r.ProductSKU,
			ID:         r.ID,
		})
	}
	return out
}

func runBudgetManage(cmd *cobra.Command, args []string) error {
	if err := requireEnterprise(); err != nil {
		return err
	}
	if err := validateBudgetProductSKU(budgetOpts.ProductSKU); err != nil {
		return err
	}
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		return errors.New("budget manage requires an interactive terminal; use 'budget list' or 'budget set' for scripting")
	}

	client, _, err := newGitHubClient()
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	store := budgetStore{client: client}
	rows, err := store.Reload(ctx, budgetOpts.ProductSKU)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return errors.New("no users found")
	}

	model := budgettui.New(ctx, store, budgetOpts.ProductSKU, opts.Enterprise, rows)
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err = program.Run()
	return err
}
```

- [ ] **Step 2: Register the command in `cmd/budget.go` `init()`**

In `func init()`, change the `AddCommand` line and add the flag. Replace:

```go
	budgetCmd.AddCommand(budgetUsersCmd, budgetListCmd, budgetSetCmd, budgetDeleteCmd)
```
with:
```go
	budgetCmd.AddCommand(budgetUsersCmd, budgetListCmd, budgetSetCmd, budgetDeleteCmd, budgetManageCmd)
```
and, after the `budgetDeleteCmd` flag block near the end of `init()`, add:
```go
	budgetManageCmd.Flags().StringVar(&budgetOpts.ProductSKU, "product-sku", githubapi.BudgetProductAICredits, "starting product SKU: ai_credits or premium_requests")
```

- [ ] **Step 3: Build and vet**

Run: `cd /Users/baris.dogu/src/coprem && go build ./... && go vet ./...`
Expected: no output (success).

- [ ] **Step 4: Confirm the command is registered and help renders**

Run: `cd /Users/baris.dogu/src/coprem && go run . budget manage --help`
Expected: usage text for `manage` with the `--product-sku` flag listed. (Running `budget manage` itself needs a real terminal + credentials; `--help` verifies wiring without them.)

- [ ] **Step 5: Run the full test suite**

Run: `cd /Users/baris.dogu/src/coprem && go test ./...`
Expected: PASS across all packages.

- [ ] **Step 6: Commit**

```bash
git add cmd/budget_manage.go cmd/budget.go
git commit -m "feat: add 'coprem budget manage' interactive TUI command"
```

---

## Manual verification (after Task 4)

With a real terminal and valid credentials/enterprise:

```bash
go run . budget manage --enterprise <slug>
```
Check: table lists users; ↑/↓ moves; `/` filters; `enter` opens amount input; entering an amount and `y` applies and the row refreshes; `d` on a budgeted row deletes after `y`; `s` toggles SKU and reloads; `q` quits. API errors appear in the status line without crashing.

---

## Self-Review notes

- **Spec coverage:** full-screen table (Task 2 `View`), navigate rows (Task 2 table), inline edit (Task 3 editing), confirmation prompt (Task 3 confirming), delete (Task 3), live filter (Task 2), SKU toggle (Task 2), async non-blocking apply + reload (Task 2/3 cmds + msgs), error-to-status (Task 3), non-TTY guard + empty-seat handling + new command (Task 4), `Store` isolation + adapter (Task 1 interface, Task 4 adapter), unit tests with fake store (Tasks 1–3). All spec sections map to a task.
- **Type consistency:** `Row`, `Store`, `Upsert/Delete/Reload`, `New(ctx, store, sku, enterprise, rows)`, message types, and `confirmSet/confirmDelete` are used identically across tasks.
