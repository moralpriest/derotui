// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/deroproject/dero-wallet-cli/internal/log"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/transaction"
	"github.com/deroproject/derohe/walletapi"
	"github.com/deroproject/derohe/walletapi/mnemonics"
)

// NormalizeDaemonAddress normalizes daemon endpoints to host:port format.
// Accepts host:port and http(s)://host[:port] inputs.
func NormalizeDaemonAddress(address string) (string, error) {
	raw := strings.TrimSpace(address)
	if raw == "" {
		return "", fmt.Errorf("daemon address is empty")
	}

	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("invalid daemon URL: %w", err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return "", fmt.Errorf("unsupported daemon URL scheme: %s", u.Scheme)
		}
		host := u.Hostname()
		if host == "" {
			return "", fmt.Errorf("daemon URL missing host")
		}
		port := u.Port()
		if port == "" {
			if u.Scheme == "https" {
				port = "443"
			} else {
				port = "80"
			}
		}
		return net.JoinHostPort(host, port), nil
	}

	if _, _, err := net.SplitHostPort(raw); err != nil {
		return "", fmt.Errorf("invalid daemon address %q: expected host:port", raw)
	}

	return raw, nil
}

func daemonRPCURL(address string) (string, error) {
	normalized, err := NormalizeDaemonAddress(address)
	if err != nil {
		return "", err
	}
	return "http://" + normalized + "/json_rpc", nil
}

const (
	DefaultMainnetDaemon   = "localhost:10102"
	DefaultTestnetDaemon   = "localhost:40402"
	DefaultSimulatorDaemon = "localhost:20000"
	FallbackMainnetDaemon  = "node.derofoundation.org:11012"
	FallbackTestnetDaemon  = "69.30.234.163:40402"
)

// sharedHTTPClient is a pooled HTTP client for all daemon RPC requests.
// Reusing connections saves ~50-100ms per request to the same daemon.
var sharedHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		DisableCompression:    false,
	},
}

// rpcBufferPool provides pooled buffers for JSON-RPC request bodies.
// This reduces allocations for frequent daemon queries.
var rpcBufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 512))
	},
}

// getRPCBuffer returns a pooled buffer for RPC requests.
// Call putRPCBuffer after use.
func getRPCBuffer() *bytes.Buffer {
	buf := rpcBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// putRPCBuffer returns a buffer to the pool.
func putRPCBuffer(buf *bytes.Buffer) {
	if buf.Cap() <= 4096 { // Don't pool huge buffers
		rpcBufferPool.Put(buf)
	}
}

// Cache TTL configuration - can be adjusted at runtime
var (
	// DaemonInfoCacheTTL is the cache duration for daemon info queries.
	// Default 3 seconds. Increase for lower churn on stable daemons.
	DaemonInfoCacheTTL = 3 * time.Second

	// TxCacheTTL is the cache duration for transaction lists.
	// Default 5 seconds to keep Recent Activity/History fresh.
	TxCacheTTL = 5 * time.Second
)

// SetCacheTTLs configures cache durations. Pass 0 to keep current values.
func SetCacheTTLs(daemonInfo, txCache time.Duration) {
	if daemonInfo > 0 {
		DaemonInfoCacheTTL = daemonInfo
	}
	if txCache > 0 {
		TxCacheTTL = txCache
	}
}

// validWordsMap is a pre-computed map of all valid seed words for fast lookup
var (
	validWordsMap  map[string]bool
	validWordsOnce sync.Once
	daemonInfoMu   sync.RWMutex
	daemonInfoMemo = map[string]daemonInfoCacheEntry{}
)

type daemonInfoCacheEntry struct {
	info      DaemonInfo
	fetchedAt time.Time
}

// getValidWordsMap returns the pre-computed map of valid words (initialized once)
func getValidWordsMap() map[string]bool {
	validWordsOnce.Do(func() {
		validWordsMap = make(map[string]bool)
		for _, lang := range mnemonics.Languages {
			for _, word := range lang.Words {
				validWordsMap[strings.ToLower(word)] = true
			}
		}
	})
	return validWordsMap
}

// Wallet wraps the DERO wallet API
type Wallet struct {
	wallet        *walletapi.Wallet_Disk
	file          string
	network       string
	testnet       bool
	simulator     bool
	daemonAddress string

	// Cache for transactions to avoid redundant syncs.
	// Protected by txCacheMu - must hold lock when reading/writing cache fields
	txCacheMu     sync.RWMutex
	txCache       []TransactionInfo
	txCacheHeight uint64
	txCacheTopo   int64
	txCacheTime   time.Time
}

// Open opens an existing wallet
func Open(file, password string, testnet, simulator bool) (*Wallet, error) {
	start := time.Now()

	// Recover from interrupted writes (e.g. sudden shutdown while wallet file was being updated).
	if err := recoverCorruptWalletFile(file); err != nil {
		log.Warn("wallet", "open.recover_warning", "Wallet recovery check warning", "error", err.Error(), "file", filepath.Base(file))
	}

	// Initialize globals BEFORE opening wallet so DERO library uses correct network
	// Simulator can run in either mainnet or testnet mode
	if simulator {
		globals.Arguments["--testnet"] = testnet
		globals.InitNetwork()
	} else if testnet {
		globals.Arguments["--testnet"] = true
		globals.InitNetwork()
	} else {
		// Ensure mainnet is properly initialized to avoid state bleeding from previous sessions
		globals.Arguments["--testnet"] = false
		globals.InitNetwork()
	}

	w, err := walletapi.Open_Encrypted_Wallet(file, password)
	if err != nil {
		log.Error("wallet", "open.failed", "Failed to open wallet", "error", err.Error(), "file", filepath.Base(file))
		return nil, fmt.Errorf("failed to open wallet: %w", err)
	}

	// Keep the requested network authoritative. Only log if address prefix looks unexpected.
	address := w.GetAddress().String()
	detectedTestnet := strings.HasPrefix(address, "deto1")
	actualTestnet := testnet
	if !simulator {
		if testnet && !detectedTestnet {
			log.Warn("wallet", "open.network_mismatch", "Wallet address looks mainnet while testnet was requested")
		} else if !testnet && detectedTestnet {
			log.Warn("wallet", "open.network_mismatch", "Wallet address looks testnet while mainnet was requested")
		}
	}
	actualSimulator := simulator

	// Determine network name
	network := "Mainnet"
	if actualSimulator {
		network = "Simulator"
		// Simulator can be mainnet (dero1) or testnet (deto1)
		// Don't override the actual testnet value
	} else if actualTestnet {
		network = "Testnet"
	}

	// Ensure globals match final network state - ALWAYS initialize, not just for testnet
	// This fixes network state bleeding between sessions
	globals.Arguments["--testnet"] = actualTestnet
	globals.InitNetwork()

	log.Debug("wallet", "open.network_set", "Network globals initialized",
		"testnet", fmt.Sprintf("%t", actualTestnet),
		"simulator", fmt.Sprintf("%t", actualSimulator),
		"network", network,
		"address_prefix", address[:8])

	wallet := &Wallet{
		wallet:    w,
		file:      file,
		network:   network,
		testnet:   actualTestnet,
		simulator: actualSimulator,
	}

	duration := time.Since(start)
	log.Info("wallet", "open.success", "Wallet opened successfully",
		"file", filepath.Base(file),
		"network", network,
		"duration", log.FormatDuration(duration))
	if err := backupWalletFile(file); err != nil {
		log.Warn("wallet", "open.backup_warning", "Failed to refresh wallet backup", "error", err.Error(), "file", filepath.Base(file))
	}
	return wallet, nil
}

// Create creates a new wallet
func Create(file, password string, testnet, simulator bool) (*Wallet, string, error) {
	// Initialize globals BEFORE wallet creation so DERO library uses correct network
	// Simulator can run in either mainnet or testnet mode
	network := "Mainnet"
	if simulator {
		network = "Simulator"
		globals.Arguments["--testnet"] = testnet
		globals.InitNetwork()
	} else if testnet {
		network = "Testnet"
		globals.Arguments["--testnet"] = true
		globals.InitNetwork()
	} else {
		// Ensure mainnet is properly initialized to avoid state bleeding
		globals.Arguments["--testnet"] = false
		globals.InitNetwork()
	}

	w, err := walletapi.Create_Encrypted_Wallet_Random(file, password)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create wallet: %w", err)
	}

	seed := w.GetSeed()
	if err := backupWalletFile(file); err != nil {
		log.Warn("wallet", "create.backup_warning", "Failed to create wallet backup", "error", err.Error(), "file", filepath.Base(file))
	}

	return &Wallet{
		wallet:    w,
		file:      file,
		network:   network,
		testnet:   testnet,
		simulator: simulator,
	}, seed, nil
}

