package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultRelay is the qim-data relay endpoint.
	DefaultRelay = "data-relay.qim.dk:9009"
)

// Config stores local qim-data settings.
type Config struct {
	Relay        string `json:"relay"`
	RelayPassFile string `json:"relay_pass_file,omitempty"`
	// RelayPass is kept for backward compatibility with early local config drafts.
	RelayPass string `json:"relay_pass,omitempty"`
	CrocPath  string `json:"croc_path,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Default returns the baseline configuration.
func Default() Config {
	return Config{
		Relay: DefaultRelay,
	}
}

// Path returns the config file path.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(dir, "qim-data", "config.json"), nil
}

// SecretPath returns the relay secret file path.
func SecretPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(dir, "qim-data", "relay.pass"), nil
}

// Load reads config from disk.
func Load() (Config, error) {
	cfg := Default()

	path, err := Path()
	if err != nil {
		return cfg, err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.Relay == "" {
		cfg.Relay = DefaultRelay
	}

	return cfg, nil
}

// Save persists config with restrictive permissions.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}

	cfg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	b = append(b, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write temp config %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace config %s: %w", path, err)
	}

	return nil
}

// WriteSecret stores the relay secret using restrictive permissions and
// returns the saved secret file path.
func WriteSecret(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", fmt.Errorf("secret cannot be empty")
	}
	path, err := SecretPath()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config directory %s: %w", dir, err)
	}
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write secret file %s: %w", path, err)
	}
	return path, nil
}
