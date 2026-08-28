// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	hgstructures "github.com/hypergnomon/hypergnomon/structures"
)

func TestIsTokenLikeClass(t *testing.T) {
	for _, c := range []string{"DERO-ASSET", "G45-AT", "G45-FAT", "T345"} {
		if !IsTokenLikeClass(c) {
			t.Fatalf("%s should match", c)
		}
	}
	if IsTokenLikeClass("UNKNOWN") || IsTokenLikeClass("TELA-INDEX-1") || IsTokenLikeClass("G45-NFT") {
		t.Fatal("non-token classes should not match")
	}
}

func TestHyperGnomonClassFilterVsPreviousScan(t *testing.T) {
	if os.Getenv("HYPER_BENCH") == "" {
		t.Skip("set HYPER_BENCH=1 to run")
	}
	endpoint := os.Getenv("HYPER_BENCH_ENDPOINT")
	if endpoint == "" {
		endpoint = "dero.geeko.cloud:10102"
	}
	dir := os.Getenv("HYPER_BENCH_DIR")
	hgstructures.TELAProbeSettled.Store(false)
	h, err := NewHyperGnomon(endpoint, "mainnet", dir, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	start := time.Now()
	deadline := time.Now().Add(2 * time.Hour)
	var clockA time.Duration
	for time.Now().Before(deadline) {
		scids, last, chain, status := h.Progress()
		if chain > 0 && last+2 >= chain && scids > 0 && clockA == 0 {
			clockA = time.Since(start)
			fmt.Fprintf(os.Stderr, "Clock A catalog ready duration=%s scids=%d last=%d chain=%d status=%s\n",
				clockA.Round(time.Millisecond), scids, last, chain, status)
			break
		}
		time.Sleep(5 * time.Second)
	}
	fmt.Fprintf(os.Stderr, "waiting for classify probe (TELAProbeSettled)...\n")
	for time.Now().Before(deadline) {
		if hgstructures.TELAProbeSettled.Load() {
			fmt.Fprintf(os.Stderr, "classify settled elapsed=%s\n", time.Since(start).Round(time.Millisecond))
			break
		}
		fmt.Fprintf(os.Stderr, "classify wait elapsed=%s token-like=%d\n",
			time.Since(start).Round(time.Second), len(h.TokenLikeSCIDs()))
		time.Sleep(10 * time.Second)
	}
	const previousAll = 50276
	const previousLike = 49326
	all := h.SCIDs()
	like := h.TokenLikeSCIDs()
	counts := h.ClassCounts()
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintf(os.Stderr, "class histogram (%d SCIDs):\n", len(all))
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %s\t%d\n", k, counts[k])
	}
	fmt.Fprintf(os.Stderr, "Clock A previous: %d SCIDs in 30.0s (NoCode)\n", previousAll)
	fmt.Fprintf(os.Stderr, "Clock B previous (UNKNOWN+TELA skip): %d\n", previousLike)
	fmt.Fprintf(os.Stderr, "Clock B candidates before (all): %d\n", len(all))
	fmt.Fprintf(os.Stderr, "Clock B candidates after (DERO-ASSET+G45-AT+G45-FAT+T345): %d\n", len(like))
	if len(all) > 0 {
		fmt.Fprintf(os.Stderr, "Clock B reduction: %d -> %d (%.2f%% remain)\n",
			len(all), len(like), 100*float64(len(like))/float64(len(all)))
	}
	if len(like) > len(all) {
		t.Fatalf("DERO-ASSET=%d all=%d", len(like), len(all))
	}
}

func TestHyperGnomonColdIndex(t *testing.T) {
	if os.Getenv("HYPER_BENCH") == "" {
		t.Skip("set HYPER_BENCH=1 to run")
	}
	endpoint := os.Getenv("HYPER_BENCH_ENDPOINT")
	if endpoint == "" {
		endpoint = "dero.geeko.cloud:10102"
	}
	dir := os.Getenv("HYPER_BENCH_DIR")
	h, err := NewHyperGnomon(endpoint, "mainnet", dir, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	start := time.Now()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for range tick.C {
		scids, last, chain, status := h.Progress()
		elapsed := time.Since(start).Round(time.Second)
		fmt.Fprintf(os.Stderr, "hyper bench elapsed=%s scids=%d last=%d chain=%d status=%s\n",
			elapsed, scids, last, chain, status)
		if chain > 0 && last+2 >= chain {
			fmt.Fprintf(os.Stderr, "hyper bench COMPLETE duration=%s scids=%d last=%d chain=%d\n",
				time.Since(start).Round(time.Millisecond), scids, last, chain)
			return
		}
	}
}