// ValidateSeed checks if a seed words is valid
func ValidateSeed(seed string) error {
	seed = strings.TrimSpace(seed)
	words := strings.Fields(seed)

	// Check word count
	if len(words) == 0 {
		return fmt.Errorf("please enter your seed words")
	}
	if len(words) > 25 {
		return fmt.Errorf("too many words: got %d, maximum is 25", len(words))
	}
	if len(words) < 25 {
		return fmt.Errorf("not enough words: got %d, need exactly 25", len(words))
	}

	// Check each word against all language wordlists
	invalidWords := findInvalidWords(words)
	if len(invalidWords) > 0 {
		if len(invalidWords) == 1 {
			return fmt.Errorf("invalid word: '%s'", invalidWords[0])
		}
		if len(invalidWords) <= 3 {
			return fmt.Errorf("invalid words: '%s'", strings.Join(invalidWords, "', '"))
		}
		return fmt.Errorf("multiple invalid words found (%d)", len(invalidWords))
	}

	// Full validation with checksum using the mnemonics library
	_, _, err := mnemonics.Words_To_Key(seed)
	if err != nil {
		return fmt.Errorf("invalid seed checksum")
	}
	return nil
}

// findInvalidWords checks each word against all language wordlists
func findInvalidWords(words []string) []string {
	validWords := getValidWordsMap()

	// Find words not in any wordlist
	var invalid []string
	for _, word := range words {
		if !validWords[strings.ToLower(word)] {
			invalid = append(invalid, word)
		}
	}
	return invalid
}

// ValidateWord checks if a single word is valid in any language wordlist
func ValidateWord(word string) error {
	word = strings.TrimSpace(word)
	if word == "" {
		return nil
	}

	validWords := getValidWordsMap()
	if !validWords[strings.ToLower(word)] {
		return fmt.Errorf("invalid word: '%s'", word)
	}
	return nil
}

// Restore restores a wallet from seed
func Restore(file, password, seed string, testnet, simulator bool) (*Wallet, error) {
	seed = strings.TrimSpace(seed)

	// Validate the seed first
	if err := ValidateSeed(seed); err != nil {
		return nil, err
	}

	// Initialize globals BEFORE wallet creation so DERO library uses correct network
	// Simulator can run in either mainnet or testnet mode
	network := "Mainnet"
	if simulator {
		network = "Simulator"
		globals.Arguments["--testnet"] = testnet
		globals.InitNetwork()
	} else if testnet {
		network = "Testnet"
		globals.Arguments["--testnet"] = true
		globals.InitNetwork()
	} else {
		// Ensure mainnet is properly initialized to avoid state bleeding
		globals.Arguments["--testnet"] = false
		globals.InitNetwork()
	}

	w, err := walletapi.Create_Encrypted_Wallet_From_Recovery_Words(file, password, seed)
	if err != nil {
		return nil, fmt.Errorf("failed to restore wallet: %w", err)
	}
	if err := backupWalletFile(file); err != nil {
		log.Warn("wallet", "restore.backup_warning", "Failed to create wallet backup", "error", err.Error(), "file", filepath.Base(file))
	}

	return &Wallet{
		wallet:    w,
		file:      file,
		network:   network,
		testnet:   testnet,
		simulator: simulator,
	}, nil
}

// RestoreFromKey restores a wallet from a 64 character hex private key
func RestoreFromKey(file, password, hexKey string, testnet, simulator bool) (*Wallet, error) {
	hexKey = strings.TrimSpace(hexKey)
	if len(hexKey) != 64 {
		return nil, fmt.Errorf("key must be exactly 64 hexadecimal characters")
	}

	// Decode hex string to bytes
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid hex key: %w", err)
	}

	// Initialize globals BEFORE wallet creation so DERO library uses correct network
	// Simulator can run in either mainnet or testnet mode
	network := "Mainnet"
	if simulator {
		network = "Simulator"
		globals.Arguments["--testnet"] = testnet
		globals.InitNetwork()
	} else if testnet {
		network = "Testnet"
		globals.Arguments["--testnet"] = true
		globals.InitNetwork()
	} else {
		// Ensure mainnet is properly initialized to avoid state bleeding
		globals.Arguments["--testnet"] = false
		globals.InitNetwork()
	}

	// Convert bytes to big.Int for BNRed
	seed := new(big.Int).SetBytes(keyBytes)
	bnredSeed := crypto.GetBNRed(seed)

	w, err := walletapi.Create_Encrypted_Wallet(file, password, bnredSeed)
	if err != nil {
		return nil, fmt.Errorf("failed to restore wallet from key: %w", err)
	}
	if err := backupWalletFile(file); err != nil {
		log.Warn("wallet", "restore_key.backup_warning", "Failed to create wallet backup", "error", err.Error(), "file", filepath.Base(file))
	}

	return &Wallet{
		wallet:    w,
		file:      file,
		network:   network,
		testnet:   testnet,
		simulator: simulator,
	}, nil
}

