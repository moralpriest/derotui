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
	LastWallet        string                   `json:"last_wallet"`
	LastMiningAddress string                   `json:"last_mining_address"` // public payout address for embedded miner
	WalletNetworks    map[string]WalletNetwork `json:"wallet_networks"`     // wallet path -> network
	Theme             string                   `json:"theme"`               // selected theme ID
	Daemon            DaemonSettings           `json:"daemon"`
}

// DaemonSettings holds local daemon launch settings.
type DaemonSettings struct {
	Mode              string   `json:"mode"`
	DownloadSource    string   `json:"download_source"`
	BinaryPath        string   `json:"binary_path"`
	Network           string   `json:"network"`
	DataDir           string   `json:"data_dir"`
	FastSync          bool     `json:"fastsync"`
	IntegratorAddress string   `json:"integrator_address"`
	NodeTag           string   `json:"node_tag"`
	RPCBind           string   `json:"rpc_bind"`
	P2PBind           string   `json:"p2p_bind"`
	GetWorkBind       string   `json:"getwork_bind"`
	SocksProxy        string   `json:"socks_proxy"`
	Debug             bool     `json:"debug"`
	TimeIsInSync      bool     `json:"timeisinsync"`
	SyncNode          bool     `json:"sync_node"`
	MinPeers          string   `json:"min_peers"`
	MaxPeers          string   `json:"max_peers"`
	LogDir            string   `json:"log_dir"`
	PriorityNodes     []string `json:"priority_nodes"`
	ExclusiveNodes    []string `json:"exclusive_nodes"`
	ConsoleLogLevel   string   `json:"console_log_level"`
	FileLogLevel      string   `json:"file_log_level"`
	PruneHistory      string   `json:"prune_history"`
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
		Daemon:         DefaultDaemonSettings(),
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
	cfg.Daemon = normalizeDaemonSettings(cfg.Daemon)
	return cfg
}

// DefaultDaemonSettings returns default local daemon settings.
func DefaultDaemonSettings() DaemonSettings {
	return DefaultDaemonSettingsForNetwork(string(NetworkMainnet))
}

// DefaultDaemonSettingsForNetwork returns default local daemon settings for a network.
func DefaultDaemonSettingsForNetwork(network string) DaemonSettings {
	home, err := os.UserHomeDir()
	baseDir := "."
	if err == nil {
		baseDir = filepath.Join(home, ".derotui")
	}
	network = normalizeDaemonNetwork(network)
	rpcBind, p2pBind, getWorkBind := daemonBindDefaultsForNetwork(network)
	return DaemonSettings{
		Mode:            "embedded",
		DownloadSource:  DefaultDaemonDownloadSource(),
		BinaryPath:      filepath.Join(baseDir, "derod"),
		Network:         network,
		DataDir:         baseDir,
		FastSync:        true,
		RPCBind:         rpcBind,
		P2PBind:         p2pBind,
		GetWorkBind:     getWorkBind,
		MinPeers:        "31",
		MaxPeers:        "101",
		ConsoleLogLevel: "1",
		FileLogLevel:    "1",
		PruneHistory:    "",
	}
}

func normalizeDaemonSettings(settings DaemonSettings) DaemonSettings {
	settings.Network = normalizeDaemonNetwork(settings.Network)
	defaults := DefaultDaemonSettingsForNetwork(settings.Network)
	if settings.Network == "" {
		settings.Network = defaults.Network
	}
	if settings.DataDir == "" {
		settings.DataDir = defaults.DataDir
	} else {
		cleanDataDir := filepath.Clean(settings.DataDir)
		legacyBase := filepath.Join(filepath.Dir(defaults.DataDir), "daemon")
		if strings.HasPrefix(cleanDataDir, legacyBase+string(os.PathSeparator)) {
			remainder := strings.TrimPrefix(cleanDataDir, legacyBase+string(os.PathSeparator))
			settings.DataDir = filepath.Join(filepath.Dir(defaults.DataDir), remainder)
			cleanDataDir = filepath.Clean(settings.DataDir)
		}
		base := filepath.Base(cleanDataDir)
		parent := filepath.Dir(cleanDataDir)
		if base == settings.Network || base == settings.Network+"_simulator" {
			settings.DataDir = parent
		}
	}
	if settings.DownloadSource == "" {
		settings.DownloadSource = defaults.DownloadSource
	}
	if settings.BinaryPath == "" {
		settings.BinaryPath = defaults.BinaryPath
	}
	if settings.RPCBind == "" {
		settings.RPCBind = defaults.RPCBind
	}
	if settings.P2PBind == "" {
		settings.P2PBind = defaults.P2PBind
	}
	if settings.GetWorkBind == "" {
		settings.GetWorkBind = defaults.GetWorkBind
	}
	if settings.MinPeers == "" {
		settings.MinPeers = defaults.MinPeers
	}
	if settings.MaxPeers == "" {
		settings.MaxPeers = defaults.MaxPeers
	}
	if settings.ConsoleLogLevel == "" {
		settings.ConsoleLogLevel = defaults.ConsoleLogLevel
	}
	if settings.FileLogLevel == "" {
		settings.FileLogLevel = defaults.FileLogLevel
	}
	if settings.PruneHistory == "" {
		settings.PruneHistory = defaults.PruneHistory
	}
	if settings.Mode == "" {
		settings.Mode = defaults.Mode
	}
	if !strings.EqualFold(strings.TrimSpace(settings.Mode), "embedded") && !strings.EqualFold(strings.TrimSpace(settings.Mode), "external") {
		settings.Mode = defaults.Mode
	}
	return settings
}

