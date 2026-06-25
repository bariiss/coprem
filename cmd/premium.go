package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	githubapi "github.com/bariiss/coprem/internal/github"
	"github.com/bariiss/coprem/internal/output"
	"github.com/spf13/cobra"
)

type premiumOptions struct {
	Year         int
	Month        int
	Day          int
	From         string
	To           string
	Timeframe    string
	Organization string
	User         string
	Users        string
	UsersFile    string
	Model        string
	Product      string
	CostCenterID string
	GroupBy      string
	Breakdown    string
	Granularity  string
	SortBy       string
}

var premiumOpts = premiumOptions{
	Timeframe:   "current-month",
	GroupBy:     "none",
	Breakdown:   "total",
	Granularity: "cumulative",
	SortBy:      "net-amount",
}

var premiumCmd = &cobra.Command{
	Use:   "premium",
	Short: "Fetch Copilot premium request analytics",
	Long: strings.TrimSpace(`
Fetches GitHub Enterprise premium request usage from:
GET /enterprises/{enterprise}/settings/billing/usage

Examples:
  coprem premium --enterprise ENTERPRISE_SLUG
  coprem premium --timeframe last-month --group-by model
  coprem premium --year 2026 --month 4 --granularity daily --format csv
  coprem premium --organization ORG_LOGIN --user octocat --model gpt-5
`),
	RunE: runPremium,
}

func init() {
	rootCmd.AddCommand(premiumCmd)

	premiumCmd.Flags().IntVar(&premiumOpts.Year, "year", 0, "year to query, for example 2026")
	premiumCmd.Flags().IntVar(&premiumOpts.Month, "month", 0, "month to query, 1-12")
	premiumCmd.Flags().IntVar(&premiumOpts.Day, "day", 0, "day to query, 1-31")
	premiumCmd.Flags().StringVar(&premiumOpts.From, "from", "", "custom start date in YYYY-MM-DD")
	premiumCmd.Flags().StringVar(&premiumOpts.To, "to", "", "custom end date in YYYY-MM-DD")
	premiumCmd.Flags().StringVar(&premiumOpts.Timeframe, "timeframe", premiumOpts.Timeframe, "timeframe: current-month, last-month, this-year, last-year, custom")
	premiumCmd.Flags().StringVar(&premiumOpts.Organization, "organization", "", "filter by organization name")
	premiumCmd.Flags().StringVar(&premiumOpts.User, "user", "", "filter by user login")
	premiumCmd.Flags().StringVar(&premiumOpts.Users, "users", "", "comma-separated user logins for --group-by user")
	premiumCmd.Flags().StringVar(&premiumOpts.UsersFile, "users-file", "", "file containing one user login per line for --group-by user")
	premiumCmd.Flags().StringVar(&premiumOpts.Model, "model", "", "filter by model name")
	premiumCmd.Flags().StringVar(&premiumOpts.Product, "product", "", "filter by product name")
	premiumCmd.Flags().StringVar(&premiumOpts.CostCenterID, "cost-center-id", "", "filter by cost center id; use 'none' for unassigned")
	premiumCmd.Flags().StringVar(&premiumOpts.GroupBy, "group-by", premiumOpts.GroupBy, "client-side grouping: none, model, user, product, organization, cost-center")
	premiumCmd.Flags().StringVar(&premiumOpts.Breakdown, "breakdown", premiumOpts.Breakdown, "secondary breakdown for grouped output: total or model")
	premiumCmd.Flags().StringVar(&premiumOpts.Granularity, "granularity", premiumOpts.Granularity, "granularity: cumulative or daily")
	premiumCmd.Flags().StringVar(&premiumOpts.SortBy, "sort-by", premiumOpts.SortBy, "table sort: net-amount, net-quantity, gross-amount, gross-quantity, key, date")
}

func runPremium(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(opts.Enterprise) == "" {
		return errors.New("missing enterprise: pass --enterprise or set COPREM_ENTERPRISE")
	}
	if err := validatePremiumOptions(premiumOpts); err != nil {
		return err
	}

	ghUser := resolveGHUser()
	token, source, err := githubapi.ResolveToken(githubapi.TokenOptions{
		PreferredEnv: opts.TokenEnv,
		UseGH:        opts.UseGHToken,
		GHUser:       ghUser,
		GHHostname:   opts.GHHostname,
		PreferGH:     githubapi.ShouldPreferGHToken(opts.GHHostname, ghUser),
	})
	if err != nil {
		return err
	}
	if token == "" {
		return errors.New("missing GitHub token: set COPREM_TOKEN, GITHUB_TOKEN, GH_TOKEN, COPILOT_PREMIUM_TOKEN, pass --token-env, or authenticate gh")
	}

	client := githubapi.NewClient(githubapi.ClientOptions{
		HTTPClient:  &http.Client{Timeout: opts.Timeout},
		BaseURL:     opts.APIBaseURL,
		APIVersion:  opts.APIVersion,
		Token:       token,
		UserAgent:   "coprem",
		TokenSource: source,
	})

	now := time.Now()
	period, err := resolvePeriod(now, premiumOpts)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	var result output.Report
	switch premiumOpts.Granularity {
	case "cumulative":
		result, err = fetchCumulative(ctx, client, period, premiumOpts)
	case "daily":
		result, err = fetchDaily(ctx, client, period, premiumOpts)
	default:
		err = fmt.Errorf("unsupported granularity %q", premiumOpts.Granularity)
	}
	if err != nil {
		return err
	}

	grouped := output.GroupReport(result, premiumOpts.GroupBy, premiumOpts.Breakdown)
	output.SortRows(grouped.Rows, premiumOpts.SortBy)

	switch opts.Format {
	case "table":
		color, err := output.ResolveColor(os.Stdout, opts.Color)
		if err != nil {
			return err
		}
		return output.WriteTable(os.Stdout, grouped, output.TableOptions{Color: color})
	case "json":
		return output.WriteJSON(os.Stdout, grouped)
	case "csv":
		return output.WriteCSV(os.Stdout, grouped)
	default:
		return fmt.Errorf("unsupported output format %q; use table, json, or csv", opts.Format)
	}
}