// GetDisk returns the underlying Wallet_Disk for use with XSWD server
func (w *Wallet) GetDisk() *walletapi.Wallet_Disk {
	return w.wallet
}

// Close closes the wallet
func (w *Wallet) Close() {
	if w.wallet != nil {
		if err := w.wallet.Save_Wallet(); err != nil {
			log.Warn("wallet", "close.save_warning", "Failed to save wallet before close", "error", err.Error(), "file", filepath.Base(w.file))
		}
		w.wallet.Close_Encrypted_Wallet()
		w.wallet = nil
		if err := backupWalletFile(w.file); err != nil {
			log.Warn("wallet", "close.backup_warning", "Failed to refresh wallet backup on close", "error", err.Error(), "file", filepath.Base(w.file))
		}
	}
}

func backupWalletFile(path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() <= 0 {
		return fmt.Errorf("wallet file is empty")
	}

	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	backupPath := path + ".bak"
	tmpPath := backupPath + ".tmp"

	dst, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	if _, err = io.Copy(dst, src); err != nil {
		dst.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err = dst.Sync(); err != nil {
		dst.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err = dst.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, backupPath)
}

func recoverCorruptWalletFile(path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if info.Size() > 0 {
		return nil
	}

	backupPath := path + ".bak"
	bakInfo, err := os.Stat(backupPath)
	if err != nil || bakInfo.Size() <= 0 {
		return nil
	}

	corruptPath := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
	if err := os.Rename(path, corruptPath); err != nil {
		return err
	}

	src, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err = io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	if err = dst.Sync(); err != nil {
		dst.Close()
		return err
	}
	if err = dst.Close(); err != nil {
		return err
	}

	log.Warn("wallet", "open.recovered", "Recovered wallet file from backup after detecting 0-byte file",
		"file", filepath.Base(path),
		"backup", filepath.Base(backupPath),
		"corrupt", filepath.Base(corruptPath))

	return nil
}

// GetInfo returns wallet information
func (w *Wallet) GetInfo() WalletInfo {
	if w.wallet == nil {
		return WalletInfo{}
	}

	addr := w.wallet.GetAddress().String()
	balance, locked := w.wallet.Get_Balance()

	height := w.wallet.Get_Height()
	topoHeight := w.wallet.Get_TopoHeight()

	// Check online status for THIS wallet connection.
	// Global walletapi.IsDaemonOnline() can be true due to other/previous connections,
	// so require an active daemon address bound to this wallet instance.
	daemonAddr := w.GetDaemonAddress()
	isOnline := daemonAddr != "" && daemonAddr != "Not connected" && walletapi.IsDaemonOnline()
	isRegistered := w.wallet.IsRegistered()

	// Use global daemon height function (maintained by background sync)
	daemonHeight := uint64(walletapi.Get_Daemon_Height())
	isSynced := isOnline && height >= daemonHeight && daemonHeight > 0

	return WalletInfo{
		Address:       addr,
		Balance:       balance,
		LockedBalance: locked,
		Height:        height,
		DaemonHeight:  daemonHeight,
		TopoHeight:    topoHeight,
		IsOnline:      isOnline,
		IsSynced:      isSynced,
		IsRegistered:  isRegistered,
		Network:       w.network,
		DaemonAddress: daemonAddr,
	}
}

// Register performs account registration and dispatches the registration transaction.
func (w *Wallet) Register() (string, error) {
	if w.wallet == nil {
		return "", fmt.Errorf("wallet not open")
	}

	if w.wallet.IsRegistered() {
		return "", nil
	}

	if !walletapi.IsDaemonOnline() {
		return "", fmt.Errorf("daemon is offline")
	}

	if !w.wallet.GetMode() {
		w.wallet.SetOnlineMode()
	}

	// Use all available CPU cores for registration (proof-of-work search).
	// GetRegistrationTX() performs CPU-intensive PoW; utilizing all cores maximizes hash rate.
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}

	resultCh := make(chan *transaction.Transaction, 1)
	done := make(chan struct{})
	var stopOnce sync.Once

	for i := 0; i < workers; i++ {
		go func() {
			// Small initial stagger to reduce lock contention on startup.
			time.Sleep(time.Duration(i) * 5 * time.Millisecond)
			for {
				select {
				case <-done:
					return
				default:
				}

				regTx := w.wallet.GetRegistrationTX()
				hash := regTx.GetHash()
				if hash[0] == 0 && hash[1] == 0 && hash[2] == 0 {
					select {
					case resultCh <- regTx:
						stopOnce.Do(func() { close(done) })
					default:
					}
					return
				}
				// Prevent busy-spin: very short yield every iteration
				// to allow other goroutines and the OS scheduler to breathe.
				runtime.Gosched()
			}
		}()
	}

	regTx := <-resultCh
	if err := w.wallet.SendTransaction(regTx); err != nil {
		return "", fmt.Errorf("failed to dispatch registration transaction: %w", err)
	}

	return regTx.GetHash().String(), nil
}

// GetSeed returns the wallet seed
func (w *Wallet) GetSeed() string {
	if w.wallet == nil {
		return ""
	}
	return w.wallet.GetSeed()
}

// GetHexKey returns the wallet's secret key as hex string
func (w *Wallet) GetHexKey() string {
	if w.wallet == nil {
		return ""
	}
	keys := w.wallet.Get_Keys()
	if keys.Secret == nil {
		return ""
	}
	// Get the BigInt and convert to 32-byte hex (64 characters)
	secretBytes := keys.Secret.BigInt().Bytes()
	// Pad to 32 bytes if needed
	padded := make([]byte, 32)
	copy(padded[32-len(secretBytes):], secretBytes)
	return hex.EncodeToString(padded)
}

