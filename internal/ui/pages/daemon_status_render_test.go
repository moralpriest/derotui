// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"strings"
	"testing"
)

func TestFormatSyncPct(t *testing.T) {
	tests := []struct {
		progress float64
		want     string
	}{
		{0, "0.0%"},
		{0.05, "<0.1%"},
		{0.09, "<0.1%"},
		{0.1, "0.1%"},
		{12.5, "12.5%"},
		{100, "100.0%"},
	}
	for _, tc := range tests {
		if got := formatSyncPct(tc.progress); got != tc.want {
			t.Errorf("formatSyncPct(%v) = %q, want %q", tc.progress, got, tc.want)
		}
	}
}

func TestRenderDaemonStateLabel(t *testing.T) {
	tests := []struct {
		name   string
		daemon DaemonStatusInfo
		want   string
	}{
		{
			name:   "synced",
			daemon: DaemonStatusInfo{IsOnline: true, IsHealthy: true, IsSynced: true},
			want:   "● Synced",
		},
		{
			name:   "syncing",
			daemon: DaemonStatusInfo{IsOnline: true, IsHealthy: true, IsSyncing: true, SyncProgress: 12.5},
			want:   "● Syncing 12.5%",
		},
		{
			name:   "bootstrapping",
			daemon: DaemonStatusInfo{IsOnline: true, IsHealthy: true, IsBootstrapping: true, SyncProgress: 0.05},
			want:   "● Bootstrapping <0.1%",
		},
		{
			name:   "online",
			daemon: DaemonStatusInfo{IsOnline: true, IsHealthy: true, BlockHeight: 100},
			want:   "● Online",
		},
		{
			name:   "starting",
			daemon: DaemonStatusInfo{IsOnline: true, IsHealthy: true},
			want:   "● Starting",
		},
		{
			name:   "unhealthy",
			daemon: DaemonStatusInfo{IsOnline: true, IsHealthy: false},
			want:   "● Unhealthy",
		},
		{
			name:   "stopped",
			daemon: DaemonStatusInfo{},
			want:   "○ Stopped",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(renderDaemonStateLabel(tc.daemon))
			if got != tc.want {
				t.Errorf("renderDaemonStateLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderWelcomeHeight(t *testing.T) {
	tests := []struct {
		name   string
		daemon DaemonStatusInfo
		want   string
	}{
		{
			name:   "synced with peer height",
			daemon: DaemonStatusInfo{IsOnline: true, IsHealthy: true, IsSynced: true, BlockHeight: 7_414_000, PeerHeight: 7_414_000, SyncProgress: 100},
			want:   "7,414,000 / 7,414,000",
		},
		{
			name:   "syncing with peer height",
			daemon: DaemonStatusInfo{IsOnline: true, IsHealthy: true, IsSyncing: true, BlockHeight: 532, PeerHeight: 7_414_000, SyncProgress: 0.007},
			want:   "532 / 7,414,000 (<0.1%)",
		},
		{
			name:   "no block height",
			daemon: DaemonStatusInfo{},
			want:   "-",
		},
		{
			name:   "offline but with known height",
			daemon: DaemonStatusInfo{BlockHeight: 532},
			want:   "532",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(renderWelcomeHeight(tc.daemon))
			if got != tc.want {
				t.Errorf("renderWelcomeHeight() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderDaemonSummaryLine(t *testing.T) {
	line := stripANSI(renderDaemonSummaryLine(DaemonStatusInfo{
		IsOnline:        true,
		IsHealthy:       true,
		IsBootstrapping: true,
		SyncProgress:    0.05,
		Network:         "Mainnet",
		Address:         "localhost:10102",
		BlockHeight:     532,
		PeerHeight:      7_414_000,
	}))

	for _, want := range []string{"Network:", "Mainnet", "● Bootstrapping <0.1%", "Daemon:", "localhost:10102", "Height:", "532 / 7,414,000 (<0.1%)"} {
		if !strings.Contains(line, want) {
			t.Errorf("summary line missing %q in %q", want, line)
		}
	}
}

func TestRenderStateLine(t *testing.T) {
	tests := []struct {
		name string
		snap DaemonStatusSnapshot
		want string
	}{
		{
			name: "syncing",
			snap: DaemonStatusSnapshot{
				Running: true, IsOnline: true, IsHealthy: true,
				IsSyncing: true, SyncProgress: 0.05,
				IncomingPeers: 1, OutgoingPeers: 1,
			},
			want: "Syncing <0.1%",
		},
		{
			name: "bootstrapping",
			snap: DaemonStatusSnapshot{
				Running: true, IsOnline: true, IsHealthy: true,
				IsBootstrapping: true, SyncProgress: 12.5,
				IncomingPeers: 1, OutgoingPeers: 1,
			},
			want: "Bootstrapping 12.5%",
		},
		{
			name: "synced",
			snap: DaemonStatusSnapshot{
				Running: true, IsOnline: true, IsHealthy: true,
				IsSynced:      true,
				IncomingPeers: 1, OutgoingPeers: 1,
			},
			want: "Synced",
		},
		{
			name: "online no sync info",
			snap: DaemonStatusSnapshot{
				Running: true, IsOnline: true, IsHealthy: true,
				BlockHeight: 100,
			},
			want: "Online",
		},
		{
			name: "starting",
			snap: DaemonStatusSnapshot{
				Running: true,
			},
			want: "Starting",
		},
		{
			name: "stopped",
			snap: DaemonStatusSnapshot{},
			want: "Stopped",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDaemonStatus()
			d.SetSnapshot(tc.snap)
			if got := stripANSI(d.renderStateLine()); got != tc.want {
				t.Errorf("renderStateLine() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderHeightLine(t *testing.T) {
	tests := []struct {
		name string
		snap DaemonStatusSnapshot
		want string
	}{
		{
			name: "synced with peer",
			snap: DaemonStatusSnapshot{
				BlockHeight: 7_414_000, PeerHeight: 7_414_000, SyncProgress: 100,
				IsSynced: true,
			},
			want: "7,414,000 / 7,414,000 (100.0%)",
		},
		{
			name: "syncing with peer",
			snap: DaemonStatusSnapshot{
				BlockHeight: 532, PeerHeight: 7_414_000, SyncProgress: 0.007,
				IsSyncing: true,
			},
			want: "532 / 7,414,000 (<0.1%)",
		},
		{
			name: "offline known height",
			snap: DaemonStatusSnapshot{
				BlockHeight: 532,
			},
			want: "532",
		},
		{
			name: "no height",
			snap: DaemonStatusSnapshot{},
			want: "-",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDaemonStatus()
			d.SetSnapshot(tc.snap)
			if got := stripANSI(d.renderHeightLine()); got != tc.want {
				t.Errorf("renderHeightLine() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderStateLineBootstrappingBeatsOnline(t *testing.T) {
	d := NewDaemonStatus()
	d.Snapshot = DaemonStatusSnapshot{
		IsOnline: true, IsHealthy: true, IsBootstrapping: true,
		IncomingPeers: 3, PeerHeight: 7000000, SyncProgress: 20,
	}
	got := stripANSI(d.renderStateLine())
	if !strings.Contains(got, "Bootstrap") {
		t.Fatalf("got %q want Bootstrapping", got)
	}
	if got == "Online" {
		t.Fatalf("Online must not win over Bootstrapping")
	}
}
