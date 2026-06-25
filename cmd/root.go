package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultAPIVersion = "2026-03-10"
	defaultAPIBaseURL = "https://api.github.com"
)

type rootOptions struct {
	Enterprise string
	APIBaseURL string
	APIVersion string
	TokenEnv   string
	UseGHToken bool
	GHUser     string
	GHHostname string
	Timeout    time.Duration
	Format     string
	Color      string
}

var opts = rootOptions{
	APIBaseURL: defaultAPIBaseURL,
	APIVersion: defaultAPIVersion,
	UseGHToken: true,
	Timeout:    30 * time.Second,
	Format:     "table",
	Color:      "auto",
}

var rootCmd = &cobra.Command{
	Use:   "coprem",
	Short: "Fetch GitHub Enterprise Copilot premium request analytics",
	Long: strings.TrimSpace(`
coprem reads GitHub Enterprise billing premium request usage
for Copilot and prints it as table, JSON, or CSV.

Set the target enterprise with --enterprise or COPREM_ENTERPRISE.
Authentication is read from COPREM_TOKEN, GITHUB_TOKEN, GH_TOKEN, COPILOT_PREMIUM_TOKEN, or
from 'gh auth token' when the GitHub CLI is already authenticated.
`),
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&opts.Enterprise, "enterprise", "e", os.Getenv("COPREM_ENTERPRISE"), "GitHub enterprise slug; defaults to COPREM_ENTERPRISE")
	rootCmd.PersistentFlags().StringVar(&opts.APIBaseURL, "api-base-url", opts.APIBaseURL, "GitHub API base URL")
	rootCmd.PersistentFlags().StringVar(&opts.APIVersion, "api-version", opts.APIVersion, "GitHub REST API version")
	rootCmd.PersistentFlags().StringVar(&opts.TokenEnv, "token-env", "", "environment variable that contains the GitHub token")
	rootCmd.PersistentFlags().BoolVar(&opts.UseGHToken, "use-gh-token", opts.UseGHToken, "fall back to 'gh auth token' when no token environment variable is set")
	ghUserDefault := strings.TrimSpace(os.Getenv("COPREM_GH_USER"))
	rootCmd.PersistentFlags().StringVar(&opts.GHUser, "gh-user", ghUserDefault, "GitHub CLI account to read with 'gh auth token --user'; defaults to COPREM_GH_USER")
	rootCmd.PersistentFlags().StringVar(&opts.GHHostname, "gh-hostname", "github.com", "GitHub CLI hostname to read with 'gh auth token --hostname'")
	rootCmd.PersistentFlags().DurationVar(&opts.Timeout, "timeout", opts.Timeout, "HTTP request timeout")
	rootCmd.PersistentFlags().StringVarP(&opts.Format, "format", "f", opts.Format, "output format: table, json, csv")
	rootCmd.PersistentFlags().StringVar(&opts.Color, "color", opts.Color, "table color mode: auto, always, never")
}