// DefaultDaemonDownloadSource returns the official derod release source.
// PrunePresets are user-friendly prune-history choices, low space to high.
// Estimated size assumes a pruned store overhead of ~40 KB per block
// (raw block plus balances/topo bookkeeping), so numbers land close to
// real on-disk sizes observed on synced nodes.
var PrunePresets = []struct {
	Blocks string
	Label  string
}{
	{"500", "500 blocks (~1 MB)"},
	{"50000", "50,000 blocks (~2 GB)"},
	{"250000", "250,000 blocks (~10 GB)"},
	{"500000", "500,000 blocks (~20 GB)"},
	{"1250000", "1,250,000 blocks (~50 GB)"},
}

// DefaultPruneHistory is the preset used when switching to the Pruned profile.
const DefaultPruneHistory = "50000"

// IsPruned reports whether the settings select the Pruned sync profile.
func (s DaemonSettings) IsPruned() bool {
	v := strings.TrimSpace(s.PruneHistory)
	return v != "" && v != "0"
}

// NextPrunePreset cycles to the next preset after current (wrapping).
func NextPrunePreset(current string) string {
	for i, p := range PrunePresets {
		if p.Blocks == strings.TrimSpace(current) {
			return PrunePresets[(i+1)%len(PrunePresets)].Blocks
		}
	}
	return PrunePresets[1].Blocks
}

// DescribePrune returns a friendly label with estimated storage.
func DescribePrune(blocks string) string {
	blocks = strings.TrimSpace(blocks)
	for _, p := range PrunePresets {
		if p.Blocks == blocks {
			return p.Label
		}
	}
	if blocks == "" || blocks == "0" {
		return ""
	}
	return blocks + " blocks"
}

func DefaultDaemonDownloadSource() string {
	return "https://api.github.com/repos/deroproject/derohe/releases"
}

func normalizeDaemonNetwork(network string) string {
	switch strings.TrimSpace(strings.ToLower(network)) {
	case string(NetworkTestnet):
		return string(NetworkTestnet)
	case string(NetworkSimulator):
		return string(NetworkSimulator)
	default:
		return string(NetworkMainnet)
	}
}

func daemonBindDefaultsForNetwork(network string) (string, string, string) {
	switch normalizeDaemonNetwork(network) {
	case string(NetworkTestnet):
		return "127.0.0.1:40402", "0.0.0.0:40402", "0.0.0.0:40400"
	case string(NetworkSimulator):
		return "127.0.0.1:20000", "0.0.0.0:20000", "0.0.0.0:20003"
	default:
		return "127.0.0.1:10102", "0.0.0.0:10102", "0.0.0.0:10100"
	}
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

// SetLastMiningAddress saves the last used mining payout address.
// The address is public (receive-only) and does not require an unlocked wallet.
func SetLastMiningAddress(address string) error {
	if strings.TrimSpace(address) == "" {
		return nil
	}
	configMu.Lock()
	defer configMu.Unlock()
	cfg := loadUnlocked()
	cfg.LastMiningAddress = strings.TrimSpace(address)
	return saveUnlocked(cfg)
}

// GetLastMiningAddress returns the last used mining payout address (empty if none).
func GetLastMiningAddress() string {
	return strings.TrimSpace(Load().LastMiningAddress)
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

// GetDaemonSettings returns saved local daemon settings.
func GetDaemonSettings() DaemonSettings {
	cfg := Load()
	return normalizeDaemonSettings(cfg.Daemon)
}

// SetDaemonSettings saves local daemon settings.
func SetDaemonSettings(settings DaemonSettings) error {
	configMu.Lock()
	defer configMu.Unlock()
	cfg := loadUnlocked()
	cfg.Daemon = normalizeDaemonSettings(settings)
	return saveUnlocked(cfg)
}
