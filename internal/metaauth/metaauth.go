// Package metaauth reads the shared token managed by meta-auth-cli.
// Config path: ~/.config/meta-auth/config.json
//
// Token resolution order used by each CLI:
//  1. META_TOKEN env var
//  2. Own local config  (~/.config/meta-ads/config.json)
//  3. meta-auth shared config (~/.config/meta-auth/config.json)
package metaauth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type sharedConfig struct {
	AccessToken    string `json:"access_token"`
	UserName       string `json:"user_name,omitempty"`
	TokenExpiresAt int64  `json:"token_expires_at,omitempty"`
}

// Token returns the token stored by meta-auth-cli, or ("", nil) if not found.
func Token() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "meta-auth", "config.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	var cfg sharedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", err
	}
	return cfg.AccessToken, nil
}

// IsExpired reports whether the shared token has a known expiry that has passed.
func IsExpired() bool {
	dir, _ := os.UserConfigDir()
	path := filepath.Join(dir, "meta-auth", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var cfg sharedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false
	}
	if cfg.TokenExpiresAt == 0 {
		return false
	}
	return time.Now().After(time.Unix(cfg.TokenExpiresAt, 0))
}

// DaysUntilExpiry returns days until the shared token expires, -1 if unknown.
func DaysUntilExpiry() int {
	dir, _ := os.UserConfigDir()
	path := filepath.Join(dir, "meta-auth", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	var cfg sharedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return -1
	}
	if cfg.TokenExpiresAt == 0 {
		return -1
	}
	d := time.Until(time.Unix(cfg.TokenExpiresAt, 0))
	if d < 0 {
		return 0
	}
	return int(d.Hours() / 24)
}
