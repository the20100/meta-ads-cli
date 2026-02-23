package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vincentmaurin/meta-ads-cli/internal/api"
	"github.com/vincentmaurin/meta-ads-cli/internal/config"
)

var (
	// Persistent flags
	accountFlag string
	jsonFlag    bool
	prettyFlag  bool

	// Global API client, set in PersistentPreRunE
	client *api.Client

	// Global config, set in PersistentPreRunE
	cfg *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "meta-ads",
	Short: "Meta Ads CLI — manage your Meta advertising campaigns",
	Long: `meta-ads is a CLI tool for the Meta Ads API.

It outputs JSON when piped (for agent use) and human-readable tables in a terminal.

Examples:
  meta-ads auth login
  meta-ads accounts list
  meta-ads campaigns list --account act_123456789
  meta-ads insights get --account act_123456789 --level campaign --since 2026-01-01 --until 2026-01-31

Set META_ADS_ACCOUNT to avoid passing --account on every command.`,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&accountFlag, "account", "a", "", "Ad account ID (act_ prefix optional). Overrides META_ADS_ACCOUNT env var.")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Force JSON output")
	rootCmd.PersistentFlags().BoolVar(&prettyFlag, "pretty", false, "Force pretty-printed JSON output (implies --json)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Skip auth check for auth subcommands
		if isAuthCommand(cmd) {
			return nil
		}

		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		if cfg.AccessToken == "" {
			return fmt.Errorf("not logged in — run: meta-ads auth login")
		}

		appSecret := cfg.AppSecret
		if appSecret == "" {
			appSecret = os.Getenv("META_APP_SECRET")
		}

		client = api.NewClient(cfg.AccessToken, appSecret)
		return nil
	}
}

// isAuthCommand returns true if cmd is a child of the "auth" command.
func isAuthCommand(cmd *cobra.Command) bool {
	if cmd.Name() == "auth" {
		return true
	}
	p := cmd.Parent()
	for p != nil {
		if p.Name() == "auth" {
			return true
		}
		p = p.Parent()
	}
	return false
}

// resolveAccount returns the account ID to use for a command.
// Priority: --account flag > META_ADS_ACCOUNT env var > config default account.
func resolveAccount() (string, error) {
	if accountFlag != "" {
		return api.NormalizeAccountID(accountFlag), nil
	}
	if env := os.Getenv("META_ADS_ACCOUNT"); env != "" {
		return api.NormalizeAccountID(env), nil
	}
	if cfg != nil && cfg.DefaultAccount != "" {
		return api.NormalizeAccountID(cfg.DefaultAccount), nil
	}
	return "", fmt.Errorf("no account specified — use --account, set META_ADS_ACCOUNT, or set a default with: meta-ads accounts list")
}
