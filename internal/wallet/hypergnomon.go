// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deroproject/derohe/rpc"
	derolog "github.com/deroproject/dero-wallet-cli/internal/log"
	hgindexer "github.com/hypergnomon/hypergnomon/pkg/gnomes/indexer"
	hgstorage "github.com/hypergnomon/hypergnomon/pkg/gnomes/storage"
	hgstructures "github.com/hypergnomon/hypergnomon/pkg/gnomes/structures"
	hgstore "github.com/hypergnomon/hypergnomon/storage"
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

	// Cached progress values, refreshed in the background by a poller
	// goroutine. Progress() reads these atomics instead of doing a
	// synchronous bbolt owners-bucket scan on every call, which used to
	// block the UI thread during startup and periodic ticks.
	cachedScids      atomic.Int64
	cachedLastHeight atomic.Int64
	cachedChain      atomic.Int64
	cachedStatus     atomic.Pointer[string]

	// stop is closed by Close to end the background poller promptly;
	// pollDone signals the poller goroutine has exited.
	stop     chan struct{}
	pollDone chan struct{}
}

// NewHyperGnomon creates and starts a standalone bbolt-backed indexer that
// does NOT require an open wallet. The database is per-network under
// ~/.derotui/hypergnomon/{mainnet|testnet|simulator}. The caller owns the
// returned instance and must call Close when done. parallelBlocks controls the
// daemon scan parallelism (8 is the historical default).
//
// This constructor returns as soon as the store handle is opened and the
// indexer goroutines are spawned — it does NOT wait for FastSync or the
// first daemon poll to complete. A background poller started here refreshes
// the cached progress values served by Progress().
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
	h := &HyperGnomon{
		index:    idx,
		store:    store,
		endpoint: endpoint,
		network:  network,
		started:  time.Now(),
		stop:     make(chan struct{}),
		pollDone: make(chan struct{}),
	}
	go h.pollProgress()
	globalAppHyperMu.Lock()
	globalAppHyper = h
	globalAppHyperMu.Unlock()
	return h, nil
}

// pollProgress periodically refreshes cached progress counters from the
// underlying store and indexer. This keeps the expensive bbolt owners-bucket
// scan off the UI thread: Progress() reads the cached atomics instead.
// Exits when both store and index are nil (i.e. Close has been called).
func (h *HyperGnomon) pollProgress() {
	defer close(h.pollDone)
	// Capture the stop channel once: Close() sets h.stop to nil while
	// shutting down, and a select on a nil channel case would block forever,
	// stranding this goroutine and deadlocking Close's <-pollDone wait.
	stop := h.stop
	if stop == nil {
		return
	}
	// One immediate sample so callers see data without waiting for the
	// first tick.
	h.sampleProgress()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			h.sampleProgress()
		}
	}
}

