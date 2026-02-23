package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/vincentmaurin/meta-ads-cli/internal/config"
)

const (
	metaDialogURL   = "https://www.facebook.com/v25.0/dialog/oauth"
	metaTokenURL    = "https://graph.facebook.com/v25.0/oauth/access_token"
	metaExchangeURL = "https://graph.facebook.com/v25.0/oauth/access_token"
	metaMeURL       = "https://graph.facebook.com/v25.0/me"
)

// ── flag vars ─────────────────────────────────────────────────────────────────

var authSetTokenNoExtend bool
var authExtendTokenSave  bool

// ── command definitions ───────────────────────────────────────────────────────

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Meta Ads authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Meta Ads via browser OAuth",
	Long: `Opens your browser to authenticate with Meta and saves a long-lived token.

Requires META_APP_ID and META_APP_SECRET environment variables,
or values stored from a previous login.`,
	RunE: runAuthLogin,
}

var authSetTokenCmd = &cobra.Command{
	Use:   "set-token <token>",
	Short: "Save a Meta access token directly (no browser needed)",
	Long: `Saves a Meta access token directly to the config file.

The token is validated by calling GET /me. If META_APP_ID and META_APP_SECRET
are available (env vars or stored config), the token is automatically upgraded
to a long-lived token (~60 days) unless --no-extend is passed.

Useful when you already have a token from the Meta Developer console,
from Graph API Explorer, or from another tool.

Examples:
  meta-ads auth set-token EAABsbCS...
  meta-ads auth set-token EAABsbCS... --no-extend
  META_APP_ID=123 META_APP_SECRET=abc meta-ads auth set-token EAABsbCS...`,
	Args: cobra.ExactArgs(1),
	RunE: runAuthSetToken,
}

var authExtendTokenCmd = &cobra.Command{
	Use:   "extend-token <short_lived_token>",
	Short: "Exchange a short-lived token for a long-lived one (~60 days)",
	Long: `Calls the Meta token exchange endpoint to upgrade a short-lived user
access token to a long-lived one that expires in approximately 60 days.

Requires META_APP_ID and META_APP_SECRET (env vars or stored config).

Reference:
  https://developers.facebook.com/docs/facebook-login/guides/access-tokens/get-long-lived/

Examples:
  # Print the long-lived token only
  meta-ads auth extend-token EAABsbCS...

  # Extend AND save to config
  meta-ads auth extend-token EAABsbCS... --save`,
	Args: cobra.ExactArgs(1),
	RunE: runAuthExtendToken,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and remove saved credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Clear(); err != nil {
			return fmt.Errorf("failed to clear config: %w", err)
		}
		fmt.Println("✓ Logged out successfully")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		if c.AccessToken == "" {
			fmt.Println("✗ Not logged in")
			fmt.Println("  → meta-ads auth login           (browser OAuth)")
			fmt.Println("  → meta-ads auth set-token       (paste a token directly)")
			return nil
		}
		fmt.Printf("✓ Logged in as %s (ID: %s)\n", c.UserName, c.UserID)
		if c.TokenType != "" {
			fmt.Printf("  Token type:      %s\n", c.TokenType)
		}
		if c.DefaultAccount != "" {
			fmt.Printf("  Default account: %s\n", c.DefaultAccount)
		}
		fmt.Printf("  Config:          %s\n", config.Path())
		return nil
	},
}

func init() {
	authSetTokenCmd.Flags().BoolVar(&authSetTokenNoExtend, "no-extend", false, "Skip upgrading to long-lived token even if app credentials are available")
	authExtendTokenCmd.Flags().BoolVar(&authExtendTokenSave, "save", false, "Save the long-lived token to config (replaces current token)")

	authCmd.AddCommand(authLoginCmd, authSetTokenCmd, authExtendTokenCmd, authLogoutCmd, authStatusCmd)
	rootCmd.AddCommand(authCmd)
}

// ── command handlers ──────────────────────────────────────────────────────────

