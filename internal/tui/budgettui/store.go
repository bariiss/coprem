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
