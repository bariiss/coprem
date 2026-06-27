package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	githubapi "github.com/bariiss/coprem/internal/github"
	"github.com/spf13/cobra"
)

type budgetOptions struct {
	User        string
	Users       string
	UsersFile   string
	Amount      int
	ProductSKU  string
	Interactive bool
	All         bool
	BudgetsOnly bool
	BudgetID    string
	Yes         bool
}

type userBudgetRow struct {
	User       string   `json:"user"`
	HasBudget  bool     `json:"hasBudget"`
	Amount     *int     `json:"amount,omitempty"`
	Consumed   *float64 `json:"consumed,omitempty"`
	ProductSKU string   `json:"productSku,omitempty"`
	ID         string   `json:"id,omitempty"`
}

var budgetOpts = budgetOptions{
	ProductSKU: githubapi.BudgetProductAICredits,
}

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Manage per-user AI credit budgets",
	Long: strings.TrimSpace(`
Manage GitHub Enterprise user-level AI credit budgets via:
  GET/POST/PATCH/DELETE /enterprises/{enterprise}/settings/billing/budgets

User-scoped budgets require prevent_further_usage=true and act as a hard stop
when the monthly limit is reached.
`),
}

var budgetUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "List Copilot seat users for budget assignment",
	RunE:  runBudgetUsers,
}

var budgetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List user-scoped budgets",
	RunE:  runBudgetList,
}

var budgetSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Create or update a per-user budget",
	Long: strings.TrimSpace(`
Set a monthly AI credits budget for one or more users.

Examples:
  coprem budget set --user alice --amount 50
  coprem budget set --users alice,bob --amount 30
  coprem budget set --all --amount 50 --yes
  coprem budget set --users-file team.txt --amount 50
  coprem budget set --interactive
`),
	RunE: runBudgetSet,
}

var budgetDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a budget by ID",
	RunE:  runBudgetDelete,
}

func init() {
	rootCmd.AddCommand(budgetCmd)
	budgetCmd.AddCommand(budgetUsersCmd, budgetListCmd, budgetSetCmd, budgetDeleteCmd, budgetManageCmd)

	budgetListCmd.Flags().StringVar(&budgetOpts.User, "user", "", "filter budgets for a specific user login")
	budgetListCmd.Flags().StringVar(&budgetOpts.ProductSKU, "product-sku", budgetOpts.ProductSKU, "filter by product SKU: ai_credits or premium_requests")
	budgetListCmd.Flags().BoolVar(&budgetOpts.BudgetsOnly, "budgets-only", false, "show only users who already have a budget")

	budgetSetCmd.Flags().StringVar(&budgetOpts.User, "user", "", "GitHub user login")
	budgetSetCmd.Flags().StringVar(&budgetOpts.Users, "users", "", "comma-separated user logins")
	budgetSetCmd.Flags().StringVar(&budgetOpts.UsersFile, "users-file", "", "file with one user login per line")
	budgetSetCmd.Flags().BoolVar(&budgetOpts.All, "all", false, "apply to all Copilot seat users")
	budgetSetCmd.Flags().IntVar(&budgetOpts.Amount, "amount", 0, "monthly budget amount in whole dollars")
	budgetSetCmd.Flags().StringVar(&budgetOpts.ProductSKU, "product-sku", budgetOpts.ProductSKU, "budget product SKU: ai_credits or premium_requests")
	budgetSetCmd.Flags().BoolVar(&budgetOpts.Interactive, "interactive", false, "pick a user from the Copilot seat list interactively")
	budgetSetCmd.Flags().BoolVarP(&budgetOpts.Yes, "yes", "y", false, "skip confirmation prompts")

	budgetDeleteCmd.Flags().StringVar(&budgetOpts.BudgetID, "id", "", "budget ID to delete")
	_ = budgetDeleteCmd.MarkFlagRequired("id")

	budgetManageCmd.Flags().StringVar(&budgetOpts.ProductSKU, "product-sku", githubapi.BudgetProductAICredits, "starting product SKU: ai_credits or premium_requests")
}

