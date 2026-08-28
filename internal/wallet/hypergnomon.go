// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	hgindexer "github.com/hypergnomon/hypergnomon/pkg/gnomes/indexer"
	hgstorage "github.com/hypergnomon/hypergnomon/pkg/gnomes/storage"
	hgstructures "github.com/hypergnomon/hypergnomon/pkg/gnomes/structures"
)

var globalAppHyper *HyperGnomon
var globalAppHyperMu sync.RWMutex

// HyperGnomon is the embedded bbolt-backed indexer session. It can be owned
// by a Wallet (legacy) or by the application Model for launch-time indexing
// without an open wallet.
type HyperGnomon struct {
	index    *hgindexer.Indexer
	store    *hgstorage.BboltStore
	mu       sync.Mutex
	endpoint string
	network  string
	started  time.Time
}

// NewHyperGnomon creates and starts a standalone bbolt-backed indexer that
// does NOT require an open wallet. The database is per-network under
// ~/.derotui/hypergnomon/{mainnet|testnet|simulator}. The caller owns the
// returned instance and must call Close when done. parallelBlocks controls the
// daemon scan parallelism (8 is the historical default).
func NewHyperGnomon(endpoint, network string, dbDir string, parallelBlocks int) (*HyperGnomon, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint required")
	}
	if strings.TrimSpace(network) == "" {
		network = "mainnet"
	}
	network = strings.ToLower(strings.TrimSpace(network))
	if dbDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dbDir = filepath.Join(home, ".derotui", "hypergnomon", network)
	}
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return nil, err
	}
	store, err := hgstorage.NewBBoltDB(dbDir, "")
	if err != nil {
		return nil, fmt.Errorf("open HyperGnomon database: %w", err)
	}
	cfg := &hgstructures.FastSyncConfig{Enabled: true, ForceFastSync: true, NoCode: false}
	idx := hgindexer.NewIndexer(nil, store, "boltdb", nil, 0, endpoint, "daemon", false, false, cfg, nil, false)
	if idx == nil || idx.DBType == "" {
		_ = store.Close()
		return nil, fmt.Errorf("initialize HyperGnomon indexer")
	}
	idx.Endpoint = endpoint
	if parallelBlocks <= 0 {
		parallelBlocks = 16
	}
	idx.StartDaemonMode(parallelBlocks)
	h := &HyperGnomon{index: idx, store: store, endpoint: endpoint, network: network, started: time.Now()}
	globalAppHyperMu.Lock()
	globalAppHyper = h
	globalAppHyperMu.Unlock()
	return h, nil
}

// SCIDs returns indexed candidate SCIDs from this indexer session.
func (h *HyperGnomon) SCIDs() []string {
	if h == nil || h.store == nil {
		return nil
	}
	h.mu.Lock()
	store := h.store
	h.mu.Unlock()
	if store == nil {
		return nil
	}
	owners := store.GetAllOwnersAndSCIDs()
	out := make([]string, 0, len(owners))
	for scid := range owners {
		out = append(out, scid)
	}
	return out
}

// SCIDsForAddress returns the SCIDs an address has interacted with, served
// from HyperGnomon's addr_scids reverse index (a direct per-address prefix
// scan). This is the fast token-discovery path: instead of walking every
// indexed SCID on chain, a wallet only probes the handful of SCIDs it
// actually touched. Returns nil when the index is unavailable or the address
// has no recorded interactions (fresh index / never-touched address).
func (h *HyperGnomon) SCIDsForAddress(addr string) []string {
	if h == nil || h.store == nil {
		return nil
	}
	h.mu.Lock()
	store := h.store
	h.mu.Unlock()
	if store == nil {
		return nil
	}
	entries, err := store.Inner().GetAddressSCIDs(strings.TrimSpace(addr))
	if err != nil || len(entries) == 0 {
		return nil
	}
	out := make([]string, 0, len(entries))
	for scid := range entries {
		out = append(out, scid)
	}
	return out
}

var tokenLikeClasses = []string{"DERO-ASSET", "G45-AT", "G45-FAT", "T345"}

func IsTokenLikeClass(class string) bool {
	switch class {
	case "DERO-ASSET", "G45-AT", "G45-FAT", "T345":
		return true
	default:
		return false
	}
}

func (h *HyperGnomon) ClassOf(scid string) string {
	if h == nil || h.store == nil {
		return ""
	}
	h.mu.Lock()
	store := h.store
	h.mu.Unlock()
	if store == nil {
		return ""
	}
	inner := store.Inner()
	if inner == nil {
		return ""
	}
	meta, err := inner.GetSCIDClass(strings.ToLower(strings.TrimSpace(scid)))
	if err != nil || meta == nil {
		return ""
	}
	return meta.Class
}

