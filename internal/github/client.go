package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type ClientOptions struct {
	HTTPClient  *http.Client
	BaseURL     string
	APIVersion  string
	Token       string
	UserAgent   string
	TokenSource string
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiVersion string
	token      string
	userAgent  string
}

type PremiumUsageQuery struct {
	Year         int
	Month        int
	Day          int
	Organization string
	User         string
	Model        string
	Product      string
	CostCenterID string
}

type TokenOptions struct {
	PreferredEnv string
	UseGH        bool
	GHUser       string
	GHHostname   string
	PreferGH     bool
}

type PremiumUsageResponse struct {
	TimePeriod map[string]any `json:"timePeriod,omitempty"`
	Enterprise string         `json:"enterprise,omitempty"`
	UsageItems []UsageItem    `json:"usageItems"`
}

type CopilotSeatsResponse struct {
	TotalSeats int           `json:"total_seats"`
	Seats      []CopilotSeat `json:"seats"`
}

type CopilotSeat struct {
	Assignee struct {
		Login string `json:"login"`
	} `json:"assignee"`
}

type UsageItem struct {
	Date             string         `json:"date,omitempty"`
	Product          string         `json:"product,omitempty"`
	SKU              string         `json:"sku,omitempty"`
	Model            string         `json:"model,omitempty"`
	User             string         `json:"user,omitempty"`
	Username         string         `json:"username,omitempty"`
	Organization     string         `json:"organization,omitempty"`
	OrganizationName string         `json:"organizationName,omitempty"`
	CostCenterID     string         `json:"costCenterId,omitempty"`
	CostCenterName   string         `json:"costCenterName,omitempty"`
	UnitType         string         `json:"unitType,omitempty"`
	PricePerUnit     float64        `json:"pricePerUnit,omitempty"`
	GrossQuantity    float64        `json:"grossQuantity,omitempty"`
	GrossAmount      float64        `json:"grossAmount,omitempty"`
	DiscountQuantity float64        `json:"discountQuantity,omitempty"`
	DiscountAmount   float64        `json:"discountAmount,omitempty"`
	NetQuantity      float64        `json:"netQuantity,omitempty"`
	NetAmount        float64        `json:"netAmount,omitempty"`
	Raw              map[string]any `json:"-"`
}

func (u *UsageItem) UnmarshalJSON(data []byte) error {
	type alias UsageItem
	var item alias
	if err := json.Unmarshal(data, &item); err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = UsageItem(item)
	u.Raw = raw
	return nil
}

