package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// TokenType describes how the access token was obtained.
type TokenType string

const (
	TokenTypeOAuth     TokenType = "oauth"      // browser OAuth flow (long-lived, ~60 days)
	TokenTypeManual    TokenType = "manual"      // pasted manually via auth set-token
	TokenTypeLongLived TokenType = "long-lived"  // explicitly extended via auth extend-token
)

// Config holds the persisted user configuration.
type Config struct {
	AccessToken    string    `json:"access_token"`
	TokenType      TokenType `json:"token_type,omitempty"`
	UserID         string    `json:"user_id"`
	UserName       string    `json:"user_name"`
	DefaultAccount string    `json:"default_account,omitempty"`
	// App credentials stored optionally; env vars META_APP_ID / META_APP_SECRET take priority.
	AppID     string `json:"app_id,omitempty"`
	AppSecret string `json:"app_secret,omitempty"`
}

// configPath returns the path to the config file.
// Uses os.UserConfigDir() for cross-platform support:
//   - macOS:   ~/Library/Application Support/meta-ads/config.json
//   - Linux:   ~/.config/meta-ads/config.json
//   - Windows: %AppData%\meta-ads\config.json
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "meta-ads", "config.json"), nil
}

// Load reads the config file. Returns an empty Config (not an error) if file doesn't exist.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes the config file with 0600 permissions.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// Clear removes the config file (logout).
func Clear() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Path returns the config file path for display purposes.
func Path() string {
	p, _ := configPath()
	return p
}