// sampleProgress reads current values from the store and indexer into the
// cached atomics. Must not be called concurrently with Close.
func (h *HyperGnomon) sampleProgress() {
	h.mu.Lock()
	store := h.store
	idx := h.index
	h.mu.Unlock()
	if store != nil {
		h.cachedScids.Store(int64(len(store.GetAllOwnersAndSCIDs())))
	}
	if idx != nil {
		h.cachedLastHeight.Store(idx.LastIndexedHeight)
		h.cachedChain.Store(idx.ChainHeight)
		s := idx.Status
		h.cachedStatus.Store(&s)
	}
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

type CatalogEntry struct {
	SCID          string
	Class         string
	Name          string
	DURL          string
	Desc          string
	Version       string
	Tags          []string
	InstallHeight int64
	// TELA ratings (indexed from SC variables). AvgRating is the mean of
	// per-rater 0-99 scores rescaled to 0-10 (Engram convention); zero when
	// nobody has rated the entry yet.
	AvgRating float64
	Likes     uint64
	Dislikes  uint64
}

// RatingDisplay returns the 0-10 average formatted for a one-cell column
// ("unrated" when nobody has rated) plus the color tier Engram uses:
// 9.0+ top, 7.0+ good, 5.0+ mid, else poor. More dislikes than likes forces
// the poor tier regardless of the average.
func (e CatalogEntry) RatingDisplay() (label string, tier RatingTier) {
	if e.Dislikes > e.Likes {
		return "poor", RatingTierPoor
	}
	if e.AvgRating <= 0 {
		return "unrated", RatingTierNone
	}
	switch {
	case e.AvgRating >= 9.0:
		tier = RatingTierTop
	case e.AvgRating >= 7.0:
		tier = RatingTierGood
	case e.AvgRating >= 5.0:
		tier = RatingTierMid
	default:
		tier = RatingTierPoor
	}
	return fmt.Sprintf("%.1f", e.AvgRating), tier
}

// RatingTier mirrors Engram's telaAverageColor tiers so the TUI colors match
// the desktop wallet's rating hexagons.
type RatingTier uint8

const (
	RatingTierNone RatingTier = iota
	RatingTierPoor
	RatingTierMid
	RatingTierGood
	RatingTierTop
)

func (h *HyperGnomon) Catalog(class string) []CatalogEntry {
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
	out := make([]CatalogEntry, 0, len(insts))
	for _, inst := range insts {
		if inst.SCID == "" {
			continue
		}
		e := CatalogEntry{SCID: inst.SCID, Class: class}
		if inst.Meta != nil {
			e.Name = inst.Meta.Name
			e.DURL = inst.Meta.DURL
			e.Desc = inst.Meta.Desc
			e.Version = inst.Meta.Version
			e.Tags = inst.Meta.Tags
			e.InstallHeight = inst.Meta.InstallHeight
			if inst.Meta.Class != "" {
				e.Class = inst.Meta.Class
			}
		}
		if e.Name == "" {
			e.Name = e.DURL
		}
		out = append(out, e)
	}
	return out
}

// RatingsForSCIDs batches rating lookups off the UI thread. Catalog() is
// called from the render loop, and each ratings lookup opens bbolt read
// transactions — with the indexer holding write locks, doing this per row on
// the UI thread froze the app. The UI calls this from a background command
// and merges results via ApplyRatings.
func (h *HyperGnomon) RatingsForSCIDs(scids []string) map[string]CatalogEntry {
	out := make(map[string]CatalogEntry, len(scids))
	if h == nil || h.store == nil {
		return out
	}
	h.mu.Lock()
	store := h.store
	h.mu.Unlock()
	if store == nil {
		return out
	}
	inner := store.Inner()
	if inner == nil {
		return out
	}
	for _, scid := range scids {
		if scid == "" {
			continue
		}
		e := CatalogEntry{SCID: scid}
		e.applyRatings(inner)
		out[strings.ToLower(scid)] = e
	}
	return out
}

// applyRatings enriches an entry with TELA rating data from the local store:
// per-rater 0-99 scores rescale to a 0-10 average (Engram convention), plus
// the likes/dislikes counters. A missing summary leaves the entry unrated.
// Takes the raw hgstorage store (Inner()), which exposes the ratings API the
// civilware-compat wrapper lacks.
func (e *CatalogEntry) applyRatings(inner *hgstore.BboltStore) {
	if summary, err := inner.GetRatingSummary(e.SCID, 0); err == nil && summary != nil {
		e.Likes = summary.Likes
		e.Dislikes = summary.Dislikes
	}
	ratings, err := inner.GetRatingsForSCID(e.SCID, 0)
	if err != nil || len(ratings) == 0 {
		return
	}
	var sum float64
	for _, r := range ratings {
		sum += r.Score
	}
	e.AvgRating = sum / float64(len(ratings)) / 10.0
}

var catalogNameKeys = []string{"var_header_name", "nameHdr", "name", "metadata"}

func nameFromCatalogVals(vals map[string]string) string {
	for _, k := range []string{"var_header_name", "namehdr", "name"} {
		if s := strings.TrimSpace(vals[k]); s != "" {
			return s
		}
	}
	if meta := vals["metadata"]; meta != "" {
		var blob map[string]interface{}
		if err := json.Unmarshal([]byte(meta), &blob); err == nil {
			if s, _ := blob["name"].(string); strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func lookupSCVars(endpoint, scid string, keys []string) map[string]string {
	scid = strings.ToLower(strings.TrimSpace(scid))
	endpoint = strings.TrimSpace(endpoint)
	if scid == "" || endpoint == "" || len(keys) == 0 {
		return nil
	}
	rpcURL, err := daemonRPCURL(endpoint)
	if err != nil {
		return nil
	}
	params := rpc.GetSC_Params{SCID: scid, KeysString: keys}
	bodyBytes, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": "1", "method": "DERO.GetSC", "params": params,
	})
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", rpcURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	var rpcResp struct {
		Result rpc.GetSC_Result `json:"result"`
	}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil
	}
	vals := map[string]string{}
	for i, k := range keys {
		if i >= len(rpcResp.Result.ValuesString) {
			break
		}
		raw := rpcResp.Result.ValuesString[i]
		if raw == "" || strings.HasPrefix(raw, "NOT AVAILABLE") {
			continue
		}
		if s := decodeSCString(raw); s != "" {
			vals[strings.ToLower(k)] = s
		}
	}
	return vals
}