func validatePremiumOptions(o premiumOptions) error {
	validGroupBy := map[string]bool{"none": true, "model": true, "user": true, "product": true, "organization": true, "cost-center": true}
	if !validGroupBy[o.GroupBy] {
		return fmt.Errorf("unsupported group-by %q", o.GroupBy)
	}
	if o.Granularity != "cumulative" && o.Granularity != "daily" {
		return fmt.Errorf("unsupported granularity %q", o.Granularity)
	}
	if o.Breakdown != "total" && o.Breakdown != "model" {
		return fmt.Errorf("unsupported breakdown %q", o.Breakdown)
	}
	if o.Timeframe == "custom" && (o.From == "" || o.To == "") && o.Year == 0 {
		return errors.New("--timeframe custom requires --from/--to or explicit --year/--month/--day")
	}
	return nil
}

type period struct {
	Start       time.Time
	End         time.Time
	Year        int
	Month       int
	Day         int
	Label       string
	CustomRange bool
}

func resolvePeriod(now time.Time, o premiumOptions) (period, error) {
	loc := now.Location()
	if o.Year > 0 {
		if o.Month < 0 || o.Month > 12 {
			return period{}, fmt.Errorf("invalid month %d", o.Month)
		}
		if o.Day < 0 || o.Day > 31 {
			return period{}, fmt.Errorf("invalid day %d", o.Day)
		}
		if o.Day > 0 && o.Month == 0 {
			return period{}, errors.New("--day requires --month")
		}
		start := time.Date(o.Year, 1, 1, 0, 0, 0, 0, loc)
		end := time.Date(o.Year, 12, 31, 0, 0, 0, 0, loc)
		if o.Month > 0 {
			start = time.Date(o.Year, time.Month(o.Month), 1, 0, 0, 0, 0, loc)
			end = start.AddDate(0, 1, -1)
		}
		if o.Day > 0 {
			start = time.Date(o.Year, time.Month(o.Month), o.Day, 0, 0, 0, 0, loc)
			end = start
		}
		return period{Start: start, End: end, Year: o.Year, Month: o.Month, Day: o.Day, Label: periodLabel(start, end)}, nil
	}

	if o.From != "" || o.To != "" {
		if o.From == "" || o.To == "" {
			return period{}, errors.New("--from and --to must be used together")
		}
		start, err := time.ParseInLocation("2006-01-02", o.From, loc)
		if err != nil {
			return period{}, fmt.Errorf("parse --from: %w", err)
		}
		end, err := time.ParseInLocation("2006-01-02", o.To, loc)
		if err != nil {
			return period{}, fmt.Errorf("parse --to: %w", err)
		}
		if end.Before(start) {
			return period{}, errors.New("--to must be on or after --from")
		}
		return period{Start: start, End: end, Year: start.Year(), Month: int(start.Month()), Label: periodLabel(start, end), CustomRange: true}, nil
	}

	today := dateOnly(now)
	switch o.Timeframe {
	case "current-month":
		start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
		return period{Start: start, End: today, Year: start.Year(), Month: int(start.Month()), Label: periodLabel(start, today)}, nil
	case "last-month":
		start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, -1, 0)
		end := start.AddDate(0, 1, -1)
		return period{Start: start, End: end, Year: start.Year(), Month: int(start.Month()), Label: periodLabel(start, end)}, nil
	case "this-year":
		start := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, loc)
		return period{Start: start, End: today, Year: start.Year(), Label: periodLabel(start, today)}, nil
	case "last-year":
		start := time.Date(today.Year()-1, 1, 1, 0, 0, 0, 0, loc)
		end := time.Date(today.Year()-1, 12, 31, 0, 0, 0, 0, loc)
		return period{Start: start, End: end, Year: start.Year(), Label: periodLabel(start, end)}, nil
	default:
		return period{}, fmt.Errorf("unsupported timeframe %q", o.Timeframe)
	}
}

