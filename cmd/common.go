package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	githubapi "github.com/bariiss/coprem/internal/github"
)

func resolveGHUser() string {
	if strings.TrimSpace(opts.GHUser) != "" {
		return strings.TrimSpace(opts.GHUser)
	}
	if user := strings.TrimSpace(os.Getenv("COPREM_GH_USER")); user != "" {
		return user
	}
	if user, ok := githubapi.PreferredEnterpriseGHUser(opts.GHHostname); ok {
		return user
	}
	return ""
}

func discoverUsersError(err error) error {
	return fmt.Errorf("discover Copilot seats: %w\n%s", err, githubapi.EnterpriseAuthHint(opts.GHHostname, resolveGHUser()))
}

func requireEnterprise() error {
	if strings.TrimSpace(opts.Enterprise) == "" {
		return errors.New("missing enterprise: pass --enterprise or set COPREM_ENTERPRISE")
	}
	return nil
}

func newGitHubClient() (*githubapi.Client, string, error) {
	ghUser := resolveGHUser()
	token, source, err := githubapi.ResolveToken(githubapi.TokenOptions{
		PreferredEnv: opts.TokenEnv,
		UseGH:        opts.UseGHToken,
		GHUser:       ghUser,
		GHHostname:   opts.GHHostname,
		PreferGH:     githubapi.ShouldPreferGHToken(opts.GHHostname, ghUser),
	})
	if err != nil {
		return nil, "", err
	}
	if token == "" {
		return nil, "", errors.New("missing GitHub token: set COPREM_TOKEN, GITHUB_TOKEN, GH_TOKEN, COPILOT_PREMIUM_TOKEN, pass --token-env, or authenticate gh")
	}
	client := githubapi.NewClient(githubapi.ClientOptions{
		HTTPClient:  &http.Client{Timeout: opts.Timeout},
		BaseURL:     opts.APIBaseURL,
		APIVersion:  opts.APIVersion,
		Token:       token,
		UserAgent:   "coprem",
		TokenSource: source,
	})
	return client, source, nil
}

func discoverUsers(ctx context.Context, client *githubapi.Client, usersCSV, usersFile string) ([]string, error) {
	var users []string
	users = append(users, splitUsers(usersCSV)...)
	if usersFile != "" {
		fromFile, err := readUsersFile(usersFile)
		if err != nil {
			return nil, err
		}
		users = append(users, fromFile...)
	}
	if len(users) == 0 {
		discovered, err := client.CopilotSeatLogins(ctx, opts.Enterprise)
		if err != nil {
			return nil, discoverUsersError(err)
		}
		users = append(users, discovered...)
	}
	return uniqueStrings(users), nil
}

func promptLine(out *os.File, in *bufio.Reader, prompt string) (string, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return "", err
	}
	line, err := in.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func resolveUserFromInput(users []string, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("user is required")
	}
	if n, err := parsePositiveInt(input); err == nil {
		if n < 1 || n > len(users) {
			return "", fmt.Errorf("invalid selection %d; choose 1-%d", n, len(users))
		}
		return users[n-1], nil
	}
	for _, user := range users {
		if strings.EqualFold(user, input) {
			return user, nil
		}
	}
	return "", fmt.Errorf("user %q not found in seat list", input)
}

func parsePositiveInt(value string) (int, error) {
	var n int
	_, err := fmt.Sscanf(value, "%d", &n)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, fmt.Errorf("not positive")
	}
	return n, nil
}

func printNumberedUsers(out *os.File, users []string) {
	for i, user := range users {
		fmt.Fprintf(out, "  %3d  %s\n", i+1, user)
	}
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