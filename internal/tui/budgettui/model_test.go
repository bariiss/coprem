package budgettui

import (
	"context"
	"strings"
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

func TestNetColumnRendered(t *testing.T) {
	net := 403.78
	m := newTestModel(&fakeStore{}, []Row{{User: "alice", Net: &net}})
	view := m.View()
	if !strings.Contains(view, "NET") {
		t.Errorf("view missing NET column header; got:\n%s", view)
	}
	if !strings.Contains(view, "$403.78") {
		t.Errorf("view missing NET value $403.78; got:\n%s", view)
	}
}

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
	m, _ = step(m, enter())    // confirming
	m, _ = step(m, runes("n")) // anything but y cancels
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