func runBudgetUsers(cmd *cobra.Command, args []string) error {
	if err := requireEnterprise(); err != nil {
		return err
	}
	client, _, err := newGitHubClient()
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	users, err := discoverUsers(ctx, client, budgetOpts.Users, budgetOpts.UsersFile)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return errors.New("no users found")
	}

	switch opts.Format {
	case "json":
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
			"enterprise": opts.Enterprise,
			"users":      users,
		})
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Copilot seats (%d users):\n\n", len(users))
		printNumberedUsers(os.Stdout, users)
		fmt.Fprintln(cmd.OutOrStdout(), "\nUse with: coprem budget set --user LOGIN --amount 50")
		return nil
	}
}

func runBudgetList(cmd *cobra.Command, args []string) error {
	if err := requireEnterprise(); err != nil {
		return err
	}
	client, _, err := newGitHubClient()
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	budgets, err := client.ListBudgets(ctx, opts.Enterprise, githubapi.BudgetListQuery{
		Scope: githubapi.BudgetScopeUser,
		User:  budgetOpts.User,
	})
	if err != nil {
		return err
	}
	budgets = filterBudgetsBySKU(budgets, budgetOpts.ProductSKU)

	var rows []userBudgetRow
	if budgetOpts.BudgetsOnly {
		rows = budgetRowsFromBudgets(budgets)
	} else {
		users, err := discoverUsers(ctx, client, "", "")
		if err != nil {
			return err
		}
		if budgetOpts.User != "" {
			users = []string{strings.TrimSpace(budgetOpts.User)}
		}
		rows = mergeUserBudgetRows(users, budgets, budgetOpts.ProductSKU)
	}

	switch opts.Format {
	case "json":
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
			"enterprise": opts.Enterprise,
			"rows":       rows,
		})
	case "csv":
		fmt.Fprintln(cmd.OutOrStdout(), "user,has_budget,amount,consumed,product_sku,id")
		for _, row := range rows {
			fmt.Fprintf(cmd.OutOrStdout(), "%s,%t,%s,%s,%s,%s\n",
				row.User, row.HasBudget, formatAmount(row.Amount), formatConsumed(row.Consumed), row.ProductSKU, row.ID)
		}
		return nil
	default:
		if len(rows) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No users found.")
			return nil
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "USER\tAMOUNT\tCONSUMED\tSKU\tID")
		for _, row := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				row.User, formatAmount(row.Amount), formatConsumed(row.Consumed), emptyDash(row.ProductSKU), emptyDash(row.ID))
		}
		return w.Flush()
	}
}

