// Copyright 2017-2026 DERO Project. All rights reserved.

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// WalletNetwork represents the network type for a wallet
type WalletNetwork string

const (
	NetworkMainnet   WalletNetwork = "mainnet"
	NetworkTestnet   WalletNetwork = "testnet"
	NetworkSimulator WalletNetwork = "simulator"
)

// Config holds application configuration
type Config struct {
	LastWallet     string                   `json:"last_wallet"`
	WalletNetworks map[string]WalletNetwork `json:"wallet_networks"` // wallet path -> network
	Theme          string                   `json:"theme"`           // selected theme ID
}

var configMu sync.Mutex

// configPath returns the path to the config file
func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".derotui.json"
	}
	return filepath.Join(home, ".derotui.json")
}

// Load loads the configuration from disk
func Load() Config {
	configMu.Lock()
	defer configMu.Unlock()
	return loadUnlocked()
}

func loadUnlocked() Config {
	cfg := Config{
		WalletNetworks: make(map[string]WalletNetwork),
	}
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	// Ignore unmarshal errors - return default config if file is corrupted
	_ = json.Unmarshal(data, &cfg)
	// Ensure map is initialized
	if cfg.WalletNetworks == nil {
		cfg.WalletNetworks = make(map[string]WalletNetwork)
	}
	return cfg
}

// Save saves the configuration to disk
func Save(cfg Config) error {
	configMu.Lock()
	defer configMu.Unlock()
	return saveUnlocked(cfg)
}

func saveUnlocked(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}

// isWalletPathFormatValid checks whether path looks like a wallet file path.
// It only enforces extension and directory checks when the path exists.
func isWalletPathFormatValid(path string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(trimmed), ".db") {
		return false
	}
	if info, err := os.Stat(trimmed); err == nil && info.IsDir() {
		return false
	}
	return true
}

// isExistingWalletFile checks whether path exists and is a non-empty wallet file.
func isExistingWalletFile(path string) bool {
	if !isWalletPathFormatValid(path) {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() || info.Size() == 0 {
		return false
	}
	return true
}

// SetLastWallet saves the last used wallet path
func SetLastWallet(path string) error {
	if !isWalletPathFormatValid(path) {
		return fmt.Errorf("invalid wallet path: %q", path)
	}
	configMu.Lock()
	defer configMu.Unlock()
	cfg := loadUnlocked()
	cfg.LastWallet = path
	return saveUnlocked(cfg)
}

// GetLastWallet returns the last used wallet path
func GetLastWallet() string {
	cfg := Load()
	// Return empty if no wallet is saved
	if cfg.LastWallet == "" {
		return ""
	}
	// Validate that saved path is a valid .db file
	if !isExistingWalletFile(cfg.LastWallet) {
		return ""
	}
	// Normalize path to absolute for consistency
	absPath, err := filepath.Abs(cfg.LastWallet)
	if err != nil {
		return cfg.LastWallet
	}
	return absPath
}

// SetWalletNetwork saves the network type for a wallet
func SetWalletNetwork(walletPath string, network WalletNetwork) error {
	if !isWalletPathFormatValid(walletPath) {
		return fmt.Errorf("invalid wallet path for network mapping: %q", walletPath)
	}
	configMu.Lock()
	defer configMu.Unlock()
	cfg := loadUnlocked()
	if cfg.WalletNetworks == nil {
		cfg.WalletNetworks = make(map[string]WalletNetwork)
	}
	// Normalize to absolute path for consistency
	absPath, err := filepath.Abs(walletPath)
	if err != nil {
		absPath = walletPath
	}
	// Also store with cleaned path
	absPath = filepath.Clean(absPath)
	cfg.WalletNetworks[absPath] = network
	return saveUnlocked(cfg)
}

// GetWalletNetwork returns the network type for a wallet (empty if unknown)
func GetWalletNetwork(walletPath string) WalletNetwork {
	cfg := Load()

	if cfg.WalletNetworks == nil {
		return ""
	}

	// Normalize input path to absolute and clean for lookup
	absPath, err := filepath.Abs(walletPath)
	if err != nil {
		absPath = walletPath
	}
	absPath = filepath.Clean(absPath)

	// Check if wallet file still exists - if deleted, return empty to force network selection
	if !isWalletPathFormatValid(absPath) {
		return ""
	}
	if info, err := os.Stat(absPath); err != nil || info.IsDir() {
		return ""
	}

	if network, ok := cfg.WalletNetworks[absPath]; ok {
		return network
	}

	return ""
}

// GetTheme returns the selected theme ID (defaults to "neon" if not set)
func GetTheme() string {
	cfg := Load()
	if cfg.Theme == "" {
		return "neon"
	}
	return cfg.Theme
}

// SetTheme saves the selected theme ID
func SetTheme(theme string) error {
	configMu.Lock()
	defer configMu.Unlock()
	cfg := loadUnlocked()
	cfg.Theme = theme
	return saveUnlocked(cfg)
}
