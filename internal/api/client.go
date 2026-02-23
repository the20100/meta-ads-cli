package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const baseURL = "https://graph.facebook.com/v25.0"

// Client is an authenticated Meta Graph API client.
type Client struct {
	token      string
	appSecret  string
	httpClient *http.Client
}

// NewClient creates a new authenticated Client.
// appSecret is optional but enables appsecret_proof for server-side calls.
func NewClient(token, appSecret string) *Client {
	return &Client{
		token:     token,
		appSecret: appSecret,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// appSecretProof computes HMAC-SHA256(token, appSecret) as a hex string.
func (c *Client) appSecretProof() string {
	if c.appSecret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(c.appSecret))
	mac.Write([]byte(c.token))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

// baseParams returns the common query parameters added to every request.
func (c *Client) baseParams() url.Values {
	params := url.Values{}
	params.Set("access_token", c.token)
	if proof := c.appSecretProof(); proof != "" {
		params.Set("appsecret_proof", proof)
	}
	return params
}

// checkRateLimit reads X-Business-Use-Case-Usage and warns to stderr if high.
func checkRateLimit(headers http.Header) {
	buc := headers.Get("X-Business-Use-Case-Usage")
	if buc == "" {
		return
	}
	// Shape: {"<id>":[{"call_count":N,"total_cputime":N,"total_time":N,"type":"..."}]}
	var parsed map[string][]struct {
		CallCount int `json:"call_count"`
		TotalTime int `json:"total_time"`
	}
	if err := json.Unmarshal([]byte(buc), &parsed); err != nil {
		return
	}
	for _, entries := range parsed {
		for _, e := range entries {
			if e.CallCount > 75 || e.TotalTime > 75 {
				fmt.Fprintf(os.Stderr, "⚠️  Rate limit: %d%% used — slow down to avoid HTTP 613\n", max(e.CallCount, e.TotalTime))
			}
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// doRequest executes an HTTP request and returns the body bytes.
// It handles Meta error responses and rate limit warnings.
func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	checkRateLimit(resp.Header)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check for Meta API error in body (Meta returns 200 even for some errors)
	var errResp struct {
		Error *MetaError `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
		return nil, errResp.Error
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// Get makes an authenticated GET request to the given path with extra params.
func (c *Client) Get(path string, params url.Values) ([]byte, error) {
	reqURL, err := buildURL(path, c.baseParams(), params)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	return c.doRequest(req)
}

// Post makes an authenticated POST request to the given path with form body.
func (c *Client) Post(path string, body url.Values) ([]byte, error) {
	reqURL, err := buildURL(path, c.baseParams(), nil)
	if err != nil {
		return nil, err
	}

	// Merge base params into body for POST
	for k, vs := range c.baseParams() {
		body.Set(k, vs[0])
	}

	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	return c.doRequest(req)
}

// GetAll fetches all pages of a list endpoint, following paging.next cursors.
// Returns all items as raw JSON messages.
func (c *Client) GetAll(path string, params url.Values) ([]json.RawMessage, error) {
	var all []json.RawMessage

	// Clone params to avoid mutating caller's map
	p := url.Values{}
	for k, v := range params {
		p[k] = v
	}
	if p.Get("limit") == "" {
		p.Set("limit", "100")
	}

	currentPath := path

	for {
		body, err := c.Get(currentPath, p)
		if err != nil {
			return nil, err
		}

		var page struct {
			Data   []json.RawMessage `json:"data"`
			Paging *Paging           `json:"paging"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("parsing page: %w", err)
		}

		all = append(all, page.Data...)

		// No more pages
		if page.Paging == nil || page.Paging.Next == "" {
			break
		}

		// Next page: use the full URL from paging.next (already includes access_token etc.)
		currentPath = page.Paging.Next
		p = url.Values{} // params are already embedded in the Next URL
	}

	return all, nil
}

// GetRaw makes a GET to a full URL (used for paging.next which is a complete URL).
func (c *Client) GetRaw(fullURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return c.doRequest(req)
}

// buildURL constructs a full URL from path, base params, and extra params.
// If path starts with "http", it's used as-is.
func buildURL(path string, base, extra url.Values) (string, error) {
	var u *url.URL
	var err error

	if strings.HasPrefix(path, "http") {
		u, err = url.Parse(path)
	} else {
		u, err = url.Parse(baseURL + path)
	}
	if err != nil {
		return "", err
	}

	q := u.Query()
	for k, vs := range base {
		q.Set(k, vs[0])
	}
	for k, vs := range extra {
		q.Set(k, vs[0])
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// NormalizeAccountID ensures the account ID has the "act_" prefix.
func NormalizeAccountID(id string) string {
	if strings.HasPrefix(id, "act_") {
		return id
	}
	return "act_" + id
}

// StripActPrefix removes the "act_" prefix if present.
func StripActPrefix(id string) string {
	return strings.TrimPrefix(id, "act_")
}