func runAuthLogin(cmd *cobra.Command, args []string) error {
	// 1. Get app credentials from env or existing config
	appID, appSecret := resolveAppCredentials()

	if appID == "" {
		return fmt.Errorf("META_APP_ID not set — export META_APP_ID=<your_app_id>")
	}
	if appSecret == "" {
		return fmt.Errorf("META_APP_SECRET not set — export META_APP_SECRET=<your_app_secret>")
	}

	// 2. Pick a random free port for the callback server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to find free port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// 3. Start callback HTTP server
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errMsg := q.Get("error"); errMsg != "" {
			desc := q.Get("error_description")
			errCh <- fmt.Errorf("OAuth error: %s — %s", errMsg, desc)
			http.Error(w, "Authentication failed. You may close this tab.", http.StatusBadRequest)
			return
		}
		code := q.Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code returned in callback")
			http.Error(w, "No code received. You may close this tab.", http.StatusBadRequest)
			return
		}
		codeCh <- code
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body style="font-family:sans-serif;text-align:center;padding:40px">
<h2>✓ Authentication successful!</h2>
<p>You may close this tab and return to the terminal.</p>
</body></html>`)
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			select {
			case errCh <- fmt.Errorf("callback server error: %w", err):
			default:
			}
		}
	}()

	// 4. Build authorize URL and open browser
	authURL := buildAuthURL(appID, redirectURI)
	fmt.Printf("\nOpening browser for Meta authentication...\n")
	fmt.Printf("If the browser does not open automatically, visit:\n  %s\n\n", authURL)
	openBrowser(authURL)
	fmt.Printf("Waiting for callback on http://127.0.0.1:%d/callback ...\n", port)

	// 5. Wait for code or error (5-minute timeout)
	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		shutdownServer(srv)
		return err
	case <-time.After(5 * time.Minute):
		shutdownServer(srv)
		return fmt.Errorf("timed out waiting for OAuth callback (5 minutes)")
	}
	shutdownServer(srv)

	// 6. Exchange code → short-lived token
	fmt.Println("Exchanging authorization code for token...")
	shortToken, err := exchangeCode(code, appID, appSecret, redirectURI)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	// 7. Upgrade short-lived → long-lived (~60 days)
	fmt.Println("Upgrading to long-lived token...")
	longToken, err := exchangeToLongLived(shortToken, appID, appSecret)
	if err != nil {
		return fmt.Errorf("failed to upgrade token: %w", err)
	}

	// 8. GET /me for user info
	fmt.Println("Fetching user info...")
	userID, userName, err := fetchMe(longToken)
	if err != nil {
		return fmt.Errorf("failed to fetch user info: %w", err)
	}

	// 9. Save config
	existingCfg, _ := config.Load()
	newCfg := &config.Config{
		AccessToken: longToken,
		TokenType:   config.TokenTypeOAuth,
		UserID:      userID,
		UserName:    userName,
		AppID:       appID,
		AppSecret:   appSecret,
	}
	if existingCfg != nil {
		newCfg.DefaultAccount = existingCfg.DefaultAccount
	}
	if err := config.Save(newCfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n✓ Logged in as %s (ID: %s)\n", userName, userID)
	fmt.Printf("  Token type: %s\n", config.TokenTypeOAuth)
	fmt.Printf("  Token saved to: %s\n", config.Path())
	return nil
}

func runAuthSetToken(cmd *cobra.Command, args []string) error {
	token := args[0]

	appID, appSecret := resolveAppCredentials()

	finalToken := token
	tokenType := config.TokenTypeManual

	// Auto-upgrade to long-lived if app credentials are available and --no-extend not set
	if !authSetTokenNoExtend && appID != "" && appSecret != "" {
		fmt.Println("App credentials found — upgrading to long-lived token (~60 days)...")
		lt, err := exchangeToLongLived(token, appID, appSecret)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not upgrade to long-lived token: %v\n", err)
			fmt.Fprintf(os.Stderr, "         Saving original token. Use --no-extend to suppress this warning.\n")
		} else {
			finalToken = lt
			tokenType = config.TokenTypeLongLived
			fmt.Println("✓ Token upgraded to long-lived.")
		}
	} else if !authSetTokenNoExtend && (appID == "" || appSecret == "") {
		fmt.Fprintln(os.Stderr, "Note: META_APP_ID / META_APP_SECRET not available — saving token as-is (not extended).")
		fmt.Fprintln(os.Stderr, "      To extend later: meta-ads auth extend-token <token> --save")
	}

	// Validate by calling /me
	fmt.Println("Validating token...")
	userID, userName, err := fetchMe(finalToken)
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	existingCfg, _ := config.Load()
	newCfg := &config.Config{
		AccessToken: finalToken,
		TokenType:   tokenType,
		UserID:      userID,
		UserName:    userName,
		AppID:       appID,
		AppSecret:   appSecret,
	}
	if existingCfg != nil {
		newCfg.DefaultAccount = existingCfg.DefaultAccount
		if newCfg.AppID == "" {
			newCfg.AppID = existingCfg.AppID
		}
		if newCfg.AppSecret == "" {
			newCfg.AppSecret = existingCfg.AppSecret
		}
	}

	if err := config.Save(newCfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n✓ Token saved — logged in as %s (ID: %s)\n", userName, userID)
	fmt.Printf("  Token type: %s\n", tokenType)
	fmt.Printf("  Config:     %s\n", config.Path())
	return nil
}

func runAuthExtendToken(cmd *cobra.Command, args []string) error {
	shortToken := args[0]

	appID, appSecret := resolveAppCredentials()
	if appID == "" {
		return fmt.Errorf("META_APP_ID not available — set env var or run: meta-ads auth login first")
	}
	if appSecret == "" {
		return fmt.Errorf("META_APP_SECRET not available — set env var or run: meta-ads auth login first")
	}

	fmt.Println("Exchanging for long-lived token...")
	longToken, err := exchangeToLongLived(shortToken, appID, appSecret)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	if authExtendTokenSave {
		fmt.Println("Validating token...")
		userID, userName, err := fetchMe(longToken)
		if err != nil {
			return fmt.Errorf("token validation failed: %w", err)
		}

		existingCfg, _ := config.Load()
		newCfg := &config.Config{
			AccessToken: longToken,
			TokenType:   config.TokenTypeLongLived,
			UserID:      userID,
			UserName:    userName,
			AppID:       appID,
			AppSecret:   appSecret,
		}
		if existingCfg != nil {
			newCfg.DefaultAccount = existingCfg.DefaultAccount
		}
		if err := config.Save(newCfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("\n✓ Long-lived token saved — logged in as %s (ID: %s)\n", userName, userID)
		fmt.Printf("  Config: %s\n", config.Path())
	} else {
		fmt.Printf("\nLong-lived token:\n%s\n", longToken)
		fmt.Println("\nTo save it to config, run:")
		fmt.Printf("  meta-ads auth set-token %s\n", longToken)
		fmt.Println("Or re-run with --save:")
		fmt.Printf("  meta-ads auth extend-token <short_token> --save\n")
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// resolveAppCredentials returns appID and appSecret from env vars, falling back to stored config.
func resolveAppCredentials() (appID, appSecret string) {
	appID = os.Getenv("META_APP_ID")
	appSecret = os.Getenv("META_APP_SECRET")

	if appID == "" || appSecret == "" {
		if c, err := config.Load(); err == nil && c != nil {
			if appID == "" {
				appID = c.AppID
			}
			if appSecret == "" {
				appSecret = c.AppSecret
			}
		}
	}
	return
}

// buildAuthURL constructs the Meta OAuth dialog URL.
func buildAuthURL(appID, redirectURI string) string {
	params := url.Values{}
	params.Set("client_id", appID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", "ads_management,ads_read,business_management")
	params.Set("response_type", "code")
	return metaDialogURL + "?" + params.Encode()
}

// openBrowser opens url in the system's default browser (best-effort).
func openBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	_ = cmd.Start()
}

// exchangeCode exchanges an OAuth authorization code for a short-lived access token.
func exchangeCode(code, appID, appSecret, redirectURI string) (string, error) {
	params := url.Values{}
	params.Set("client_id", appID)
	params.Set("client_secret", appSecret)
	params.Set("redirect_uri", redirectURI)
	params.Set("code", code)

	return metaTokenFetch(metaTokenURL + "?" + params.Encode())
}

// exchangeToLongLived upgrades a short-lived token to a ~60-day long-lived token.
func exchangeToLongLived(shortToken, appID, appSecret string) (string, error) {
	params := url.Values{}
	params.Set("grant_type", "fb_exchange_token")
	params.Set("client_id", appID)
	params.Set("client_secret", appSecret)
	params.Set("fb_exchange_token", shortToken)

	return metaTokenFetch(metaExchangeURL + "?" + params.Encode())
}

// metaTokenFetch performs a GET to a Meta token endpoint and returns the access_token.
func metaTokenFetch(reqURL string) (string, error) {
	resp, err := http.Get(reqURL) //nolint:noctx
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		AccessToken string `json:"access_token"`
		Error       *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("meta api error: %s", result.Error.Message)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("no access_token in response: %s", string(body))
	}
	return result.AccessToken, nil
}

// fetchMe calls GET /me and returns (userID, userName, error).
func fetchMe(token string) (string, string, error) {
	params := url.Values{}
	params.Set("access_token", token)
	params.Set("fields", "id,name")

	resp, err := http.Get(metaMeURL + "?" + params.Encode()) //nolint:noctx
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var result struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("parsing /me response: %w", err)
	}
	if result.Error != nil {
		return "", "", fmt.Errorf("meta api error: %s", result.Error.Message)
	}
	return result.ID, result.Name, nil
}

// shutdownServer gracefully stops the HTTP server.
func shutdownServer(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