func (h *HyperGnomon) SCIDsByClass(class string) []string {
	if h == nil || h.store == nil || class == "" {
		return nil
	}
	h.mu.Lock()
	store := h.store
	h.mu.Unlock()
	if store == nil {
		return nil
	}
	inner := store.Inner()
	if inner == nil {
		return nil
	}
	insts, err := inner.GetClassInstalls(class, 0)
	if err != nil || len(insts) == 0 {
		return nil
	}
	out := make([]string, 0, len(insts))
	for _, inst := range insts {
		if inst.SCID != "" {
			out = append(out, inst.SCID)
		}
	}
	return out
}

func (h *HyperGnomon) ClassCounts() map[string]int {
	counts := make(map[string]int)
	for _, scid := range h.SCIDs() {
		c := h.ClassOf(scid)
		if c == "" {
			c = "UNKNOWN"
		}
		counts[c]++
	}
	return counts
}

func (h *HyperGnomon) TokenLikeSCIDs() []string {
	seen := make(map[string]bool)
	var out []string
	for _, class := range tokenLikeClasses {
		for _, scid := range h.SCIDsByClass(class) {
			if seen[scid] {
				continue
			}
			seen[scid] = true
			out = append(out, scid)
		}
	}
	return out
}

// Count returns the number of indexed SCIDs.
func (h *HyperGnomon) Count() int {
	if h == nil || h.store == nil {
		return 0
	}
	h.mu.Lock()
	store := h.store
	h.mu.Unlock()
	if store == nil {
		return 0
	}
	return len(store.GetAllOwnersAndSCIDs())
}

// Endpoint returns the daemon endpoint this indexer is polling.
func (h *HyperGnomon) Endpoint() string {
	if h == nil {
		return ""
	}
	return h.endpoint
}

// Network returns the network label (mainnet/testnet/simulator) used for the DB dir.
func (h *HyperGnomon) Network() string {
	if h == nil {
		return ""
	}
	return h.network
}

// IsRunning reports whether the indexer has an active store/index.
func (h *HyperGnomon) IsRunning() bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.store != nil && h.index != nil
}

func (h *HyperGnomon) StartedAt() time.Time {
	if h == nil {
		return time.Time{}
	}
	return h.started
}

// Progress returns SCID count and height progress. chainHeight is 0 while
// the indexer has not yet polled GetInfo.
func (h *HyperGnomon) Progress() (scids int, lastHeight int64, chainHeight int64, status string) {
	if h == nil {
		return 0, 0, 0, ""
	}
	h.mu.Lock()
	idx := h.index
	store := h.store
	h.mu.Unlock()
	if store != nil {
		scids = len(store.GetAllOwnersAndSCIDs())
	}
	if idx != nil {
		lastHeight = idx.LastIndexedHeight
		chainHeight = idx.ChainHeight
		status = idx.Status
	}
	return
}

// Close stops the indexer and releases its bbolt lock.
func (h *HyperGnomon) Close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.index != nil {
		h.index.Close()
		h.index = nil
	}
	if h.store != nil {
		_ = h.store.Close()
		h.store = nil
	}
	globalAppHyperMu.Lock()
	if globalAppHyper == h {
		globalAppHyper = nil
	}
	globalAppHyperMu.Unlock()
}

// TokenMetadataFromStore tries the local HyperGnomon bbolt store for name/symbol/decimals.
// It is a fallback when the daemon RPC is unavailable or times out.
func TokenMetadataFromStore(scid string) (name, ticker string, decimals uint64, ok bool) {
	scid = strings.ToLower(strings.TrimSpace(scid))
	if scid == "" {
		return "", "", 0, false
	}
	globalAppHyperMu.RLock()
	h := globalAppHyper
	globalAppHyperMu.RUnlock()
	if h == nil || h.store == nil {
		return "", "", 0, false
	}
	h.mu.Lock()
	store := h.store
	h.mu.Unlock()
	if store == nil {
		return "", "", 0, false
	}
	// Fast path: try direct key lookups (case-sensitive via store, try both cases)
	for _, key := range []string{"name", "Name", "NAME"} {
		if vals, _ := store.GetSCIDValuesByKey(scid, key, 0, false); len(vals) > 0 && vals[0] != "" {
			name = vals[0]
			break
		}
	}
	for _, key := range []string{"symbol", "Symbol", "SYMBOL", "ticker", "Ticker", "TICKER"} {
		if vals, _ := store.GetSCIDValuesByKey(scid, key, 0, false); len(vals) > 0 && vals[0] != "" {
			ticker = vals[0]
			break
		}
	}
	if vals, _ := store.GetSCIDValuesByKey(scid, "decimals", 0, false); len(vals) > 0 {
		// string values path returns string, uint64 path returns uint64 slice
		// This call returns string slice, so decimals stored as string would be here
		if vals[0] != "" {
			if d, err := strconv.ParseUint(vals[0], 10, 64); err == nil {
				decimals = d
			} else if b, err := hex.DecodeString(vals[0]); err == nil {
				decoded := strings.TrimSpace(string(b))
				if d, err := strconv.ParseUint(decoded, 10, 64); err == nil {
					decimals = d
				}
			}
		}
	}
	if _, vals := store.GetSCIDValuesByKey(scid, "decimals", 0, false); len(vals) > 0 && decimals == 0 {
		// try uint64 key path (some SCs use uint64 key 0 for decimals? fallback)
		// The uint64 values are returned as second return value of the same call when key is uint64.
		// Since we passed string key, we won't get uint64 values here.
	}
	// Fallback: scan all variables for case-insensitive match (covers lowercase/uppercase variants)
	if name == "" || ticker == "" || decimals == 0 {
		vars := store.GetAllSCIDVariableDetails(scid)
		for _, v := range vars {
			if v == nil {
				continue
			}
			keyStr, isStr := v.Key.(string)
			if isStr {
				lk := strings.ToLower(keyStr)
				switch lk {
				case "name":
					if name == "" {
						if s := storeVariableToString(v.Value); s != "" {
							name = s
						}
					}
				case "symbol", "ticker":
					if ticker == "" {
						if s := storeVariableToString(v.Value); s != "" {
							ticker = s
						}
					}
				case "decimals":
					if decimals == 0 {
						if u := storeVariableToUint64(v.Value); u != 0 {
							decimals = u
						} else if s := storeVariableToString(v.Value); s != "" {
							if d, err := strconv.ParseUint(s, 10, 64); err == nil {
								decimals = d
							}
						}
					}
				}
			} else if keyNum, ok := v.Key.(uint64); ok && keyNum == 0 && decimals == 0 {
				// some SCs use uint64 key 0 for decimals
				if u := storeVariableToUint64(v.Value); u != 0 && u <= 18 {
					decimals = u
				}
			}
			if name != "" && ticker != "" && decimals != 0 {
				break
			}
		}
	}
	if name != "" || ticker != "" || decimals != 0 {
		return name, ticker, decimals, true
	}
	return "", "", 0, false
}

func storeVariableToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		s := strings.TrimSpace(val)
		if s == "" || strings.HasPrefix(s, "NOT AVAILABLE") {
			return ""
		}
		if b, err := hex.DecodeString(s); err == nil {
			decoded := string(b)
			printable := true
			for _, r := range decoded {
				if r < 32 || r > 126 {
					if r != ' ' {
						printable = false
						break
					}
				}
			}
			if printable && strings.TrimSpace(decoded) != "" {
				return strings.TrimSpace(decoded)
			}
		}
		return s
	case []byte:
		return strings.TrimSpace(string(val))
	default:
		return fmt.Sprintf("%v", val)
	}
}

func storeVariableToUint64(v interface{}) uint64 {
	switch val := v.(type) {
	case uint64:
		return val
	case int64:
		return uint64(val)
	case float64:
		return uint64(val)
	case string:
		if d, err := strconv.ParseUint(strings.TrimSpace(val), 10, 64); err == nil {
			return d
		}
		if b, err := hex.DecodeString(strings.TrimSpace(val)); err == nil && len(b) <= 8 {
			var n uint64
			for i := len(b) - 1; i >= 0; i-- {
				n = n*256 + uint64(b[i])
			}
			return n
		}
	case []byte:
		if len(val) == 8 {
			var n uint64
			for i := 0; i < 8; i++ {
				n = n*256 + uint64(val[i])
			}
			return n
		}
	}
	return 0
}

// StartHyperGnomon starts an embedded bbolt-backed indexer owned by the wallet.
// The database is separate from the encrypted wallet and is safe to rebuild
// independently. Prefer NewHyperGnomon for app-level (wallet-independent) indexing.
func (w *Wallet) StartHyperGnomon(dbDir string, parallelBlocks int) error {
	if w == nil || w.wallet == nil {
		return fmt.Errorf("wallet not open")
	}
	w.closeMu.Lock()
	defer w.closeMu.Unlock()
	if w.hyper != nil {
		return nil
	}
	endpoint := w.GetDaemonAddress()
	network := w.GetNetworkType()
	h, err := NewHyperGnomon(endpoint, network, dbDir, parallelBlocks)
	if err != nil {
		return err
	}
	w.hyper = h
	return nil
}

// HyperGnomonSCIDs returns indexed candidate SCIDs from the wallet-owned indexer.
func (w *Wallet) HyperGnomonSCIDs() []string {
	if w == nil || w.hyper == nil {
		return nil
	}
	return w.hyper.SCIDs()
}

// HyperGnomonSCIDsForAddress returns the SCIDs the address interacted with,
// from the wallet-owned indexer's addr_scids reverse index.
func (w *Wallet) HyperGnomonSCIDsForAddress(addr string) []string {
	if w == nil || w.hyper == nil {
		return nil
	}
	return w.hyper.SCIDsForAddress(addr)
}

// CloseHyperGnomon stops the wallet-owned indexer and releases its bbolt lock.
func (w *Wallet) CloseHyperGnomon() {
	if w == nil {
		return
	}
	w.closeMu.Lock()
	defer w.closeMu.Unlock()
	if w.hyper == nil {
		return
	}
	w.hyper.Close()
	w.hyper = nil
}