// GetTransactions returns recent transactions
func (w *Wallet) GetTransactions(count int) []TransactionInfo {
	if w.wallet == nil {
		return nil
	}

	// Check if we can use cached transactions (thread-safe read)
	w.txCacheMu.RLock()
	currentHeight := w.wallet.Get_Height()
	currentTopo := w.wallet.Get_TopoHeight()
	cacheTTL := TxCacheTTL
	if w.simulator && cacheTTL > 2*time.Second {
		cacheTTL = 2 * time.Second
	}
	cacheValid := w.txCache != nil &&
		w.txCacheHeight == currentHeight &&
		w.txCacheTopo == currentTopo &&
		time.Since(w.txCacheTime) < cacheTTL
	if cacheValid {
		// Return a copy of cached data to avoid data races
		result := make([]TransactionInfo, len(w.txCache))
		copy(result, w.txCache)
		w.txCacheMu.RUnlock()
		if len(result) <= count {
			return result
		}
		return result[:count]
	}
	w.txCacheMu.RUnlock()

	// Sync with daemon first to pick up outgoing transactions
	// Must check GetMode() (wallet online mode) AND IsDaemonOnline() - same as original CLI
	var scid crypto.Hash
	if w.wallet.GetMode() && walletapi.IsDaemonOnline() {
		if err := w.wallet.Sync_Wallet_Memory_With_Daemon(); err != nil {
			log.Warn("wallet", "sync.warning", "Wallet sync warning", "error", err.Error())
		}
	}

	entries := w.wallet.Show_Transfers(scid, true, true, true, 0, 0, "", "", 0, 0)

	var txs []TransactionInfo
	for i := len(entries) - 1; i >= 0 && len(txs) < count; i-- {
		e := entries[i]

		var amount int64
		if e.Coinbase || e.Incoming {
			amount = int64(e.Amount)
		} else {
			amount = -int64(e.Amount)
		}

		// Extract message and ports from payload if present
		var message string
		destinationPort := e.DestinationPort
		sourcePort := e.SourcePort
		valueTransfer := uint64(0)

		// For incoming transactions, if destination is empty, use wallet address
		destination := e.Destination
		if e.Incoming && destination == "" {
			destination = w.wallet.GetAddress().String()
		}

		// Try to extract data from the payload
		// The payload may be zero-padded, so we need to trim trailing zeros
		if len(e.Payload) > 0 {
			// Trim trailing zeros (padding added by CheckPack)
			payload := e.Payload
			for len(payload) > 0 && payload[len(payload)-1] == 0 {
				payload = payload[:len(payload)-1]
			}

			if len(payload) > 0 {
				var args rpc.Arguments
				if err := args.UnmarshalBinary(payload); err == nil {
					if args.Has(rpc.RPC_COMMENT, rpc.DataString) {
						message = args.Value(rpc.RPC_COMMENT, rpc.DataString).(string)
					}
					if args.Has(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64) {
						valueTransfer = args.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64)
					}
					// Extract destination port if not already set
					if destinationPort == 0 && args.Has(rpc.RPC_DESTINATION_PORT, rpc.DataUint64) {
						destinationPort = args.Value(rpc.RPC_DESTINATION_PORT, rpc.DataUint64).(uint64)
					}
					// Extract source port if not already set
					if sourcePort == 0 && args.Has(rpc.RPC_SOURCE_PORT, rpc.DataUint64) {
						sourcePort = args.Value(rpc.RPC_SOURCE_PORT, rpc.DataUint64).(uint64)
					}

					// Reconstruct integrated address for display if payload has integrated args
					if hasIntegratedArgs(args) {
						if intAddr, err := w.reconstructIntegratedAddress(destination, args); err == nil {
							destination = intAddr
						}
					}
				} else {
					// CBOR may have extraneous data - try to parse error and truncate
					errStr := err.Error()
					if idx := strings.Index(errStr, "starting at index "); idx != -1 {
						var validLen int
						fmt.Sscanf(errStr[idx:], "starting at index %d", &validLen)
						if validLen > 0 && validLen < len(payload) {
							// Retry with truncated payload
							payload = payload[:validLen]
							if err2 := args.UnmarshalBinary(payload); err2 == nil {
								if args.Has(rpc.RPC_COMMENT, rpc.DataString) {
									message = args.Value(rpc.RPC_COMMENT, rpc.DataString).(string)
								}
								if args.Has(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64) {
									valueTransfer = args.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64)
								}
								if destinationPort == 0 && args.Has(rpc.RPC_DESTINATION_PORT, rpc.DataUint64) {
									destinationPort = args.Value(rpc.RPC_DESTINATION_PORT, rpc.DataUint64).(uint64)
								}
								if sourcePort == 0 && args.Has(rpc.RPC_SOURCE_PORT, rpc.DataUint64) {
									sourcePort = args.Value(rpc.RPC_SOURCE_PORT, rpc.DataUint64).(uint64)
								}

								// Reconstruct integrated address for display if payload has integrated args
								if hasIntegratedArgs(args) {
									if intAddr, err := w.reconstructIntegratedAddress(destination, args); err == nil {
										destination = intAddr
									}
								}
							}
						}
					}
				}
			}
		}

		// Some outgoing/incoming transfers can report e.Amount as 0 while payload carries RPC_VALUE_TRANSFER.
		// Use the payload value so Recent Activity and History show actual transfer amounts.
		if valueTransfer > 0 && amount == 0 {
			if e.Coinbase || e.Incoming {
				amount = int64(valueTransfer)
			} else {
				amount = -int64(valueTransfer)
			}
		}

		txs = append(txs, TransactionInfo{
			TxID:            e.TXID,
			Amount:          amount,
			Fee:             e.Fees,
			Height:          e.Height,
			TopoHeight:      e.TopoHeight,
			Timestamp:       e.Time.Unix(),
			Destination:     destination,
			Coinbase:        e.Coinbase,
			Incoming:        e.Incoming,
			BlockHash:       e.BlockHash,
			Proof:           e.Proof,
			Sender:          e.Sender,
			Burn:            e.Burn,
			DestinationPort: destinationPort,
			SourcePort:      sourcePort,
			Status:          e.Status,
			Message:         message,
		})
	}

	// Update cache (thread-safe write)
	w.txCacheMu.Lock()
	w.txCache = make([]TransactionInfo, len(txs))
	copy(w.txCache, txs)
	w.txCacheHeight = currentHeight
	w.txCacheTopo = currentTopo
	w.txCacheTime = time.Now()
	w.txCacheMu.Unlock()

	return txs
}