func runBudgetSet(cmd *cobra.Command, args []string) error {
	if err := requireEnterprise(); err != nil {
		return err
	}
	if err := validateBudgetProductSKU(budgetOpts.ProductSKU); err != nil {
		return err
	}
	client, _, err := newGitHubClient()
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	if budgetOpts.Interactive {
		return runBudgetSetInteractive(cmd, ctx, client)
	}

	users, err := resolveBudgetSetUsers(ctx, client)
	if err != nil {
		return err
	}
	if budgetOpts.Amount <= 0 {
		return errors.New("--amount is required and must be a positive whole dollar amount")
	}

	if !budgetOpts.Yes {
		if budgetOpts.All || len(users) > 1 {
			if err := confirmBulkBudgetSet(cmd, len(users), budgetOpts.Amount); err != nil {
				return err
			}
		} else if err := confirmBudgetSet(cmd, users[0], budgetOpts.Amount); err != nil {
			return err
		}
	}

	for _, user := range users {
		budget, action, err := upsertUserBudget(ctx, client, user, budgetOpts.Amount, budgetOpts.ProductSKU)
		if err != nil {
			return fmt.Errorf("user %s: %w", user, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s budget for %s: $%d/month (id: %s, sku: %s)\n",
			action, user, budget.BudgetAmount, budget.ID, budget.BudgetProductSKU)
	}
	return nil
}

func runBudgetSetInteractive(cmd *cobra.Command, ctx context.Context, client *githubapi.Client) error {
	users, err := discoverUsers(ctx, client, budgetOpts.Users, budgetOpts.UsersFile)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return errors.New("no users found for interactive selection")
	}

	in := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "Select a user (%d Copilot seats):\n\n", len(users))
	printNumberedUsers(os.Stdout, users)
	fmt.Fprintln(out)

	selection, err := promptLine(os.Stdout, in, "User number or login: ")
	if err != nil {
		return err
	}
	user, err := resolveUserFromInput(users, selection)
	if err != nil {
		return err
	}

	amountInput, err := promptLine(os.Stdout, in, "Monthly budget amount ($): ")
	if err != nil {
		return err
	}
	amount, err := strconv.Atoi(strings.TrimSpace(amountInput))
	if err != nil || amount <= 0 {
		return fmt.Errorf("invalid amount %q; enter a positive whole dollar amount", amountInput)
	}

	existing, found, err := client.UserAICreditsBudget(ctx, opts.Enterprise, user, budgetOpts.ProductSKU)
	if err != nil {
		return err
	}
	if found {
		fmt.Fprintf(out, "\nExisting budget for %s: $%d/month (consumed: $%.2f, id: %s)\n",
			user, existing.BudgetAmount, existing.ConsumedAmount, existing.ID)
	}

	if !budgetOpts.Yes {
		confirm, err := promptLine(os.Stdout, in, fmt.Sprintf("Set $%d/month for %s with hard stop? [y/N]: ", amount, user))
		if err != nil {
			return err
		}
		if !strings.EqualFold(confirm, "y") && !strings.EqualFold(confirm, "yes") {
			return errors.New("cancelled")
		}
	}

	budget, action, err := upsertUserBudget(ctx, client, user, amount, budgetOpts.ProductSKU)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%s budget for %s: $%d/month (id: %s)\n", action, user, budget.BudgetAmount, budget.ID)
	return nil
}

func runBudgetDelete(cmd *cobra.Command, args []string) error {
	if err := requireEnterprise(); err != nil {
		return err
	}
	client, _, err := newGitHubClient()
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := client.DeleteBudget(ctx, opts.Enterprise, budgetOpts.BudgetID); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "deleted budget %s\n", budgetOpts.BudgetID)
	return nil
}

func resolveBudgetSetUsers(ctx context.Context, client *githubapi.Client) ([]string, error) {
	if budgetOpts.All {
		if budgetOpts.User != "" || budgetOpts.Users != "" || budgetOpts.UsersFile != "" {
			return nil, errors.New("--all cannot be combined with --user, --users, or --users-file")
		}
		return discoverUsers(ctx, client, "", "")
	}

	var users []string
	if budgetOpts.User != "" {
		users = append(users, strings.TrimSpace(budgetOpts.User))
	}
	users = append(users, splitUsers(budgetOpts.Users)...)
	if budgetOpts.UsersFile != "" {
		fromFile, err := readUsersFile(budgetOpts.UsersFile)
		if err != nil {
			return nil, err
		}
		users = append(users, fromFile...)
	}
	users = uniqueStrings(users)
	if len(users) == 0 {
		return nil, errors.New("specify at least one user with --user, --users, --users-file, or --interactive")
	}

	if budgetOpts.UsersFile == "" && budgetOpts.Users == "" {
		return users, nil
	}

	known, err := discoverUsers(ctx, client, "", "")
	if err != nil {
		return users, nil
	}
	knownSet := map[string]bool{}
	for _, login := range known {
		knownSet[strings.ToLower(login)] = true
	}
	for _, user := range users {
		if !knownSet[strings.ToLower(user)] {
			fmt.Fprintf(os.Stderr, "warning: %s not found in Copilot seat list\n", user)
		}
	}
	return users, nil
}

func upsertUserBudget(ctx context.Context, client *githubapi.Client, user string, amount int, productSKU string) (githubapi.Budget, string, error) {
	existing, found, err := client.UserAICreditsBudget(ctx, opts.Enterprise, user, productSKU)
	if err != nil {
		return githubapi.Budget{}, "", err
	}
	if found {
		updated, err := client.UpdateBudget(ctx, opts.Enterprise, existing.ID, githubapi.UpdateBudgetRequest{
			BudgetAmount: &amount,
		})
		if err != nil {
			return githubapi.Budget{}, "", err
		}
		return updated, "Updated", nil
	}

	created, err := client.CreateBudget(ctx, opts.Enterprise, githubapi.NewUserBudgetRequest(user, amount, productSKU))
	if err != nil {
		return githubapi.Budget{}, "", err
	}
	return created, "Created", nil
}

