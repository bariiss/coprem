package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const (
	BudgetScopeUser          = "user"
	BudgetTypeBundlePricing  = "BundlePricing"
	BudgetProductAICredits   = "ai_credits"
	BudgetProductPremiumReqs = "premium_requests"
)

type BudgetAlerting struct {
	WillAlert       bool     `json:"will_alert"`
	AlertRecipients []string `json:"alert_recipients"`
}

type Budget struct {
	ID                  string          `json:"id"`
	BudgetType          string          `json:"budget_type"`
	BudgetAmount        int             `json:"budget_amount"`
	PreventFurtherUsage bool            `json:"prevent_further_usage"`
	BudgetScope         string          `json:"budget_scope"`
	BudgetEntityName    string          `json:"budget_entity_name,omitempty"`
	User                string          `json:"user,omitempty"`
	BudgetProductSKU    string          `json:"budget_product_sku"`
	BudgetAlerting      BudgetAlerting  `json:"budget_alerting"`
	ConsumedAmount      float64         `json:"consumed_amount,omitempty"`
	EffectiveBudget     *EffectiveBudget `json:"effective_budget,omitempty"`
}

type EffectiveBudget struct {
	ID             string  `json:"id"`
	BudgetAmount   int     `json:"budget_amount"`
	ConsumedAmount float64 `json:"consumed_amount"`
}

type BudgetsListResponse struct {
	Budgets     []Budget `json:"budgets"`
	HasNextPage bool     `json:"has_next_page,omitempty"`
	TotalCount  int      `json:"total_count,omitempty"`
}

type CreateBudgetRequest struct {
	BudgetAmount        int            `json:"budget_amount"`
	PreventFurtherUsage bool           `json:"prevent_further_usage"`
	BudgetScope         string         `json:"budget_scope"`
	BudgetEntityName    string         `json:"budget_entity_name"`
	BudgetType          string         `json:"budget_type"`
	BudgetProductSKU    string         `json:"budget_product_sku"`
	BudgetAlerting      BudgetAlerting `json:"budget_alerting"`
	User                string         `json:"user,omitempty"`
}

type UpdateBudgetRequest struct {
	BudgetAmount        *int            `json:"budget_amount,omitempty"`
	PreventFurtherUsage *bool           `json:"prevent_further_usage,omitempty"`
	BudgetAlerting      *BudgetAlerting `json:"budget_alerting,omitempty"`
}

type BudgetMutationResponse struct {
	Message string `json:"message"`
	Budget  Budget `json:"budget"`
}

type BudgetDeleteResponse struct {
	Message string `json:"message"`
	ID      string `json:"id"`
}

type BudgetListQuery struct {
	Scope   string
	User    string
	PerPage int
}

func (c *Client) ListBudgets(ctx context.Context, enterprise string, query BudgetListQuery) ([]Budget, error) {
	path := fmt.Sprintf("/enterprises/%s/settings/billing/budgets", url.PathEscape(enterprise))
	values := url.Values{}
	addString(values, "scope", query.Scope)
	addString(values, "user", query.User)
	perPage := query.PerPage
	if perPage <= 0 {
		perPage = 100
	}
	values.Set("per_page", strconv.Itoa(perPage))

	var budgets []Budget
	page := 1
	for {
		values.Set("page", strconv.Itoa(page))
		var out BudgetsListResponse
		next, err := c.getWithNext(ctx, path, values, &out)
		if err != nil {
			return nil, err
		}
		budgets = append(budgets, out.Budgets...)
		if next == "" && !out.HasNextPage {
			break
		}
		if next == "" && len(out.Budgets) == 0 {
			break
		}
		if page >= maxPaginationPages {
			break
		}
		page++
	}
	return budgets, nil
}

func (c *Client) CreateBudget(ctx context.Context, enterprise string, req CreateBudgetRequest) (Budget, error) {
	path := fmt.Sprintf("/enterprises/%s/settings/billing/budgets", url.PathEscape(enterprise))
	var out BudgetMutationResponse
	if err := c.doJSON(ctx, http.MethodPost, path, nil, req, &out); err != nil {
		return Budget{}, err
	}
	return out.Budget, nil
}

func (c *Client) UpdateBudget(ctx context.Context, enterprise, budgetID string, req UpdateBudgetRequest) (Budget, error) {
	path := fmt.Sprintf("/enterprises/%s/settings/billing/budgets/%s", url.PathEscape(enterprise), url.PathEscape(budgetID))
	var out BudgetMutationResponse
	if err := c.doJSON(ctx, http.MethodPatch, path, nil, req, &out); err != nil {
		return Budget{}, err
	}
	return out.Budget, nil
}

func (c *Client) DeleteBudget(ctx context.Context, enterprise, budgetID string) error {
	path := fmt.Sprintf("/enterprises/%s/settings/billing/budgets/%s", url.PathEscape(enterprise), url.PathEscape(budgetID))
	var out BudgetDeleteResponse
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, &out)
}

func (c *Client) UserAICreditsBudget(ctx context.Context, enterprise, user, productSKU string) (Budget, bool, error) {
	budgets, err := c.ListBudgets(ctx, enterprise, BudgetListQuery{
		Scope:   BudgetScopeUser,
		User:    user,
		PerPage: 100,
	})
	if err != nil {
		return Budget{}, false, err
	}
	for _, budget := range budgets {
		if budget.BudgetScope != BudgetScopeUser {
			continue
		}
		if budget.User != "" && budget.User != user {
			continue
		}
		if productSKU != "" && budget.BudgetProductSKU != productSKU {
			continue
		}
		return budget, true, nil
	}
	return Budget{}, false, nil
}

func NewUserBudgetRequest(user string, amount int, productSKU string) CreateBudgetRequest {
	if productSKU == "" {
		productSKU = BudgetProductAICredits
	}
	return CreateBudgetRequest{
		BudgetAmount:        amount,
		PreventFurtherUsage: true,
		BudgetScope:         BudgetScopeUser,
		BudgetEntityName:    "",
		BudgetType:          BudgetTypeBundlePricing,
		BudgetProductSKU:    productSKU,
		BudgetAlerting: BudgetAlerting{
			WillAlert:       false,
			AlertRecipients: []string{},
		},
		User: user,
	}
}