// InvalidateTxCache clears the transaction cache (call after sending transactions)
func (w *Wallet) InvalidateTxCache() {
	w.txCacheMu.Lock()
	w.txCache = nil
	w.txCacheHeight = 0
	w.txCacheTopo = 0
	w.txCacheTime = time.Time{}
	w.txCacheMu.Unlock()
}

// Transfer sends DERO to a destination
func (w *Wallet) Transfer(params TransferParams) TransferResult {
	transferStart := time.Now()

	if w.wallet == nil {
		log.Error("wallet", "transfer.failed", "Transfer failed: wallet not open")
		return TransferResult{Error: "wallet not open"}
	}

	// Check if wallet is online
	if !walletapi.IsDaemonOnline() {
		log.Error("wallet", "transfer.failed", "Transfer failed: daemon offline")
		return TransferResult{Error: "daemon not connected - cannot send transaction"}
	}

	// Log current balance for debugging
	balance, _ := w.wallet.Get_Balance()

	// Need to leave room for fee (minimum ~0.00001 DERO = 1000 atomic units, but use 2000 to be safe)
	minFee := uint64(2000)
	if params.Amount+minFee > balance {
		log.Error("wallet", "transfer.failed", "Transfer failed: insufficient balance",
			"amount", log.FormatAmount(params.Amount),
			"balance", log.FormatAmount(balance))
		return TransferResult{Error: fmt.Sprintf("insufficient balance: have %.5f, need %.5f + fee",
			float64(balance)/100000, float64(params.Amount)/100000)}
	}

	// Validate destination address and parse for integrated address handling
	addr, err := globals.ParseValidateAddress(params.Destination)
	if err != nil {
		log.Error("wallet", "transfer.failed", "Transfer failed: invalid address", "error", err.Error())
		return TransferResult{Error: fmt.Sprintf("invalid address: %v", err)}
	}

	// Validate ringsize - must be power of 2 between 2 and 128
	ringsize := params.Ringsize
	if ringsize == 0 {
		if w.simulator {
			ringsize = 2 // Simulator often has limited ring members
		} else {
			ringsize = 16 // Default to 16 (Recommended)
		}
	} else if ringsize > 128 {
		ringsize = 128
	} else if !isPowerOf2(int(ringsize)) {
		if w.simulator {
			ringsize = 2
		} else {
			ringsize = 16
		}
	}

	// Build RPC arguments for message/comment and payment ID
	var arguments rpc.Arguments

	// Handle integrated addresses - extract embedded arguments
	if addr.IsIntegratedAddress() {
		// Copy arguments from integrated address
		for _, arg := range addr.Arguments {
			arguments = append(arguments, arg)
		}

		// Validate the integrated address arguments
		if err := arguments.Validate_Arguments(); err != nil {
			log.Error("wallet", "transfer.failed", "Transfer failed: invalid integrated address arguments", "error", err.Error())
			return TransferResult{Error: fmt.Sprintf("integrated address has invalid arguments: %v", err)}
		}
	} else if params.PaymentID != 0 {
		// Non-integrated address with manual Payment ID - add as RPC_DESTINATION_PORT
		arguments = append(arguments, rpc.Argument{
			Name:     rpc.RPC_DESTINATION_PORT,
			DataType: rpc.DataUint64,
			Value:    params.PaymentID,
		})
	}

	// Handle message/comment - user clearing the message means NO comment
	if params.Message != "" {
		// User provided a message - add/replace any existing comment
		if arguments.Has(rpc.RPC_COMMENT, rpc.DataString) {
			// Remove existing comment to replace with user's message
			newArgs := make(rpc.Arguments, 0, len(arguments))
			for _, arg := range arguments {
				if arg.Name != rpc.RPC_COMMENT {
					newArgs = append(newArgs, arg)
				}
			}
			arguments = newArgs
		}
		arguments = append(arguments, rpc.Argument{
			Name:     rpc.RPC_COMMENT,
			DataType: rpc.DataString,
			Value:    params.Message,
		})
	} else {
		// User left message empty - remove any embedded comment from integrated address
		if arguments.Has(rpc.RPC_COMMENT, rpc.DataString) {
			newArgs := make(rpc.Arguments, 0, len(arguments))
			for _, arg := range arguments {
				if arg.Name != rpc.RPC_COMMENT {
					newArgs = append(newArgs, arg)
				}
			}
			arguments = newArgs
		}
	}

	// Validate payload size (144 byte limit for PAYLOAD0)
	if len(arguments) > 0 {
		if _, err := arguments.CheckPack(144); err != nil {
			log.Error("wallet", "transfer.failed", "Transfer failed: payload arguments too large", "error", err.Error())
			return TransferResult{Error: fmt.Sprintf("payload arguments too large (max 144 bytes): %v", err)}
		}
	}

	// Build transfer
	transfers := []rpc.Transfer{
		{
			Amount:      params.Amount,
			Destination: params.Destination,
			Payload_RPC: arguments,
		},
	}

	// Execute transfer with ringsize
	buildStart := time.Now()
	tx, err := w.wallet.TransferPayload0(transfers, ringsize, false, rpc.Arguments{}, 0, false)
	buildDuration := time.Since(buildStart)
	if err != nil {
		errStr := err.Error()
		// Provide more helpful error messages
		if strings.Contains(errStr, "verification failed") {
			// Check if it's a balance issue
			if params.Amount >= balance {
				return TransferResult{Error: fmt.Sprintf("insufficient funds (need amount + fee, have %d atomic)", balance)}
			}
			return TransferResult{Error: fmt.Sprintf("TX failed: %v (balance=%d, amount=%d, try smaller amount for fees)", err, balance, params.Amount)}
		}
		log.Error("wallet", "transfer.failed", "Transfer failed", "error", err.Error())
		return TransferResult{Error: fmt.Sprintf("transfer failed: %v", err)}
	}

	txID := tx.GetHash().String()
	log.Info("wallet", "transfer.created", "Transaction created", "txid", log.TruncateID(txID), "build_ms", fmt.Sprintf("%d", buildDuration.Milliseconds()))

	// Dispatch the transaction to the daemon (CRITICAL: TransferPayload0 only creates locally)
	dispatchStart := time.Now()
	if err = w.wallet.SendTransaction(tx); err != nil {
		log.Error("wallet", "transfer.dispatch_failed", "Failed to dispatch transaction", "txid", log.TruncateID(txID), "error", err.Error())
		return TransferResult{Error: fmt.Sprintf("failed to dispatch transaction: %v", err)}
	}
	dispatchDuration := time.Since(dispatchStart)
	totalDuration := time.Since(transferStart)

	log.Info("wallet", "transfer.success", "Transfer dispatched successfully", "txid", log.TruncateID(txID), "amount", log.FormatAmount(params.Amount), "dispatch_ms", fmt.Sprintf("%d", dispatchDuration.Milliseconds()), "total_ms", fmt.Sprintf("%d", totalDuration.Milliseconds()))

	// Invalidate transaction cache so next fetch picks up the new transaction
	w.InvalidateTxCache()

	return TransferResult{
		TxID:   txID,
		Status: "success",
	}
}