func fetchCumulative(ctx context.Context, client *githubapi.Client, p period, o premiumOptions) (output.Report, error) {
	if o.GroupBy == "user" && o.User == "" {
		return fetchUserGrouped(ctx, client, p, o)
	}
	if p.CustomRange {
		report, err := fetchDaily(ctx, client, p, o)
		if err != nil {
			return output.Report{}, err
		}
		for i := range report.Rows {
			report.Rows[i].Date = p.Label
		}
		return report, nil
	}

	query := premiumQuery(o)
	query.Year = p.Year
	query.Month = p.Month
	query.Day = p.Day
	if p.Start.Year() != p.End.Year() || p.Start.Month() != p.End.Month() {
		query.Month = 0
		query.Day = 0
	}

	resp, err := client.PremiumRequestUsage(ctx, opts.Enterprise, query)
	if err != nil {
		return output.Report{}, err
	}
	rows := output.RowsFromUsageItems(p.Label, resp.UsageItems, query.User)
	return output.Report{
		Enterprise: opts.Enterprise,
		Period:     p.Label,
		Source:     resp,
		Rows:       rows,
	}, nil
}

func fetchDaily(ctx context.Context, client *githubapi.Client, p period, o premiumOptions) (output.Report, error) {
	if o.GroupBy == "user" && o.User == "" {
		return fetchUserGrouped(ctx, client, p, o)
	}
	var rows []output.Row
	var raw []githubapi.PremiumUsageResponse
	for day := p.Start; !day.After(p.End); day = day.AddDate(0, 0, 1) {
		query := premiumQuery(o)
		query.Year = day.Year()
		query.Month = int(day.Month())
		query.Day = day.Day()
		resp, err := client.PremiumRequestUsage(ctx, opts.Enterprise, query)
		if err != nil {
			return output.Report{}, fmt.Errorf("%s: %w", day.Format("2006-01-02"), err)
		}
		raw = append(raw, resp)
		rows = append(rows, output.RowsFromUsageItems(day.Format("2006-01-02"), resp.UsageItems, query.User)...)
	}
	return output.Report{
		Enterprise: opts.Enterprise,
		Period:     p.Label,
		Source:     raw,
		Rows:       rows,
	}, nil
}

func fetchUserGrouped(ctx context.Context, client *githubapi.Client, p period, o premiumOptions) (output.Report, error) {
	users, err := resolveUsers(ctx, client, o)
	if err != nil {
		return output.Report{}, err
	}
	if len(users) == 0 {
		return output.Report{}, errors.New("no users found for user grouping")
	}

	var rows []output.Row
	var raw []githubapi.PremiumUsageResponse
	for _, user := range users {
		userOpts := o
		userOpts.User = user
		userOpts.GroupBy = "none"

		var report output.Report
		var err error
		if o.Granularity == "daily" {
			report, err = fetchDaily(ctx, client, p, userOpts)
		} else {
			report, err = fetchCumulative(ctx, client, p, userOpts)
		}
		if err != nil {
			return output.Report{}, fmt.Errorf("user %s: %w", user, err)
		}
		rows = append(rows, report.Rows...)
		if report.Source != nil {
			if source, ok := report.Source.(githubapi.PremiumUsageResponse); ok {
				raw = append(raw, source)
			}
		}
	}

	return output.Report{
		Enterprise: opts.Enterprise,
		Period:     p.Label,
		Source:     raw,
		Rows:       rows,
	}, nil
}

func resolveUsers(ctx context.Context, client *githubapi.Client, o premiumOptions) ([]string, error) {
	var users []string
	users = append(users, splitUsers(o.Users)...)
	if o.UsersFile != "" {
		fromFile, err := readUsersFile(o.UsersFile)
		if err != nil {
			return nil, err
		}
		users = append(users, fromFile...)
	}
	if len(users) == 0 {
		discovered, err := client.CopilotSeatLogins(ctx, opts.Enterprise)
		if err != nil {
			return nil, fmt.Errorf("discover Copilot seats for --group-by user: %w\n%s", err, githubapi.EnterpriseAuthHint(opts.GHHostname, resolveGHUser()))
		}
		users = append(users, discovered...)
	}
	return uniqueStrings(users), nil
}

func readUsersFile(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var users []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		users = append(users, splitUsers(line)...)
	}
	return users, nil
}

func splitUsers(value string) []string {
	var users []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			users = append(users, part)
		}
	}
	return users
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var unique []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

func premiumQuery(o premiumOptions) githubapi.PremiumUsageQuery {
	return githubapi.PremiumUsageQuery{
		Organization: o.Organization,
		User:         o.User,
		Model:        o.Model,
		Product:      o.Product,
		CostCenterID: o.CostCenterID,
	}
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func periodLabel(start, end time.Time) string {
	if start.Equal(end) {
		return start.Format("2006-01-02")
	}
	return start.Format("2006-01-02") + ".." + end.Format("2006-01-02")
}

func init() {
	sort.Strings(output.SortKeys)
}
