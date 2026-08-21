// Copyright 2017-2026 DERO Project. All rights reserved.

package config

import "testing"

func TestIsPruned(t *testing.T) {
	cases := []struct {
		prune string
		want  bool
	}{
		{"", false},
		{"0", false},
		{"500", true},
		{"5000", true},
	}
	for _, c := range cases {
		s := DaemonSettings{PruneHistory: c.prune}
		if got := s.IsPruned(); got != c.want {
			t.Fatalf("PruneHistory=%q: IsPruned=%v want %v", c.prune, got, c.want)
		}
	}
}

func TestNextPrunePresetCycles(t *testing.T) {
	first := NextPrunePreset("") // unknown -> first meaningful preset
	if first != PrunePresets[1].Blocks {
		t.Fatalf("unknown current should default to %q, got %q", PrunePresets[1].Blocks, first)
	}
	got := NextPrunePreset(PrunePresets[0].Blocks)
	if got != PrunePresets[1].Blocks {
		t.Fatalf("expected wrap to second preset, got %q", got)
	}
	last := NextPrunePreset(PrunePresets[len(PrunePresets)-1].Blocks)
	if last != PrunePresets[0].Blocks {
		t.Fatalf("expected wrap to first preset, got %q", last)
	}
}

func TestDescribePrune(t *testing.T) {
	if DescribePrune("") != "" || DescribePrune("0") != "" {
		t.Fatal("empty/0 must describe as empty (Full)")
	}
	if got := DescribePrune("5000"); got != PrunePresets[1].Label {
		t.Fatalf("preset label mismatch: %q", got)
	}
	if got := DescribePrune("123456"); got != "123456 blocks" {
		t.Fatalf("custom value label mismatch: %q", got)
	}
}
