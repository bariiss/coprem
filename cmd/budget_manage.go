package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	githubapi "github.com/bariiss/coprem/internal/github"
	"github.com/bariiss/coprem/internal/output"
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
	client    *githubapi.Client
	netByUser map[string]float64 // additional usage billed per user this period
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
	return s.toTUIRows(rows), nil
}

func (s budgetStore) toTUIRows(rows []userBudgetRow) []budgettui.Row {
	out := make([]budgettui.Row, 0, len(rows))
	for _, r := range rows {
		row := budgettui.Row{
			User:       r.User,
			HasBudget:  r.HasBudget,
			Amount:     r.Amount,
			Consumed:   r.Consumed,
			ProductSKU: r.ProductSKU,
			ID:         r.ID,
		}
		if net, ok := s.netByUser[r.User]; ok {
			row.Net = &net
		}
		out = append(out, row)
	}
	return out
}

// fetchNetByUser returns the additional usage billed (NET) per user for the
// current month. The premium usage endpoint only attributes line items to a
// user when queried with a user filter, so this fetches per user (same as
// `premium --group-by user`) and aggregates NET by user.
func fetchNetByUser(ctx context.Context, client *githubapi.Client) (map[string]float64, error) {
	p, err := resolvePeriod(time.Now(), premiumOptions{Timeframe: "current-month"})
	if err != nil {
		return nil, err
	}
	report, err := fetchUserGrouped(ctx, client, p, premiumOptions{
		Timeframe:   "current-month",
		GroupBy:     "user",
		Breakdown:   "total",
		Granularity: "cumulative",
	})
	if err != nil {
		return nil, err
	}
	grouped := output.GroupReport(report, "user", "total")
	net := make(map[string]float64, len(grouped.Rows))
	for _, row := range grouped.Rows {
		net[row.Key] = row.NetAmount
	}
	return net, nil
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
	netByUser, err := fetchNetByUser(ctx, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load NET usage; NET column will be empty: %v\n", err)
	}
	store.netByUser = netByUser

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
