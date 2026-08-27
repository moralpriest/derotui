// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	hgindexer "github.com/hypergnomon/hypergnomon/pkg/gnomes/indexer"
	hgstorage "github.com/hypergnomon/hypergnomon/pkg/gnomes/storage"
	hgstructures "github.com/hypergnomon/hypergnomon/pkg/gnomes/structures"
)

// HyperGnomon is the embedded, wallet-owned indexer session.
type HyperGnomon struct {
	index *hgindexer.Indexer
	store *hgstorage.BboltStore
	mu    sync.Mutex
}

// StartHyperGnomon starts an embedded bbolt-backed indexer. The database is
// separate from the encrypted wallet and is safe to rebuild independently.
func (w *Wallet) StartHyperGnomon(dbDir string, parallelBlocks int) error {
	if w == nil || w.wallet == nil {
		return fmt.Errorf("wallet not open")
	}
	w.closeMu.Lock()
	defer w.closeMu.Unlock()
	if dbDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dbDir = filepath.Join(home, ".derotui", "hypergnomon", strings.ToLower(w.GetNetworkType()))
	}
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return err
	}
	if w.hyper != nil {
		return nil
	}
	store, err := hgstorage.NewBBoltDB(dbDir, "")
	if err != nil {
		return fmt.Errorf("open HyperGnomon database: %w", err)
	}
	cfg := &hgstructures.FastSyncConfig{Enabled: true, ForceFastSync: true, NoCode: true}
	idx := hgindexer.NewIndexer(nil, store, "boltdb", nil, 0, w.GetDaemonAddress(), "daemon", false, false, cfg, nil, false)
	if idx == nil || idx.DBType == "" {
		_ = store.Close()
		return fmt.Errorf("initialize HyperGnomon indexer")
	}
	idx.Endpoint = w.GetDaemonAddress()
	idx.StartDaemonMode(parallelBlocks)
	w.hyper = &HyperGnomon{index: idx, store: store}
	return nil
}

// HyperGnomonSCIDs returns indexed candidate SCIDs.
func (w *Wallet) HyperGnomonSCIDs() []string {
	if w == nil || w.hyper == nil || w.hyper.store == nil {
		return nil
	}
	owners := w.hyper.store.GetAllOwnersAndSCIDs()
	out := make([]string, 0, len(owners))
	for scid := range owners {
		out = append(out, scid)
	}
	return out
}

// CloseHyperGnomon stops the embedded indexer and releases its bbolt lock.
func (w *Wallet) CloseHyperGnomon() {
	if w == nil {
		return
	}
	w.closeMu.Lock()
	defer w.closeMu.Unlock()
	if w.hyper == nil {
		return
	}
	if w.hyper.index != nil {
		w.hyper.index.Close()
	}
	if w.hyper.store != nil {
		_ = w.hyper.store.Close()
	}
	w.hyper = nil
}