func confirmBudgetSet(cmd *cobra.Command, user string, amount int) error {
	if !isTerminal(os.Stdin) {
		return nil
	}
	in := bufio.NewReader(cmd.InOrStdin())
	confirm, err := promptLine(os.Stdout, in, fmt.Sprintf("Set $%d/month for %s with hard stop? [y/N]: ", amount, user))
	if err != nil {
		return err
	}
	if !strings.EqualFold(confirm, "y") && !strings.EqualFold(confirm, "yes") {
		return errors.New("cancelled")
	}
	return nil
}

func confirmBulkBudgetSet(cmd *cobra.Command, count int, amount int) error {
	if !isTerminal(os.Stdin) {
		return nil
	}
	in := bufio.NewReader(cmd.InOrStdin())
	confirm, err := promptLine(os.Stdout, in, fmt.Sprintf("Set $%d/month for %d users with hard stop? [y/N]: ", amount, count))
	if err != nil {
		return err
	}
	if !strings.EqualFold(confirm, "y") && !strings.EqualFold(confirm, "yes") {
		return errors.New("cancelled")
	}
	return nil
}

func filterBudgetsBySKU(budgets []githubapi.Budget, productSKU string) []githubapi.Budget {
	if productSKU == "" {
		return budgets
	}
	filtered := budgets[:0]
	for _, budget := range budgets {
		if budget.BudgetProductSKU == productSKU {
			filtered = append(filtered, budget)
		}
	}
	return filtered
}

func budgetRowsFromBudgets(budgets []githubapi.Budget) []userBudgetRow {
	rows := make([]userBudgetRow, 0, len(budgets))
	for _, budget := range budgets {
		rows = append(rows, budgetToRow(budget))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].User < rows[j].User })
	return rows
}

func mergeUserBudgetRows(users []string, budgets []githubapi.Budget, productSKU string) []userBudgetRow {
	byUser := map[string]githubapi.Budget{}
	for _, budget := range budgets {
		if budget.User != "" {
			byUser[budget.User] = budget
		}
	}

	seen := map[string]bool{}
	rows := make([]userBudgetRow, 0, len(users)+len(byUser))
	for _, user := range users {
		seen[user] = true
		if budget, ok := byUser[user]; ok {
			rows = append(rows, budgetToRow(budget))
			continue
		}
		rows = append(rows, userBudgetRow{
			User:       user,
			HasBudget:  false,
			ProductSKU: productSKU,
		})
	}
	for user, budget := range byUser {
		if seen[user] {
			continue
		}
		rows = append(rows, budgetToRow(budget))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].User < rows[j].User })
	return rows
}

func budgetToRow(budget githubapi.Budget) userBudgetRow {
	consumed := budget.ConsumedAmount
	if budget.EffectiveBudget != nil {
		consumed = budget.EffectiveBudget.ConsumedAmount
	}
	amount := budget.BudgetAmount
	return userBudgetRow{
		User:       budget.User,
		HasBudget:  true,
		Amount:     &amount,
		Consumed:   &consumed,
		ProductSKU: budget.BudgetProductSKU,
		ID:         budget.ID,
	}
}

func formatAmount(amount *int) string {
	if amount == nil {
		return "-"
	}
	return fmt.Sprintf("$%d", *amount)
}

func formatConsumed(consumed *float64) string {
	if consumed == nil {
		return "-"
	}
	return fmt.Sprintf("$%.2f", *consumed)
}

func validateBudgetProductSKU(sku string) error {
	switch sku {
	case githubapi.BudgetProductAICredits, githubapi.BudgetProductPremiumReqs:
		return nil
	default:
		return fmt.Errorf("unsupported product SKU %q; use ai_credits or premium_requests", sku)
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
