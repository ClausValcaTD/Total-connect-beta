package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `json:"server"`
	Vault    VaultConfig    `json:"vault"`
	UI       UIConfig       `json:"ui"`
	Defaults DefaultsConfig `json:"defaults"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type VaultConfig struct {
	Timeout int    `json:"timeout_seconds"`
	Path    string `json:"path"`
}

type UIConfig struct {
	Theme        string `json:"theme"`
	ShowHidden   bool   `json:"show_hidden_files"`
	ConfirmDelete bool `json:"confirm_delete"`
}

type DefaultsConfig struct {
	DownloadPath string `json:"download_path"`
	ParallelTransfers int `json:"parallel_transfers"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	
	return &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 50051,
		},
		Vault: VaultConfig{
			Timeout: 300,
			Path:    filepath.Join(home, ".totalconnect", "vault"),
		},
		UI: UIConfig{
			Theme:           "dark",
			ShowHidden:      false,
			ConfirmDelete:   true,
		},
		Defaults: DefaultsConfig{
			DownloadPath:      filepath.Join(home, "Downloads"),
			ParallelTransfers: 4,
		},
	}
}

// Load loads config from file or creates default
func Load(path string) (*Config, error) {
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".totalconnect", "config.json")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			return cfg, cfg.Save(path)
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// Save saves config to file
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
