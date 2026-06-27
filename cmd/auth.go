package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	githubapi "github.com/bariiss/coprem/internal/github"
	"github.com/spf13/cobra"
)

type authOptions struct {
	EnvName string
	Zshrc   string
	GHUser  string
}

var authOpts = authOptions{
	EnvName: "COPREM_TOKEN",
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Helpers for local token setup",
}

var saveZshrcCmd = &cobra.Command{
	Use:   "save-zshrc",
	Short: "Save the current gh token into ~/.zshrc as an export",
	Long: strings.TrimSpace(`
Reads the token from the configured environment variable or from 'gh auth token'
and writes a managed export block to ~/.zshrc.

This is convenient for local usage, but it stores the token as plaintext in the
shell startup file. Prefer GitHub CLI auth when possible.
`),
	RunE: runSaveZshrc,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(saveZshrcCmd)

	saveZshrcCmd.Flags().StringVar(&authOpts.EnvName, "env", authOpts.EnvName, "environment variable name to write")
	saveZshrcCmd.Flags().StringVar(&authOpts.Zshrc, "zshrc", "", "path to zshrc file")
	saveZshrcCmd.Flags().StringVar(&authOpts.GHUser, "gh-user", "", "GitHub CLI account to read with 'gh auth token --user'")
}

func runSaveZshrc(cmd *cobra.Command, _ []string) error {
	ghUser := authOpts.GHUser
	if ghUser == "" {
		ghUser = opts.GHUser
	}
	token, source, err := githubapi.ResolveToken(githubapi.TokenOptions{
		PreferredEnv: authOpts.EnvName,
		UseGH:        true,
		GHUser:       ghUser,
		GHHostname:   opts.GHHostname,
		PreferGH:     true,
	})
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("no token resolved for %s", authOpts.EnvName)
	}

	zshrc := authOpts.Zshrc
	if zshrc == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		zshrc = filepath.Join(home, ".zshrc")
	}

	content, err := os.ReadFile(zshrc) //nolint:gosec // CLI manages the user configuration file
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	block := managedTokenBlock(authOpts.EnvName, token)
	updated := replaceManagedBlock(string(content), block)
	if err := os.WriteFile(zshrc, []byte(updated), 0o600); err != nil { //nolint:gosec // CLI manages the user configuration file
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved %s from %s to %s\n", authOpts.EnvName, source, zshrc)
	return nil
}

func managedTokenBlock(envName, token string) string {
	escaped := strings.ReplaceAll(token, "'", "'\\''")
	return fmt.Sprintf("# BEGIN coprem\nexport %s='%s'\n# END coprem\n", envName, escaped)
}

func replaceManagedBlock(content, block string) string {
	const begin = "# BEGIN coprem"
	const end = "# END coprem"
	const legacyBegin = "# BEGIN copilot-premium-observer"
	const legacyEnd = "# END copilot-premium-observer"

	start := strings.Index(content, begin)
	stop := strings.Index(content, end)
	if start < 0 || stop < start {
		start = strings.Index(content, legacyBegin)
		stop = strings.Index(content, legacyEnd)
	}
	if start >= 0 && stop >= start {
		stop += len(endMarkerAt(content, stop, end, legacyEnd))
		for stop < len(content) && (content[stop] == '\n' || content[stop] == '\r') {
			stop++
		}
		prefix := strings.TrimRight(content[:start], "\r\n")
		suffix := strings.TrimLeft(content[stop:], "\r\n")
		if suffix == "" {
			return prefix + "\n" + block
		}
		return prefix + "\n" + block + "\n" + suffix
	}
	if strings.TrimSpace(content) == "" {
		return block
	}
	return strings.TrimRight(content, "\r\n") + "\n\n" + block
}

func endMarkerAt(content string, index int, markers ...string) string {
	for _, marker := range markers {
		if strings.HasPrefix(content[index:], marker) {
			return marker
		}
	}
	return markers[0]
}
