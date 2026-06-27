package github

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// EnterpriseAuthHint returns a formatted helper message with CLI command suggestions
// for setting up enterprise billing token authentication.
func EnterpriseAuthHint(hostname, ghUser string) string {
	var b strings.Builder
	b.WriteString("enterprise billing APIs need a token with admin:enterprise scope.\n")

	if ghUser != "" {
		fmt.Fprintf(&b, "  coprem ... --gh-user %s\n", ghUser)
		b.WriteString("  gh auth switch   # select that account\n")
		fmt.Fprintf(&b, "  gh auth refresh -h %s -s admin:enterprise\n", hostname)
		b.WriteString("if COPREM_TOKEN is set from another account, unset it or replace it with the enterprise admin token.")
		return b.String()
	}

	if accounts := GHAccountsWithEnterpriseScope(hostname); len(accounts) > 0 {
		b.WriteString("logged-in gh accounts with admin:enterprise:\n")
		for _, account := range accounts {
			fmt.Fprintf(&b, "  coprem ... --gh-user %s\n", account)
		}
		b.WriteString("or:\n")
		b.WriteString("  gh auth switch   # pick an enterprise admin account\n")
		fmt.Fprintf(&b, "  gh auth refresh -h %s -s admin:enterprise\n", hostname)
		b.WriteString("or set COPREM_GH_USER / COPREM_TOKEN for that account.\n")
		b.WriteString("if COPREM_TOKEN is set from a personal account, unset it so gh can be used.")
		return b.String()
	}

	b.WriteString("  gh auth switch   # pick an enterprise admin account\n")
	fmt.Fprintf(&b, "  gh auth refresh -h %s -s admin:enterprise\n", hostname)
	b.WriteString("or pass --users/--users-file instead of auto-discovery.")
	return b.String()
}

// PreferredEnterpriseGHUser searches authenticated gh CLI profiles to find one
// with enterprise scope when the active account does not have it.
func PreferredEnterpriseGHUser(hostname string) (string, bool) {
	accounts, err := parseGHAuthAccounts(hostname)
	if err != nil || len(accounts) == 0 {
		return "", false
	}
	return preferredEnterpriseGHUserFromAccounts(accounts)
}

// GHAccountsWithEnterpriseScope returns the usernames of all authenticated GitHub
// CLI accounts that possess the admin:enterprise scope.
func GHAccountsWithEnterpriseScope(hostname string) []string {
	accounts, err := parseGHAuthAccounts(hostname)
	if err != nil {
		return nil
	}
	var logins []string
	for _, account := range accounts {
		if account.HasEnterpriseScope {
			logins = append(logins, account.Login)
		}
	}
	return logins
}

type ghAccount struct {
	Login                string
	Active               bool
	HasEnterpriseScope   bool
}

// parseGHAuthAccounts executes 'gh auth status' to retrieve the list of authenticated
// accounts for the given hostname.
func parseGHAuthAccounts(hostname string) ([]ghAccount, error) {
	args := []string{"auth", "status"}
	if hostname != "" {
		args = append(args, "--hostname", hostname)
	}
	out, err := exec.CommandContext(context.Background(), "gh", args...).CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("gh auth status: %w", err)
	}
	return parseGHAuthAccountsFromOutput(string(out), hostname)
}

// parseGHAuthAccountsFromOutput parses the raw output of 'gh auth status' into a slice of ghAccount structs.
func parseGHAuthAccountsFromOutput(output, hostname string) ([]ghAccount, error) {
	host := hostname
	if host == "" {
		host = "github.com"
	}
	prefix := "Logged in to " + host + " account "

	var accounts []ghAccount
	var current *ghAccount
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "✓ ")
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			login := strings.TrimSuffix(strings.TrimPrefix(line, prefix), " (keyring)")
			login = strings.TrimSpace(login)
			accounts = append(accounts, ghAccount{Login: login})
			current = &accounts[len(accounts)-1]
			continue
		}
		if current == nil {
			continue
		}
		if strings.Contains(line, "Active account: true") {
			current.Active = true
		}
		if strings.Contains(line, "Token scopes:") && strings.Contains(line, "admin:enterprise") {
			current.HasEnterpriseScope = true
		}
	}
	return accounts, nil
}

// ActiveGHAccountHasEnterpriseScope checks if the active GitHub CLI profile for the hostname has the admin:enterprise scope.
func ActiveGHAccountHasEnterpriseScope(hostname string) bool {
	accounts, err := parseGHAuthAccounts(hostname)
	if err != nil {
		return false
	}
	for _, account := range accounts {
		if account.Active && account.HasEnterpriseScope {
			return true
		}
	}
	return false
}

// ShouldPreferGHToken determines if coprem should use the gh CLI token helper rather than falling back to environment variables.
func ShouldPreferGHToken(hostname, ghUser string) bool {
	if ghUser != "" {
		return true
	}
	return ActiveGHAccountHasEnterpriseScope(hostname)
}

// preferredEnterpriseGHUserFromAccounts checks if there is exactly one account with enterprise scope that is not currently active.
func preferredEnterpriseGHUserFromAccounts(accounts []ghAccount) (string, bool) {
	var active *ghAccount
	var enterprise []ghAccount
	for i := range accounts {
		account := accounts[i]
		if account.Active {
			active = &account
		}
		if account.HasEnterpriseScope {
			enterprise = append(enterprise, account)
		}
	}
	if active != nil && active.HasEnterpriseScope {
		return "", false
	}
	if len(enterprise) == 1 {
		return enterprise[0].Login, true
	}
	return "", false
}