func NewClient(o ClientOptions) *Client {
	httpClient := o.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	baseURL := strings.TrimRight(o.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	userAgent := o.UserAgent
	if userAgent == "" {
		userAgent = "coprem"
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		apiVersion: o.APIVersion,
		token:      o.Token,
		userAgent:  userAgent,
	}
}

func (c *Client) PremiumRequestUsage(ctx context.Context, enterprise string, query PremiumUsageQuery) (PremiumUsageResponse, error) {
	path := fmt.Sprintf("/enterprises/%s/settings/billing/usage", url.PathEscape(enterprise))
	values := url.Values{}
	addInt(values, "year", query.Year)
	addInt(values, "month", query.Month)
	addInt(values, "day", query.Day)
	addString(values, "organization", query.Organization)
	addString(values, "user", query.User)
	addString(values, "model", query.Model)
	addString(values, "product", query.Product)
	addString(values, "cost_center_id", query.CostCenterID)

	var out PremiumUsageResponse
	if err := c.get(ctx, path, values, &out); err != nil {
		return PremiumUsageResponse{}, err
	}
	return out, nil
}

func (c *Client) CopilotSeatLogins(ctx context.Context, enterprise string) ([]string, error) {
	path := fmt.Sprintf("/enterprises/%s/copilot/billing/seats", url.PathEscape(enterprise))
	values := url.Values{}
	values.Set("per_page", "100")

	seen := map[string]bool{}
	var logins []string
	page := 1
	for {
		values.Set("page", strconv.Itoa(page))
		var out CopilotSeatsResponse
		next, err := c.getWithNext(ctx, path, values, &out)
		if err != nil {
			return nil, err
		}
		for _, seat := range out.Seats {
			login := strings.TrimSpace(seat.Assignee.Login)
			if login == "" || seen[login] {
				continue
			}
			seen[login] = true
			logins = append(logins, login)
		}
		if next == "" {
			break
		}
		page++
	}
	return logins, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	_, err := c.getWithNext(ctx, path, query, out)
	return err
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	endpoint := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/json")
	if c.apiVersion != "" {
		req.Header.Set("X-GitHub-Api-Version", c.apiVersion)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return formatAPIError(resp.StatusCode, respBody)
	}
	if out == nil || len(strings.TrimSpace(string(respBody))) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}

func (c *Client) getWithNext(ctx context.Context, path string, query url.Values, out any) (string, error) {
	endpoint := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", c.userAgent)
	if c.apiVersion != "" {
		req.Header.Set("X-GitHub-Api-Version", c.apiVersion)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", formatAPIError(resp.StatusCode, body)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nextLink(resp.Header.Get("Link")), nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return "", fmt.Errorf("decode GitHub response: %w", err)
	}
	return nextLink(resp.Header.Get("Link")), nil
}

func formatAPIError(status int, body []byte) error {
	var payload struct {
		Message string `json:"message"`
		DocURL  string `json:"documentation_url"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Message != "" {
		if payload.DocURL != "" {
			return fmt.Errorf("github api error %d: %s (%s)", status, payload.Message, payload.DocURL)
		}
		return fmt.Errorf("github api error %d: %s", status, payload.Message)
	}
	return fmt.Errorf("github api error %d: %s", status, strings.TrimSpace(string(body)))
}

func ResolveToken(options TokenOptions) (token string, source string, err error) {
	if options.UseGH && (options.PreferGH || options.GHUser != "") {
		token, err = tokenFromGH(options.GHHostname, options.GHUser)
		if err == nil && token != "" {
			source = "gh auth token"
			if options.GHUser != "" {
				source += " --user " + options.GHUser
			}
			return token, source, nil
		}
		if err != nil && !errors.Is(err, exec.ErrNotFound) {
			return "", "", err
		}
	}

	envs := []string{}
	if options.PreferredEnv != "" {
		envs = append(envs, options.PreferredEnv)
	}
	envs = append(envs, "COPREM_TOKEN", "GITHUB_TOKEN", "GH_TOKEN", "COPILOT_PREMIUM_TOKEN")
	seen := map[string]bool{}
	for _, env := range envs {
		if env == "" || seen[env] {
			continue
		}
		seen[env] = true
		if token := strings.TrimSpace(os.Getenv(env)); token != "" {
			return token, env, nil
		}
	}
	if !options.UseGH {
		return "", "", nil
	}
	token, err = tokenFromGH(options.GHHostname, options.GHUser)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", "", nil
		}
		return "", "", err
	}
	source = "gh auth token"
	if options.GHUser != "" {
		source += " --user " + options.GHUser
	}
	return token, source, nil
}

func tokenFromGH(hostname, user string) (string, error) {
	args := []string{"auth", "token"}
	if hostname != "" {
		args = append(args, "--hostname", hostname)
	}
	if user != "" {
		args = append(args, "--user", user)
	}
	cmd := exec.Command("gh", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func addInt(values url.Values, name string, value int) {
	if value > 0 {
		values.Set(name, fmt.Sprintf("%d", value))
	}
}

func addString(values url.Values, name string, value string) {
	if strings.TrimSpace(value) != "" {
		values.Set(name, value)
	}
}

func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		sections := strings.Split(part, ";")
		if len(sections) < 2 {
			continue
		}
		if strings.TrimSpace(sections[1]) != `rel="next"` {
			continue
		}
		return strings.Trim(strings.TrimSpace(sections[0]), "<>")
	}
	return ""
}
