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
	"unicode"

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
// For URL inputs, the scheme is preserved so websocket transport can
// correctly choose ws:// vs wss:// during wallet daemon connection.
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
		if u.Path != "" && u.Path != "/" {
			return "", fmt.Errorf("daemon URL must not include path")
		}
		if u.RawQuery != "" || u.Fragment != "" {
			return "", fmt.Errorf("daemon URL must not include query or fragment")
		}
		port := u.Port()
		if port == "" {
			if u.Scheme == "https" {
				port = "443"
			} else {
				port = "80"
			}
		}
		return u.Scheme + "://" + net.JoinHostPort(host, port), nil
	}

	if _, _, err := net.SplitHostPort(raw); err != nil {
		return "", fmt.Errorf("invalid daemon address %q: expected host:port", raw)
	}

	return raw, nil
}

func daemonHostPort(address string) (string, error) {
	normalized, err := NormalizeDaemonAddress(address)
	if err != nil {
		return "", err
	}
	if strings.Contains(normalized, "://") {
		u, err := url.Parse(normalized)
		if err != nil {
			return "", fmt.Errorf("invalid daemon URL: %w", err)
		}
		if u.Host == "" {
			return "", fmt.Errorf("daemon URL missing host")
		}
		return u.Host, nil
	}
	return normalized, nil
}

func daemonRPCURL(address string) (string, error) {
	normalized, err := NormalizeDaemonAddress(address)
	if err != nil {
		return "", err
	}
	if strings.Contains(normalized, "://") {
		return normalized + "/json_rpc", nil
	}
	return "http://" + normalized + "/json_rpc", nil
}

const (
	DefaultMainnetDaemon   = "localhost:10102"
	DefaultTestnetDaemon   = "localhost:40402"
	DefaultSimulatorDaemon = "localhost:20000"
	FallbackMainnetDaemon  = "dero.geeko.cloud:10102"
	FallbackTestnetDaemon  = "69.30.234.163:40402"
)

// MainnetPublicDaemons lists public mainnet RPC nodes used as fallbacks when
// no local daemon is running or the local daemon is still bootstrapping.
// Entries are probed in order on the non-explicit connection path only.
var MainnetPublicDaemons = []string{
	"dero.geeko.cloud:10102",
	"213.171.208.37:10102",
	"85.214.253.170:10102",
	"82.65.143.182:10102",
	"178.255.169.125:10102",
	"51.222.86.51:10102",
}

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
	syncMu        sync.Mutex
	syncInFlight  bool

	// Lifecycle control: Close cancels tracked background work and waits
	// for it to finish before releasing the underlying wallet. This prevents
	// background sync/registration goroutines from using a nil wallet.
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeMu   sync.Mutex

	// Cache for transactions to avoid redundant syncs.
	// Protected by txCacheMu - must hold lock when reading/writing cache fields
	txCacheMu     sync.RWMutex
	txCache       []TransactionInfo
	txCacheHeight uint64
	txCacheTopo   int64
	txCacheTime   time.Time
	hyper         *HyperGnomon
}