// isPowerOf2 checks if a number is a power of 2
func isPowerOf2(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// SetDaemon sets the daemon address
func (w *Wallet) SetDaemon(address string) error {
	if w.wallet == nil {
		return fmt.Errorf("wallet not open")
	}
	w.wallet.SetDaemonAddress(address)
	return nil
}

// ChangePassword changes the wallet password
func (w *Wallet) ChangePassword(newPass string) error {
	if w.wallet == nil {
		return fmt.Errorf("wallet not open")
	}

	// Change the password (internally saves to disk)
	_ = w.wallet.Set_Encrypted_Wallet_Password(newPass)

	// Small delay to let any in-flight sync operations complete
	time.Sleep(200 * time.Millisecond)

	// Force another save to ensure our password change is the final state on disk
	if err := w.wallet.Save_Wallet(); err != nil {
		return fmt.Errorf("failed to save wallet: %w", err)
	}

	// Verify the password change actually worked
	if !w.wallet.Check_Password(newPass) {
		return fmt.Errorf("password change failed verification")
	}

	return nil
}

// CheckPassword verifies if the given password is correct for the open wallet
func (w *Wallet) CheckPassword(password string) bool {
	if w.wallet == nil {
		return false
	}
	return w.wallet.Check_Password(password)
}

// GetFileName returns the wallet file name
func (w *Wallet) GetFileName() string {
	return filepath.Base(w.file)
}

// GetNetworkType returns the wallet's network type as a string
func (w *Wallet) GetNetworkType() string {
	if w.simulator {
		return "simulator"
	} else if w.testnet {
		return "testnet"
	}
	return "mainnet"
}

// IsTestnet returns true if wallet is on testnet
func (w *Wallet) IsTestnet() bool {
	return w.testnet
}

// IsSimulator returns true if wallet is on simulator
func (w *Wallet) IsSimulator() bool {
	return w.simulator
}

// FormatTimestamp formats a unix timestamp for display
func FormatTimestamp(ts int64) string {
	t := time.Unix(ts, 0)
	return t.Format("2006-01-02")
}

// CheckDaemon checks if a daemon is reachable at the given address
func CheckDaemon(address string) bool {
	normalized, err := NormalizeDaemonAddress(address)
	if err != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", normalized, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// CheckDaemonFast checks if a daemon is reachable with a shorter timeout
func CheckDaemonFast(address string) bool {
	normalized, err := NormalizeDaemonAddress(address)
	if err != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", normalized, 800*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ConnectToLocalDaemonFast connects to local daemon that matches wallet's network.
// Returns connection status and error message if daemon not available.
func (w *Wallet) ConnectToLocalDaemonFast(knownHealthy bool, knownAddress string) (connected bool, errMsg string) {
	// Recover from any panics in the walletapi library
	defer func() {
		if r := recover(); r != nil {
			connected = false
			errMsg = "Connection failed (internal error)"
		}
	}()

	if w.wallet == nil {
		return false, "Wallet not open"
	}

	// Determine expected daemon address based on wallet's network.
	// This is used when no explicit daemon address is provided.
	expectedDaemon := DefaultMainnetDaemon
	if w.simulator {
		expectedDaemon = DefaultSimulatorDaemon
	} else if w.testnet {
		expectedDaemon = DefaultTestnetDaemon
	}

	// If a known daemon address is provided, prefer it.
	// Network compatibility is validated below using daemon RPC info.
	daemon := expectedDaemon
	explicitDaemon := knownAddress != ""
	if knownAddress != "" {
		daemon = knownAddress
	}
	normalizedDaemon, normalizeErr := NormalizeDaemonAddress(daemon)
	if normalizeErr != nil {
		return false, normalizeErr.Error()
	}
	daemon = normalizedDaemon

	// Check if target daemon is healthy BEFORE attempting any switch
	// This prevents Keep_Connectivity from reconnecting to a non-existent daemon
	_ = knownHealthy
	info := GetDaemonInfo(context.Background(), daemon)
	if !info.IsHealthy && !explicitDaemon && !w.simulator && !w.testnet {
		fallbackInfo := GetDaemonInfo(context.Background(), FallbackMainnetDaemon)
		if fallbackInfo.IsHealthy {
			log.Info("daemon", "connect.fallback", "Using fallback mainnet daemon",
				"from", daemon,
				"to", FallbackMainnetDaemon)
			daemon = FallbackMainnetDaemon
			info = fallbackInfo
		}
	}
	if !info.IsHealthy && !explicitDaemon && w.testnet && !w.simulator {
		fallbackInfo := GetDaemonInfo(context.Background(), FallbackTestnetDaemon)
		if fallbackInfo.IsHealthy {
			log.Info("daemon", "connect.fallback", "Using fallback testnet daemon",
				"from", daemon,
				"to", FallbackTestnetDaemon)
			daemon = FallbackTestnetDaemon
			info = fallbackInfo
		}
	}
	if !info.IsHealthy {
		networkName := "mainnet"
		if w.simulator {
			networkName = "simulator"
		} else if w.testnet {
			networkName = "testnet"
		}
		if !explicitDaemon && networkName == "mainnet" {
			return false, fmt.Sprintf("%s daemon not available at %s or %s", networkName, expectedDaemon, FallbackMainnetDaemon)
		}
		if !explicitDaemon && networkName == "testnet" {
			return false, fmt.Sprintf("%s daemon not available at %s or %s", networkName, expectedDaemon, FallbackTestnetDaemon)
		}
		log.Warn("daemon", "connect.failed", "Daemon not available",
			"network", networkName,
			"daemon", daemon)
		return false, fmt.Sprintf("%s daemon not available at %s", networkName, daemon)
	}

	// Check if we need to switch daemons (e.g., opening simulator wallet after mainnet)
	currentEndpoint := walletapi.Daemon_Endpoint_Active
	if currentEndpoint != "" && currentEndpoint != daemon {
		log.Info("daemon", "connect.switching", "Switching daemon",
			"from", currentEndpoint,
			"to", daemon)
	}

	// Verify daemon network matches wallet network
	// Daemon can be: Mainnet, Testnet, Mainnet-Simulator (dero1), or Testnet-Simulator (deto1)
	daemonIsSimulator := info.Network == "Simulator"
	daemonIsTestnet := info.Testnet

	if w.simulator && !daemonIsSimulator {
		return false, fmt.Sprintf("Wallet is Simulator but daemon at %s is not", daemon)
	}
	if !w.simulator && daemonIsSimulator {
		return false, fmt.Sprintf("Wallet is not Simulator but daemon at %s is Simulator", daemon)
	}
	if w.testnet && !daemonIsTestnet {
		return false, fmt.Sprintf("Wallet is Testnet but daemon at %s is Mainnet", daemon)
	}
	if !w.testnet && daemonIsTestnet {
		return false, fmt.Sprintf("Wallet is Mainnet but daemon at %s is Testnet", daemon)
	}

	// Update globals to match wallet network
	globals.Arguments["--testnet"] = w.testnet
	globals.InitNetwork()

	// Set the active daemon endpoint globally
	walletapi.Daemon_Endpoint_Active = daemon
	// Establish WebSocket connection to daemon
	if err := walletapi.Connect(daemon); err != nil {
		errStr := err.Error()
		// Provide clearer error message for network mismatch
		if strings.Contains(errStr, "Mainnet/TestNet is different") || strings.Contains(errStr, "different between") {
			// Determine daemon network from the address
			daemonNetwork := "Mainnet"
			suggestedFlag := ""
			if strings.Contains(daemon, "20000") {
				daemonNetwork = "Simulator"
				suggestedFlag = "--simulator"
			} else if strings.Contains(daemon, "40402") {
				daemonNetwork = "Testnet"
				suggestedFlag = "--testnet"
			}
			if suggestedFlag != "" {
				return false, fmt.Sprintf("Network mismatch: daemon is %s. Restart app with %s flag to use this wallet", daemonNetwork, suggestedFlag)
			}
			return false, fmt.Sprintf("Network mismatch between wallet and daemon at %s", daemon)
		}
		return false, fmt.Sprintf("Failed to connect to daemon: %v", err)
	}
	w.wallet.SetDaemonAddress(daemon)
	w.wallet.SetNetwork(!w.testnet)
	w.wallet.SetOnlineMode()
	w.wallet.Sync_Wallet_Memory_With_Daemon()
	w.daemonAddress = daemon

	// Update network string
	if w.simulator {
		w.network = "Simulator"
	} else if w.testnet {
		w.network = "Testnet"
	} else {
		w.network = "Mainnet"
	}

	return true, ""
}

// GetDaemonAddress returns the current daemon address
func (w *Wallet) GetDaemonAddress() string {
	if w.wallet == nil {
		return ""
	}
	if w.daemonAddress != "" {
		return w.daemonAddress
	}
	return "Not connected"
}

// ClearDaemonAddress clears the cached daemon address
func (w *Wallet) ClearDaemonAddress() {
	w.daemonAddress = ""
}

// DaemonInfo contains basic daemon information
type DaemonInfo struct {
	Height     uint64
	TopoHeight int64
	IsOnline   bool
	IsSynced   bool
	IsHealthy  bool   // true if daemon responds without errors
	Testnet    bool   // true if daemon is running testnet
	Network    string // "Simulator" if simulator mode, empty otherwise
}

// TxStatus contains daemon-side status for a transaction hash.
type TxStatus struct {
	Found     bool
	Status    string
	InPool    bool
	Confirmed bool
	Rejected  bool
}

// GetTxStatus attempts to query daemon state for a transaction hash.
func GetTxStatus(ctx context.Context, address, txID string) (TxStatus, error) {
	status := TxStatus{Found: false, Status: "not found"}
	if address == "" || txID == "" {
		return status, nil
	}

	reqBody := fmt.Sprintf(`{"jsonrpc":"2.0","id":"1","method":"DERO.GetTransaction","params":{"txs_hashes":["%s"]}}`, txID)
	rpcURL, err := daemonRPCURL(address)
	if err != nil {
		return status, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, strings.NewReader(reqBody))
	if err != nil {
		return status, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return status, err
	}
	defer resp.Body.Close()

	var rpcResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return status, err
	}

	if errField, ok := rpcResp["error"]; ok && errField != nil {
		errData, _ := errField.(map[string]interface{})
		if msg, ok := errData["message"].(string); ok {
			lower := strings.ToLower(msg)
			if strings.Contains(lower, "not found") {
				return status, nil
			}
			return status, fmt.Errorf("%s", msg)
		}
		return status, fmt.Errorf("daemon returned tx lookup error")
	}

	result, ok := rpcResp["result"].(map[string]interface{})
	if !ok {
		return status, nil
	}

	if state, ok := result["status"].(string); ok {
		lower := strings.ToLower(state)
		if strings.Contains(lower, "not found") {
			return status, nil
		}
	}

	txsRaw, ok := result["txs"].([]interface{})
	if !ok || len(txsRaw) == 0 {
		if _, hasHex := result["txs_as_hex"]; hasHex {
			status.Found = true
			status.Status = "seen by daemon"
		}
		return status, nil
	}

	txMap, ok := txsRaw[0].(map[string]interface{})
	if !ok {
		return status, nil
	}

	status.Found = true

	if b, ok := txMap["in_pool"].(bool); ok && b {
		status.InPool = true
		status.Status = "in pool"
		return status, nil
	}

	if b, ok := txMap["rejected"].(bool); ok && b {
		status.Rejected = true
		status.Status = "rejected"
		return status, nil
	}
	if b, ok := txMap["invalid_block"].(bool); ok && b {
		status.Rejected = true
		status.Status = "rejected"
		return status, nil
	}

	if h, ok := txMap["block_height"].(float64); ok && h > 0 {
		status.Confirmed = true
		status.Status = fmt.Sprintf("confirmed @%d", uint64(h))
		return status, nil
	}

	if b, ok := txMap["valid_block"].(string); ok && b != "" {
		status.Confirmed = true
		status.Status = "confirmed"
		return status, nil
	}

	status.Status = "seen by daemon"
	return status, nil
}

// GetDaemonInfo queries a daemon for its current info without needing a wallet
func GetDaemonInfo(ctx context.Context, address string) DaemonInfo {
	if address == "" {
		return DaemonInfo{}
	}
	normalized, err := NormalizeDaemonAddress(address)
	if err != nil {
		return DaemonInfo{}
	}

	now := time.Now()
	daemonInfoMu.RLock()
	if cached, ok := daemonInfoMemo[normalized]; ok && now.Sub(cached.fetchedAt) < DaemonInfoCacheTTL {
		daemonInfoMu.RUnlock()
		return cached.info
	}
	daemonInfoMu.RUnlock()

	info := DaemonInfo{}

	// Check if daemon is reachable first
	if !CheckDaemon(normalized) {
		daemonInfoMu.Lock()
		daemonInfoMemo[normalized] = daemonInfoCacheEntry{info: info, fetchedAt: now}
		daemonInfoMu.Unlock()
		return info
	}

	// Build JSON-RPC request
	reqBody := `{"jsonrpc":"2.0","id":"1","method":"DERO.GetInfo"}`
	rpcURL, err := daemonRPCURL(normalized)
	if err != nil {
		daemonInfoMu.Lock()
		daemonInfoMemo[normalized] = daemonInfoCacheEntry{info: info, fetchedAt: now}
		daemonInfoMu.Unlock()
		return info
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, strings.NewReader(reqBody))
	if err != nil {
		daemonInfoMu.Lock()
		daemonInfoMemo[normalized] = daemonInfoCacheEntry{info: info, fetchedAt: now}
		daemonInfoMu.Unlock()
		return info
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		daemonInfoMu.Lock()
		daemonInfoMemo[normalized] = daemonInfoCacheEntry{info: info, fetchedAt: now}
		daemonInfoMu.Unlock()
		return info
	}
	defer resp.Body.Close()

	var result struct {
		Error  *json.RawMessage `json:"error"`
		Result struct {
			Height     uint64 `json:"height"`
			TopoHeight int64  `json:"topoheight"`
			Status     string `json:"status"`
			Testnet    bool   `json:"testnet"`
			Network    string `json:"network"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		daemonInfoMu.Lock()
		daemonInfoMemo[normalized] = daemonInfoCacheEntry{info: info, fetchedAt: now}
		daemonInfoMu.Unlock()
		return info
	}

	// Check if daemon returned an error (including panic responses)
	if result.Error != nil {
		info.IsOnline = true // Daemon is reachable but unhealthy
		info.IsHealthy = false
		daemonInfoMu.Lock()
		daemonInfoMemo[normalized] = daemonInfoCacheEntry{info: info, fetchedAt: now}
		daemonInfoMu.Unlock()
		return info
	}

	info.Height = result.Result.Height
	info.TopoHeight = result.Result.TopoHeight
	info.IsOnline = true
	info.IsHealthy = true
	info.IsSynced = result.Result.Status == "OK"
	info.Testnet = result.Result.Testnet
	info.Network = result.Result.Network

	daemonInfoMu.Lock()
	daemonInfoMemo[normalized] = daemonInfoCacheEntry{info: info, fetchedAt: now}
	daemonInfoMu.Unlock()

	return info
}

// ExportHistory exports transaction history to JSON files in the specified directory
// Creates dero.json for native DERO and <scid>.json for other tokens
// Returns the number of files exported and any error encountered
func (w *Wallet) ExportHistory(dir string) (int, error) {
	if w.wallet == nil {
		return 0, fmt.Errorf("wallet not open")
	}

	// Create directory if it doesn't exist
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(dir, 0700); err != nil {
			return 0, fmt.Errorf("error creating directory: %w", err)
		}
	}

	var zeroscid crypto.Hash
	account := w.wallet.GetAccount()
	exported := 0

	for k, v := range account.EntriesNative {
		filename := filepath.Join(dir, k.String()+".json")
		if k == zeroscid {
			filename = filepath.Join(dir, "dero.json")
		}

		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return exported, fmt.Errorf("error marshaling data: %w", err)
		}

		if err = os.WriteFile(filename, data, 0600); err != nil {
			return exported, fmt.Errorf("error writing file: %w", err)
		}
		exported++
	}

	return exported, nil
}

// IsDaemonHealthy checks if daemon is responding correctly without errors
// This validates that the daemon can handle RPC requests without panicking
func IsDaemonHealthy(ctx context.Context, address string) bool {
	normalized, err := NormalizeDaemonAddress(address)
	if err != nil {
		return false
	}
	// First check TCP connectivity
	if !CheckDaemonFast(normalized) {
		return false
	}

	// Make HTTP JSON-RPC request
	reqBody := `{"jsonrpc":"2.0","id":"1","method":"DERO.GetInfo"}`
	rpcURL, err := daemonRPCURL(normalized)
	if err != nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, strings.NewReader(reqBody))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Parse response and check for error field
	var result struct {
		Error  *json.RawMessage `json:"error"`
		Result *json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	// If there's an error field (including panic responses), daemon is unhealthy
	if result.Error != nil {
		return false
	}

	// Must have a valid result
	return result.Result != nil
}

// hasIntegratedArgs checks if the arguments contain integrated address fields
func hasIntegratedArgs(args rpc.Arguments) bool {
	// Check for common integrated address arguments
	integratedArgNames := []string{
		rpc.RPC_DESTINATION_PORT,
		rpc.RPC_COMMENT,
		rpc.RPC_VALUE_TRANSFER,
		rpc.RPC_EXPIRY,
		rpc.RPC_NEEDS_REPLYBACK_ADDRESS,
	}

	for _, arg := range args {
		for _, name := range integratedArgNames {
			if arg.Name == name {
				return true
			}
		}
	}
	return false
}

// reconstructIntegratedAddress creates an integrated address string from base address and arguments
func (w *Wallet) reconstructIntegratedAddress(baseAddr string, args rpc.Arguments) (string, error) {
	if w.wallet == nil {
		return baseAddr, fmt.Errorf("wallet not initialized")
	}

	// Parse the base address
	addr, err := globals.ParseValidateAddress(baseAddr)
	if err != nil {
		return baseAddr, err
	}

	// Build integrated address arguments from payload args
	// Only include arguments that are part of integrated address standard
	var intArgs rpc.Arguments
	for _, arg := range args {
		switch arg.Name {
		case rpc.RPC_DESTINATION_PORT,
			rpc.RPC_COMMENT,
			rpc.RPC_VALUE_TRANSFER,
			rpc.RPC_EXPIRY,
			rpc.RPC_NEEDS_REPLYBACK_ADDRESS:
			intArgs = append(intArgs, arg)
		}
	}

	if len(intArgs) == 0 {
		return baseAddr, nil
	}

	// Set arguments and encode
	addr.Arguments = intArgs
	if _, err := addr.MarshalText(); err != nil {
		return baseAddr, err
	}

	return addr.String(), nil
}
