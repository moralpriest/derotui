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
	if got := DescribePrune(PrunePresets[1].Blocks); got != PrunePresets[1].Label {
		t.Fatalf("preset label mismatch: %q", got)
	}
	if got := DescribePrune("5000"); got != "5000 blocks" {
		t.Fatalf("legacy value should render as custom: %q", got)
	}
	if got := DescribePrune("123456"); got != "123456 blocks" {
		t.Fatalf("custom value label mismatch: %q", got)
	}
}

func TestConvertPruneKeepLastToCutDeferUnknownTopo(t *testing.T) {
	s := DaemonSettings{PruneHistory: "50000"}
	if !ConvertPruneKeepLastToCut(&s, 0) {
		t.Fatal("expected defer when topo is 0")
	}
	if s.PruneHistory != "50000" {
		t.Fatalf("keep-last must stay unchanged when deferred, got %q", s.PruneHistory)
	}
}

func TestConvertPruneKeepLastToCutRewritesAbsoluteCut(t *testing.T) {
	s := DaemonSettings{PruneHistory: "50000"}
	if ConvertPruneKeepLastToCut(&s, 600000) {
		t.Fatal("expected conversion, not defer")
	}
	if s.PruneHistory != "550000" {
		t.Fatalf("cut = topo-keep = 550000, got %q", s.PruneHistory)
	}
}

func TestConvertPruneKeepLastToCutFullProfileNoOp(t *testing.T) {
	s := DaemonSettings{PruneHistory: ""}
	if ConvertPruneKeepLastToCut(&s, 600000) {
		t.Fatal("full profile is not deferred")
	}
	if s.PruneHistory != "" {
		t.Fatalf("full profile must stay empty, got %q", s.PruneHistory)
	}
}

func TestConvertPruneKeepLastToCutNilSettings(t *testing.T) {
	if ConvertPruneKeepLastToCut(nil, 600000) {
		t.Fatal("nil settings is not deferred")
	}
}

func TestConvertPruneKeepLastToCutTooYoung(t *testing.T) {
	s := DaemonSettings{PruneHistory: "50000"}
	if !ConvertPruneKeepLastToCut(&s, 50050) {
		t.Fatal("expected defer when remaining would be exactly 50")
	}
	if s.PruneHistory != "50000" {
		t.Fatalf("must not rewrite when deferred, got %q", s.PruneHistory)
	}
}