// newWallet constructs a Wallet bound to the given network config and equips
// it with a lifecycle context used to cancel background work on Close.
func newWallet(w *walletapi.Wallet_Disk, file, network string, testnet, simulator bool) *Wallet {
	ctx, cancel := context.WithCancel(context.Background())
	return &Wallet{
		wallet:    w,
		file:      file,
		network:   network,
		testnet:   testnet,
		simulator: simulator,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Context returns the wallet's lifecycle context. It is cancelled when the
// wallet is closed. Callers deriving operation contexts from it automatically
// abort background work on Close.
func (w *Wallet) Context() context.Context {
	if w == nil || w.ctx == nil {
		return context.Background()
	}
	return w.ctx
}

// trackBackground registers one unit of background work with the wallet's
// WaitGroup. Returns false (without registering) if the wallet is closing or
// already closed, so the caller can skip starting the goroutine. Holding
// lifecycleMu during Add ensures no Add can race with Close's Wait.
func (w *Wallet) trackBackground() bool {
	w.closeMu.Lock()
	defer w.closeMu.Unlock()
	select {
	case <-w.ctx.Done():
		return false
	default:
	}
	w.wg.Add(1)
	return true
}

func (w *Wallet) syncWalletMemoryAsync() bool {
	if w.wallet == nil {
		return false
	}

	w.syncMu.Lock()
	if w.syncInFlight {
		w.syncMu.Unlock()
		return false
	}
	w.syncInFlight = true
	w.syncMu.Unlock()

	if !w.trackBackground() {
		w.syncMu.Lock()
		w.syncInFlight = false
		w.syncMu.Unlock()
		return false
	}

	// Capture the underlying wallet here so a Close that times out waiting for
	// this goroutine cannot nil-deref w.wallet underneath us.
	wlt := w.wallet
	go func() {
		defer w.wg.Done()
		start := time.Now()
		defer func() {
			w.syncMu.Lock()
			w.syncInFlight = false
			w.syncMu.Unlock()
		}()

		select {
		case <-w.ctx.Done():
			return
		default:
		}

		if err := wlt.Sync_Wallet_Memory_With_Daemon(); err != nil {
			log.Warn("wallet", "sync.warning", "Background wallet sync warning", "error", err.Error())
			return
		}

		log.Debug("wallet", "sync.done", "Background wallet sync completed", "duration", log.FormatDuration(time.Since(start)))
	}()

	return true
}

func (w *Wallet) isSyncInFlight() bool {
	w.syncMu.Lock()
	inFlight := w.syncInFlight
	w.syncMu.Unlock()
	return inFlight
}

func shouldUseCachedTxsDuringSync(started, inFlight bool) bool {
	// If this call started a sync, still fetch transfers once so cache can be
	// populated for recent activity/history. Only short-circuit when another
	// sync was already in progress.
	if started {
		return false
	}
	return inFlight
}

// applyNetwork initializes the DERO network globals for the requested network
// and returns the human-readable network name. Simulator can run in either
// mainnet or testnet mode, so it passes its own testnet flag through.
func applyNetwork(testnet, simulator bool) string {
	network := "Mainnet"
	if simulator {
		network = "Simulator"
		globals.Arguments["--testnet"] = testnet
	} else if testnet {
		network = "Testnet"
		globals.Arguments["--testnet"] = true
	} else {
		// Ensure mainnet is properly initialized to avoid state bleeding between sessions
		globals.Arguments["--testnet"] = false
	}
	globals.InitNetwork()
	return network
}

// Open opens an existing wallet
func Open(file, password string, testnet, simulator bool) (*Wallet, error) {
	start := time.Now()

	// Recover from interrupted writes (e.g. sudden shutdown while wallet file was being updated).
	if err := recoverCorruptWalletFile(file); err != nil {
		log.Warn("wallet", "open.recover_warning", "Wallet recovery check warning", "error", err.Error(), "file", filepath.Base(file))
	}

	// Initialize globals BEFORE opening wallet so DERO library uses correct network
	network := applyNetwork(testnet, simulator)

	w, err := walletapi.Open_Encrypted_Wallet(file, password)
	if err != nil {
		log.Error("wallet", "open.failed", "Failed to open wallet", "error", err.Error(), "file", filepath.Base(file))
		return nil, fmt.Errorf("failed to open wallet: %w", err)
	}

	actualTestnet := testnet
	actualSimulator := simulator

	log.Debug("wallet", "open.network_set", "Network globals initialized",
		"testnet", fmt.Sprintf("%t", actualTestnet),
		"simulator", fmt.Sprintf("%t", actualSimulator),
		"network", network)

	wallet := newWallet(w, file, network, actualTestnet, actualSimulator)

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
	network := applyNetwork(testnet, simulator)

	w, err := walletapi.Create_Encrypted_Wallet_Random(file, password)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create wallet: %w", err)
	}

	seed := w.GetSeed()
	if err := backupWalletFile(file); err != nil {
		log.Warn("wallet", "create.backup_warning", "Failed to create wallet backup", "error", err.Error(), "file", filepath.Base(file))
	}

	return newWallet(w, file, network, testnet, simulator), seed, nil
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
	network := applyNetwork(testnet, simulator)

	w, err := walletapi.Create_Encrypted_Wallet_From_Recovery_Words(file, password, seed)
	if err != nil {
		return nil, fmt.Errorf("failed to restore wallet: %w", err)
	}
	if err := backupWalletFile(file); err != nil {
		log.Warn("wallet", "restore.backup_warning", "Failed to create wallet backup", "error", err.Error(), "file", filepath.Base(file))
	}

	return newWallet(w, file, network, testnet, simulator), nil
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
	network := applyNetwork(testnet, simulator)

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

	return newWallet(w, file, network, testnet, simulator), nil
}

// GetDisk returns the underlying Wallet_Disk for use with XSWD server
func (w *Wallet) GetDisk() *walletapi.Wallet_Disk {
	return w.wallet
}

// Close closes the wallet. It cancels background sync/registration work and
// waits for it to finish before releasing the underlying wallet, preventing
// goroutines from operating on a nil wallet.
// closeWaitTimeout bounds Close's wait for tracked background workers.
// Workers can be stuck inside derohe's no-timeout RPCs or on its global
// sync_multilock; never let that block session shutdown (and the UI thread
// that runs it) indefinitely.
const closeWaitTimeout = 10 * time.Second

// deroheCloseWaitTimeout bounds the derohe teardown steps that contend on the
// underlying wallet's RWMutex (Save_Wallet / Close_Encrypted_Wallet). The
// derohe sync_loop goroutine (never stopped on close — w.Quit is never
// signalled by walletapi) can hold that lock while it sleeps or rescans, so
// these calls can block well past the tracked-worker wait. Without a bound,
// a slow or wedged sync loop freezes the caller — the UI thread that runs
// Esc/quit — for minutes.
const deroheCloseWaitTimeout = 5 * time.Second

func (w *Wallet) Close() {
	if w.wallet == nil {
		return
	}
	w.closeOnce.Do(func() {
		log.Info("wallet", "close.begin", "Closing wallet", "file", filepath.Base(w.file))
		started := time.Now()
		// Hold closeMu so a concurrent trackBackground cannot Add new work
		// after we observe the counter at zero. Workers only call Done, which
		// does not need closeMu, so Wait never deadlocks here.
		w.closeMu.Lock()
		if w.cancel != nil {
			w.cancel()
		}
		waitDone := make(chan struct{})
		go func() {
			w.wg.Wait()
			close(waitDone)
		}()
		select {
		case <-waitDone:
		case <-time.After(closeWaitTimeout):
			log.Warn("wallet", "close.wait_timeout", "Timed out waiting for background tasks; closing anyway", "file", filepath.Base(w.file))
		}
		w.closeMu.Unlock()
		// Save + Close_Encrypted_Wallet contend on derohe's wallet RWMutex.
		// Close_Encrypted_Wallet also sleeps a full second. Run them under a
		// deadline so a wedged sync loop cannot hang the UI thread: on timeout
		// the wallet struct is dropped and OS exit reclaims the resource.
		teardownDone := make(chan struct{})
		go func(disk *walletapi.Wallet_Disk) {
			defer close(teardownDone)
			if err := disk.Save_Wallet(); err != nil {
				log.Warn("wallet", "close.save_warning", "Failed to save wallet before close", "error", err.Error(), "file", filepath.Base(w.file))
			}
			disk.Close_Encrypted_Wallet()
		}(w.wallet)
		select {
		case <-teardownDone:
		case <-time.After(deroheCloseWaitTimeout):
			log.Warn("wallet", "close.teardown_timeout", "Timed out waiting for wallet save/close; dropping wallet handle", "file", filepath.Base(w.file))
		}
		w.CloseHyperGnomon()
		w.wallet = nil
		if err := backupWalletFile(w.file); err != nil {
			log.Warn("wallet", "close.backup_warning", "Failed to refresh wallet backup on close", "error", err.Error(), "file", filepath.Base(w.file))
		}
		log.Info("wallet", "close.success", "Wallet closed", "file", filepath.Base(w.file), "duration", log.FormatDuration(time.Since(started)))
	})
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

	// Use global daemon height function (maintained by background sync).
	// DAG tip may have topo ahead of height by 1-2 blocks; allow small lag as synced.
	daemonHeight := uint64(walletapi.Get_Daemon_Height())
	isSynced := isOnline && daemonHeight > 0 && height+2 >= daemonHeight

	log.Debug("wallet.sync", "getinfo", fmt.Sprintf("height=%d daemon=%d isOnline=%v isSynced=%v isReg=%v daemonAddr=%s",
		height, daemonHeight, isOnline, isSynced, isRegistered, log.TruncateAddress(daemonAddr)))

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
// The supplied context bounds the proof-of-work search: it returns an error
// (including ctx cancellation) instead of blocking forever if no valid PoW is found.
func (w *Wallet) Register(ctx context.Context) (string, error) {
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

	// Capture the underlying wallet before spawning: Close() may nil w.wallet
	// on a timed-out wait while PoW workers are still searching.
	wlt := w.wallet

	for i := 0; i < workers; i++ {
		if !w.trackBackground() {
			return "", fmt.Errorf("wallet is closing")
		}
		go func(i int) {
			defer w.wg.Done()
			// Small initial stagger to reduce lock contention on startup.
			time.Sleep(time.Duration(i) * 5 * time.Millisecond)
			for {
				select {
				case <-done:
					return
				case <-ctx.Done():
					return
				default:
				}

				regTx := wlt.GetRegistrationTX()
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
		}(i)
	}

	select {
	case regTx := <-resultCh:
		stopOnce.Do(func() { close(done) })
		if err := w.wallet.SendTransaction(regTx); err != nil {
			return "", fmt.Errorf("failed to dispatch registration transaction: %w", err)
		}
		return regTx.GetHash().String(), nil
	case <-ctx.Done():
		stopOnce.Do(func() { close(done) })
		return "", fmt.Errorf("registration cancelled: %w", ctx.Err())
	}
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

	// Sync with daemon in background to pick up outgoing transactions.
	// To avoid freezes and lock contention, do not run Show_Transfers while
	// a background sync is in flight.
	//
	// If derohe's own sync loop (started by SetOnlineMode) is already running,
	// do NOT start a redundant tracked sync here: the untracked loop already
	// calls Sync_Wallet_Memory_With_Daemon every ~5s, and a second concurrent
	// sync duplicates that work and can block Close() (its no-timeout RPC call
	// runs in a tracked goroutine that wg.Wait would await).
	var scid crypto.Hash
	if walletapi.IsDaemonOnline() {
		shouldSync := false
		if !w.wallet.GetMode() {
			shouldSync = true
		} else {
			// Online but wallet is behind daemon tip: the 5s sync_loop can be
			// stalled by transient RPC failures (public node rate-limit, websocket
			// reconnect). Kick an extra tracked sync so the HUD does not freeze at
			// SYNCING while a new block is available.
			daemonH := uint64(walletapi.Get_Daemon_Height())
			walletH := w.wallet.Get_Height()
			if daemonH > 0 && walletH+2 < daemonH {
				shouldSync = true
			}
		}
		if shouldSync {
			started := w.syncWalletMemoryAsync()
			if shouldUseCachedTxsDuringSync(started, w.isSyncInFlight()) {
				w.txCacheMu.RLock()
				defer w.txCacheMu.RUnlock()
				if len(w.txCache) > 0 {
					result := make([]TransactionInfo, len(w.txCache))
					copy(result, w.txCache)
					if len(result) <= count {
						return result
					}
					return result[:count]
				}
				return nil
			}
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
		message, valueTransfer, destinationPort, sourcePort, destination = w.parseTransferPayload(e.Payload, destination, destinationPort, sourcePort)

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

// parseTransferPayload extracts message, value transfer, and ports from a
// transfer payload. The payload may be zero-padded, so trailing zeros are
// trimmed. Uses comma-ok type assertions to avoid panics on malformed args.
// Returns base message/valueTransfer/ports, and updates destination with a
// reconstructed integrated address when the payload carries integrated args.
func (w *Wallet) parseTransferPayload(payload []byte, destination string, destinationPort, sourcePort uint64) (string, uint64, uint64, uint64, string) {
	if len(payload) == 0 {
		return "", 0, destinationPort, sourcePort, destination
	}

	// Trim trailing zeros (padding added by CheckPack)
	for len(payload) > 0 && payload[len(payload)-1] == 0 {
		payload = payload[:len(payload)-1]
	}
	if len(payload) == 0 {
		return "", 0, destinationPort, sourcePort, destination
	}

	var args rpc.Arguments
	if err := args.UnmarshalBinary(payload); err != nil {
		// CBOR may have extraneous data - retry with truncated payload
		payload = truncatePayloadOnCborError(payload, err)
		if len(payload) == 0 {
			return "", 0, destinationPort, sourcePort, destination
		}
		if args.UnmarshalBinary(payload) != nil {
			return "", 0, destinationPort, sourcePort, destination
		}
	}

	var message string
	var valueTransfer uint64
	// Comma-ok to avoid panics on unexpected arg value types.
	if v, ok := args.Value(rpc.RPC_COMMENT, rpc.DataString).(string); ok {
		message = v
	}
	if v, ok := args.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64); ok {
		valueTransfer = v
	}
	// Extract destination port if not already set
	if destinationPort == 0 {
		if v, ok := args.Value(rpc.RPC_DESTINATION_PORT, rpc.DataUint64).(uint64); ok {
			destinationPort = v
		}
	}
	// Extract source port if not already set
	if sourcePort == 0 {
		if v, ok := args.Value(rpc.RPC_SOURCE_PORT, rpc.DataUint64).(uint64); ok {
			sourcePort = v
		}
	}

	// Reconstruct integrated address for display if payload has integrated args
	if hasIntegratedArgs(args) {
		if intAddr, err := w.reconstructIntegratedAddress(destination, args); err == nil {
			destination = intAddr
		}
	}

	return message, valueTransfer, destinationPort, sourcePort, destination
}

// truncatePayloadOnCborError attempts to recover from a CBOR decode error by
// retrying with the payload truncated at the reported extraneous-data index.
// Returns the usable prefix, or nil if it cannot be recovered.
func truncatePayloadOnCborError(payload []byte, err error) []byte {
	errStr := err.Error()
	idx := strings.Index(errStr, "starting at index ")
	if idx == -1 {
		return nil
	}
	var validLen int
	if _, scanErr := fmt.Sscanf(errStr[idx:], "starting at index %d", &validLen); scanErr != nil || validLen <= 0 || validLen >= len(payload) {
		return nil
	}
	return payload[:validLen]
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

	// Validate destination and resolve username to address if needed.
	resolvedDestination, addr, err := resolveTransferDestination(
		params.Destination,
		globals.ParseValidateAddress,
		func(name string) (string, error) {
			return w.wallet.NameToAddress(name)
		},
	)
	if err != nil {
		log.Error("wallet", "transfer.failed", "Transfer failed: invalid recipient", "error", err.Error())
		return TransferResult{Error: err.Error()}
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
			Destination: resolvedDestination,
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

// IsValidUsernameCandidate reports whether name looks like a DERO name-service
// username (alphanumeric plus . _ -, no @ prefix, <= 64 chars) rather than a
// full address.
func IsValidUsernameCandidate(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "@") || len(name) > 64 {
		return false
	}

	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}

	return true
}

func resolveTransferDestination(destination string, parseFn func(string) (*rpc.Address, error), resolveNameFn func(string) (string, error)) (string, *rpc.Address, error) {
	destination = strings.TrimSpace(destination)
	if destination == "" {
		return "", nil, fmt.Errorf("recipient is required")
	}

	if addr, err := parseFn(destination); err == nil {
		return destination, addr, nil
	}

	if !IsValidUsernameCandidate(destination) {
		return "", nil, fmt.Errorf("invalid DERO address or username")
	}

	resolved, err := resolveNameFn(destination)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve username %q: %w", destination, err)
	}

	addr, err := parseFn(strings.TrimSpace(resolved))
	if err != nil {
		return "", nil, fmt.Errorf("username %q resolved to invalid address", destination)
	}

	return strings.TrimSpace(resolved), addr, nil
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

// GetAddress returns the wallet's primary address string ("" if not open).
func (w *Wallet) GetAddress() string {
	if w == nil || w.wallet == nil {
		return ""
	}
	return w.wallet.GetAddress().String()
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
	hostPort, err := daemonHostPort(address)
	if err != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", hostPort, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// InvalidateDaemonInfoCache clears cached daemon probe results for one address.
func InvalidateDaemonInfoCache(address string) {
	if strings.TrimSpace(address) == "" {
		return
	}
	normalized, err := NormalizeDaemonAddress(address)
	if err != nil {
		return
	}
	daemonInfoMu.Lock()
	delete(daemonInfoMemo, normalized)
	daemonInfoMu.Unlock()
}

// CheckDaemonFast checks if a daemon is reachable with a shorter timeout
func CheckDaemonFast(address string) bool {
	hostPort, err := daemonHostPort(address)
	if err != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", hostPort, 800*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// findSyncedMainnetDaemon returns the best public mainnet node near the chain
// tip. The skip address is not probed; it is normally the local candidate
// that was found behind tip. Probing is bounded so callers (which run under
// a connect timeout) never wait for the full list. Sync is decided by height
// proximity, not topo==height (DAG gap is normal at tip).
func findSyncedMainnetDaemon(ctx context.Context, skip string) (string, DaemonInfo) {
	normSkip, _ := NormalizeDaemonAddress(skip)
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var best string
	var bestInfo DaemonInfo
	var maxHeight uint64
	for _, addr := range MainnetPublicDaemons {
		normalized, err := NormalizeDaemonAddress(addr)
		if err != nil || normalized == normSkip {
			continue
		}
		info := GetDaemonInfo(probeCtx, normalized)
		if !info.IsHealthy || info.Height == 0 {
			continue
		}
		if info.Height > maxHeight {
			maxHeight = info.Height
			best = normalized
			bestInfo = info
		}
	}
	if best != "" {
		return best, bestInfo
	}
	return "", DaemonInfo{}
}

// PreferredMainnetDaemon selects the daemon the wallet should use when the
// user did not choose one explicitly: the local mainnet daemon when it is
// healthy and within a few blocks of the public tip, otherwise the best
// synced public node, otherwise "".
func PreferredMainnetDaemon(ctx context.Context) string {
	local := GetDaemonInfo(ctx, DefaultMainnetDaemon)
	// Find best public height for comparison (bounded probe).
	bestPublic := ""
	var bestPublicHeight uint64
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, addr := range MainnetPublicDaemons {
		normalized, err := NormalizeDaemonAddress(addr)
		if err != nil {
			continue
		}
		if normalized == DefaultMainnetDaemon {
			continue
		}
		info := GetDaemonInfo(probeCtx, normalized)
		if !info.IsHealthy || info.Height == 0 {
			continue
		}
		if info.Height > bestPublicHeight {
			bestPublicHeight = info.Height
			bestPublic = normalized
		}
	}
	if local.IsHealthy && local.Height > 0 {
		// Local is usable if it is near tip (allow 3-block lag for DAG) or
		// if no public node is reachable to establish a tip.
		if bestPublicHeight == 0 || local.Height+3 >= bestPublicHeight {
			return DefaultMainnetDaemon
		}
	}
	if bestPublic != "" {
		return bestPublic
	}
	if addr, _ := findSyncedMainnetDaemon(ctx, DefaultMainnetDaemon); addr != "" {
		return addr
	}
	return ""
}

// ConnectToDaemonForTest connects to an explicitly supplied daemon. It is
// intentionally small and used only by the local end-to-end verification.
func (w *Wallet) ConnectToDaemonForTest(address string) (bool, string) {
	return w.ConnectToLocalDaemonFast(false, address)
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

	// Check if target daemon is healthy BEFORE attempting any switch.
	// Prefer a direct RPC probe but do not reject a reachable websocket daemon
	// merely because GetInfo's optional health fields are absent.
	_ = knownHealthy
	info := GetDaemonInfo(context.Background(), daemon)
	if !info.IsHealthy && CheckDaemonFast(daemon) {
		info.IsHealthy = true
		info.IsOnline = true
	}
	// On the non-explicit mainnet path, only bind the wallet to a daemon that
	// is BOTH healthy and caught up. A local daemon that is still bootstrapping
	// (e.g. a freshly started embedded node at height 0) answers RPC but cannot
	// serve balances or the chain tip, so fall back to a synced public node.
	// Health is checked via TCP + HTTP; sync is decided by height proximity to
	// the public tip (DAG topo gap is normal), not topo==height.
	if !info.IsHealthy && !explicitDaemon && !w.simulator && !w.testnet {
		fallbackInfo := GetDaemonInfo(context.Background(), FallbackMainnetDaemon)
		if fallbackInfo.IsHealthy && fallbackInfo.Height > 0 {
			log.Info("daemon", "connect.fallback", "Using fallback mainnet daemon",
				"from", daemon,
				"to", FallbackMainnetDaemon)
			daemon = FallbackMainnetDaemon
			info = fallbackInfo
		}
	}
	if !explicitDaemon && !w.simulator && !w.testnet {
		// If local is behind public tip by more than a few blocks, fall back.
		if info.Height > 0 {
			// Find max public height for comparison.
			probeCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			var maxPublic uint64
			for _, addr := range MainnetPublicDaemons {
				normalized, err := NormalizeDaemonAddress(addr)
				if err != nil || normalized == daemon {
					continue
				}
				pInfo := GetDaemonInfo(probeCtx, normalized)
				if pInfo.IsHealthy && pInfo.Height > maxPublic {
					maxPublic = pInfo.Height
				}
			}
			cancel()
			if maxPublic > 0 && info.Height+5 < maxPublic {
				if fallbackAddr, fallbackInfo := findSyncedMainnetDaemon(context.Background(), daemon); fallbackAddr != "" {
					log.Info("daemon", "connect.synced_fallback", "Using synced public mainnet daemon",
						"from", daemon,
						"to", fallbackAddr,
						"height", fmt.Sprintf("%d", fallbackInfo.Height))
					daemon = fallbackAddr
					info = fallbackInfo
				}
			}
		} else if fallbackAddr, fallbackInfo := findSyncedMainnetDaemon(context.Background(), daemon); fallbackAddr != "" {
			log.Info("daemon", "connect.synced_fallback", "Using synced public mainnet daemon",
				"from", daemon,
				"to", fallbackAddr,
				"height", fmt.Sprintf("%d", fallbackInfo.Height))
			daemon = fallbackAddr
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

	// Keep both endpoint setters in sync. The wallet API's websocket connector
	// reads Daemon_Endpoint_Active, while wallet RPC state reads the disk
	// wallet's endpoint.
	walletapi.Daemon_Endpoint_Active = daemon
	w.wallet.SetDaemonAddress(daemon)

	// walletapi.Connect uses Daemon_Endpoint_Active to select the endpoint;
	// publish the candidate before connecting (the previous clearing change
	// made valid connections silently use an empty endpoint).
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

// DaemonInfo contains daemon information from DERO.GetInfo RPC
type DaemonInfo struct {
	Height                uint64
	StableHeight          int64
	TopoHeight            int64
	IsOnline              bool
	IsSynced              bool
	IsSyncing             bool
	IsBootstrapping       bool
	IsFinalizingBootstrap bool
	PeerHeight            int64
	SyncProgress          float64
	IsHealthy             bool
	Testnet               bool
	Network               string
	Version               string
	Difficulty            uint64
	AvgBlockTime          float32
	IncomingPeers         uint64
	OutgoingPeers         uint64
	KnownPeers            uint64
	Uptime                uint64
	TxPoolSize            uint64
	Hashrate1hr           uint64
	Hashrate1d            uint64
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
			Height                     uint64  `json:"height"`
			StableHeight               int64   `json:"stableheight"`
			TopoHeight                 int64   `json:"topoheight"`
			Status                     string  `json:"status"`
			Testnet                    bool    `json:"testnet"`
			Network                    string  `json:"network"`
			Version                    string  `json:"version"`
			Difficulty                 uint64  `json:"difficulty"`
			AverageBlockTime50         float32 `json:"averageblocktime50"`
			Incoming_connections_count uint64  `json:"incoming_connections_count"`
			Outgoing_connections_count uint64  `json:"outgoing_connections_count"`
			White_peerlist_size        uint64  `json:"white_peerlist_size"`
			Uptime                     uint64  `json:"uptime"`
			Tx_pool_size               uint64  `json:"tx_pool_size"`
			Hashrate_1hr               uint64  `json:"hashrate_1hr"`
			Hashrate_1d                uint64  `json:"hashrate_1d"`
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
	info.StableHeight = result.Result.StableHeight
	info.TopoHeight = result.Result.TopoHeight
	info.IsOnline = true
	info.IsHealthy = true
	// Sync/Bootstrap are decided by classifyDaemonSync using peer/reference
	// heights. At the RPC layer a node with height>0 is considered provisionally
	// synced; a 1-2 block topo vs height DAG gap at tip is normal and must not
	// permanently mark the daemon as bootstrapping/unsynced.
	info.IsSynced = result.Result.Height > 0
	info.IsBootstrapping = false
	info.Testnet = result.Result.Testnet
	info.Network = result.Result.Network
	info.Version = result.Result.Version
	info.Difficulty = result.Result.Difficulty
	info.AvgBlockTime = result.Result.AverageBlockTime50
	info.IncomingPeers = result.Result.Incoming_connections_count
	info.OutgoingPeers = result.Result.Outgoing_connections_count
	info.KnownPeers = result.Result.White_peerlist_size
	info.Uptime = result.Result.Uptime
	info.TxPoolSize = result.Result.Tx_pool_size
	info.StableHeight = result.Result.StableHeight
	info.Hashrate1hr = result.Result.Hashrate_1hr
	info.Hashrate1d = result.Result.Hashrate_1d

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

	// A reachable daemon may return a JSON-RPC error for a malformed or
	// version-specific request. Reachability is still sufficient for wallet
	// connection; only a missing response is unhealthy.
	return result.Result != nil || result.Error != nil
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
