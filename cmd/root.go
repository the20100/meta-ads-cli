package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/vincentmaurin/meta-ads-cli/internal/api"
	"github.com/vincentmaurin/meta-ads-cli/internal/config"
	"github.com/vincentmaurin/meta-ads-cli/internal/metaauth"
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

Token resolution order:
  1. META_TOKEN env var
  2. Own config    (~/.config/meta-ads/config.json   via: meta-ads auth login)
  3. Shared config (~/.config/meta-auth/config.json  via: meta-auth login)

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
		if isAuthCommand(cmd) {
			return nil
		}

		token, appSecret, err := resolveToken()
		if err != nil {
			return err
		}

		client = api.NewClient(token, appSecret)
		return nil
	}
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show tool info: config paths, token status, and environment",
	Run: func(cmd *cobra.Command, args []string) {
		printInfo()
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func printInfo() {
	configDir, _ := os.UserConfigDir()
	ownConfig := filepath.Join(configDir, "meta-ads", "config.json")
	sharedConfig := filepath.Join(configDir, "meta-auth", "config.json")

	fmt.Println("meta-ads — Meta Ads CLI")
	fmt.Println()

	exe, _ := os.Executable()
	fmt.Printf("  binary:  %s\n", exe)
	fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	fmt.Println("  config paths by OS:")
	fmt.Println("    macOS:    ~/Library/Application Support/meta-ads/config.json")
	fmt.Println("    Linux:    ~/.config/meta-ads/config.json")
	fmt.Println("    Windows:  %AppData%\\meta-ads\\config.json")
	fmt.Printf("  own config:    %s\n", ownConfig)
	fmt.Printf("  shared config: %s\n", sharedConfig)
	fmt.Println()

	// Token source
	tokenSource := "(not set)"
	userName := ""
	if t := os.Getenv("META_TOKEN"); t != "" {
		tokenSource = "META_TOKEN env var"
	} else if tok, name := readTokenFromFile(ownConfig); tok != "" {
		tokenSource = "own config"
		userName = name
	} else if tok, name := readTokenFromFile(sharedConfig); tok != "" {
		tokenSource = "meta-auth shared config"
		userName = name
	}
	fmt.Printf("  token source: %s\n", tokenSource)
	if userName != "" {
		fmt.Printf("  user:         %s\n", userName)
	}
	printExpiryFromFile(ownConfig, sharedConfig)

	fmt.Println()
	fmt.Println("  env vars:")
	fmt.Printf("    META_TOKEN       = %s\n", maskOrEmpty(os.Getenv("META_TOKEN")))
	fmt.Printf("    META_ADS_ACCOUNT = %s\n", maskOrEmpty(os.Getenv("META_ADS_ACCOUNT")))
	fmt.Printf("    META_APP_SECRET  = %s\n", maskOrEmpty(os.Getenv("META_APP_SECRET")))
	fmt.Println()
	fmt.Println("  token resolution order:")
	fmt.Println("    1. META_TOKEN env var")
	fmt.Println("    2. own config   (meta-ads auth login)")
	fmt.Println("    3. shared config (meta-auth login)  ← recommended")
}

func readTokenFromFile(path string) (token, userName string) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || err != nil {
		return "", ""
	}
	var cfg struct {
		AccessToken string `json:"access_token"`
		UserName    string `json:"user_name"`
	}
	if json.Unmarshal(data, &cfg) == nil {
		return cfg.AccessToken, cfg.UserName
	}
	return "", ""
}

func printExpiryFromFile(ownPath, sharedPath string) {
	for _, path := range []string{ownPath, sharedPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg struct {
			AccessToken    string `json:"access_token"`
			TokenExpiresAt int64  `json:"token_expires_at"`
		}
		if json.Unmarshal(data, &cfg) != nil || cfg.AccessToken == "" {
			continue
		}
		if cfg.TokenExpiresAt == 0 {
			fmt.Println("  expires:      unknown")
		} else {
			exp := time.Unix(cfg.TokenExpiresAt, 0)
			days := int(time.Until(exp).Hours() / 24)
			if days < 0 {
				fmt.Printf("  expires:      EXPIRED on %s\n", exp.Format("2006-01-02"))
			} else {
				fmt.Printf("  expires:      %s (%d days left)\n", exp.Format("2006-01-02"), days)
			}
		}
		return
	}
}

func maskOrEmpty(v string) string {
	if v == "" {
		return "(not set)"
	}
	if len(v) <= 8 {
		return "***"
	}
	return v[:4] + "..." + v[len(v)-4:]
}

// resolveToken returns the best available token using the priority chain.
// Returns (token, appSecret, error).
func resolveToken() (string, string, error) {
	// 1. META_TOKEN env var (universal override for all Meta CLIs)
	if t := os.Getenv("META_TOKEN"); t != "" {
		return t, os.Getenv("META_APP_SECRET"), nil
	}

	// 2. Own config
	var err error
	cfg, err = config.Load()
	if err != nil {
		return "", "", fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.AccessToken != "" {
		appSecret := cfg.AppSecret
		if appSecret == "" {
			appSecret = os.Getenv("META_APP_SECRET")
		}
		return cfg.AccessToken, appSecret, nil
	}

	// 3. meta-auth shared config
	sharedToken, err := metaauth.Token()
	if err != nil {
		return "", "", fmt.Errorf("failed to read meta-auth config: %w", err)
	}
	if sharedToken != "" {
		warnSharedExpiry()
		return sharedToken, os.Getenv("META_APP_SECRET"), nil
	}

	return "", "", fmt.Errorf("not authenticated — run: meta-ads auth login\nor: meta-auth login  (shared auth)")
}

// warnSharedExpiry prints a stderr warning if the shared meta-auth token is expiring soon.
func warnSharedExpiry() {
	days := metaauth.DaysUntilExpiry()
	switch {
	case metaauth.IsExpired():
		fmt.Fprintf(os.Stderr, "warning: meta-auth token has expired — run: meta-auth refresh\n")
	case days >= 0 && days <= 7:
		fmt.Fprintf(os.Stderr, "warning: meta-auth token expires in %d day(s) — run: meta-auth refresh\n", days)
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