func LookupSCName(endpoint, scid string) string {
	scid = strings.ToLower(strings.TrimSpace(scid))
	if scid == "" {
		return ""
	}
	if n, _, _, ok := TokenMetadataFromStore(scid); ok && n != "" {
		return n
	}
	return nameFromCatalogVals(lookupSCVars(endpoint, scid, catalogNameKeys))
}

func LookupSCOwner(endpoint, scid string) string {
	vals := lookupSCVars(endpoint, scid, []string{"owner"})
	if vals == nil {
		return ""
	}
	return strings.TrimSpace(vals["owner"])
}

func FilterCatalogBySCIDs(entries []CatalogEntry, scids []string) []CatalogEntry {
	if len(entries) == 0 || len(scids) == 0 {
		return nil
	}
	want := make(map[string]bool, len(scids))
	for _, s := range scids {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" {
			want[s] = true
		}
	}
	var out []CatalogEntry
	for _, e := range entries {
		if want[strings.ToLower(e.SCID)] {
			out = append(out, e)
		}
	}
	return out
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

// Count returns the number of indexed SCIDs. Served from cached atomics.
func (h *HyperGnomon) Count() int {
	if h == nil {
		return 0
	}
	return int(h.cachedScids.Load())
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
// the indexer has not yet polled GetInfo. Values are served from atomics
// refreshed in the background by pollProgress, so this call is cheap and
// never blocks on bbolt or the RPC pool.
func (h *HyperGnomon) Progress() (scids int, lastHeight int64, chainHeight int64, status string) {
	if h == nil {
		return 0, 0, 0, ""
	}
	scids = int(h.cachedScids.Load())
	lastHeight = h.cachedLastHeight.Load()
	chainHeight = h.cachedChain.Load()
	if p := h.cachedStatus.Load(); p != nil {
		status = *p
	}
	return
}

// hyperCloseWaitTimeout bounds Close's wait for the poll goroutine and the
// indexer/store teardown. The indexer can be stuck inside a long bbolt write
// transaction or a no-timeout RPC; blocking shutdown (and the UI thread that
// runs it — Ctrl+C) on that froze the app on exit.
const hyperCloseWaitTimeout = 5 * time.Second

// Close stops the indexer and releases its bbolt lock. Never blocks longer
// than hyperCloseWaitTimeout: on timeout the resources are leaked to the OS
// (process exit reclaims them) instead of hanging the caller.
func (h *HyperGnomon) Close() {
	if h == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		h.closeInner()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(hyperCloseWaitTimeout):
		derolog.Warn("hypergnomon", "close.timeout", "HyperGnomon close timed out; leaking resources to exit", "timeout", hyperCloseWaitTimeout.String())
	}
}

// closeInner performs the actual teardown. Called from Close's goroutine;
// idempotent via the nil-ing of h.index/h.store/h.stop under h.mu.
func (h *HyperGnomon) closeInner() {
	h.mu.Lock()
	index, store, stop, done := h.index, h.store, h.stop, h.pollDone
	h.index, h.store, h.stop = nil, nil, nil
	h.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if index != nil {
		index.Close()
	}
	if store != nil {
		_ = store.Close()
	}
	if done != nil {
		<-done
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
						if s := storeVariableToString(v.Value); s != "" {
							if d, err := strconv.ParseUint(s, 10, 64); err == nil {
								decimals = d
							}
						}
					}
				}
			}
		}
	}
	return name, ticker, decimals, name != "" || ticker != ""
}

// storeVariableToString converts a store variable value to a human-readable string.
func storeVariableToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case uint64:
		return strconv.FormatUint(val, 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		if b, err := json.Marshal(val); err == nil {
			return string(b)
		}
	}
	return ""
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
	if w.hyper != nil {
		w.hyper.Close()
		w.hyper = nil
	}
}
