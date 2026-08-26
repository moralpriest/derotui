// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDirHasDataIgnoresConfigDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "derotui.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if dataDirHasData(dir, "mainnet") {
		t.Fatal("config dir with only derotui.json must not count as chain data")
	}
}

func TestDataDirHasDataDetectsMainnetChain(t *testing.T) {
	dir := t.TempDir()
	chain := filepath.Join(dir, "mainnet")
	if err := os.MkdirAll(chain, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chain, "balances"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if !dataDirHasData(dir, "mainnet") {
		t.Fatal("mainnet/balances should count as chain data")
	}
}

func TestDataDirHasDataIgnoresLogsOnly(t *testing.T) {
	dir := t.TempDir()
	chain := filepath.Join(dir, "mainnet")
	if err := os.MkdirAll(chain, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chain, "derod.log"), []byte("log"), 0600); err != nil {
		t.Fatal(err)
	}
	if dataDirHasData(dir, "mainnet") {
		t.Fatal("log-only mainnet dir must not disable fastsync")
	}
}
