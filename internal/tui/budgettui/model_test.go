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
