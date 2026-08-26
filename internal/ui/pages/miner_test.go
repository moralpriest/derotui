// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"testing"
	"time"
)

func TestFormatMinerUptime(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00:00"},
		{time.Second, "00:00:01"},
		{90 * time.Second, "00:01:30"},
		{3661 * time.Second, "01:01:01"},
		{5*time.Hour + 4*time.Minute + 3*time.Second, "05:04:03"},
		{-time.Second, "00:00:00"}, // negative clamps to zero
	}
	for _, c := range cases {
		if got := formatMinerUptime(c.d); got != c.want {
			t.Errorf("formatMinerUptime(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

// TestMinerModelSetStatsWiresAllIndicators pins the stats plumbing: every
// indicator the page renders must be settable via SetStats (the message the
// app layer feeds from the engine backend).
func TestMinerModelSetStatsWiresAllIndicators(t *testing.T) {
	m := NewMiner()
	m.SetStats(1234, 99999, 7, 1, 2, 424242, 1000000000, 65*time.Second)

	if m.Hashrate != 1234 {
		t.Errorf("Hashrate = %d, want 1234", m.Hashrate)
	}
	if m.Hashes != 99999 {
		t.Errorf("Hashes = %d, want 99999", m.Hashes)
	}
	if m.Minis != 7 {
		t.Errorf("Minis = %d, want 7", m.Minis)
	}
	if m.BlocksFound != 1 {
		t.Errorf("BlocksFound = %d, want 1", m.BlocksFound)
	}
	if m.Rejected != 2 {
		t.Errorf("Rejected = %d, want 2", m.Rejected)
	}
	if m.Height != 424242 {
		t.Errorf("Height = %d, want 424242", m.Height)
	}
	if m.Difficulty != 1000000000 {
		t.Errorf("Difficulty = %d, want 1000000000", m.Difficulty)
	}
	if m.Uptime != 65*time.Second {
		t.Errorf("Uptime = %v, want 65s", m.Uptime)
	}
}

func TestFormatMinerDifficulty(t *testing.T) {
	cases := []struct {
		n    uint64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1K"},
		{20480, "20K"},
		{999999, "999K"},
		{1000000, "1M"},
		{1_500_000, "1M"},
		{2_000_000_000, "2G"},
		{1_000_000_000_000, "1000G"},
	}
	for _, c := range cases {
		if got := formatMinerDifficulty(c.n); got != c.want {
			t.Errorf("formatMinerDifficulty(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
