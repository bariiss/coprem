package budgettui

import (
	"context"
	"fmt"
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
	return b.String()
}
